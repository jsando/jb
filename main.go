package main

import (
	"flag"
	"fmt"
	"github.com/jsando/jb/builder"
	"github.com/jsando/jb/version"
	"github.com/pterm/pterm"
	"os"
	"slices"
)

const USAGE = `jb - The Easier Java Build Tool
Usage: jb [command] [command-options] [arguments]

Execute a command.

Commands:
  build    Build a module.
  clean    Clean build outputs.
  convert  Convert module(s) from another build system to jb.
  help     Show command line help.
  publish  Publish a module to the local maven repository or a remote repository.
  run      Build and run an ExecutableJar module.
  test     Run tests for a module.
  version  Show version information.

Run 'jb [command] --help' for more information on a command.`

func usage(exitCode int) {
	fmt.Println(USAGE)
	os.Exit(exitCode)
}

func versionCommand() {
	fmt.Printf("jb version %s\n", version.Version)
	fmt.Printf(" commit: %s\n", version.Commit)
	fmt.Printf(" built: %s\n", version.Date)
	os.Exit(0)
}

func main() {
	if len(os.Args) < 2 {
		usage(1)
	}

	command := os.Args[1]
	switch command {
	case "build":
		buildCommand(os.Args[2:])
	case "clean":
		cleanCommand(os.Args[2:])
	case "convert":
		convertCommand(os.Args[2:])
	case "help", "-help", "--help":
		usage(0)
	case "publish":
		publishCommand(os.Args[2:])
	case "run":
		runCommand(os.Args[2:])
	case "test":
		testCommand(os.Args[2:])
	case "version", "-v", "--version":
		versionCommand()
	default:
		fmt.Printf("jb: unknown command %s\n", command)
		usage(1)
	}
}

func buildCommand(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: jb build [path]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	path := "."
	buildArgs := fs.Args()
	if len(buildArgs) > 0 && buildArgs[0] != "--" {
		path = buildArgs[0]
	}
	err := builder.BuildModule(path)
	if err != nil {
		pterm.Fatal.Printf("BUILD FAILED: %s\n", err)
	}
}

func cleanCommand(args []string) {
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: jb clean [path]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	path := "."
	err := builder.Clean(path)
	if err != nil {
		pterm.Fatal.Printf("BUILD FAILED: %s\n", err)
	}
}

func convertCommand(args []string) {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: jb convert path")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	path := "."
	buildArgs := fs.Args()
	if len(buildArgs) > 0 && buildArgs[0] != "--" {
		path = buildArgs[0]
	}
	builder.ConvertToJB(path)
}

func publishCommand(args []string) {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	var jarFile string
	var gav string
	fs.StringVar(&jarFile, "jar", "", "jar file to publish (use with --gav to set maven coordinates)")
	fs.StringVar(&gav, "gav", "", "maven coordinates for pushing jar into maven repository")
	fs.Usage = func() {
		fmt.Println("Usage: jb publish [path] [--jar jarfile --gav \"group:artifact:version\"]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	path := "."
	buildArgs := fs.Args()
	if len(buildArgs) > 0 && buildArgs[0] != "--" {
		path = buildArgs[0]
	}
	if jarFile == "" && gav == "" {
		err := builder.BuildAndPublishModule(path)
		if err != nil {
			pterm.Fatal.Printf("BUILD FAILED: %s\n", err)
		}
	} else {
		if jarFile == "" || gav == "" {
			fmt.Println("jar and gav must be specified together")
			os.Exit(1)
		}
		err := builder.PublishRawJAR(jarFile, gav)
		if err != nil {
			pterm.Fatal.Printf("BUILD FAILED: %s\n", err)
		}
	}
}

func runCommand(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: jb run [path] [-- program args]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	runArgs, progArgs := splitArgs(fs.Args())
	path := "."
	if len(runArgs) > 0 {
		path = runArgs[0]
	}
	err := builder.BuildAndRunModule(path, progArgs)
	if err != nil {
		pterm.Fatal.Printf("BUILD FAILED: %s\n", err)
	}
}

func testCommand(strings []string) {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println("Usage: jb test [path]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(strings); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
	runArgs, progArgs := splitArgs(fs.Args())
	path := "."
	if len(runArgs) > 0 {
		path = runArgs[0]
	}
	if len(progArgs) > 0 {
		fmt.Println("jb test does not support running tests with arguments")
	}
	builder.BuildAndTestModule(path)
}

func splitArgs(args []string) ([]string, []string) {
	dash := slices.Index(args, "--")
	if dash < 0 {
		return args, []string{}
	}
	return args[:dash], args[dash+1:]
}
