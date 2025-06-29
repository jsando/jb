package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// DefaultJavaRunner implements JavaRunner using the system java command
type DefaultJavaRunner struct {
	javaPath string
	version  *JavaVersion
}

// NewDefaultJavaRunner creates a new DefaultJavaRunner
func NewDefaultJavaRunner() *DefaultJavaRunner {
	return &DefaultJavaRunner{}
}

// Run executes a Java program
func (r *DefaultJavaRunner) Run(args RunArgs) error {
	if !r.IsAvailable() {
		return fmt.Errorf("java not found in PATH")
	}

	// Build command arguments
	cmdArgs := []string{}

	// Add JVM arguments
	cmdArgs = append(cmdArgs, args.JvmArgs...)

	// Add classpath if specified
	if args.ClassPath != "" {
		cmdArgs = append(cmdArgs, "-cp", args.ClassPath)
	}

	// Add main class or jar file
	if args.JarFile != "" {
		cmdArgs = append(cmdArgs, "-jar", args.JarFile)
	} else if args.MainClass != "" {
		cmdArgs = append(cmdArgs, args.MainClass)
	} else {
		return fmt.Errorf("either MainClass or JarFile must be specified")
	}

	// Add program arguments
	cmdArgs = append(cmdArgs, args.ProgramArgs...)

	// Create command
	cmd := exec.Command(r.javaPath, cmdArgs...)

	// Set working directory
	if args.WorkDir != "" {
		cmd.Dir = args.WorkDir
	}

	// Set environment
	if len(args.Env) > 0 {
		cmd.Env = append(os.Environ(), args.Env...)
	}

	// Set I/O
	if args.Stdin != nil {
		cmd.Stdin = args.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}

	if args.Stdout != nil {
		cmd.Stdout = args.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}

	if args.Stderr != nil {
		cmd.Stderr = args.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	// Run the command
	return cmd.Run()
}

// RunWithTimeout executes a Java program with a timeout
func (r *DefaultJavaRunner) RunWithTimeout(args RunArgs, timeout time.Duration) error {
	if !r.IsAvailable() {
		return fmt.Errorf("java not found in PATH")
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Build command arguments
	cmdArgs := []string{}

	// Add JVM arguments
	cmdArgs = append(cmdArgs, args.JvmArgs...)

	// Add classpath if specified
	if args.ClassPath != "" {
		cmdArgs = append(cmdArgs, "-cp", args.ClassPath)
	}

	// Add main class or jar file
	if args.JarFile != "" {
		cmdArgs = append(cmdArgs, "-jar", args.JarFile)
	} else if args.MainClass != "" {
		cmdArgs = append(cmdArgs, args.MainClass)
	} else {
		return fmt.Errorf("either MainClass or JarFile must be specified")
	}

	// Add program arguments
	cmdArgs = append(cmdArgs, args.ProgramArgs...)

	// Create command with context
	cmd := exec.CommandContext(ctx, r.javaPath, cmdArgs...)

	// Set working directory
	if args.WorkDir != "" {
		cmd.Dir = args.WorkDir
	}

	// Set environment
	if len(args.Env) > 0 {
		cmd.Env = append(os.Environ(), args.Env...)
	}

	// Set I/O
	if args.Stdin != nil {
		cmd.Stdin = args.Stdin
	} else {
		cmd.Stdin = os.Stdin
	}

	if args.Stdout != nil {
		cmd.Stdout = args.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}

	if args.Stderr != nil {
		cmd.Stderr = args.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	// Run the command
	err := cmd.Run()

	// Check if the context was cancelled (timeout)
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("java process timed out after %v", timeout)
	}

	return err
}

// Version returns the Java runtime version information
func (r *DefaultJavaRunner) Version() (JavaVersion, error) {
	if r.version != nil {
		return *r.version, nil
	}

	if !r.IsAvailable() {
		return JavaVersion{}, fmt.Errorf("java not found")
	}

	cmd := exec.Command(r.javaPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return JavaVersion{}, fmt.Errorf("failed to get java version: %w", err)
	}

	version := parseJavaVersion(string(output))
	r.version = &version
	return version, nil
}

// IsAvailable checks if Java runtime is available on the system
func (r *DefaultJavaRunner) IsAvailable() bool {
	if r.javaPath != "" {
		return true
	}

	// Try to find java
	path, err := exec.LookPath("java")
	if err != nil {
		return false
	}

	r.javaPath = path
	return true
}
