package tools

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DefaultJavaCompiler implements JavaCompiler using the system javac command
type DefaultJavaCompiler struct {
	javacPath string
	version   *JavaVersion
}

// NewDefaultJavaCompiler creates a new DefaultJavaCompiler
func NewDefaultJavaCompiler() *DefaultJavaCompiler {
	return &DefaultJavaCompiler{}
}

// Compile compiles Java source files
func (c *DefaultJavaCompiler) Compile(args CompileArgs) (CompileResult, error) {
	result := CompileResult{
		Success:  true,
		Errors:   []CompileError{},
		Warnings: []CompileWarning{},
	}

	// Ensure javac is available
	if !c.IsAvailable() {
		return result, fmt.Errorf("javac not found in PATH")
	}

	// Create temporary files for arguments to avoid command line length limits
	tmpDir := os.TempDir()

	// Write compiler flags to file
	flagsFile := filepath.Join(tmpDir, fmt.Sprintf("jb-javac-flags-%d.txt", os.Getpid()))
	defer os.Remove(flagsFile)

	var flags []string
	flags = append(flags, "-d", args.DestDir)

	if args.ClassPath != "" {
		flags = append(flags, "-cp", args.ClassPath)
	}

	if args.SourceVersion != "" {
		flags = append(flags, "-source", args.SourceVersion)
	}

	if args.TargetVersion != "" {
		flags = append(flags, "-target", args.TargetVersion)
	}

	flags = append(flags, args.ExtraFlags...)

	flagsContent := strings.Join(flags, "\n")
	if err := os.WriteFile(flagsFile, []byte(flagsContent), 0644); err != nil {
		return result, fmt.Errorf("failed to write flags file: %w", err)
	}

	// Write source files list
	sourcesFile := filepath.Join(tmpDir, fmt.Sprintf("jb-javac-sources-%d.txt", os.Getpid()))
	defer os.Remove(sourcesFile)

	sourcesContent := strings.Join(args.SourceFiles, "\n")
	if err := os.WriteFile(sourcesFile, []byte(sourcesContent), 0644); err != nil {
		return result, fmt.Errorf("failed to write sources file: %w", err)
	}

	// Execute javac
	cmd := exec.Command(c.javacPath, "@"+flagsFile, "@"+sourcesFile)
	if args.WorkDir != "" {
		cmd.Dir = args.WorkDir
	}

	// Capture output
	output, err := cmd.CombinedOutput()
	result.RawOutput = string(output)

	// Parse output for errors and warnings
	c.parseCompilerOutput(result.RawOutput, &result)

	if err != nil {
		result.Success = false
		if result.ErrorCount == 0 && len(result.Errors) == 0 {
			// If we couldn't parse any errors, create a generic one
			result.Errors = append(result.Errors, CompileError{
				Message: fmt.Sprintf("compilation failed: %v", err),
			})
			result.ErrorCount = 1
		}
	}

	return result, nil
}

// parseCompilerOutput parses javac output for errors and warnings
func (c *DefaultJavaCompiler) parseCompilerOutput(output string, result *CompileResult) {
	// Regex patterns for different javac error formats
	// Format: filename.java:line: error: message
	errorPattern := regexp.MustCompile(`^(.+?):(\d+):\s*(error|warning):\s*(.+)$`)
	// Format: filename.java:line:column: error: message (newer javac versions)
	errorPatternWithColumn := regexp.MustCompile(`^(.+?):(\d+):(\d+):\s*(error|warning):\s*(.+)$`)

	scanner := bufio.NewScanner(strings.NewReader(output))
	var currentError *CompileError
	var currentWarning *CompileWarning

	for scanner.Scan() {
		line := scanner.Text()

		// Try to match error/warning patterns
		if matches := errorPatternWithColumn.FindStringSubmatch(line); len(matches) > 0 {
			file := matches[1]
			lineNum, _ := strconv.Atoi(matches[2])
			column, _ := strconv.Atoi(matches[3])
			errorType := matches[4]
			message := matches[5]

			if errorType == "error" {
				currentError = &CompileError{
					File:    file,
					Line:    lineNum,
					Column:  column,
					Message: message,
				}
				result.Errors = append(result.Errors, *currentError)
				result.ErrorCount++
			} else {
				currentWarning = &CompileWarning{
					File:    file,
					Line:    lineNum,
					Column:  column,
					Message: message,
				}
				result.Warnings = append(result.Warnings, *currentWarning)
				result.WarningCount++
			}
		} else if matches := errorPattern.FindStringSubmatch(line); len(matches) > 0 {
			file := matches[1]
			lineNum, _ := strconv.Atoi(matches[2])
			errorType := matches[3]
			message := matches[4]

			if errorType == "error" {
				currentError = &CompileError{
					File:    file,
					Line:    lineNum,
					Message: message,
				}
				result.Errors = append(result.Errors, *currentError)
				result.ErrorCount++
			} else {
				currentWarning = &CompileWarning{
					File:    file,
					Line:    lineNum,
					Message: message,
				}
				result.Warnings = append(result.Warnings, *currentWarning)
				result.WarningCount++
			}
		} else if strings.Contains(line, "error") && currentError != nil {
			// Continuation of error message
			currentError.Message += "\n" + line
		} else if strings.Contains(line, "warning") && currentWarning != nil {
			// Continuation of warning message
			currentWarning.Message += "\n" + line
		}
	}

	// Check for summary line (e.g., "2 errors")
	summaryPattern := regexp.MustCompile(`^(\d+)\s+errors?$`)
	if matches := summaryPattern.FindStringSubmatch(strings.TrimSpace(output)); len(matches) > 0 {
		count, _ := strconv.Atoi(matches[1])
		if count > result.ErrorCount {
			result.ErrorCount = count
		}
	}
}

// Version returns the compiler version information
func (c *DefaultJavaCompiler) Version() (JavaVersion, error) {
	if c.version != nil {
		return *c.version, nil
	}

	if !c.IsAvailable() {
		return JavaVersion{}, fmt.Errorf("javac not found")
	}

	cmd := exec.Command(c.javacPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return JavaVersion{}, fmt.Errorf("failed to get javac version: %w", err)
	}

	version := parseJavaVersion(string(output))
	c.version = &version
	return version, nil
}

// IsAvailable checks if the compiler is available on the system
func (c *DefaultJavaCompiler) IsAvailable() bool {
	if c.javacPath != "" {
		return true
	}

	// Try to find javac
	path, err := exec.LookPath("javac")
	if err != nil {
		return false
	}

	c.javacPath = path
	return true
}

// parseJavaVersion parses version output from Java tools
func parseJavaVersion(output string) JavaVersion {
	version := JavaVersion{
		Full: strings.TrimSpace(output),
	}

	// Try different version patterns
	// Pattern 1: javac 1.8.0_292 (older format)
	// Pattern 2: javac 11.0.11 (newer format)
	// Pattern 3: javac 17 (even newer format)

	versionPattern := regexp.MustCompile(`(\d+)(?:\.(\d+))?(?:\.(\d+))?`)
	if matches := versionPattern.FindStringSubmatch(output); len(matches) > 0 {
		version.Major, _ = strconv.Atoi(matches[1])
		if len(matches) > 2 && matches[2] != "" {
			version.Minor, _ = strconv.Atoi(matches[2])
		}
		if len(matches) > 3 && matches[3] != "" {
			version.Patch, _ = strconv.Atoi(matches[3])
		}

		// Handle old version format (1.x -> x)
		if version.Major == 1 && version.Minor > 0 {
			version.Major = version.Minor
			version.Minor = 0
		}
	}

	// Detect vendor
	lowerOutput := strings.ToLower(output)
	switch {
	case strings.Contains(lowerOutput, "openjdk"):
		version.Vendor = "OpenJDK"
	case strings.Contains(lowerOutput, "oracle"):
		version.Vendor = "Oracle"
	case strings.Contains(lowerOutput, "graalvm"):
		version.Vendor = "GraalVM"
	case strings.Contains(lowerOutput, "adoptopenjdk"):
		version.Vendor = "AdoptOpenJDK"
	case strings.Contains(lowerOutput, "temurin"):
		version.Vendor = "Eclipse Temurin"
	default:
		version.Vendor = "Unknown"
	}

	return version
}
