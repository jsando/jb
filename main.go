package main

import (
	"flag"
	"fmt"
	"github.com/jsando/jb/builder"
	"github.com/pterm/pterm"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		helpCommand()
	}

	command := os.Args[1]
	switch command {
	case "build":
		buildCommand(os.Args[2:])
	case "run":
		runCommand(os.Args[2:])
	case "help":
		helpCommand()
	default:
		fmt.Printf("jb: unknown command %s\n", command)
		helpCommand()
	}
}

func helpCommand() {
	fmt.Println("jb help")
	os.Exit(1)
}

func buildCommand(args []string) {
	buildFlags := flag.NewFlagSet("build", flag.ExitOnError)
	buildFlags.Usage = func() {
		fmt.Println("Usage: jb build [path]")
		buildFlags.PrintDefaults()
	}
	if err := buildFlags.Parse(args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	path := "."
	buildArgs := buildFlags.Args()
	if len(buildArgs) > 0 && buildArgs[0] != "--" {
		path = buildArgs[0]
	}
	err := builder.BuildModule(path)
	if err != nil {
		pterm.Fatal.Printf("BUILD FAILED: %s\n", err)
	}
}

func runCommand(args []string) {
	runFlags := flag.NewFlagSet("run", flag.ExitOnError)
	runFlags.Usage = func() {
		fmt.Println("Usage: jb run [path] [-- program args]")
		runFlags.PrintDefaults()
	}
	if err := runFlags.Parse(args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	runArgs, progArgs := splitArgs(runFlags.Args())
	path := "."
	if len(runArgs) > 0 {
		path = runArgs[0]
	}
	err := builder.BuildAndRunModule(path, progArgs)
	if err != nil {
		pterm.Fatal.Printf("BUILD FAILED: %s\n", err)
	}
}

func splitArgs(args []string) ([]string, []string) {
	dash := argDash(args)
	if dash < 0 {
		return args, []string{}
	}
	return args[:dash], args[dash+1:]
}

func argDash(args []string) int {
	for i, arg := range args {
		if arg == "--" {
			return i
		}
	}
	return -1
}
