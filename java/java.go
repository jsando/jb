package java

import (
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
	"strings"
)

const (
	PropertyOutputType    = "OutputType"
	PropertyMainClass     = "MainClass"
	PropertyCompilerFlags = "CompilerFlags"
	PropertyJarDate       = "JarDate"
	OutputTypeJar         = "Jar"
	OutputTypeExeJar      = "ExecutableJar"
	buildHashFile         = "build.sha1"
)

type Builder struct {
	repo *maven.LocalRepository
}

func NewBuilder() *Builder {
	return &Builder{
		repo: maven.OpenLocalRepository(),
	}
}

func (j *Builder) Clean(module *project.Module) error {
	buildDir := filepath.Join(module.ModuleDir, "build")
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("failed to remove build dir %s: %w", buildDir, err)
	}
	return nil
}

func (j *Builder) Run(module *project.Module, progArgs []string) error {
	jarPath := j.getModuleJarPath(module)
	args := []string{"-jar", jarPath}
	args = append(args, progArgs...)
	cmd := exec.Command("java", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("running %s with args %v\n", cmd.Path, cmd.Args)
	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func (j *Builder) getModuleJarPath(module *project.Module) string {
	buildDir := filepath.Join(module.ModuleDir, "build")
	return filepath.Join(buildDir, module.Name+"-"+module.Version+".jar")
}

func (j *Builder) Build(module *project.Module) error {
	// Parse and validate build args for the module
	outputType := module.GetProperty(PropertyOutputType, OutputTypeJar)
	mainClass := module.GetProperty(PropertyMainClass, "")
	if outputType == OutputTypeExeJar && len(mainClass) == 0 {
		return fmt.Errorf("%s requires property %s to be set", OutputTypeExeJar, PropertyMainClass)
	}
	if outputType == OutputTypeJar && len(mainClass) > 0 {
		return fmt.Errorf("%s only allowed with %s %s", PropertyMainClass, PropertyOutputType, OutputTypeExeJar)
	}
	extraFlags := module.GetProperty(PropertyCompilerFlags, "")
	jarDate := module.GetProperty(PropertyJarDate, "")

	// Gather java source files
	sources, err := util.FindFilesBySuffixR(module.ModuleDir, ".java")
	if err != nil {
		return fmt.Errorf("failed to find java sources: %w", err)
	}
	if len(sources) == 0 {
		fmt.Printf("warning: no java sources found in %s\n", module.ModuleDir)
	}

	// Gather embeds
	embeds := module.GetPropertyList("Embed")
	embedFiles, err := util.FindFilesByGlob(module.ModuleDir, embeds)

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
		fmt.Printf("file %s modtime %d\n", embed.Path, embed.Info.ModTime().UnixNano())
		binary.LittleEndian.PutUint64(bytes, uint64(embed.Info.ModTime().UnixNano()))
		hasher.Write(bytes)
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	oldHash, err := util.ReadFileAsString(filepath.Join(module.ModuleDir, "build", buildHashFile))
	if err != nil {
		return fmt.Errorf("failed to read build hash: %w", err)
	}
	if hash == oldHash {
		fmt.Printf("module %s is up to date\n", module.Name)
		return nil
	}

	// Create the build dir(s)
	buildDir := filepath.Join(module.ModuleDir, "build")
	buildTmpDir := filepath.Join(buildDir, "tmp")
	buildClasses := filepath.Join(buildTmpDir, "classes")

	err = os.RemoveAll(buildDir)
	if err != nil {
		return fmt.Errorf("failed to remove build dir %s: %w", buildDir, err)
	}
	err = os.MkdirAll(buildClasses, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create build dir %s: %w", buildClasses, err)
	}

	// For compilation, use the absolute paths to all jar dependencies
	classPath := ""
	deps, err := module.GetModuleReferencesInBuildOrder()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	compileClasspath, err := j.getBuildDependencies(module)
	if err != nil {
		return fmt.Errorf("failed to get build dependencies: %w", err)
	}
	if len(compileClasspath) > 0 {
		classPath = "--class-path " + strings.Join(compileClasspath, string(os.PathListSeparator))
	}

	// Compile java sources (if there are any)
	if len(sources) > 0 {
		if err := j.compileJava(module, buildTmpDir, buildClasses, classPath, extraFlags, sources); err != nil {
			return fmt.Errorf("failed to compile java sources: %w", err)
		}
	}

	// Copy embeds to output folder then jar can just jar everything
	for _, embed := range embedFiles {
		relPath, err := filepath.Rel(module.ModuleDir, embed.Path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for embed %s: %w", embed.Path, err)
		}
		dst := filepath.Join(buildClasses, relPath)
		if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", filepath.Dir(dst), err)
		}
		if err := util.CopyFile(embed.Path, dst); err != nil {
			return fmt.Errorf("failed to copy embed %s to %s: %w", embed.Path, dst, err)
		}
	}

	// Build into .jar
	if err := j.buildJar(module, buildDir, jarDate, mainClass, compileClasspath, buildTmpDir, buildClasses); err != nil {
		return fmt.Errorf("failed to build jar: %w", err)
	}

	// write pom file
	if err := j.writePOM(module, deps); err != nil {
		return fmt.Errorf("failed to write pom: %w", err)
	}

	// write build hash
	return util.WriteFile(filepath.Join(buildDir, buildHashFile), hash)
}

func (j *Builder) compileJava(module *project.Module, buildTmpDir, buildClasses, classPath, extraFlags string, sourceFiles []util.SourceFileInfo) error {
	// Write javac args to a single file, avoid potential shell args length limitation (esp with large classpath)
	buildArgsPath := filepath.Join(buildTmpDir, "javac-flags.txt")
	buildArgs := fmt.Sprintf("-d %s\n%s\n%s\n", buildClasses, extraFlags, classPath)
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
	optionsPathRel, _ := filepath.Rel(module.ModuleDir, buildArgsPath)
	sourceFilesPathRel, _ := filepath.Rel(module.ModuleDir, sourceFilesPath)

	// Compile sources
	return execCommand("javac", module.ModuleDir, "@"+optionsPathRel, "@"+sourceFilesPathRel)
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
		GroupID:           module.GroupID,
		ArtifactID:        module.Name,
		Version:           module.Version,
		Name:              module.Name,
		Description:       module.Name,
	}
	jarPath := j.getModuleJarPath(module)
	pomPath := strings.TrimSuffix(jarPath, ".jar") + ".pom"

	if len(deps) > 0 {
		pom.Dependencies = make([]maven.Dependency, len(deps))
		for i, dep := range deps {
			pom.Dependencies[i] = maven.Dependency{
				GroupID:    dep.GroupID,
				ArtifactID: dep.Name,
				Version:    dep.Version,
			}
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
		"-c", "-f", jarPath,
	}
	if jarDate != "" {
		jarArgs = append(jarArgs, "--date", jarDate)
	}
	// If executable jar, gather all dependencies and append with relative path to main jar to 'ClassPath' in manifest
	if mainClass != "" {
		jarArgs = append(jarArgs, "--main-class", mainClass)

		manifestClasspath := ""
		for _, dep := range jarPaths {
			jarName := filepath.Base(dep)
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
	return execCommand("jar", module.ModuleDir, jarArgs...)
}

func (j *Builder) Publish(m *project.Module, repoURL, user, password string) error {
	jarPath := j.getModuleJarPath(m)
	pomPath := strings.TrimSuffix(jarPath, ".jar") + ".pom"
	return j.repo.InstallPackage(m.GroupID, m.Name, m.Version, jarPath, pomPath)
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

func (j *Builder) ResolveDependencies(module *project.Module) ([]PackageDependency, error) {
	deps := make([]PackageDependency, 0)
	for _, ref := range module.Packages.References {
		parts := strings.Split(ref.URL, ":")
		if len(parts) != 3 {
			return deps, fmt.Errorf("invalid package URL: %s", ref.URL)
		}
		groupID := parts[0]
		artifactID := parts[1]
		version := parts[2]

		if len(groupID) == 0 || len(artifactID) == 0 || len(version) == 0 {
			return deps, fmt.Errorf("invalid package URL: %s", ref.URL)
		}

		dep, err := j.addDependency(groupID, artifactID, version)
		if err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, nil
}

func (j *Builder) addDependency(groupID, artifactID, version string) (PackageDependency, error) {
	dep := PackageDependency{}
	fmt.Printf("resolving %s:%s:%s\n", groupID, artifactID, version)
	pom, err := j.repo.GetPOM(groupID, artifactID, version)
	if err != nil {
		return dep, err
	}
	switch pom.Packaging {
	case "", "jar":
		jarPath, err := j.repo.GetJAR(groupID, artifactID, version)
		if err != nil {
			return dep, err
		}
		dep = PackageDependency{
			Path:       jarPath,
			URL:        maven.GAV(groupID, artifactID, version),
			GroupID:    groupID,
			ArtifactID: artifactID,
			Version:    version,
			Transitive: []PackageDependency{},
		}
	default:
		return dep, fmt.Errorf("packaging type not supported: %s", pom.Packaging)
	}
	for _, childDep := range pom.Dependencies {
		if childDep.Scope == "test" || childDep.Scope == "provided" {
			continue // Skip test and provided scope dependencies
		}
		fmt.Printf("recursive resolving %s:%s:%s\n", childDep.GroupID, childDep.ArtifactID, childDep.Version)
		childDepDep, err := j.addDependency(childDep.GroupID, childDep.ArtifactID, childDep.Version)
		if err != nil {
			return dep, err
		}
		dep.Transitive = append(dep.Transitive, childDepDep)
	}
	return dep, nil
}

func (j *Builder) getBuildDependencies(module *project.Module) ([]string, error) {
	seenDeps := make(map[string]string) // Map to store seen GAV (GroupID:ArtifactID) and their versions
	jars := make(map[string]struct{})   // Set to ensure unique jar paths

	// Get the list of modules this module depends on
	refs, err := module.GetModuleReferencesInBuildOrder()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve references for module %s: %w", module.Name, err)
	}

	var addPkg func(pkg PackageDependency) error
	addPkg = func(pkg PackageDependency) error {
		key := pkg.GroupID + ":" + pkg.ArtifactID // GroupID:ArtifactID
		if existingVersion, exists := seenDeps[key]; exists {
			// Check for version conflict
			if existingVersion != pkg.Version {
				return fmt.Errorf("version conflict for dependency %s: %s vs %s",
					key, existingVersion, pkg.Version)
			}
		} else {
			// Store seen dependency version
			seenDeps[key] = pkg.Version
			jars[pkg.Path] = struct{}{}
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
	deps, err := j.ResolveDependencies(module)
	if err != nil {
		return nil, err
	}
	for _, dep := range deps {
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
		deps, err := j.ResolveDependencies(ref)
		if err != nil {
			return nil, err
		}
		for _, dep := range deps {
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

func (j *Builder) getModulePackage(ref *project.Module) PackageDependency {
	return PackageDependency{
		Path:       j.getModuleJarPath(ref),
		GroupID:    ref.GroupID,
		ArtifactID: ref.Name,
		Version:    ref.Version,
		URL:        maven.GAV(ref.GroupID, ref.Name, ref.Version),
		Transitive: []PackageDependency{},
	}
}
