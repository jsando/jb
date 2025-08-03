package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultToolProvider(t *testing.T) {
	provider := NewDefaultToolProvider()
	assert.NotNil(t, provider)

	// Test getting compiler
	compiler := provider.GetCompiler()
	assert.NotNil(t, compiler)
	// Should return same instance on subsequent calls
	compiler2 := provider.GetCompiler()
	assert.Same(t, compiler, compiler2)

	// Test getting jar tool
	jarTool := provider.GetJarTool()
	assert.NotNil(t, jarTool)
	// Should return same instance on subsequent calls
	jarTool2 := provider.GetJarTool()
	assert.Same(t, jarTool, jarTool2)

	// Test getting runner
	runner := provider.GetRunner()
	assert.NotNil(t, runner)
	// Should return same instance on subsequent calls
	runner2 := provider.GetRunner()
	assert.Same(t, runner, runner2)
}

func TestDefaultToolProvider_DetectJDK(t *testing.T) {
	provider := NewDefaultToolProvider()

	// Save original JAVA_HOME
	originalJavaHome := os.Getenv("JAVA_HOME")
	defer os.Setenv("JAVA_HOME", originalJavaHome)

	t.Run("with valid JAVA_HOME", func(t *testing.T) {
		// Skip if javac is not available
		if !isJavacAvailable() {
			t.Skip("javac not available in PATH")
		}

		// Try to detect JDK with current environment
		info, err := provider.DetectJDK()
		if err != nil {
			// This might fail on CI environments without JDK
			t.Skip("JDK not available: " + err.Error())
		}

		assert.NotNil(t, info)
		assert.NotEmpty(t, info.Home)
		assert.NotZero(t, info.Version.Major)
		assert.NotEmpty(t, info.Vendor)
		assert.Equal(t, runtime.GOOS, info.OS)
		assert.Equal(t, runtime.GOARCH, info.Arch)

		// Should cache the result
		info2, err2 := provider.DetectJDK()
		assert.NoError(t, err2)
		assert.Same(t, info, info2)
	})

	t.Run("without JAVA_HOME", func(t *testing.T) {
		// Clear JAVA_HOME
		os.Unsetenv("JAVA_HOME")

		provider2 := NewDefaultToolProvider()
		info, err := provider2.DetectJDK()
		
		if !isJavacAvailable() {
			// Should fail when javac is not in PATH
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "JDK not found")
		} else {
			// If javac is in PATH, it should still work
			if err == nil {
				assert.NotNil(t, info)
				assert.NotEmpty(t, info.Home)
			}
		}
	})

	t.Run("with invalid JAVA_HOME", func(t *testing.T) {
		// Set invalid JAVA_HOME
		os.Setenv("JAVA_HOME", "/invalid/java/home")

		provider3 := NewDefaultToolProvider()
		info, err := provider3.DetectJDK()
		
		if !isJavacAvailable() {
			// Should fail when javac is not in PATH
			assert.Error(t, err)
		} else {
			// If javac is in PATH, it should still work by falling back
			if err == nil {
				assert.NotNil(t, info)
				assert.NotEmpty(t, info.Home)
			}
		}
	})
}

func TestIsValidJDKHome(t *testing.T) {
	// Create a mock JDK structure
	tempDir := t.TempDir()
	binDir := filepath.Join(tempDir, "bin")
	libDir := filepath.Join(tempDir, "lib")
	require.NoError(t, os.MkdirAll(binDir, 0755))
	require.NoError(t, os.MkdirAll(libDir, 0755))

	// Create mock executables
	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
	
	javacPath := filepath.Join(binDir, "javac"+exeSuffix)
	javaPath := filepath.Join(binDir, "java"+exeSuffix)
	jarPath := filepath.Join(binDir, "jar"+exeSuffix)
	
	require.NoError(t, os.WriteFile(javacPath, []byte("mock"), 0755))
	require.NoError(t, os.WriteFile(javaPath, []byte("mock"), 0755))
	require.NoError(t, os.WriteFile(jarPath, []byte("mock"), 0755))

	// Test valid JDK home
	assert.True(t, isValidJDKHome(tempDir))

	// Test invalid JDK home (missing javac)
	require.NoError(t, os.Remove(javacPath))
	assert.False(t, isValidJDKHome(tempDir))

	// Test invalid JDK home (missing lib)
	require.NoError(t, os.WriteFile(javacPath, []byte("mock"), 0755))
	require.NoError(t, os.RemoveAll(libDir))
	assert.False(t, isValidJDKHome(tempDir))

	// Test non-existent directory
	assert.False(t, isValidJDKHome("/non/existent/path"))
}

func TestGlobalDefaultProvider(t *testing.T) {
	// Test getting default provider
	provider := GetDefaultToolProvider()
	assert.NotNil(t, provider)

	// Test setting custom provider
	mockProvider := &MockToolProvider{}
	SetDefaultToolProvider(mockProvider)
	
	provider2 := GetDefaultToolProvider()
	assert.Same(t, mockProvider, provider2)

	// Reset to default
	SetDefaultToolProvider(NewDefaultToolProvider())
}

func TestJavaVersion_Helpers(t *testing.T) {
	tests := []struct {
		name     string
		version  JavaVersion
		is8      bool
		is11     bool
		is17     bool
		is21     bool
		toString string
	}{
		{
			name:     "Java 8",
			version:  JavaVersion{Major: 8},
			is8:      true,
			is11:     false,
			is17:     false,
			is21:     false,
			toString: "8",
		},
		{
			name:     "Java 11",
			version:  JavaVersion{Major: 11, Minor: 0, Patch: 2},
			is8:      true,
			is11:     true,
			is17:     false,
			is21:     false,
			toString: "11.0.2",
		},
		{
			name:     "Java 17",
			version:  JavaVersion{Major: 17, Minor: 1},
			is8:      true,
			is11:     true,
			is17:     true,
			is21:     false,
			toString: "17.1",
		},
		{
			name:     "Java 21",
			version:  JavaVersion{Major: 21},
			is8:      true,
			is11:     true,
			is17:     true,
			is21:     true,
			toString: "21",
		},
		{
			name:     "Java 7",
			version:  JavaVersion{Major: 7},
			is8:      false,
			is11:     false,
			is17:     false,
			is21:     false,
			toString: "7",
		},
		{
			name:     "With full version string",
			version:  JavaVersion{Major: 17, Full: "javac 17.0.1"},
			is8:      true,
			is11:     true,
			is17:     true,
			is21:     false,
			toString: "javac 17.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.is8, tt.version.IsJava8OrLater())
			assert.Equal(t, tt.is11, tt.version.IsJava11OrLater())
			assert.Equal(t, tt.is17, tt.version.IsJava17OrLater())
			assert.Equal(t, tt.is21, tt.version.IsJava21OrLater())
			assert.Equal(t, tt.toString, tt.version.String())
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       JavaVersion
		v2       JavaVersion
		expected int
	}{
		{
			name:     "equal versions",
			v1:       JavaVersion{Major: 11, Minor: 0, Patch: 2},
			v2:       JavaVersion{Major: 11, Minor: 0, Patch: 2},
			expected: 0,
		},
		{
			name:     "v1 major version higher",
			v1:       JavaVersion{Major: 17},
			v2:       JavaVersion{Major: 11},
			expected: 1,
		},
		{
			name:     "v1 major version lower",
			v1:       JavaVersion{Major: 8},
			v2:       JavaVersion{Major: 11},
			expected: -1,
		},
		{
			name:     "same major, v1 minor higher",
			v1:       JavaVersion{Major: 11, Minor: 2},
			v2:       JavaVersion{Major: 11, Minor: 1},
			expected: 1,
		},
		{
			name:     "same major, v1 minor lower",
			v1:       JavaVersion{Major: 11, Minor: 1},
			v2:       JavaVersion{Major: 11, Minor: 2},
			expected: -1,
		},
		{
			name:     "same major/minor, v1 patch higher",
			v1:       JavaVersion{Major: 11, Minor: 0, Patch: 3},
			v2:       JavaVersion{Major: 11, Minor: 0, Patch: 2},
			expected: 1,
		},
		{
			name:     "same major/minor, v1 patch lower",
			v1:       JavaVersion{Major: 11, Minor: 0, Patch: 2},
			v2:       JavaVersion{Major: 11, Minor: 0, Patch: 3},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.v1, tt.v2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPlatformHelpers(t *testing.T) {
	t.Run("executable names", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			assert.Equal(t, "java.exe", GetJavaExecutable())
			assert.Equal(t, "javac.exe", GetJavacExecutable())
			assert.Equal(t, "jar.exe", GetJarExecutable())
		} else {
			assert.Equal(t, "java", GetJavaExecutable())
			assert.Equal(t, "javac", GetJavacExecutable())
			assert.Equal(t, "jar", GetJarExecutable())
		}
	})

	t.Run("normalize path", func(t *testing.T) {
		// Test forward slash conversion
		path := NormalizePath("foo/bar/baz")
		assert.Equal(t, filepath.Join("foo", "bar", "baz"), path)

		// Test path cleaning
		path = NormalizePath("foo//bar/../baz")
		assert.Equal(t, filepath.Join("foo", "baz"), path)

		// Test with trailing separator
		path = NormalizePath("foo/bar/")
		assert.Equal(t, filepath.Join("foo", "bar"), path)
	})

	t.Run("join classpath", func(t *testing.T) {
		paths := []string{"lib/a.jar", "lib/b.jar", "classes"}
		result := JoinClassPath(paths...)

		if runtime.GOOS == "windows" {
			expected := filepath.Join("lib", "a.jar") + ";" + 
						filepath.Join("lib", "b.jar") + ";" + 
						"classes"
			assert.Equal(t, expected, result)
		} else {
			expected := filepath.Join("lib", "a.jar") + ":" + 
						filepath.Join("lib", "b.jar") + ":" + 
						"classes"
			assert.Equal(t, expected, result)
		}

		// Test empty input
		assert.Equal(t, "", JoinClassPath())

		// Test single path
		assert.Equal(t, filepath.Clean("lib/test.jar"), JoinClassPath("lib/test.jar"))
	})
}

// Helper function to check if javac is available
func isJavacAvailable() bool {
	_, err := exec.LookPath("javac")
	return err == nil
}

