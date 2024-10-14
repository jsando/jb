/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package packages

import (
	"fmt"
	"github.com/spf13/cobra"
)

var modulePath string

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add PACKAGE",
	Short: "Add a package dependency",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		packageName := args[0]
		fmt.Printf("Adding dependency %s to module %s\n", packageName, modulePath)
	},
}

func init() {
	addCmd.Flags().StringVarP(&modulePath, "module", "m", ".", "The module to add the package to (optional)")
}
