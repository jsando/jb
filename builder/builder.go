package builder

import (
	"encoding/xml"
	"fmt"
	"github.com/jsando/jb/java"
	"github.com/jsando/jb/maven"
	"github.com/jsando/jb/project"
	"github.com/jsando/jb/util"
	"path/filepath"
	"strings"
)

type moduleBuilder struct {
	loader  *project.ModuleLoader
	builder *java.Builder
	module  *project.Module
	logger  project.BuildLog
}

func newModuleBuilder(path string, logger project.BuildLog) (*moduleBuilder, error) {
	builder := &moduleBuilder{
		loader:  project.NewModuleLoader(),
		builder: java.NewBuilder(logger),
		logger:  logger,
	}
	// Load the module and recursively load its referenced modules
	path, err := filepath.Abs(path)
	module, err := builder.loader.GetModule(path)
	if err != nil {
		return nil, fmt.Errorf("error loading module '%s': %w", path, err)
	}
	builder.module = module
	return builder, nil
}

func (b *moduleBuilder) Build() {
	references, err := b.module.GetModuleReferencesInBuildOrder()
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
	b.builder.Build(b.module)
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
	return builder.builder.Run(builder.module, args)
}

func BuildAndPublishModule(path string) error {
	logger := NewBuildLog()
	builder, err := newModuleBuilder(path, logger)
	if err != nil {
		return err
	}
	builder.Build()
	logger.BuildFinish()
	return builder.builder.Publish(builder.module, "", "", "")
}

func Clean(path string) error {
	logger := NewBuildLog()
	builder, err := newModuleBuilder(path, logger)
	if err != nil {
		return err
	}
	builder.builder.Clean(builder.module)
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
