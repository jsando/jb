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

var RunCmd = &cobra.Command{
	Use:   "run [path]",
	Short: "Run a executable jar module, use '--' to separate program args",
	Run: func(cmd *cobra.Command, args []string) {
		baseArgs := args
		progArgs := []string{}
		dash := cmd.ArgsLenAtDash()
		if dash > 0 {
			baseArgs = args[:dash]
			progArgs = args[dash:]
		}

		path := "."
		if len(baseArgs) > 0 {
			path = baseArgs[0]
		}
		if len(baseArgs) > 1 {
			fmt.Fprintf(os.Stderr, "error: too many arguments (hint: use '--' to separate program args)\n")
			os.Exit(1)
		}
		err := builder.BuildAndRunModule(path, progArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
		}
	},
}
