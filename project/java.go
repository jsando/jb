package project

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type JavaBuilder struct {
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

const (
	PropertyOutputType    = "OutputType"
	PropertyMainClass     = "MainClass"
	PropertyCompilerFlags = "CompilerFlags"
	PropertyJarDate       = "JarDate"
	OutputTypeJar         = "Jar"
	OutputTypeExeJar      = "ExecutableJar"
)

func (j *JavaBuilder) Build(module *Module) error {
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

	sources, err := module.FindFilesBySuffixR(".java")
	if err != nil {
		return err
	}
	buildDir := filepath.Join(module.ModuleDir, "build")
	buildTmpDir := filepath.Join(buildDir, "tmp")
	buildClasses := filepath.Join(buildTmpDir, "classes")
	err = os.RemoveAll(buildTmpDir)
	if err != nil {
		return err
	}
	err = os.MkdirAll(buildClasses, os.ModePerm)
	if err != nil {
		return err
	}

	sourceFilesPath := filepath.Join(buildTmpDir, "sourcefiles.txt")
	sourcefilesFile, err := os.Create(sourceFilesPath)
	if err != nil {
		return err
	}
	defer sourcefilesFile.Close()
	for _, sourceFile := range sources {
		_, err = sourcefilesFile.WriteString(sourceFile.Path + "\n")
		if err != nil {
			return err
		}
	}
	sourcefilesFile.Close()

	// For compilation, use the absolute paths to all jar dependencies
	classPath := ""
	deps, err := module.ResolveReferences()
	if err != nil {
		return err
	}
	if len(deps) > 0 {
		for _, dep := range deps {
			if len(classPath) > 0 {
				classPath += string(os.PathListSeparator)
			}
			jarPath := j.getModuleJarPath(dep)
			classPath += jarPath
		}
		classPath = "--class-path " + classPath
	}

	optionsPath := filepath.Join(buildTmpDir, "options.txt")
	optionsFile, err := os.Create(optionsPath)
	rel, err := filepath.Rel(module.ModuleDir, buildClasses)
	_, err = optionsFile.WriteString(fmt.Sprintf(`
-d %s
%s
%s
`, rel, extraFlags, classPath))
	optionsFile.Close()
	if err != nil {
		return err
	}

	optionsPathRel, err := filepath.Rel(module.ModuleDir, optionsPath)
	if err != nil {
		return err
	}
	sourceFilesPathRel, err := filepath.Rel(module.ModuleDir, sourceFilesPath)
	if err != nil {
		return err
	}

	cmd := exec.Command("javac", "@"+optionsPathRel, "@"+sourceFilesPathRel)
	cmd.Dir = module.ModuleDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	fmt.Printf("running %s with args %v\n", cmd.Path, cmd.Args)
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Build into .jar
	jarpath := filepath.Join(buildDir, module.Name+".jar")
	jarArgs := []string{
		"--create", "--file", jarpath,
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
			err = copyFile(depJarPath, filepath.Join(buildDir, jarName))
		}
		if len(manifestClasspath) > 0 {
			manifestPath := filepath.Join(buildTmpDir, "manifest")
			manifestFile, err := os.Create(manifestPath)
			if err != nil {
				return err
			}
			_, err = manifestFile.WriteString(fmt.Sprintf(`Manifest-Version: 1.0
Class-Path: %s
`, manifestClasspath))
			manifestFile.Close()
			if err != nil {
				return err
			}
			jarArgs = append(jarArgs, "--manifest", manifestPath)
		}
	}
	jarArgs = append(jarArgs, "-C", buildClasses, ".")
	cmd = exec.Command("jar", jarArgs...)
	fmt.Printf("running %s with args %v\n", cmd.Path, cmd.Args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
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
