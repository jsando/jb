package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultToolProvider provides access to system Java development tools
type DefaultToolProvider struct {
	compiler JavaCompiler
	jarTool  JarTool
	runner   JavaRunner
	jdkInfo  *JDKInfo
}

// NewDefaultToolProvider creates a new DefaultToolProvider
func NewDefaultToolProvider() *DefaultToolProvider {
	return &DefaultToolProvider{}
}

// GetCompiler returns a JavaCompiler instance
func (p *DefaultToolProvider) GetCompiler() JavaCompiler {
	if p.compiler == nil {
		p.compiler = NewDefaultJavaCompiler()
	}
	return p.compiler
}

// GetJarTool returns a JarTool instance
func (p *DefaultToolProvider) GetJarTool() JarTool {
	if p.jarTool == nil {
		p.jarTool = NewDefaultJarTool()
	}
	return p.jarTool
}

// GetRunner returns a JavaRunner instance
func (p *DefaultToolProvider) GetRunner() JavaRunner {
	if p.runner == nil {
		p.runner = NewDefaultJavaRunner()
	}
	return p.runner
}

// DetectJDK detects and returns information about the available JDK
func (p *DefaultToolProvider) DetectJDK() (*JDKInfo, error) {
	if p.jdkInfo != nil {
		return p.jdkInfo, nil
	}

	info := &JDKInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// First, check JAVA_HOME environment variable
	javaHome := os.Getenv("JAVA_HOME")
	if javaHome != "" {
		// Verify it's a valid JDK home
		javacPath := filepath.Join(javaHome, "bin", "javac")
		if runtime.GOOS == "windows" {
			javacPath += ".exe"
		}

		if _, err := os.Stat(javacPath); err == nil {
			info.Home = javaHome
		}
	}

	// If no valid JAVA_HOME, try to find JDK from javac in PATH
	if info.Home == "" {
		javacPath, err := exec.LookPath("javac")
		if err != nil {
			return nil, fmt.Errorf("JDK not found: no JAVA_HOME set and javac not in PATH")
		}

		// Try to determine JAVA_HOME from javac path
		// javac is typically at $JAVA_HOME/bin/javac
		binDir := filepath.Dir(javacPath)
		possibleHome := filepath.Dir(binDir)

		// Verify this looks like a JDK home
		if isValidJDKHome(possibleHome) {
			info.Home = possibleHome
		}
	}

	// Get version information
	runner := p.GetRunner()
	version, err := runner.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to get Java version: %w", err)
	}

	info.Version = version
	info.Vendor = version.Vendor

	p.jdkInfo = info
	return info, nil
}

// isValidJDKHome checks if a directory looks like a valid JDK home
func isValidJDKHome(path string) bool {
	// Check for typical JDK directories and files
	requiredPaths := []string{
		filepath.Join(path, "bin", "javac"),
		filepath.Join(path, "bin", "java"),
		filepath.Join(path, "bin", "jar"),
	}

	if runtime.GOOS == "windows" {
		for i, p := range requiredPaths {
			requiredPaths[i] = p + ".exe"
		}
	}

	for _, reqPath := range requiredPaths {
		if _, err := os.Stat(reqPath); err != nil {
			return false
		}
	}

	// Check for lib directory (contains tools.jar in older JDKs or modules in newer ones)
	libPath := filepath.Join(path, "lib")
	if _, err := os.Stat(libPath); err != nil {
		return false
	}

	return true
}

// Global default provider instance
var defaultProvider ToolProvider = NewDefaultToolProvider()

// GetDefaultToolProvider returns the global default tool provider
func GetDefaultToolProvider() ToolProvider {
	return defaultProvider
}

// SetDefaultToolProvider sets the global default tool provider (useful for testing)
func SetDefaultToolProvider(provider ToolProvider) {
	defaultProvider = provider
}

// Helper functions for common version comparisons

// IsJava8OrLater returns true if the version is Java 8 or later
func (v JavaVersion) IsJava8OrLater() bool {
	return v.Major >= 8
}

// IsJava11OrLater returns true if the version is Java 11 or later
func (v JavaVersion) IsJava11OrLater() bool {
	return v.Major >= 11
}

// IsJava17OrLater returns true if the version is Java 17 or later
func (v JavaVersion) IsJava17OrLater() bool {
	return v.Major >= 17
}

// IsJava21OrLater returns true if the version is Java 21 or later
func (v JavaVersion) IsJava21OrLater() bool {
	return v.Major >= 21
}

// String returns a string representation of the version
func (v JavaVersion) String() string {
	if v.Full != "" {
		return v.Full
	}
	if v.Patch > 0 {
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
	if v.Minor > 0 {
		return fmt.Sprintf("%d.%d", v.Major, v.Minor)
	}
	return fmt.Sprintf("%d", v.Major)
}

// CompareVersions compares two Java versions
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func CompareVersions(v1, v2 JavaVersion) int {
	if v1.Major != v2.Major {
		if v1.Major < v2.Major {
			return -1
		}
		return 1
	}

	if v1.Minor != v2.Minor {
		if v1.Minor < v2.Minor {
			return -1
		}
		return 1
	}

	if v1.Patch != v2.Patch {
		if v1.Patch < v2.Patch {
			return -1
		}
		return 1
	}

	return 0
}

// Platform-specific helpers

// GetJavaExecutable returns the platform-specific java executable name
func GetJavaExecutable() string {
	if runtime.GOOS == "windows" {
		return "java.exe"
	}
	return "java"
}

// GetJavacExecutable returns the platform-specific javac executable name
func GetJavacExecutable() string {
	if runtime.GOOS == "windows" {
		return "javac.exe"
	}
	return "javac"
}

// GetJarExecutable returns the platform-specific jar executable name
func GetJarExecutable() string {
	if runtime.GOOS == "windows" {
		return "jar.exe"
	}
	return "jar"
}

// NormalizePath normalizes a file path for the current platform
func NormalizePath(path string) string {
	// Convert forward slashes to the platform separator
	path = filepath.FromSlash(path)

	// Clean the path
	path = filepath.Clean(path)

	return path
}

// JoinClassPath joins multiple classpath entries with the platform-specific separator
func JoinClassPath(paths ...string) string {
	separator := ":"
	if runtime.GOOS == "windows" {
		separator = ";"
	}

	// Normalize all paths
	normalizedPaths := make([]string, len(paths))
	for i, p := range paths {
		normalizedPaths[i] = NormalizePath(p)
	}

	return strings.Join(normalizedPaths, separator)
}
