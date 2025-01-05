/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/jsando/jb/builder"
	"github.com/spf13/cobra"
	"os"
)

var PublishCmd = &cobra.Command{
	Use:   "publish [path]",
	Short: "Publish a module",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}
		err := builder.BuildAndPublishModule(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build encountered an error: %s\n", err.Error())
			os.Exit(1)
		}
	},
}
