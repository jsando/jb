/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
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
		module, err := loadModule(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		err = module.Build()
		if err != nil {
			fmt.Fprintf(os.Stderr, "build encountered an error: %s\n", err.Error())
			os.Exit(1)
		}
		err = module.Publish("", "", "")
	},
}
