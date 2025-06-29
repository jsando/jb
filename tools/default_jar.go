package tools

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultJarTool implements JarTool using the system jar command
type DefaultJarTool struct {
	jarPath string
	version *JavaVersion
}

// NewDefaultJarTool creates a new DefaultJarTool
func NewDefaultJarTool() *DefaultJarTool {
	return &DefaultJarTool{}
}

// Create creates a new JAR file
func (t *DefaultJarTool) Create(args JarArgs) error {
	if !t.IsAvailable() {
		return fmt.Errorf("jar tool not found in PATH")
	}

	// Build jar command arguments
	cmdArgs := []string{}

	// Use create flag with file option
	// Java 8 uses short form: -cf
	// Newer versions also support long form: --create --file
	cmdArgs = append(cmdArgs, "-cf", args.JarFile)

	// Add date if specified (for reproducible builds)
	// This is only available in newer JDK versions
	if args.Date != "" {
		version, err := t.Version()
		if err == nil && version.Major >= 11 {
			cmdArgs = append(cmdArgs, "--date", args.Date)
		}
	}

	// Handle manifest - need to create manifest for main class and/or classpath
	var manifestContent string
	needManifest := false

	if args.ManifestFile != "" {
		// Use provided manifest file with -m short form
		cmdArgs = append(cmdArgs, "-m", args.ManifestFile)
	} else {
		// Build manifest content if needed
		manifestContent = "Manifest-Version: 1.0\n"

		if args.MainClass != "" {
			manifestContent += "Main-Class: " + args.MainClass + "\n"
			needManifest = true
		}

		if len(args.ClassPath) > 0 {
			manifestContent += "Class-Path:"
			for i, cp := range args.ClassPath {
				if i > 0 {
					manifestContent += "\n "
				} else {
					manifestContent += " "
				}
				manifestContent += cp
			}
			manifestContent += "\n"
			needManifest = true
		}

		if needManifest {
			tmpManifest := filepath.Join(os.TempDir(), fmt.Sprintf("jb-manifest-%d.txt", os.Getpid()))
			defer os.Remove(tmpManifest)

			if err := os.WriteFile(tmpManifest, []byte(manifestContent), 0644); err != nil {
				return fmt.Errorf("failed to write manifest: %w", err)
			}

			cmdArgs = append(cmdArgs, "-m", tmpManifest)
		}
	}

	// Add base directory if specified
	if args.BaseDir != "" {
		cmdArgs = append(cmdArgs, "-C", args.BaseDir)
	}

	// Add files
	if len(args.Files) == 0 {
		// If no files specified, include everything in base directory
		cmdArgs = append(cmdArgs, ".")
	} else {
		cmdArgs = append(cmdArgs, args.Files...)
	}

	// Execute jar command
	cmd := exec.Command(t.jarPath, cmdArgs...)
	if args.WorkDir != "" {
		cmd.Dir = args.WorkDir
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jar creation failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Extract extracts files from a JAR
func (t *DefaultJarTool) Extract(jarFile, destDir string) error {
	if !t.IsAvailable() {
		return fmt.Errorf("jar tool not found in PATH")
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract: jar -xf jarfile
	cmd := exec.Command(t.jarPath, "-xf", jarFile)
	cmd.Dir = destDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("jar extraction failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// List lists the contents of a JAR file
func (t *DefaultJarTool) List(jarFile string) ([]string, error) {
	if !t.IsAvailable() {
		return nil, fmt.Errorf("jar tool not found in PATH")
	}

	// List: jar -tf jarfile
	cmd := exec.Command(t.jarPath, "-tf", jarFile)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list jar contents: %w", err)
	}

	// Parse output into file list
	var files []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		file := strings.TrimSpace(scanner.Text())
		if file != "" {
			files = append(files, file)
		}
	}

	return files, nil
}

// Update adds or updates files in an existing JAR
func (t *DefaultJarTool) Update(jarFile string, files map[string]string) error {
	if !t.IsAvailable() {
		return fmt.Errorf("jar tool not found in PATH")
	}

	// For each file, we need to update the jar
	// jar -uf jarfile -C dir file
	for jarPath, localPath := range files {
		dir := filepath.Dir(localPath)
		file := filepath.Base(localPath)

		cmdArgs := []string{"-uf", jarFile}
		if dir != "." && dir != "" {
			cmdArgs = append(cmdArgs, "-C", dir)
		}
		cmdArgs = append(cmdArgs, file)

		cmd := exec.Command(t.jarPath, cmdArgs...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to update jar with %s: %w\nOutput: %s", jarPath, err, string(output))
		}
	}

	return nil
}

// Version returns the tool version information
func (t *DefaultJarTool) Version() (JavaVersion, error) {
	if t.version != nil {
		return *t.version, nil
	}

	if !t.IsAvailable() {
		return JavaVersion{}, fmt.Errorf("jar tool not found")
	}

	// The jar tool doesn't have a direct version flag, but we can use java -version
	// since they're typically from the same JDK
	javaPath, err := exec.LookPath("java")
	if err != nil {
		return JavaVersion{}, fmt.Errorf("java not found")
	}

	cmd := exec.Command(javaPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return JavaVersion{}, fmt.Errorf("failed to get java version: %w", err)
	}

	version := parseJavaVersion(string(output))
	t.version = &version
	return version, nil
}

// IsAvailable checks if the tool is available on the system
func (t *DefaultJarTool) IsAvailable() bool {
	if t.jarPath != "" {
		return true
	}

	// Try to find jar
	path, err := exec.LookPath("jar")
	if err != nil {
		return false
	}

	t.jarPath = path
	return true
}
