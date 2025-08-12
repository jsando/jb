package builder

import (
	"io"
	"time"
)

// JavaVersion represents a Java version with major, minor, and patch components
type JavaVersion struct {
	Major  int
	Minor  int
	Patch  int
	Full   string // Full version string as reported by the tool
	Vendor string // OpenJDK, Oracle, etc.
}

// CompileArgs represents arguments for Java compilation
type CompileArgs struct {
	SourceFiles   []string
	ClassPath     string
	DestDir       string
	SourceVersion string // e.g., "8", "11", "17"
	TargetVersion string // e.g., "8", "11", "17"
	ExtraFlags    []string
	WorkDir       string // Working directory for the compilation
}

// CompileResult represents the result of a compilation
type CompileResult struct {
	Success      bool
	ErrorCount   int
	WarningCount int
	Errors       []CompileError
	Warnings     []CompileWarning
	RawOutput    string // Raw compiler output for debugging
}

// CompileError represents a compilation error
type CompileError struct {
	File    string
	Line    int
	Column  int
	Message string
	Code    string // Error code if available
}

// CompileWarning represents a compilation warning
type CompileWarning struct {
	File    string
	Line    int
	Column  int
	Message string
	Code    string // Warning code if available
}

// JarArgs represents arguments for creating a JAR file
type JarArgs struct {
	JarFile      string
	BaseDir      string   // Directory to change to before adding files
	Files        []string // Files/directories to include (relative to BaseDir)
	MainClass    string   // Main class for executable JARs
	ClassPath    []string // Class-Path entries for manifest
	ManifestFile string   // Custom manifest file
	Date         string   // Creation date (for reproducible builds)
	WorkDir      string   // Working directory
}

// RunArgs represents arguments for running a Java program
type RunArgs struct {
	MainClass   string   // Either main class or -jar jarfile
	JarFile     string   // JAR file to run (if applicable)
	ClassPath   string   // Classpath
	JvmArgs     []string // JVM arguments (e.g., -Xmx512m)
	ProgramArgs []string // Arguments to pass to the program
	WorkDir     string   // Working directory
	Env         []string // Environment variables
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
}

// JavaCompiler is the interface for Java compilation
type JavaCompiler interface {
	// Compile compiles Java source files
	Compile(args CompileArgs) (CompileResult, error)

	// Version returns the compiler version information
	Version() (JavaVersion, error)

	// IsAvailable checks if the compiler is available on the system
	IsAvailable() bool
}

// JarTool is the interface for JAR file operations
type JarTool interface {
	// Create creates a new JAR file
	Create(args JarArgs) error

	// Extract extracts files from a JAR
	Extract(jarFile, destDir string) error

	// List lists the contents of a JAR file
	List(jarFile string) ([]string, error)

	// Update adds or updates files in an existing JAR
	Update(jarFile string, files map[string]string) error

	// Version returns the tool version information
	Version() (JavaVersion, error)

	// IsAvailable checks if the tool is available on the system
	IsAvailable() bool
}

// JavaRunner is the interface for running Java programs
type JavaRunner interface {
	// Run executes a Java program
	Run(args RunArgs) error

	// RunWithTimeout executes a Java program with a timeout
	RunWithTimeout(args RunArgs, timeout time.Duration) error

	// Version returns the Java runtime version information
	Version() (JavaVersion, error)

	// IsAvailable checks if Java runtime is available on the system
	IsAvailable() bool
}

// ToolProvider provides access to Java development tools
type ToolProvider interface {
	// GetCompiler returns a JavaCompiler instance
	GetCompiler() JavaCompiler

	// GetJarTool returns a JarTool instance
	GetJarTool() JarTool

	// GetRunner returns a JavaRunner instance
	GetRunner() JavaRunner

	// DetectJDK detects and returns information about the available JDK
	DetectJDK() (*JDKInfo, error)
}

// JDKInfo represents information about an installed JDK
type JDKInfo struct {
	Version JavaVersion
	Home    string // JAVA_HOME path
	Vendor  string
	Arch    string // Architecture (x64, aarch64, etc.)
	OS      string // Operating system
}
