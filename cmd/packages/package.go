/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package packages

import (
	"fmt"

	"github.com/spf13/cobra"
)

// PackageCmd represents the packages command
var PackageCmd = &cobra.Command{
	Use:     "package",
	Aliases: []string{"pkg"},
	Short:   "Manage maven package dependencies",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("packages called")
	},
}

func init() {
	// Here you will define your flags and configuration settings.
	PackageCmd.AddCommand(removeCmd)
	PackageCmd.AddCommand(addCmd)
}
