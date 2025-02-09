package java

import (
	"bufio"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"github.com/jsando/jb/maven"
	"github.com/jsando/jb/project"
	"github.com/jsando/jb/util"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	buildHashFile = "build.sha1"
)

type Builder struct {
	repo   *maven.LocalRepository
	logger project.BuildLog
}

func NewBuilder(logger project.BuildLog) *Builder {
	return &Builder{
		repo:   maven.OpenLocalRepository(),
		logger: logger,
	}
}

func (j *Builder) Clean(module *project.Module) {
	task := j.logger.TaskStart("cleaning build dir")
	buildDir := filepath.Join(module.ModuleDirAbs, "build")
	err := os.RemoveAll(buildDir)
	task.Done(err)
}

func (j *Builder) Run(module *project.Module, progArgs []string) error {
	jarPath := j.getModuleJarPath(module)
	args := []string{"-jar", jarPath}
	args = append(args, progArgs...)
	cmd := exec.Command("java", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	//fmt.Printf("running %s with args %v\n", cmd.Path, cmd.Args)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func (j *Builder) getModuleJarPath(module *project.Module) string {
	buildDir := filepath.Join(module.ModuleDirAbs, "build")
	return filepath.Join(buildDir, module.Name+"-"+module.Version+".jar")
}

func (j *Builder) Build(module *project.Module) {
	j.logger.ModuleStart(module.Name)

	// Parse and validate build args for the module
	jarDate := ""

	// Gather java source files
	sources, err := util.FindFilesBySuffixR(module.SourceDirAbs, ".java")
	if j.logger.CheckError("finding java sources", err) {
		return
	}
	if len(sources) == 0 {
		fmt.Printf("warning: no java sources found in %s\n", module.ModuleDirAbs)
	}
	if len(sources) == 0 {
		fmt.Printf("warning: no java sources found in %s\n", module.ModuleDirAbs)
	}

	// Find source files in the source dir but make the path relative to the module dir
	for i, source := range sources {
		sources[i].Path, err = filepath.Rel(module.ModuleDirAbs, filepath.Join(module.SourceDirAbs, source.Path))
		if j.logger.CheckError("getting relative path for source", err) {
			return
		}
	}

	// Gather embeds
	embedFiles, err := util.FindFilesByGlob(module.ResourceDirAbs, module.Resources)
	if j.logger.CheckError("finding embeds", err) {
		return
	}

	// Compute sha1(project-file, sources, embeds) and see if we're up to date
	hasher := sha1.New()
	_ = module.HashContent(hasher)
	for _, source := range sources {
		hasher.Write([]byte(source.Path))
		bytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(bytes, uint64(source.Info.ModTime().UnixNano()))
		hasher.Write(bytes)
	}
	for _, embed := range embedFiles {
		hasher.Write([]byte(embed.Path))
		bytes := make([]byte, 8)
		//fmt.Printf("file %s modtime %d\n", embed.Path, embed.Info.ModTime().UnixNano())
		binary.LittleEndian.PutUint64(bytes, uint64(embed.Info.ModTime().UnixNano()))
		hasher.Write(bytes)
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	oldHash, err := util.ReadFileAsString(filepath.Join(module.ModuleDirAbs, "build", buildHashFile))
	if j.logger.CheckError("reading build hash", err) {
		return
	}
	if hash == oldHash {
		j.logger.TaskStart("up to date").Done(nil)
		return
	}

	// Create the build dir(s)
	buildDir := filepath.Join(module.ModuleDirAbs, "build")
	buildTmpDir := filepath.Join(buildDir, "tmp")
	buildClasses := filepath.Join(buildTmpDir, "classes")

	err = os.RemoveAll(buildDir)
	if j.logger.CheckError("removing build dir", err) {
		return
	}
	err = os.MkdirAll(buildClasses, os.ModePerm)
	if j.logger.CheckError(fmt.Sprintf("creating build dir %s", buildClasses), err) {
		return
	}

	// For compilation, use the absolute paths to all jar dependencies
	classPath := ""
	deps, err := module.GetModuleReferencesInBuildOrder()
	if j.logger.CheckError("getting module references", err) {
		return
	}
	compileClasspath, err := j.getBuildDependencies(module)
	if j.logger.CheckError("getting build dependencies", err) {
		return
	}
	if len(compileClasspath) > 0 {
		classPath = "-cp " + strings.Join(compileClasspath, string(os.PathListSeparator))
	}

	// Compile java sources (if there are any)
	if len(sources) > 0 {
		task := j.logger.TaskStart("compile java sources")
		err := j.compileJava(module, task, buildTmpDir, buildClasses, classPath, module.CompileArgs, sources)
		if task.Done(err) {
			return
		}
	}

	// Copy embeds to output folder then jar can just jar everything
	task := j.logger.TaskStart("building jar")
	for _, embed := range embedFiles {
		relPath, err := filepath.Rel(embed.Dir, embed.Path)
		if j.logger.CheckError("getting relative path for embed", err) {
			return
		}
		dst := filepath.Join(buildClasses, relPath)
		if embed.Info.IsDir() {
			err = os.MkdirAll(dst, os.ModePerm)
			if j.logger.CheckError("mkdir for embed", err) {
				return
			}
		} else {
			err = os.MkdirAll(filepath.Dir(dst), os.ModePerm)
			if j.logger.CheckError(fmt.Sprintf("creating directory %s", filepath.Dir(dst)), err) {
				return
			}

			err = util.CopyFile(embed.Path, dst)
			if j.logger.CheckError(fmt.Sprintf("copying embed %s to %s", embed.Path, dst), err) {
				return
			}
		}
	}

	// Build into .jar
	err = j.buildJar(module, buildDir, jarDate, module.MainClass, compileClasspath, buildTmpDir, buildClasses)
	if task.Done(err) {
		return
	}

	// write pom file
	err = j.writePOM(module, deps)
	if j.logger.CheckError("writing pom file", err) {
		return
	}

	// write build hash
	err = util.WriteFile(filepath.Join(buildDir, buildHashFile), hash)
	j.logger.CheckError("writing build hash", err)
}

func (j *Builder) compileJava(module *project.Module, task project.TaskLog, buildTmpDir, buildClasses, classPath string, extraFlags []string, sourceFiles []util.SourceFileInfo) error {
	// Write javac args to a single file, avoid potential shell args length limitation (esp with large classpath)
	buildArgsPath := filepath.Join(buildTmpDir, "javac-flags.txt")
	buildArgs := fmt.Sprintf("-d %s\n%s\n%s\n", buildClasses, strings.Join(extraFlags, "\n"), classPath)
	if err := util.WriteFile(buildArgsPath, buildArgs); err != nil {
		return err
	}

	// Write all source file paths to a file for javac, avoids potential shell args length limitation
	sourceFilesPath := filepath.Join(buildTmpDir, "javac-sources.txt")
	sourceFileList := ""
	for _, sourceFile := range sourceFiles {
		sourceFileList += sourceFile.Path + "\n"
	}
	if err := util.WriteFile(sourceFilesPath, sourceFileList); err != nil {
		return err
	}

	// Convert paths to relative to the module dir (todo: why?)
	optionsPathRel, _ := filepath.Rel(module.ModuleDirAbs, buildArgsPath)
	sourceFilesPathRel, _ := filepath.Rel(module.ModuleDirAbs, sourceFilesPath)

	// Compile sources
	cmd := exec.Command("javac", "@"+optionsPathRel, "@"+sourceFilesPathRel)
	cmd.Dir = module.ModuleDirAbs

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// Start the javac process
	if err := cmd.Start(); err != nil {
		return err
	}

	// Initialize counters
	warningCount, errorCount := 0, 0

	// Regex patterns for warnings and errors
	warningPattern := regexp.MustCompile(`(?i)warning`)
	errorPattern := regexp.MustCompile(`(?i)error`)

	// Read and process output streams
	processStream := func(scanner *bufio.Scanner) {
		for scanner.Scan() {
			line := scanner.Text()

			switch {
			case errorPattern.MatchString(line):
				errorCount++
				task.Error(line)
			case warningPattern.MatchString(line):
				warningCount++
				task.Warn(line)
			default:
				task.Info(line)
			}
		}
	}

	// Scan stderr and stdout
	go processStream(bufio.NewScanner(stderr))
	go processStream(bufio.NewScanner(stdout))

	// Wait for javac to finish
	return cmd.Wait()
}

func execCommand(name string, dir string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	fmt.Printf("running %s with args %v\n", cmd.Path, cmd.Args)
	return cmd.Run()
}

func (j *Builder) writePOM(module *project.Module, deps []*project.Module) error {
	pom := maven.POM{
		Xmlns:             "http://maven.apache.org/POM/4.0.0",                                          // Default namespace
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",                                  // XML Schema instance namespace
		XsiSchemaLocation: "http://maven.apache.org/POM/4.0.0 http://maven.apache.org/maven-v4_0_0.xsd", // Schema location
		ModelVersion:      "4.0.0",
		Packaging:         "jar",
		GroupID:           module.Group,
		ArtifactID:        module.Name,
		Version:           module.Version,
		Name:              module.Name,
		Description:       module.Name,
	}
	jarPath := j.getModuleJarPath(module)
	pomPath := strings.TrimSuffix(jarPath, ".jar") + ".pom"

	pom.Dependencies = make([]maven.Dependency, len(deps))
	if len(deps) > 0 {
		for i, dep := range deps {
			pom.Dependencies[i] = maven.Dependency{
				GroupID:    dep.Group,
				ArtifactID: dep.Name,
				Version:    dep.Version,
			}
		}
	}
	if module.Dependencies != nil {
		for _, dep := range module.Dependencies {
			mavenDep := maven.Dependency{
				GroupID:    dep.Group,
				ArtifactID: module.Name,
				Version:    dep.Version,
			}
			pom.Dependencies = append(pom.Dependencies, mavenDep)
		}
	}
	pomXML, err := xml.MarshalIndent(pom, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize POM to XML: %w", err)
	}
	xmlHeader := []byte(xml.Header)
	pomContent := append(xmlHeader, pomXML...)
	if err := util.WriteFile(pomPath, string(pomContent)); err != nil {
		return fmt.Errorf("failed to write POM to %s: %w", pomPath, err)
	}
	return nil
}

func (j *Builder) buildJar(module *project.Module,
	buildDir string,
	jarDate string,
	mainClass string,
	jarPaths []string,
	buildTmpDir string,
	buildClasses string) error {

	jarPath := j.getModuleJarPath(module)
	jarArgs := []string{
		// Java 1.8 only had the short form args, later versions allow "--create", "--file"
		"-cf", jarPath,
	}
	if jarDate != "" {
		jarArgs = append(jarArgs, "--date", jarDate)
	}
	// If executable jar, gather all dependencies and append with relative path to main jar to 'ClassPath' in manifest
	if mainClass != "" {
		jarArgs = append(jarArgs, "--main-class", mainClass)

		manifestClasspath := ""
		firstLine := true
		for _, dep := range jarPaths {
			jarName := filepath.Base(dep)
			if !firstLine {
				manifestClasspath += "\n "
			}
			firstLine = false
			manifestClasspath += jarName + " "
			if err := util.CopyFile(dep, filepath.Join(buildDir, jarName)); err != nil {
				return err
			}
		}
		if len(manifestClasspath) > 0 {
			manifestPath := filepath.Join(buildTmpDir, "manifest")
			manifestString := fmt.Sprintf(`Manifest-Version: 1.0
Class-Path: %s
`, manifestClasspath)
			if err := util.WriteFile(manifestPath, manifestString); err != nil {
				return err
			}
			jarArgs = append(jarArgs, "--manifest", manifestPath)
		}
	}
	jarArgs = append(jarArgs, "-C", buildClasses, ".")
	return execCommand("jar", module.ModuleDirAbs, jarArgs...)
}

func (j *Builder) Publish(m *project.Module, repoURL, user, password string) error {
	jarPath := j.getModuleJarPath(m)
	pomPath := strings.TrimSuffix(jarPath, ".jar") + ".pom"
	return j.repo.InstallPackage(m.Group, m.Name, m.Version, jarPath, pomPath)
}

type PackageDependency struct {
	Path       string
	GroupID    string
	ArtifactID string
	Version    string
	URL        string
	Transitive []PackageDependency
}

func (d PackageDependency) PrintTree(index int) {
	fmt.Printf("%s%s\n", strings.Repeat("  ", index), d.URL)
	for _, t := range d.Transitive {
		t.PrintTree(index + 1)
	}
}

func (j *Builder) ResolveDependencies(module *project.Module) error {
	visited := make(map[string]string)
	for _, ref := range module.Dependencies {
		if ref == nil {
			panic("how can ref be nil?")
		}
		err := j.resolveDependency(ref, visited)
		if err != nil {
			return err
		}
	}
	return nil
}

func (j *Builder) resolveDependency(dep *project.Dependency, visited map[string]string) error {
	// already done?
	if dep.Path != "" {
		return nil
	}
	key := fmt.Sprintf("%s:%s", dep.Group, dep.Artifact)
	if _, exists := visited[key]; exists {
		// circular, duplicate, or conflicting reference
		return nil
	}
	visited[key] = dep.Version
	//fmt.Printf("visited: %s\n", key)

	//fmt.Printf("resolving %s:%s:%s\n", dep.Group, dep.Artifact, dep.Version)
	pom, err := j.repo.GetPOM(dep.Group, dep.Artifact, dep.Version)
	if err != nil {
		return err
	}
	switch pom.Packaging {
	case "", "jar", "bundle":
		jarPath, err := j.repo.GetJAR(dep.Group, dep.Artifact, dep.Version)
		if err != nil {
			return err
		}
		dep.Path = jarPath
	case "pom":
		// process the POM but there is no jar
	default:
		return fmt.Errorf("packaging type not supported: %s", pom.Packaging)
	}
	if dep.Transitive == nil {
		dep.Transitive = make([]*project.Dependency, 0)
	}
	for _, pomChild := range pom.Dependencies {
		if pomChild.Scope == "test" || pomChild.Scope == "provided" || pomChild.Optional == "true" {
			continue // Skip test and provided scope dependencies
		}
		gav := maven.GAV(pomChild.GroupID, pomChild.ArtifactID, pomChild.Version)
		if pomChild.GroupID == "" || pomChild.ArtifactID == "" || pomChild.Version == "" {
			return fmt.Errorf("invalid maven package '%s' referenced from %s", gav, dep.Coordinates)
		}
		//fmt.Printf("recursive resolving %s\n", gav)
		child := &project.Dependency{
			Coordinates: gav,
			Group:       pomChild.GroupID,
			Artifact:    pomChild.ArtifactID,
			Version:     pomChild.Version,
			Path:        "",
			Transitive:  make([]*project.Dependency, 0),
		}
		err := j.resolveDependency(child, visited)
		if err != nil {
			return err
		}
		dep.Transitive = append(dep.Transitive, child)
	}
	return nil
}

func (j *Builder) getBuildDependencies(module *project.Module) ([]string, error) {
	seenDeps := make(map[string]string) // Map to store seen GAV (Group:ArtifactID) and their versions
	jars := make(map[string]struct{})   // Set to ensure unique jar paths

	// Get the list of modules this module depends on
	refs, err := module.GetModuleReferencesInBuildOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve references for module %s: %w", module.Name, err)
	}

	var addPkg func(pkg *project.Dependency) error
	addPkg = func(pkg *project.Dependency) error {
		key := pkg.Group + ":" + pkg.Artifact // Group:ArtifactID
		if existingVersion, exists := seenDeps[key]; exists {
			// Check for version conflict
			if existingVersion != pkg.Version {
				//fmt.Printf("evict %s:%s (have %s)\n", key, pkg.Version, existingVersion)
				return nil
				//return fmt.Errorf("version conflict for dependency %s: %s vs %s",
				//	key, existingVersion, pkg.Version)
			}
		} else {
			// Store seen dependency version
			seenDeps[key] = pkg.Version

			// pkg path can be empty if packaging=pom was encountered
			if len(pkg.Path) > 0 {
				jars[pkg.Path] = struct{}{}
			}
		}
		// Recursively collect transitive dependencies
		for _, dep := range pkg.Transitive {
			if err := addPkg(dep); err != nil {
				return err
			}
		}
		return nil
	}

	// Add the package dependencies for this module and each of its referenced modules
	err = j.ResolveDependencies(module)
	if err != nil {
		return nil, err
	}
	for _, dep := range module.Dependencies {
		if err := addPkg(dep); err != nil {
			return nil, err
		}
	}
	for _, ref := range refs {
		// add module jar
		dep := j.getModulePackage(ref)
		if err := addPkg(dep); err != nil {
			return nil, err
		}
		// add module dependencies
		err := j.ResolveDependencies(ref)
		if err != nil {
			return nil, err
		}
		for _, dep := range ref.Dependencies {
			if err := addPkg(dep); err != nil {
				return nil, err
			}
		}
	}

	// Convert the unique jar paths in the jars set into a slice
	jarPaths := make([]string, 0, len(jars))
	for jar := range jars {
		jarPaths = append(jarPaths, jar)
	}

	return jarPaths, nil
}

func (j *Builder) getModulePackage(ref *project.Module) *project.Dependency {
	return &project.Dependency{
		Path:        j.getModuleJarPath(ref),
		Group:       ref.Group,
		Artifact:    ref.Name,
		Version:     ref.Version,
		Coordinates: maven.GAV(ref.Group, ref.Name, ref.Version),
		Transitive:  make([]*project.Dependency, 0),
	}
}

func (j *Builder) RunTest(module *project.Module) {
	task := j.logger.TaskStart("Running tests")
	framework := j.detectTestFramework(module)
	if framework == "" {
		task.Error("Test framework not detected (only junit4 and junit5 are currently supported)")
		return
	}

	buildDir := filepath.Join(module.ModuleDirAbs, "build")
	buildTmpDir := filepath.Join(buildDir, "tmp")
	buildClasses := filepath.Join(buildTmpDir, "classes")
	testResultsDir := filepath.Join(buildTmpDir, "test-results")

	// Absolute paths to all jar dependencies
	compileClasspath, err := j.getBuildDependencies(module)
	if j.logger.CheckError("getting build dependencies", err) {
		return
	}
	compileClasspath = append(compileClasspath, buildClasses)
	classPath := strings.Join(compileClasspath, string(os.PathListSeparator))
	//buildArgsPath := filepath.Join(buildTmpDir, "test-classpath.txt")
	//buildArgs := fmt.Sprintf("-cp %s\n", classPath)
	//err = util.WriteFile(buildArgsPath, buildArgs)
	//if j.logger.CheckError("writing build args", err) {
	//	return
	//}

	//err = execCommand("java", module.ModuleDirAbs, "@"+buildArgsPath, "org.junit.platform.console.ConsoleLauncher",
	//	"execute", "--scan-classpath", buildClasses, "--details=tree")

	cmd := exec.Command("java",
		"org.junit.platform.console.ConsoleLauncher",
		"execute",
		"--scan-classpath", buildClasses,
		"--details=tree",
		"--reports-dir", testResultsDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = module.ModuleDirAbs
	cmd.Env = append(os.Environ(), "CLASSPATH="+classPath)
	//fmt.Printf("running %s with args %v\n", cmd.Path, cmd.Args)
	task.Done(cmd.Run())
}

func (j *Builder) detectTestFramework(module *project.Module) string {
	for _, dep := range module.Dependencies {
		if dep.Group == "junit" {
			return "junit"
		}
		if dep.Group == "org.junit.jupiter" {
			return "junit"
		}
		//if dep.Group == "org.testng" {
		//	return "testng"
		//}
	}
	return ""
}
