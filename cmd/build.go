/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/jsando/jb/project"
	"github.com/spf13/cobra"
	"path/filepath"
)

// BuildCmd represents the build command
var BuildCmd = &cobra.Command{
	Use:   "build [path]",
	Short: "Build a module",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		module, err := loadModule(path)
		if err != nil {
			project.JBLog.Fatalf("error loading module: %s", err)
		}
		err = module.Build()
		if err != nil {
			project.JBLog.Fatalf("build encountered an error: %s", err)
		}
	},
}

func loadModule(path string) (*project.Module, error) {
	path, err := filepath.Abs(path)
	loader := project.NewModuleLoader()
	module, err := loader.GetModule(path)
	if err != nil {
		return nil, err
	}
	return module, nil
}
