/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/jsando/jb/builder"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
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
		err := builder.BuildModule(path)
		if err != nil {
			pterm.Fatal.Printf("BUILD FAILED: %s\n", err)
		}
	},
}
