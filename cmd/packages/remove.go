/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package packages

import (
	"fmt"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:     "remove PACKAGE",
	Aliases: []string{"rm"},
	Short:   "Remove a package dependency",
	Args:    cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		packageName := args[0]
		fmt.Printf("Removing dependency %s from module %s\n", packageName, modulePath)
	},
}

func init() {
	removeCmd.Flags().StringP("module", "m", "", "The module to remove the package from (optional)")
}
