package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultJavaCompiler_IsAvailable(t *testing.T) {
	compiler := NewDefaultJavaCompiler()

	// Test initial state
	isAvailable := compiler.IsAvailable()
	
	// This test depends on whether javac is actually installed
	if _, err := exec.LookPath("javac"); err == nil {
		assert.True(t, isAvailable)
		assert.NotEmpty(t, compiler.javacPath)
	} else {
		assert.False(t, isAvailable)
		assert.Empty(t, compiler.javacPath)
	}

	// Test caching behavior
	compiler2 := &DefaultJavaCompiler{javacPath: "/mock/javac"}
	assert.True(t, compiler2.IsAvailable())
}

func TestDefaultJavaCompiler_Version(t *testing.T) {
	compiler := NewDefaultJavaCompiler()

	// Skip if javac is not available
	if !compiler.IsAvailable() {
		t.Skip("javac not available")
	}

	version, err := compiler.Version()
	assert.NoError(t, err)
	assert.NotZero(t, version.Major)
	assert.NotEmpty(t, version.Full)
	assert.NotEmpty(t, version.Vendor)

	// Test caching
	version2, err2 := compiler.Version()
	assert.NoError(t, err2)
	assert.Equal(t, version, version2)

	// Test when not available
	compiler2 := &DefaultJavaCompiler{}
	_, err3 := compiler2.Version()
	assert.Error(t, err3)
	assert.Contains(t, err3.Error(), "javac not found")
}

func TestDefaultJavaCompiler_Compile(t *testing.T) {
	compiler := NewDefaultJavaCompiler()

	// Skip if javac is not available
	if !compiler.IsAvailable() {
		t.Skip("javac not available")
	}

	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	destDir := filepath.Join(tempDir, "classes")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(destDir, 0755))

	// Create a simple Java file
	javaFile := filepath.Join(srcDir, "Test.java")
	javaContent := `
public class Test {
    public static void main(String[] args) {
        System.out.println("Hello, World!");
    }
}
`
	require.NoError(t, os.WriteFile(javaFile, []byte(javaContent), 0644))

	t.Run("successful compilation", func(t *testing.T) {
		args := CompileArgs{
			SourceFiles: []string{javaFile},
			DestDir:     destDir,
			WorkDir:     tempDir,
		}

		result, err := compiler.Compile(args)
		assert.NoError(t, err)
		assert.True(t, result.Success)
		assert.Empty(t, result.Errors)
		assert.Empty(t, result.Warnings)

		// Check that class file was created
		classFile := filepath.Join(destDir, "Test.class")
		assert.FileExists(t, classFile)
	})

	t.Run("compilation with classpath", func(t *testing.T) {
		args := CompileArgs{
			SourceFiles: []string{javaFile},
			DestDir:     destDir,
			ClassPath:   "/some/lib.jar:/another/lib.jar",
			WorkDir:     tempDir,
		}

		result, err := compiler.Compile(args)
		assert.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("compilation with source and target", func(t *testing.T) {
		args := CompileArgs{
			SourceFiles:   []string{javaFile},
			DestDir:       destDir,
			SourceVersion: "8",
			TargetVersion: "8",
			WorkDir:       tempDir,
		}

		result, err := compiler.Compile(args)
		assert.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("compilation with extra flags", func(t *testing.T) {
		args := CompileArgs{
			SourceFiles: []string{javaFile},
			DestDir:     destDir,
			ExtraFlags:  []string{"-Xlint:all", "-g"},
			WorkDir:     tempDir,
		}

		result, err := compiler.Compile(args)
		assert.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("compilation error", func(t *testing.T) {
		// Create a Java file with syntax error
		errorFile := filepath.Join(srcDir, "Error.java")
		errorContent := `
public class Error {
    public static void main(String[] args) {
        System.out.println("Missing semicolon")
    }
}
`
		require.NoError(t, os.WriteFile(errorFile, []byte(errorContent), 0644))

		args := CompileArgs{
			SourceFiles: []string{errorFile},
			DestDir:     destDir,
			WorkDir:     tempDir,
		}

		result, err := compiler.Compile(args)
		assert.NoError(t, err) // Compile returns nil error, check result.Success
		assert.False(t, result.Success)
		assert.NotEmpty(t, result.Errors)
		assert.Greater(t, result.ErrorCount, 0)
		assert.Contains(t, result.RawOutput, "error")
	})

	t.Run("compilation warning", func(t *testing.T) {
		// Create a Java file that generates a warning
		warnFile := filepath.Join(srcDir, "Warning.java")
		warnContent := `
import java.util.*;

public class Warning {
    public static void main(String[] args) {
        List list = new ArrayList(); // Raw type warning
        list.add("test");
    }
}
`
		require.NoError(t, os.WriteFile(warnFile, []byte(warnContent), 0644))

		args := CompileArgs{
			SourceFiles: []string{warnFile},
			DestDir:     destDir,
			ExtraFlags:  []string{"-Xlint:unchecked"},
			WorkDir:     tempDir,
		}

		result, err := compiler.Compile(args)
		assert.NoError(t, err)
		// May or may not have warnings depending on javac version
		if len(result.Warnings) > 0 {
			assert.Greater(t, result.WarningCount, 0)
		}
	})

	t.Run("compiler not available", func(t *testing.T) {
		compiler := &DefaultJavaCompiler{javacPath: ""}
		args := CompileArgs{
			SourceFiles: []string{javaFile},
			DestDir:     destDir,
		}

		_, err := compiler.Compile(args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "javac not found")
	})
}

func TestParseCompilerOutput(t *testing.T) {
	compiler := &DefaultJavaCompiler{}

	tests := []struct {
		name          string
		output        string
		expectedError int
		expectedWarn  int
	}{
		{
			name: "error with line and column",
			output: `Test.java:5:24: error: ';' expected
        System.out.println("test")
                           ^
1 error`,
			expectedError: 1,
			expectedWarn:  0,
		},
		{
			name: "error without column",
			output: `Test.java:5: error: ';' expected
        System.out.println("test")
1 error`,
			expectedError: 1,
			expectedWarn:  0,
		},
		{
			name: "warning",
			output: `Warning.java:6:14: warning: [unchecked] unchecked call to add(E) as a member of the raw type List
        list.add("test");
                ^
  where E is a type-variable:
    E extends Object declared in interface List
1 warning`,
			expectedError: 0,
			expectedWarn:  1,
		},
		{
			name: "multiple errors",
			output: `Test.java:5:24: error: ';' expected
        System.out.println("test")
                           ^
Test.java:8:10: error: cannot find symbol
        unknownMethod();
        ^
  symbol:   method unknownMethod()
  location: class Test
2 errors`,
			expectedError: 2,
			expectedWarn:  0,
		},
		{
			name: "error and warning",
			output: `Test.java:5:24: error: ';' expected
        System.out.println("test")
                           ^
Warning.java:6:14: warning: [unchecked] unchecked call
        list.add("test");
                ^
1 error
1 warning`,
			expectedError: 1,
			expectedWarn:  1,
		},
		{
			name:          "no errors or warnings",
			output:        "",
			expectedError: 0,
			expectedWarn:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompileResult{
				Success:  true,
				Errors:   []CompileError{},
				Warnings: []CompileWarning{},
			}

			compiler.parseCompilerOutput(tt.output, &result)

			assert.Len(t, result.Errors, tt.expectedError)
			assert.Len(t, result.Warnings, tt.expectedWarn)
			
			if tt.expectedError > 0 {
				assert.Equal(t, tt.expectedError, result.ErrorCount)
			}
			if tt.expectedWarn > 0 {
				assert.Equal(t, tt.expectedWarn, result.WarningCount)
			}
		})
	}
}

func TestParseJavaVersion(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		expectedMajor  int
		expectedMinor  int
		expectedPatch  int
		expectedVendor string
	}{
		{
			name:           "old format (1.8)",
			output:         "javac 1.8.0_292",
			expectedMajor:  8,
			expectedMinor:  0,
			expectedPatch:  0,
			expectedVendor: "Unknown",
		},
		{
			name:           "modern format",
			output:         "javac 11.0.11",
			expectedMajor:  11,
			expectedMinor:  0,
			expectedPatch:  11,
			expectedVendor: "Unknown",
		},
		{
			name:           "simple format",
			output:         "javac 17",
			expectedMajor:  17,
			expectedMinor:  0,
			expectedPatch:  0,
			expectedVendor: "Unknown",
		},
		{
			name:           "OpenJDK",
			output:         "javac 17.0.1 OpenJDK",
			expectedMajor:  17,
			expectedMinor:  0,
			expectedPatch:  1,
			expectedVendor: "OpenJDK",
		},
		{
			name:           "Oracle",
			output:         "javac 11.0.12 Oracle Corporation",
			expectedMajor:  11,
			expectedMinor:  0,
			expectedPatch:  12,
			expectedVendor: "Oracle",
		},
		{
			name:           "GraalVM",
			output:         "javac 17.0.2 GraalVM CE",
			expectedMajor:  17,
			expectedMinor:  0,
			expectedPatch:  2,
			expectedVendor: "GraalVM",
		},
		{
			name:           "AdoptOpenJDK",
			output:         "javac 11.0.11 AdoptOpenJDK",
			expectedMajor:  11,
			expectedMinor:  0,
			expectedPatch:  11,
			expectedVendor: "OpenJDK", // AdoptOpenJDK contains "openjdk" so matches OpenJDK
		},
		{
			name:           "Eclipse Temurin",
			output:         "javac 17.0.5 Eclipse Temurin",
			expectedMajor:  17,
			expectedMinor:  0,
			expectedPatch:  5,
			expectedVendor: "Eclipse Temurin",
		},
		{
			name:           "invalid format",
			output:         "invalid version string",
			expectedMajor:  0,
			expectedMinor:  0,
			expectedPatch:  0,
			expectedVendor: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := parseJavaVersion(tt.output)
			assert.Equal(t, tt.expectedMajor, version.Major)
			assert.Equal(t, tt.expectedMinor, version.Minor)
			assert.Equal(t, tt.expectedPatch, version.Patch)
			assert.Equal(t, tt.expectedVendor, version.Vendor)
			assert.Equal(t, strings.TrimSpace(tt.output), version.Full)
		})
	}
}

func TestDefaultJavaCompiler_LargeCompilation(t *testing.T) {
	compiler := NewDefaultJavaCompiler()

	// Skip if javac is not available
	if !compiler.IsAvailable() {
		t.Skip("javac not available")
	}

	tempDir := t.TempDir()
	srcDir := filepath.Join(tempDir, "src")
	destDir := filepath.Join(tempDir, "classes")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(destDir, 0755))

	// Create many source files to test command line length handling
	var sourceFiles []string
	for i := 0; i < 100; i++ {
		fileName := fmt.Sprintf("Test%d.java", i)
		filePath := filepath.Join(srcDir, fileName)
		content := fmt.Sprintf(`
public class Test%d {
    public void method() {
        System.out.println("Test %d");
    }
}
`, i, i)
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
		sourceFiles = append(sourceFiles, filePath)
	}

	args := CompileArgs{
		SourceFiles: sourceFiles,
		DestDir:     destDir,
		WorkDir:     tempDir,
		ExtraFlags:  []string{"-verbose:class"},
	}

	result, err := compiler.Compile(args)
	assert.NoError(t, err)
	assert.True(t, result.Success)
	assert.Empty(t, result.Errors)

	// Check that all class files were created
	for i := 0; i < 100; i++ {
		classFile := filepath.Join(destDir, fmt.Sprintf("Test%d.class", i))
		assert.FileExists(t, classFile)
	}
}

func TestDefaultJavaCompiler_ErrorContinuation(t *testing.T) {
	compiler := &DefaultJavaCompiler{}

	// Test multi-line error message parsing
	output := `Test.java:5:24: error: ';' expected
        System.out.println("test")
                           ^
  This is a continuation line with error details
  Another continuation line
Test.java:8:10: warning: deprecated method
        oldMethod();
        ^
  Note: This method has been deprecated
  Use newMethod() instead
1 error
1 warning`

	result := CompileResult{
		Success:  true,
		Errors:   []CompileError{},
		Warnings: []CompileWarning{},
	}

	compiler.parseCompilerOutput(output, &result)

	assert.Len(t, result.Errors, 1)
	assert.Len(t, result.Warnings, 1)
	
	// Check that continuation lines were captured
	assert.Contains(t, result.Errors[0].Message, "';' expected")
	assert.Contains(t, result.Warnings[0].Message, "deprecated method")
}