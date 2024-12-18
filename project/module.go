package project

import (
	"encoding/xml"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Module struct {
	XMLName    xml.Name    `xml:"Module"`
	ModuleFile string      `xml:"-"`        // Absolute path to jbm file
	ModuleDir  string      `xml:"-"`        // Absolute path to module directory
	Name       string      `xml:"-"`        // Simple module name (ie the dir name)
	SDK        string      `xml:"Sdk,attr"` // Name of dev kit (which builder to use)
	Properties *Properties `xml:"Properties"`
	References *References `xml:"References"`
	Packages   *Packages   `xml:"Packages"`
}

type Properties struct {
	Properties []Property `xml:",any"` // Collection of key/value pairs to pass to builder
}

type Property struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type References struct {
	Modules []*ModuleReference `xml:"Module"`
}

type ModuleReference struct {
	Path   string  `xml:"Path,attr"`
	Module *Module `xml:"-"`
}

type Packages struct {
	References []*PackageReference `xml:"Package"`
}

type PackageReference struct {
	URL string `xml:"URL,attr"`
}

type SourceFileInfo struct {
	Info os.FileInfo
	Path string // path relative to module root
}

type ModuleLoader struct {
	modules map[string]*Module
}

func NewModuleLoader() *ModuleLoader {
	return &ModuleLoader{
		modules: make(map[string]*Module),
	}
}

func (l *ModuleLoader) GetModule(path string) (*Module, error) {
	var err error
	if !filepath.IsAbs(path) {
		return nil, errors.New("path must be absolute")
	}
	module := l.modules[path]
	if module == nil {
		module, err = l.loadModule(path)
		if err != nil {
			return nil, err
		}
		l.modules[module.Name] = module
		if module.References.Modules == nil {
			module.References.Modules = make([]*ModuleReference, 0)
		}

		// Load module references
		// TODO erm .. how to detect circular reference?
		for _, ref := range module.References.Modules {
			refPath := filepath.Join(module.ModuleDir, ref.Path)
			refModule, err := l.GetModule(refPath)
			if err != nil {
				return module, err
			}
			ref.Module = refModule
		}
	}
	return module, nil
}

func (l *ModuleLoader) loadModule(modulePath string) (*Module, error) {
	var err error
	info, err := os.Stat(modulePath)
	if err != nil {
		return nil, err
	}
	// If given a folder, find the module file in it
	if info.IsDir() {
		modulePath = filepath.Join(modulePath, ModuleFilename)
	}
	file, err := os.Open(modulePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	// todo verify schema and give hints if errors found
	// a) verify all parameters
	// b) does go return an error if nothing mapped?
	var module = Module{}
	err = xml.Unmarshal(data, &module)
	if err != nil {
		return nil, err
	}
	module.ModuleFile = modulePath
	module.ModuleDir = filepath.Dir(modulePath)
	module.Name = filepath.Base(module.ModuleDir)
	if module.References == nil {
		module.References = &References{
			Modules: make([]*ModuleReference, 0),
		}
	}
	if module.Properties == nil {
		module.Properties = &Properties{
			Properties: make([]Property, 0),
		}
	}
	return &module, nil
}

func (m *Module) FindFilesBySuffixR(suffix string) ([]SourceFileInfo, error) {
	var sources []SourceFileInfo
	err := filepath.Walk(m.ModuleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == "build" {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(info.Name())) == suffix {
			relPath, err := filepath.Rel(m.ModuleDir, path)
			if err != nil {
				return err
			}
			sources = append(sources, SourceFileInfo{Info: info, Path: relPath})
		}
		return nil
	})
	return sources, err
}

func (m *Module) GetProperty(key, defaultValue string) string {
	value := defaultValue
	for _, property := range m.Properties.Properties {
		if property.XMLName.Local == key {
			value = strings.TrimSpace(property.Value)
			break
		}
	}
	return value
}

func (m *Module) ResolveDependencies() ([]PackageDependency, error) {
	builder, err := GetBuilder(m.SDK)
	if err != nil {
		return nil, err
	}
	return builder.ResolveDependencies(m)

}

func (m *Module) Build() error {
	// build dependencies of this module first, in the right order
	deps, err := m.ResolveReferences()
	if err != nil {
		return err
	}
	for _, dep := range deps {
		err = dep.Build()
		if err != nil {
			return err
		}
	}

	// Build this module
	builder, err := GetBuilder(m.SDK)
	if err != nil {
		return err
	}
	return builder.Build(m)
}

func (m *Module) Run(progArgs []string) error {
	builder, err := GetBuilder(m.SDK)
	if err != nil {
		return err
	}
	return builder.Run(m, progArgs)
}

func (m *Module) ResolveReferences() ([]*Module, error) {
	visited := make(map[*Module]bool)
	result := []*Module{}
	for _, dep := range m.References.Modules {
		if !visited[dep.Module] {
			topologicalSort(dep.Module, visited, &result)
		}
	}
	slices.Reverse(result)
	return result, nil
}

// topologicalSort sorts the projects in the order they should be built.
// It performs a depth-first search and adds each project to the result after all its dependencies.
func topologicalSort(module *Module, visited map[*Module]bool, result *[]*Module) {
	if visited[module] {
		return
	}
	visited[module] = true
	for _, dep := range module.References.Modules {
		topologicalSort(dep.Module, visited, result)
	}
	*result = append(*result, module)
}
