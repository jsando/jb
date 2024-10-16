/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"github.com/jsando/jb/builders"
	"github.com/spf13/cobra"
	"os"
	"slices"
)

var RunCmd = &cobra.Command{
	Use:   "run [path]",
	Short: "Run a executable jar module, use '--' to separate program args",
	Run: func(cmd *cobra.Command, args []string) {
		baseArgs, progArgs, err := parseArgs(args)
		path := "."
		if len(baseArgs) > 0 {
			path = baseArgs[0]
		}
		module, err := loadModule(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		err = doBuild(module)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build encountered an error: %s\n", err.Error())
			os.Exit(1)
		}
		builder, err := builders.GetBuilder(module.SDK)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build encountered an error: %s\n", err.Error())
			os.Exit(1)
		}
		err = builder.Run(module, progArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error running module: %s\n", err.Error())
			os.Exit(1)
		}
	},
}

func parseArgs(args []string) (baseArgs []string, progArgs []string, err error) {
	i := slices.Index(args, "--")
	if i == -1 {
		baseArgs = args
		progArgs = []string{}
	} else {
		baseArgs = args[:i]
		progArgs = args[i+1:]
	}
	return
}
