package project

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const ModuleFilename = ".jbm"

const DefaultGroupID = "com.example"
const DefaultVersion = "1.0.0-snapshot"

type Module struct {
	XMLName    xml.Name    `xml:"Module"`
	ModuleFile string      `xml:"-"`        // Absolute path to jbm file
	ModuleDir  string      `xml:"-"`        // Absolute path to module directory
	GroupID    string      `xml:"GroupID"`  // Organization (used as maven group id)
	Name       string      `xml:"-"`        // Simple module name (ie the dir name)
	Version    string      `xml:"Version"`  // Version as a semver string, defaults to "1.0.0-snapshot"
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
	if module.Version == "" {
		module.Version = DefaultVersion
		fmt.Printf("WARNING: no 'Version' specified for module %s, using default %s\n", module.Name, module.Version)
	}
	if module.GroupID == "" {
		module.GroupID = DefaultGroupID
		fmt.Printf("WARNING: no 'GroupID' specified for module %s, using default %s\n", module.Name, module.GroupID)
	}
	if module.Packages == nil {
		module.Packages = &Packages{
			References: make([]*PackageReference, 0),
		}
	}
	return &module, nil
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

func (m *Module) GetPropertyList(key string) []string {
	var values []string
	for _, property := range m.Properties.Properties {
		if property.XMLName.Local == key {
			values = append(values, strings.TrimSpace(property.Value))
		}
	}
	return values
}

// GetModuleReferencesInBuildOrder returns the list of referenced modules this module depends on,
// sorted in order that they should be built.  This module is not included in the list.
func (m *Module) GetModuleReferencesInBuildOrder() ([]*Module, error) {
	if m == nil {
		return nil, errors.New("module is nil")
	}
	visited := make(map[*Module]bool) // has been visited already
	stack := make(map[*Module]bool)   // if currently being visited, to detect circular references
	result := []*Module{}
	var visit func(m *Module) error
	visit = func(m *Module) error {
		if visited[m] {
			return nil
		}
		if stack[m] {
			return fmt.Errorf("circular reference detected for module %s", m.Name)
		}
		stack[m] = true
		for _, ref := range m.References.Modules {
			if err := visit(ref.Module); err != nil {
				return err
			}
		}
		stack[m] = false
		visited[m] = true
		result = append(result, m)
		return nil
	}
	for _, ref := range m.References.Modules {
		if err := visit(ref.Module); err != nil {
			return nil, err
		}
	}
	return result, nil
}
