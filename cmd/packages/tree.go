/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package packages

import (
	"fmt"
	"github.com/jsando/jb/project"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

// addCmd represents the add command
var treeCmd = &cobra.Command{
	Use:   "tree [module]]",
	Short: "Print dependency tree",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		module, err := loadModule(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("loaded module, resolving dependencies (%d packages)\n", len(module.Packages.References))
		deps, err := module.ResolveDependencies()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("found %d dependencies:\n", len(deps))
		for _, dep := range deps {
			dep.PrintTree(0)
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
