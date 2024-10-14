/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/jsando/jb/builders"
	"github.com/jsando/jb/project"
	"github.com/spf13/cobra"
)

// BuildCmd represents the build command
var BuildCmd = &cobra.Command{
	Use:   "build [path]",
	Short: "Build a module",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		return doBuild(path)
	},
}

func init() {

}

func doBuild(path string) error {
	module, err := project.LoadModule(path)
	if err != nil {
		return err
	}
	builder, err := builders.GetBuilder(module.SDK)
	if err != nil {
		return err
	}
	return builder.Build(module, project.BuildContext{})
}
