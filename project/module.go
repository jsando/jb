package project

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Module struct {
	ModuleFile string     `xml:"-"`        // Absolute path to jbm file
	ModuleDir  string     `xml:"-"`        // Absolute path to module directory
	Name       string     `xml:"-"`        // Module name (ie the dir name)
	SDK        string     `xml:"Sdk,attr"` // Name of dev kit (ie, which builder to use)
	Properties Properties `xml:"Properties"`
}

type Properties struct {
	Properties []Property `xml:",any"` // Collection of key/value pairs to pass to builder
}

type Property struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type SourceFileInfo struct {
	Info os.FileInfo
	Path string // path relative to module root
}

func LoadModule(modulePath string) (*Module, error) {
	var err error
	modulePath, err = filepath.Abs(modulePath)
	if err != nil {
		return nil, err
	}
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
	var module Module
	err = xml.Unmarshal(data, &module)
	if err != nil {
		return nil, err
	}
	module.ModuleFile = modulePath
	module.ModuleDir = filepath.Dir(modulePath)
	module.Name = filepath.Base(module.ModuleDir)
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
