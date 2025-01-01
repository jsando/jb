package project

import (
	"fmt"
	"github.com/jsando/jb/artifact"
	"io"
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
)

type JavaBuilder struct {
	repo *artifact.JarCache
}

func NewJavaBuilder() *JavaBuilder {
	return &JavaBuilder{
		repo: artifact.NewJarCache(),
	}
}

func (j *JavaBuilder) Clean(module *Module) error {
	buildDir := filepath.Join(module.ModuleDir, "build")
	if err := os.RemoveAll(buildDir); err != nil {
		return fmt.Errorf("failed to remove build dir %s: %w", buildDir, err)
	}
	return nil
}

func (j *JavaBuilder) Run(module *Module, progArgs []string) error {
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

func (j *JavaBuilder) getModuleJarPath(module *Module) string {
	buildDir := filepath.Join(module.ModuleDir, "build")
	return filepath.Join(buildDir, module.Name+".jar")
}

func (j *JavaBuilder) Build(module *Module) error {
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
	sources, err := module.FindFilesBySuffixR(".java")
	if err != nil {
		return fmt.Errorf("failed to find java sources: %w", err)
	}
	if len(sources) == 0 {
		fmt.Printf("warning: no java sources found in %s\n", module.ModuleDir)
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
	deps, err := module.ResolveReferences()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}
	var classPathParts []string
	if len(deps) > 0 {
		for _, dep := range deps {
			jarPath := j.getModuleJarPath(dep)
			classPathParts = append(classPathParts, jarPath)
		}
		classPath = "--class-path " + strings.Join(classPathParts, string(os.PathListSeparator))
	}

	// Compile java sources (if there are any)
	if len(sources) > 0 {
		if err := j.compileJava(module, buildTmpDir, buildClasses, classPath, extraFlags, sources); err != nil {
			return fmt.Errorf("failed to compile java sources: %w", err)
		}
	}

	// Build into .jar
	if err := j.buildJar(module, buildDir, jarDate, mainClass, deps, buildTmpDir, buildClasses); err != nil {
		return fmt.Errorf("failed to build jar: %w", err)
	}
	return nil
}

func (j *JavaBuilder) compileJava(module *Module, buildTmpDir, buildClasses, classPath, extraFlags string, sourceFiles []SourceFileInfo) error {
	// Write javac args to a single file, avoid potential shell args length limitation (esp with large classpath)
	buildArgsPath := filepath.Join(buildTmpDir, "javac-flags.txt")
	buildArgs := fmt.Sprintf("-d %s\n%s\n%s\n", buildClasses, extraFlags, classPath)
	if err := writeFile(buildArgsPath, buildArgs); err != nil {
		return err
	}

	// Write all source file paths to a file for javac, avoids potential shell args length limitation
	sourceFilesPath := filepath.Join(buildTmpDir, "javac-sources.txt")
	sourceFileList := ""
	for _, sourceFile := range sourceFiles {
		sourceFileList += sourceFile.Path + "\n"
	}
	if err := writeFile(sourceFilesPath, sourceFileList); err != nil {
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

func (j *JavaBuilder) buildJar(module *Module,
	buildDir string,
	jarDate string,
	mainClass string,
	deps []*Module,
	buildTmpDir string,
	buildClasses string) error {

	jarPath := filepath.Join(buildDir, module.Name+".jar")
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
		for _, dep := range deps {
			depJarPath := j.getModuleJarPath(dep)
			jarName := filepath.Base(depJarPath)
			manifestClasspath += jarName + " "
			if err := copyFile(depJarPath, filepath.Join(buildDir, jarName)); err != nil {
				return err
			}
		}
		if len(manifestClasspath) > 0 {
			manifestPath := filepath.Join(buildTmpDir, "manifest")
			manifestString := fmt.Sprintf(`Manifest-Version: 1.0
Class-Path: %s
`, manifestClasspath)
			if err := writeFile(manifestPath, manifestString); err != nil {
				return err
			}
			jarArgs = append(jarArgs, "--manifest", manifestPath)
		}
	}
	jarArgs = append(jarArgs, "-C", buildClasses, ".")
	return execCommand("jar", module.ModuleDir, jarArgs...)
}

func writeFile(filePath, content string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(content)
	return err
}

func copyFile(src, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// Copy the contents from source to destination
	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

type PackageDependency struct {
	Path       string
	URL        string
	Transitive []PackageDependency
}

func (d PackageDependency) PrintTree(index int) {
	fmt.Printf("%s%s\n", strings.Repeat("  ", index), d.URL)
	for _, t := range d.Transitive {
		t.PrintTree(index + 1)
	}
}

func (j *JavaBuilder) ResolveDependencies(module *Module) ([]PackageDependency, error) {
	deps := make([]PackageDependency, 0)
	for _, ref := range module.Packages.References {
		parts := strings.Split(ref.URL, ":")
		if len(parts) != 3 {
			return deps, fmt.Errorf("invalid package URL: %s", ref.URL)
		}
		groupID := parts[0]
		artifactID := parts[1]
		version := parts[2]

		dep, err := j.addDependency(groupID, artifactID, version)
		if err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, nil
}

func (j *JavaBuilder) addDependency(groupID, artifactID, version string) (PackageDependency, error) {
	dep := PackageDependency{}
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
			URL:        artifact.GAV(groupID, artifactID, version),
			Transitive: []PackageDependency{},
		}
	default:
		return dep, fmt.Errorf("packaging type not supported: %s", pom.Packaging)
	}
	for _, childDep := range pom.Dependencies {
		if childDep.Scope == "test" || childDep.Scope == "provided" {
			continue // Skip test and provided scope dependencies
		}
		childDepDep, err := j.addDependency(childDep.GroupID, childDep.ArtifactID, childDep.Version)
		if err != nil {
			return dep, err
		}
		dep.Transitive = append(dep.Transitive, childDepDep)
	}
	return dep, nil
}
