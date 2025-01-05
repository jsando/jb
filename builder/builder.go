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
}

func newModuleBuilder(path string) (*moduleBuilder, error) {
	builder := &moduleBuilder{
		loader:  project.NewModuleLoader(),
		builder: java.NewBuilder(),
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

func (b *moduleBuilder) Build() error {
	references, err := b.module.GetModuleReferencesInBuildOrder()
	if err != nil {
		return fmt.Errorf("error resolving module references: %w", err)
	}
	// build referenced modules first
	for _, ref := range references {
		err = b.builder.Build(ref)
		if err != nil {
			return fmt.Errorf("error building module %s: %w", ref.Name, err)
		}
	}
	// build the target module
	err = b.builder.Build(b.module)
	if err != nil {
		return fmt.Errorf("error building module %s: %w", b.module.Name, err)
	}
	return nil
}

func BuildModule(path string) error {
	builder, err := newModuleBuilder(path)
	if err != nil {
		return err
	}
	return builder.Build()
}

func BuildAndRunModule(path string, args []string) error {
	builder, err := newModuleBuilder(path)
	if err != nil {
		return err
	}
	if err = builder.Build(); err != nil {
		return err
	}
	return builder.builder.Run(builder.module, args)
}

func BuildAndPublishModule(path string) error {
	builder, err := newModuleBuilder(path)
	if err != nil {
		return err
	}
	if err = builder.Build(); err != nil {
		return err
	}
	return builder.builder.Publish(builder.module, "", "", "")
}
