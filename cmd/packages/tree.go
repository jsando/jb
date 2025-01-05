/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package packages

import (
	"github.com/spf13/cobra"
)

// addCmd represents the add command
var treeCmd = &cobra.Command{
	Use:   "tree [module]]",
	Short: "Print dependency tree",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
	},
}
