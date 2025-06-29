package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jsando/jb/util"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const ModuleFilename = "jb-module.json"
const ProjectFilename = "jb-project.json"
const DefaultGroupID = "com.example"
const DefaultVersion = "1.0.0-snapshot"

type ModuleFileJSON struct {
	Group        string   `json:"group,omitempty"`
	Version      string   `json:"version,omitempty"`
	SourceDir    string   `json:"source_dir,omitempty"`
	ResourcesDir string   `json:"resources_dir,omitempty"`
	CompileArgs  []string `json:"javac_args,omitempty"`
	OutputType   string   `json:"output_type,omitempty"`
	MainClass    string   `json:"main_class,omitempty"`
	Resources    []string `json:"resources,omitempty"`
	References   []string `json:"references,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

type Resource struct {
	Dir     string `json:"dir,omitempty"`
	Include string `json:"include,omitempty"`
}

type ProjectFileJSON struct {
	Name    string   `json:"name"`
	Modules []string `json:"modules"`
}

type BuildLog interface {
	Failed() bool
	BuildStart()
	BuildFinish()
	ModuleStart(name string)
	CheckError(task string, err error) bool
	TaskStart(name string) TaskLog
}

type TaskLog interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
	Done(err error) bool
}

type Module struct {
	ModuleFileBytes []byte // to compute hash for up-to-date check
	ModuleDirAbs    string
	SourceDirAbs    string
	ResourceDirAbs  string
	Group           string
	Name            string
	Version         string
	CompileArgs     []string
	OutputType      string
	MainClass       string
	Resources       []string
	References      []*Module
	Dependencies    []*Dependency
}

type Dependency struct {
	Coordinates string        // raw string given such as "org.junit:junit:1.2.3"
	Group       string        // maven organization id
	Artifact    string        // maven artifact id
	Version     string        // maven version string
	Path        string        // empty unless resolved, path to cache folder containing artifacts (pom, jar)
	Transitive  []*Dependency // nil unless resolved
}

type Project struct {
	ProjectDirAbs string
	Name          string
	Modules       []*Module
}

type ModuleLoader struct {
	modules map[string]*Module
}

func NewModuleLoader() *ModuleLoader {
	return &ModuleLoader{
		modules: make(map[string]*Module),
	}
}

// LoadProject loads the given path and returns the Project and optionally the specific
// module.  The path can be to a module or project file or to a directory containing
// either of them.
// If a module is found but no project, a fake one is returned to wrap the module.
func (l *ModuleLoader) LoadProject(path string) (*Project, *Module, error) {
	var err error
	var project *Project
	var module *Module

	path, err = filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	projectSearchDir := ""
	pathInfo, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if pathInfo.IsDir() {
		if util.FileExists(filepath.Join(path, ProjectFilename)) {
			project, err = l.internalLoadProject(filepath.Join(path, ProjectFilename))
			if err != nil {
				return nil, nil, err
			}
			projectSearchDir = path
		} else if util.FileExists(filepath.Join(path, ModuleFilename)) {
			module, err = l.GetModule(filepath.Join(path, ModuleFilename))
			if err != nil {
				return nil, nil, err
			}
			projectSearchDir = filepath.Dir(path)
		} else {
			return nil, nil, fmt.Errorf("path '%s' is a directory but does not contain a project or module file", path)
		}
	} else {
		filename := filepath.Base(path)
		if filename == ProjectFilename {
			project, err = l.internalLoadProject(path)
			if err != nil {
				return nil, nil, err
			}
		} else if filename == ModuleFilename {
			module, err = l.GetModule(path)
			if err != nil {
				return nil, nil, err
			}
			projectSearchDir = filepath.Dir(path)
		} else {
			return nil, nil, fmt.Errorf("path '%s' is not a project or module file", path)
		}
	}

	// If we loaded a module, project is still nil and we need to search parent folders for it
	if project == nil {
		for {
			projectPath := filepath.Join(projectSearchDir, ProjectFilename)
			if util.FileExists(projectPath) {
				project, err = l.internalLoadProject(projectPath)
				if err != nil {
					return nil, nil, err
				}
				break
			}
			parentDir := filepath.Dir(projectSearchDir)
			if parentDir == projectSearchDir {
				break
			}
			projectSearchDir = parentDir
		}
	}

	// If loaded a module AND a project, make sure the module really "belongs" to this project
	if module != nil && project != nil {
		found := false
		for _, m := range project.Modules {
			if m == module {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("WARNING module '%s' does not belong to project '%s'", module.Name, project.Name)
			project = nil
		}
	}

	// If its still nil this must be a standalone module, create a Project to contain it
	if project == nil {
		project = &Project{
			ProjectDirAbs: path,
			Name:          filepath.Base(path),
			Modules:       []*Module{module},
		}
	}

	return project, module, nil
}

func (l *ModuleLoader) internalLoadProject(projectPath string) (*Project, error) {
	var err error
	projectPath, err = filepath.Abs(projectPath)
	if err != nil {
		return nil, err
	}
	data, err := readFile(projectPath)
	if err != nil {
		return nil, err
	}
	var projectJSON = &ProjectFileJSON{}
	err = json.Unmarshal(data, projectJSON)
	if err != nil {
		return nil, err
	}
	project := &Project{
		ProjectDirAbs: filepath.Dir(projectPath),
		Name:          projectJSON.Name,
		Modules:       make([]*Module, 0),
	}
	for _, modulePath := range projectJSON.Modules {
		modulePath := filepath.Join(project.ProjectDirAbs, modulePath)
		module, err := l.GetModule(modulePath)
		if err != nil {
			return nil, err
		}
		project.Modules = append(project.Modules, module)
	}

	return project, nil
}

func (l *ModuleLoader) GetModule(modulePath string) (*Module, error) {
	var err error

	// Require absolute path to avoid confusion with which base to use for relative paths,
	// the initial module loaded would be relative to the cwd but modules use relative
	// references to each other from their point of view.
	if !filepath.IsAbs(modulePath) {
		return nil, errors.New("path must be absolute")
	}

	// Normalize the path to ensure it points to the module file and not just the folder
	modulePath, err = getModuleFilePath(modulePath)
	if err != nil {
		return nil, err
	}

	// already loaded?
	module := l.modules[modulePath]
	if module != nil {
		return module, nil
	}

	// read module file contents
	data, err := readFile(modulePath)
	if err != nil {
		return nil, err
	}

	// deserialize and verify, set defaults
	moduleFile, err := loadModuleFile(data)
	if err != nil {
		return nil, err
	}

	// create module
	module = &Module{}
	module.ModuleFileBytes = data
	module.ModuleDirAbs = filepath.Dir(modulePath)
	module.SourceDirAbs = filepath.Join(module.ModuleDirAbs, moduleFile.SourceDir)
	module.ResourceDirAbs = filepath.Join(module.ModuleDirAbs, moduleFile.ResourcesDir)
	module.Group = moduleFile.Group
	module.Name = filepath.Base(module.ModuleDirAbs)
	module.Version = moduleFile.Version
	module.CompileArgs = moduleFile.CompileArgs
	module.OutputType = moduleFile.OutputType
	module.MainClass = moduleFile.MainClass
	module.Resources = moduleFile.Resources

	module.Dependencies = make([]*Dependency, len(moduleFile.Dependencies))
	for i, s := range moduleFile.Dependencies {
		dep, err := ParseCoordinates(s)
		if err != nil {
			return nil, err
		}
		module.Dependencies[i] = dep
	}

	// save new module to cache before recursively loading references to other modules
	l.modules[module.Name] = module

	// resolve module references
	module.References = []*Module{}
	if moduleFile.References != nil {
		for _, ref := range moduleFile.References {
			refModulePath := filepath.Join(module.ModuleDirAbs, ref)
			refModule, err := l.GetModule(refModulePath)
			// todo error reporting ("error loading module <refname>, loaded from module <thisname>: error")
			if err != nil {
				return nil, err
			}
			module.References = append(module.References, refModule)
		}
	}

	return module, nil
}

func ParseCoordinates(gav string) (*Dependency, error) {
	parts := strings.Split(gav, ":")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid dependency '%s', must be in the form <group>:<artifact>:<version>", gav)
	}
	dep := &Dependency{
		Coordinates: gav,
		Group:       parts[0],
		Artifact:    parts[1],
		Version:     parts[2],
	}
	if dep.Group == "" || dep.Artifact == "" || dep.Version == "" {
		return nil, fmt.Errorf("invalid dependency '%s', must be in the form <group>:<artifact>:<version>", gav)
	}
	return dep, nil
}

// If given a folder, find the module file in it
func getModuleFilePath(modulePath string) (string, error) {
	moduleFilePath := modulePath
	info, err := os.Stat(modulePath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		moduleFilePath = filepath.Join(modulePath, ModuleFilename)
	} else if !strings.HasSuffix(modulePath, ModuleFilename) {
		return "", fmt.Errorf("module file must be named '%s'", ModuleFilename)
	}
	return moduleFilePath, nil
}

func readFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}

func loadModuleFile(data []byte) (*ModuleFileJSON, error) {
	var m = &ModuleFileJSON{}
	err := json.Unmarshal(data, m)
	if err != nil {
		return nil, err
	}
	if m.Version == "" {
		// todo emit warning
		m.Version = DefaultVersion
	}
	if m.Group == "" {
		// todo emit warning
		m.Group = DefaultGroupID
	}
	if m.SourceDir == "" {
		m.SourceDir = "."
	}
	if m.ResourcesDir == "" {
		m.ResourcesDir = "."
	}
	switch m.OutputType {
	case "jar":
	case "executable_jar":
	case "":
		m.OutputType = "jar"
	default:
		return nil, fmt.Errorf("invalid output type '%s'", m.OutputType)
	}
	if m.OutputType == "jar" && m.MainClass != "" {
		return nil, fmt.Errorf("output type 'jar' does not support a main class, use 'executable_jar' instead")
	}
	if m.OutputType == "executable_jar" && m.MainClass == "" {
		return nil, fmt.Errorf("output type 'executable_jar' requires a main class")
	}
	if m.CompileArgs == nil {
		m.CompileArgs = []string{}
	}
	if m.Resources == nil {
		m.Resources = []string{}
	}
	if m.Dependencies == nil {
		m.Dependencies = []string{}
	}
	if m.References == nil {
		m.References = []string{}
	}
	return m, nil
}

func (m *Module) HashContent(hasher hash.Hash) error {
	_, err := hasher.Write(m.ModuleFileBytes)
	return err
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
		for _, ref := range m.References {
			if err := visit(ref); err != nil {
				return err
			}
		}
		stack[m] = false
		visited[m] = true
		result = append(result, m)
		return nil
	}
	for _, ref := range m.References {
		if err := visit(ref); err != nil {
			return nil, err
		}
	}
	return result, nil
}
