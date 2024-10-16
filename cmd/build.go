/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/jsando/jb/builders"
	"github.com/jsando/jb/project"
	"github.com/spf13/cobra"
	"os"
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
		err := doBuild(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build encountered an error: %s\n", err.Error())
			os.Exit(1)
		}
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
