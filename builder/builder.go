package builder

import (
	"fmt"
	"github.com/jsando/jb/java"
	"github.com/jsando/jb/project"
	"path/filepath"
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
