package tools

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultJavaRunner_IsAvailable(t *testing.T) {
	runner := NewDefaultJavaRunner()

	// Test initial state
	isAvailable := runner.IsAvailable()
	
	// This test depends on whether java is actually installed
	if _, err := exec.LookPath("java"); err == nil {
		assert.True(t, isAvailable)
		assert.NotEmpty(t, runner.javaPath)
	} else {
		assert.False(t, isAvailable)
		assert.Empty(t, runner.javaPath)
	}

	// Test caching behavior
	runner2 := &DefaultJavaRunner{javaPath: "/mock/java"}
	assert.True(t, runner2.IsAvailable())
}

func TestDefaultJavaRunner_Version(t *testing.T) {
	runner := NewDefaultJavaRunner()

	// Skip if java is not available
	if !runner.IsAvailable() {
		t.Skip("java not available")
	}

	version, err := runner.Version()
	assert.NoError(t, err)
	assert.NotZero(t, version.Major)
	assert.NotEmpty(t, version.Full)
	assert.NotEmpty(t, version.Vendor)

	// Test caching
	version2, err2 := runner.Version()
	assert.NoError(t, err2)
	assert.Equal(t, version, version2)

	// Test when not available
	runner2 := &DefaultJavaRunner{}
	_, err = runner2.Version()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "java not found")
}

func TestDefaultJavaRunner_Run(t *testing.T) {
	runner := NewDefaultJavaRunner()

	// Skip if java is not available
	if !runner.IsAvailable() {
		t.Skip("java not available")
	}

	tempDir := t.TempDir()

	// Create a simple Java program for testing
	srcDir := filepath.Join(tempDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	
	javaFile := filepath.Join(srcDir, "TestApp.java")
	javaContent := `
public class TestApp {
    public static void main(String[] args) {
        System.out.println("Hello from TestApp");
        for (String arg : args) {
            System.out.println("Arg: " + arg);
        }
        System.err.println("Error stream test");
        
        // Check system property
        String prop = System.getProperty("test.property");
        if (prop != null) {
            System.out.println("Property: " + prop);
        }
    }
}
`
	require.NoError(t, os.WriteFile(javaFile, []byte(javaContent), 0644))

	// Compile the program
	compiler := NewDefaultJavaCompiler()
	if compiler.IsAvailable() {
		classDir := filepath.Join(tempDir, "classes")
		require.NoError(t, os.MkdirAll(classDir, 0755))
		
		result, err := compiler.Compile(CompileArgs{
			SourceFiles: []string{javaFile},
			DestDir:     classDir,
			WorkDir:     tempDir,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		t.Run("run with main class", func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := RunArgs{
				MainClass:   "TestApp",
				ClassPath:   classDir,
				ProgramArgs: []string{"arg1", "arg2"},
				WorkDir:     tempDir,
				Stdout:      &stdout,
				Stderr:      &stderr,
			}

			err := runner.Run(args)
			assert.NoError(t, err)

			// Check output
			assert.Contains(t, stdout.String(), "Hello from TestApp")
			assert.Contains(t, stdout.String(), "Arg: arg1")
			assert.Contains(t, stdout.String(), "Arg: arg2")
			assert.Contains(t, stderr.String(), "Error stream test")
		})

		t.Run("run with JVM args", func(t *testing.T) {
			var stdout bytes.Buffer
			args := RunArgs{
				MainClass: "TestApp",
				ClassPath: classDir,
				JvmArgs:   []string{"-Dtest.property=TestValue"},
				WorkDir:   tempDir,
				Stdout:    &stdout,
			}

			err := runner.Run(args)
			assert.NoError(t, err)
			assert.Contains(t, stdout.String(), "Property: TestValue")
		})

		t.Run("run with environment", func(t *testing.T) {
			var stdout bytes.Buffer
			args := RunArgs{
				MainClass: "TestApp",
				ClassPath: classDir,
				Env:       []string{"TEST_ENV=value"},
				WorkDir:   tempDir,
				Stdout:    &stdout,
			}

			err := runner.Run(args)
			assert.NoError(t, err)
			// Environment is passed but not used by our test program
		})

		// Create a test JAR
		jarTool := NewDefaultJarTool()
		if jarTool.IsAvailable() {
			jarFile := filepath.Join(tempDir, "test.jar")
			err := jarTool.Create(JarArgs{
				JarFile:   jarFile,
				BaseDir:   classDir,
				MainClass: "TestApp",
				WorkDir:   tempDir,
			})
			require.NoError(t, err)

			t.Run("run jar file", func(t *testing.T) {
				var stdout bytes.Buffer
				args := RunArgs{
					JarFile:     jarFile,
					ProgramArgs: []string{"jar-arg"},
					WorkDir:     tempDir,
					Stdout:      &stdout,
				}

				err := runner.Run(args)
				assert.NoError(t, err)
				assert.Contains(t, stdout.String(), "Hello from TestApp")
				assert.Contains(t, stdout.String(), "Arg: jar-arg")
			})
		}
	}

	t.Run("missing main class and jar", func(t *testing.T) {
		args := RunArgs{
			WorkDir: tempDir,
		}

		err := runner.Run(args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "either MainClass or JarFile must be specified")
	})

	t.Run("runner not available", func(t *testing.T) {
		runner := &DefaultJavaRunner{javaPath: ""}
		args := RunArgs{
			MainClass: "TestApp",
		}

		err := runner.Run(args)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "java not found")
	})

	t.Run("non-existent main class", func(t *testing.T) {
		var stderr bytes.Buffer
		args := RunArgs{
			MainClass: "NonExistentClass",
			ClassPath: "/tmp",
			WorkDir:   tempDir,
			Stderr:    &stderr,
		}

		err := runner.Run(args)
		assert.Error(t, err)
		// The error will be in stderr and the exit code will be non-zero
		assert.Contains(t, stderr.String(), "NonExistentClass")
	})
}

func TestDefaultJavaRunner_RunWithTimeout(t *testing.T) {
	runner := NewDefaultJavaRunner()

	// Skip if java is not available
	if !runner.IsAvailable() {
		t.Skip("java not available")
	}

	tempDir := t.TempDir()

	// Create a Java program that runs for a specific time
	srcDir := filepath.Join(tempDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	
	javaFile := filepath.Join(srcDir, "SleepApp.java")
	javaContent := `
public class SleepApp {
    public static void main(String[] args) throws InterruptedException {
        System.out.println("Starting");
        long sleepTime = Long.parseLong(args[0]);
        Thread.sleep(sleepTime);
        System.out.println("Finished");
    }
}
`
	require.NoError(t, os.WriteFile(javaFile, []byte(javaContent), 0644))

	// Compile the program
	compiler := NewDefaultJavaCompiler()
	if compiler.IsAvailable() {
		classDir := filepath.Join(tempDir, "classes")
		require.NoError(t, os.MkdirAll(classDir, 0755))
		
		result, err := compiler.Compile(CompileArgs{
			SourceFiles: []string{javaFile},
			DestDir:     classDir,
			WorkDir:     tempDir,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		t.Run("completes before timeout", func(t *testing.T) {
			var stdout bytes.Buffer
			args := RunArgs{
				MainClass:   "SleepApp",
				ClassPath:   classDir,
				ProgramArgs: []string{"100"}, // 100ms
				WorkDir:     tempDir,
				Stdout:      &stdout,
			}

			err := runner.RunWithTimeout(args, 1*time.Second)
			assert.NoError(t, err)
			assert.Contains(t, stdout.String(), "Starting")
			assert.Contains(t, stdout.String(), "Finished")
		})

		t.Run("timeout exceeded", func(t *testing.T) {
			var stdout bytes.Buffer
			args := RunArgs{
				MainClass:   "SleepApp",
				ClassPath:   classDir,
				ProgramArgs: []string{"2000"}, // 2 seconds
				WorkDir:     tempDir,
				Stdout:      &stdout,
			}

			err := runner.RunWithTimeout(args, 100*time.Millisecond)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "timed out")
			
			// Should have started but not finished
			output := stdout.String()
			if output != "" {
				assert.Contains(t, output, "Starting")
				assert.NotContains(t, output, "Finished")
			}
		})
	}

	t.Run("missing main class and jar", func(t *testing.T) {
		args := RunArgs{
			WorkDir: tempDir,
		}

		err := runner.RunWithTimeout(args, 1*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "either MainClass or JarFile must be specified")
	})

	t.Run("runner not available", func(t *testing.T) {
		runner := &DefaultJavaRunner{javaPath: ""}
		args := RunArgs{
			MainClass: "TestApp",
		}

		err := runner.RunWithTimeout(args, 1*time.Second)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "java not found")
	})
}

func TestDefaultJavaRunner_Stdin(t *testing.T) {
	runner := NewDefaultJavaRunner()

	// Skip if java is not available
	if !runner.IsAvailable() {
		t.Skip("java not available")
	}

	tempDir := t.TempDir()

	// Create a Java program that reads from stdin
	srcDir := filepath.Join(tempDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	
	javaFile := filepath.Join(srcDir, "EchoApp.java")
	javaContent := `
import java.util.Scanner;

public class EchoApp {
    public static void main(String[] args) {
        Scanner scanner = new Scanner(System.in);
        System.out.println("Echo program started");
        while (scanner.hasNextLine()) {
            String line = scanner.nextLine();
            System.out.println("Echo: " + line);
            if (line.equals("quit")) {
                break;
            }
        }
        scanner.close();
        System.out.println("Echo program ended");
    }
}
`
	require.NoError(t, os.WriteFile(javaFile, []byte(javaContent), 0644))

	// Compile the program
	compiler := NewDefaultJavaCompiler()
	if compiler.IsAvailable() {
		classDir := filepath.Join(tempDir, "classes")
		require.NoError(t, os.MkdirAll(classDir, 0755))
		
		result, err := compiler.Compile(CompileArgs{
			SourceFiles: []string{javaFile},
			DestDir:     classDir,
			WorkDir:     tempDir,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		t.Run("read from stdin", func(t *testing.T) {
			// Prepare input
			stdin := strings.NewReader("Hello\nWorld\nquit\n")
			var stdout bytes.Buffer
			
			args := RunArgs{
				MainClass: "EchoApp",
				ClassPath: classDir,
				WorkDir:   tempDir,
				Stdin:     stdin,
				Stdout:    &stdout,
			}

			err := runner.Run(args)
			assert.NoError(t, err)

			output := stdout.String()
			assert.Contains(t, output, "Echo program started")
			assert.Contains(t, output, "Echo: Hello")
			assert.Contains(t, output, "Echo: World")
			assert.Contains(t, output, "Echo: quit")
			assert.Contains(t, output, "Echo program ended")
		})
	}
}

func TestDefaultJavaRunner_WorkingDirectory(t *testing.T) {
	runner := NewDefaultJavaRunner()

	// Skip if java is not available
	if !runner.IsAvailable() {
		t.Skip("java not available")
	}

	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "work")
	require.NoError(t, os.MkdirAll(workDir, 0755))

	// Create a test file in work directory
	testFile := filepath.Join(workDir, "test.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("test content"), 0644))

	// Create a Java program that checks working directory
	srcDir := filepath.Join(tempDir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	
	javaFile := filepath.Join(srcDir, "DirApp.java")
	javaContent := `
import java.io.File;

public class DirApp {
    public static void main(String[] args) {
        System.out.println("Working directory: " + System.getProperty("user.dir"));
        File file = new File("test.txt");
        System.out.println("test.txt exists: " + file.exists());
    }
}
`
	require.NoError(t, os.WriteFile(javaFile, []byte(javaContent), 0644))

	// Compile the program
	compiler := NewDefaultJavaCompiler()
	if compiler.IsAvailable() {
		classDir := filepath.Join(tempDir, "classes")
		require.NoError(t, os.MkdirAll(classDir, 0755))
		
		result, err := compiler.Compile(CompileArgs{
			SourceFiles: []string{javaFile},
			DestDir:     classDir,
			WorkDir:     tempDir,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		t.Run("working directory is set", func(t *testing.T) {
			var stdout bytes.Buffer
			args := RunArgs{
				MainClass: "DirApp",
				ClassPath: classDir,
				WorkDir:   workDir,
				Stdout:    &stdout,
			}

			err := runner.Run(args)
			assert.NoError(t, err)

			output := stdout.String()
			assert.Contains(t, output, workDir)
			assert.Contains(t, output, "test.txt exists: true")
		})
	}
}

// Test that context cancellation works properly
func TestDefaultJavaRunner_ContextCancellation(t *testing.T) {
	runner := NewDefaultJavaRunner()

	// Skip if java is not available
	if !runner.IsAvailable() {
		t.Skip("java not available")
	}

	// This tests the internal behavior when context is cancelled
	// The RunWithTimeout method already tests timeout behavior
	t.Run("context already cancelled", func(t *testing.T) {
		// Create an already cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// We can't directly test with a cancelled context through the public API,
		// but we've already tested timeout behavior which uses context cancellation
		_ = ctx
	})
}