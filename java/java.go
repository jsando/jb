package java

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"github.com/jsando/jb/maven"
	"github.com/jsando/jb/project"
	"github.com/jsando/jb/tools"
	"github.com/jsando/jb/util"
	"os"
	"path/filepath"
	"strings"
)

const (
	buildHashFile = "build.sha1"
)

type Builder struct {
	repo         *maven.LocalRepository
	logger       project.BuildLog
	toolProvider tools.ToolProvider
}

func NewBuilder(logger project.BuildLog) *Builder {
	return &Builder{
		repo:         maven.OpenLocalRepository(),
		logger:       logger,
		toolProvider: tools.GetDefaultToolProvider(),
	}
}

// NewBuilderWithTools creates a new Builder with a custom tool provider
func NewBuilderWithTools(logger project.BuildLog, toolProvider tools.ToolProvider) *Builder {
	return &Builder{
		repo:         maven.OpenLocalRepository(),
		logger:       logger,
		toolProvider: toolProvider,
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

	runner := j.toolProvider.GetRunner()
	runArgs := tools.RunArgs{
		JarFile:     jarPath,
		ProgramArgs: progArgs,
		WorkDir:     module.ModuleDirAbs,
	}

	return runner.Run(runArgs)
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
	compiler := j.toolProvider.GetCompiler()

	// Check if compiler is available
	if !compiler.IsAvailable() {
		return fmt.Errorf("Java compiler (javac) not found. Please ensure JDK is installed and javac is in your PATH")
	}

	// Log compiler version
	if version, err := compiler.Version(); err == nil {
		task.Info(fmt.Sprintf("Using Java compiler version: %s", version.String()))
	}

	// Convert source files to string paths
	sourcePaths := make([]string, len(sourceFiles))
	for i, sf := range sourceFiles {
		sourcePaths[i] = sf.Path
	}

	// Prepare compilation arguments
	compileArgs := tools.CompileArgs{
		SourceFiles: sourcePaths,
		ClassPath:   classPath,
		DestDir:     buildClasses,
		ExtraFlags:  extraFlags,
		WorkDir:     module.ModuleDirAbs,
	}

	// Compile
	result, err := compiler.Compile(compileArgs)

	// Process compilation result
	if result.WarningCount > 0 {
		task.Warn(fmt.Sprintf("Compilation completed with %d warning(s)", result.WarningCount))
	}

	// Log warnings
	for _, warning := range result.Warnings {
		if warning.File != "" {
			task.Warn(fmt.Sprintf("%s:%d:%d: %s", warning.File, warning.Line, warning.Column, warning.Message))
		} else {
			task.Warn(warning.Message)
		}
	}

	// Log errors
	for _, error := range result.Errors {
		if error.File != "" {
			task.Error(fmt.Sprintf("%s:%d:%d: %s", error.File, error.Line, error.Column, error.Message))
		} else {
			task.Error(error.Message)
		}
	}

	// If we have raw output but no parsed errors/warnings, show the raw output
	if !result.Success && len(result.Errors) == 0 && result.RawOutput != "" {
		task.Error("Compilation failed. Raw output:")
		for _, line := range strings.Split(result.RawOutput, "\n") {
			if strings.TrimSpace(line) != "" {
				task.Error(line)
			}
		}
	}

	if !result.Success {
		return fmt.Errorf("compilation failed with %d error(s)", result.ErrorCount)
	}

	return err
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

	jarTool := j.toolProvider.GetJarTool()

	// Check if jar tool is available
	if !jarTool.IsAvailable() {
		return fmt.Errorf("JAR tool not found. Please ensure JDK is installed and jar is in your PATH")
	}

	jarPath := j.getModuleJarPath(module)

	// Prepare classpath entries for executable JARs
	var classPathEntries []string
	if mainClass != "" && len(jarPaths) > 0 {
		// Copy dependencies and build classpath entries
		for _, dep := range jarPaths {
			jarName := filepath.Base(dep)
			classPathEntries = append(classPathEntries, jarName)
			if err := util.CopyFile(dep, filepath.Join(buildDir, jarName)); err != nil {
				return err
			}
		}
	}

	// Create JAR arguments
	jarArgs := tools.JarArgs{
		JarFile:   jarPath,
		BaseDir:   buildClasses,
		Files:     []string{"."}, // Include all files in the base directory
		MainClass: mainClass,
		ClassPath: classPathEntries,
		Date:      jarDate,
		WorkDir:   module.ModuleDirAbs,
	}

	return jarTool.Create(jarArgs)
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

	runner := j.toolProvider.GetRunner()

	runArgs := tools.RunArgs{
		MainClass: "org.junit.platform.console.ConsoleLauncher",
		ProgramArgs: []string{
			"execute",
			"--scan-classpath", buildClasses,
			"--details=tree",
			"--reports-dir", testResultsDir,
		},
		WorkDir: module.ModuleDirAbs,
		Env:     []string{"CLASSPATH=" + classPath},
	}

	err = runner.Run(runArgs)
	task.Done(err)
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
