package builders

import (
	"archive/zip"
	"fmt"
	"github.com/jsando/jb/project"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type JavaBuilder struct {
}

const (
	PropertyOutputType    = "OutputType"
	PropertyMainClass     = "MainClass"
	PropertyCompilerFlags = "CompilerFlags"
	PropertyJarDate       = "JarDate"
	OutputTypeJar         = "Jar"
	OutputTypeExeJar      = "ExecutableJar"
)

func (j *JavaBuilder) Build(module *project.Module, ctx project.BuildContext) error {
	outputType := module.GetProperty(PropertyOutputType, OutputTypeJar)
	mainClass := module.GetProperty(PropertyMainClass, "")
	if outputType == OutputTypeExeJar && len(mainClass) == 0 {
		return fmt.Errorf("%s requires property %s to be set", OutputTypeExeJar, PropertyMainClass)
	}
	if outputType == OutputTypeJar && len(mainClass) > 0 {
		return fmt.Errorf("%s only allowed with %s %s", PropertyMainClass, PropertyOutputType, OutputTypeExeJar)
	}
	extraFlags := module.GetProperty(PropertyCompilerFlags, "")
	timestampString := module.GetProperty(PropertyJarDate, "")
	jarTime := time.Now()
	var err error
	if timestampString != "" {
		jarTime, err = time.Parse(time.RFC3339, timestampString)
		if err != nil {
			return fmt.Errorf("error parsing JarDate %s: %s (use format %s)", timestampString, err.Error(), time.RFC3339)
		}
	}

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

	optionsPath := filepath.Join(buildTmpDir, "options.txt")
	optionsFile, err := os.Create(optionsPath)
	rel, err := filepath.Rel(module.ModuleDir, buildClasses)
	_, err = optionsFile.WriteString(fmt.Sprintf(`
-d %s
%s
`, rel, extraFlags))
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
	err = cmd.Run()
	if err != nil {
		return err
	}

	jarpath := filepath.Join(buildDir, module.Name+".jar")
	return createJar(jarpath, buildClasses, jarTime)
}

// Function to create a zipfile given a path
func createJar(jarname string, basedir string, jarTime time.Time) error {
	// Create the zip file
	zipFile, err := os.Create(jarname)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	// Create a new zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through the directory and add files to the zip
	err = filepath.Walk(basedir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(basedir, path)
		if err != nil {
			return err
		}

		// Create a zip header for the file entry
		zipHeader, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Set the name and modified time on the header for reproducibility
		if info.IsDir() {
			relPath += "/"
		}
		zipHeader.Name = relPath
		zipHeader.Method = zip.Deflate
		zipHeader.Modified = jarTime

		// Create the file in the zip archive
		zipWriterEntry, err := zipWriter.CreateHeader(zipHeader)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Copy the file content to the zip entry
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(zipWriterEntry, file)
		return err
	})
	return err
}
