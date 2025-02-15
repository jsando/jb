package builder

import (
	"encoding/xml"
	"fmt"
	"github.com/jsando/jb/java"
	"github.com/jsando/jb/maven"
	"github.com/jsando/jb/project"
	"github.com/jsando/jb/util"
	"strings"
)

type moduleBuilder struct {
	loader       *project.ModuleLoader
	builder      *java.Builder
	project      *project.Project
	buildModules []*project.Module
	logger       project.BuildLog
}

func newModuleBuilder(path string, logger project.BuildLog) (*moduleBuilder, error) {
	builder := &moduleBuilder{
		loader:       project.NewModuleLoader(),
		builder:      java.NewBuilder(logger),
		logger:       logger,
		buildModules: make([]*project.Module, 0),
	}

	// Load the module and recursively load its referenced modules
	project, module, err := builder.loader.LoadProject(path)
	if err != nil {
		return nil, fmt.Errorf("error loading '%s': %w", path, err)
	}
	builder.project = project
	if module != nil {
		builder.buildModules = append(builder.buildModules, module)
	} else {
		builder.buildModules = append(builder.buildModules, project.Modules...)
	}
	if len(builder.buildModules) == 0 {
		return nil, fmt.Errorf("no modules found in '%s'", path)
	}
	for _, module := range builder.buildModules {
		if module == nil {
			panic("someone added a nil module to the build list")
		}
	}
	return builder, nil
}

func (b *moduleBuilder) Build() {
	for _, module := range b.buildModules {
		references, err := module.GetModuleReferencesInBuildOrder()
		if b.logger.CheckError("Resolving module references", err) {
			return
		}
		// build referenced modules first
		for _, ref := range references {
			b.builder.Build(ref)

			// abort if errors
			if b.logger.Failed() {
				return
			}
		}

		// build the target module
		b.builder.Build(module)
	}
}

func BuildModule(path string) error {
	logger := NewBuildLog()
	builder, err := newModuleBuilder(path, logger)
	if err != nil {
		return err
	}
	builder.Build()
	logger.BuildFinish()
	return nil
}

func BuildAndRunModule(path string, args []string) error {
	logger := NewBuildLog()
	builder, err := newModuleBuilder(path, logger)
	if err != nil {
		return err
	}
	builder.Build()
	logger.BuildFinish()
	if len(builder.buildModules) != 1 {
		return fmt.Errorf("expected exactly one module to build and run got %d", len(builder.buildModules))
	}
	return builder.builder.Run(builder.buildModules[0], args)
}

func BuildAndPublishModule(path string) error {
	logger := NewBuildLog()
	builder, err := newModuleBuilder(path, logger)
	if err != nil {
		return err
	}
	builder.Build()
	logger.BuildFinish()
	for _, module := range builder.buildModules {
		if err := builder.builder.Publish(module, "", "", ""); err != nil {
			return err
		}
	}
	return nil
}

func Clean(path string) error {
	logger := NewBuildLog()
	builder, err := newModuleBuilder(path, logger)
	if err != nil {
		return err
	}
	for _, module := range builder.buildModules {
		builder.builder.Clean(module)
	}
	logger.BuildFinish()
	return nil
}

func PublishRawJAR(jarPath, gav string) error {
	logger := NewBuildLog()
	logger.ModuleStart("Publishing existing jar file")
	d, err := project.ParseCoordinates(gav)
	if logger.CheckError("Parsing GAV", err) {
		return nil
	}
	task := logger.TaskStart("Creating pom file")
	pomPath, err := writePOM(jarPath, d)
	task.Done(err)
	task = logger.TaskStart("Installing package")
	repo := maven.OpenLocalRepository()
	err = repo.InstallPackage(d.Group, d.Artifact, d.Version, jarPath, pomPath)
	task.Done(err)
	logger.BuildFinish()
	return nil
}

func writePOM(jarPath string, dep *project.Dependency) (string, error) {
	pom := maven.POM{
		Xmlns:             "http://maven.apache.org/POM/4.0.0",                                          // Default namespace
		XmlnsXsi:          "http://www.w3.org/2001/XMLSchema-instance",                                  // XML Schema instance namespace
		XsiSchemaLocation: "http://maven.apache.org/POM/4.0.0 http://maven.apache.org/maven-v4_0_0.xsd", // Schema location
		ModelVersion:      "4.0.0",
		Packaging:         "jar",
		GroupID:           dep.Group,
		ArtifactID:        dep.Artifact,
		Version:           dep.Version,
		Name:              dep.Artifact,
		Description:       dep.Artifact,
	}
	pomPath := strings.TrimSuffix(jarPath, ".jar") + ".pom"
	pomXML, err := xml.MarshalIndent(pom, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to serialize POM to XML: %w", err)
	}
	xmlHeader := []byte(xml.Header)
	pomContent := append(xmlHeader, pomXML...)
	if err := util.WriteFile(pomPath, string(pomContent)); err != nil {
		return "", fmt.Errorf("failed to write POM to %s: %w", pomPath, err)
	}
	return pomPath, nil
}

func BuildAndTestModule(path string) {
	logger := NewBuildLog()
	builder, err := newModuleBuilder(path, logger)
	if logger.CheckError("loading project", err) {
		return
	}
	builder.Build()
	for _, module := range builder.buildModules {
		builder.builder.RunTest(module)
	}
	logger.BuildFinish()
}

// ConvertToJB converts the maven project at the given path to jb.
func ConvertToJB(path string) {

}
