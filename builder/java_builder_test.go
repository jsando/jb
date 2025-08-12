package builder

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jsando/jb/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockBuildLog implements project.BuildLog for testing
type MockBuildLog struct {
	Errors   []string
	Warnings []string
	Tasks    []string
	failed   bool
}

func (m *MockBuildLog) Failed() bool {
	return m.failed
}

func (m *MockBuildLog) BuildStart() {}

func (m *MockBuildLog) BuildFinish() {}

func (m *MockBuildLog) TaskStart(name string) project.TaskLog {
	m.Tasks = append(m.Tasks, name)
	return &MockTaskLog{name: name, parent: m}
}

func (m *MockBuildLog) CheckError(context string, err error) bool {
	if err != nil {
		m.Errors = append(m.Errors, fmt.Sprintf("%s: %v", context, err))
		m.failed = true
		return true
	}
	return false
}

func (m *MockBuildLog) ModuleStart(name string) {
	m.Tasks = append(m.Tasks, fmt.Sprintf("module: %s", name))
}

type MockTaskLog struct {
	name   string
	parent *MockBuildLog
}

func (m *MockTaskLog) Done(err error) bool {
	if err != nil {
		m.parent.Errors = append(m.parent.Errors, fmt.Sprintf("%s: %v", m.name, err))
		m.parent.failed = true
		return true
	}
	return false
}

func (m *MockTaskLog) Info(msg string)  {}
func (m *MockTaskLog) Warn(msg string)  { m.parent.Warnings = append(m.parent.Warnings, msg) }
func (m *MockTaskLog) Error(msg string) { m.parent.Errors = append(m.parent.Errors, msg) }

func TestNewBuilder(t *testing.T) {
	logger := &MockBuildLog{}
	builder := NewBuilder(logger)

	assert.NotNil(t, builder)
	assert.NotNil(t, builder.repo)
	assert.Equal(t, logger, builder.logger)
	assert.NotNil(t, builder.toolProvider)
}

func TestNewBuilderWithTools(t *testing.T) {
	logger := &MockBuildLog{}
	mockProvider := &MockToolProvider{}
	builder := NewBuilderWithTools(logger, mockProvider)

	assert.NotNil(t, builder)
	assert.NotNil(t, builder.repo)
	assert.Equal(t, logger, builder.logger)
	assert.Equal(t, mockProvider, builder.toolProvider)
}

func TestClean(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()
	buildDir := filepath.Join(tempDir, "build")
	require.NoError(t, os.MkdirAll(buildDir, 0755))

	// Create a test file in build directory
	testFile := filepath.Join(buildDir, "test.class")
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	// Create module
	module := &project.Module{
		ModuleDirAbs: tempDir,
	}

	// Create builder and clean
	logger := &MockBuildLog{}
	builder := NewBuilder(logger)
	builder.Clean(module)

	// Verify build directory was removed
	_, err := os.Stat(buildDir)
	assert.True(t, os.IsNotExist(err))
	assert.Contains(t, logger.Tasks, "cleaning build dir")
}

func TestGetModuleJarPath(t *testing.T) {
	module := &project.Module{
		ModuleDirAbs: "/test/project",
		Name:         "mymodule",
		Version:      "1.0.0",
	}

	logger := &MockBuildLog{}
	builder := NewBuilder(logger)
	jarPath := builder.getModuleJarPath(module)

	expected := filepath.Join("/test/project", "build", "mymodule-1.0.0.jar")
	assert.Equal(t, expected, jarPath)
}

func TestCompileJava_Success(t *testing.T) {
	// Setup
	mockCompiler := &MockJavaCompiler{
		IsAvailableFunc: func() bool { return true },
		VersionFunc: func() (JavaVersion, error) {
			return JavaVersion{Major: 17, Minor: 0, Patch: 1}, nil
		},
		CompileFunc: func(args CompileArgs) (CompileResult, error) {
			return CompileResult{
				Success:      true,
				WarningCount: 0,
				ErrorCount:   0,
			}, nil
		},
	}
	mockProvider := &MockToolProvider{
		Compiler: mockCompiler,
	}

	logger := &MockBuildLog{}
	taskLog := &MockTaskLog{name: "compile", parent: logger}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: "/test/project",
	}

	sourceFiles := []project.SourceFileInfo{
		{Path: "src/Main.java"},
		{Path: "src/Helper.java"},
	}

	// Execute
	err := builder.compileJava(module, taskLog, "/build/tmp", "/build/classes", "lib.jar", []string{"-g"}, sourceFiles)

	// Verify
	assert.NoError(t, err)
	assert.Len(t, mockCompiler.CompileCalls, 1)

	call := mockCompiler.CompileCalls[0]
	assert.Equal(t, []string{"src/Main.java", "src/Helper.java"}, call.SourceFiles)
	assert.Equal(t, "lib.jar", call.ClassPath)
	assert.Equal(t, "/build/classes", call.DestDir)
	assert.Equal(t, []string{"-g"}, call.ExtraFlags)
	assert.Equal(t, "/test/project", call.WorkDir)
}

func TestCompileJava_CompilerNotAvailable(t *testing.T) {
	// Setup
	mockCompiler := &MockJavaCompiler{
		IsAvailableFunc: func() bool { return false },
	}
	mockProvider := &MockToolProvider{
		Compiler: mockCompiler,
	}

	logger := &MockBuildLog{}
	taskLog := &MockTaskLog{name: "compile", parent: logger}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: "/test/project",
	}

	// Execute
	err := builder.compileJava(module, taskLog, "", "", "", nil, nil)

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "java compiler (javac) not found")
}

func TestCompileJava_WithWarnings(t *testing.T) {
	// Setup
	mockCompiler := &MockJavaCompiler{
		IsAvailableFunc: func() bool { return true },
		CompileFunc: func(args CompileArgs) (CompileResult, error) {
			return CompileResult{
				Success:      true,
				WarningCount: 2,
				Warnings: []CompileWarning{
					{File: "Main.java", Line: 10, Column: 5, Message: "deprecated method"},
					{Message: "unchecked cast"},
				},
			}, nil
		},
	}
	mockProvider := &MockToolProvider{
		Compiler: mockCompiler,
	}

	logger := &MockBuildLog{}
	taskLog := &MockTaskLog{name: "compile", parent: logger}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: "/test/project",
	}

	// Execute
	err := builder.compileJava(module, taskLog, "", "", "", nil, []project.SourceFileInfo{{Path: "Main.java"}})

	// Verify
	assert.NoError(t, err)
	assert.Contains(t, logger.Warnings, "Compilation completed with 2 warning(s)")
	assert.Contains(t, logger.Warnings, "Main.java:10:5: deprecated method")
	assert.Contains(t, logger.Warnings, "unchecked cast")
}

func TestCompileJava_WithErrors(t *testing.T) {
	// Setup
	mockCompiler := &MockJavaCompiler{
		IsAvailableFunc: func() bool { return true },
		CompileFunc: func(args CompileArgs) (CompileResult, error) {
			return CompileResult{
				Success:    false,
				ErrorCount: 2,
				Errors: []CompileError{
					{File: "Main.java", Line: 15, Column: 10, Message: "cannot find symbol"},
					{Message: "package does not exist"},
				},
			}, nil
		},
	}
	mockProvider := &MockToolProvider{
		Compiler: mockCompiler,
	}

	logger := &MockBuildLog{}
	taskLog := &MockTaskLog{name: "compile", parent: logger}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: "/test/project",
	}

	// Execute
	err := builder.compileJava(module, taskLog, "", "", "", nil, []project.SourceFileInfo{{Path: "Main.java"}})

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compilation failed with 2 error(s)")
	assert.Contains(t, logger.Errors, "Main.java:15:10: cannot find symbol")
	assert.Contains(t, logger.Errors, "package does not exist")
}

func TestBuildJar_Success(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	buildDir := filepath.Join(tempDir, "build")
	buildClasses := filepath.Join(buildDir, "classes")
	require.NoError(t, os.MkdirAll(buildClasses, 0755))

	// Create a fake dependency jar
	libDir := filepath.Join(tempDir, "lib")
	require.NoError(t, os.MkdirAll(libDir, 0755))
	depJar := filepath.Join(libDir, "dep.jar")
	require.NoError(t, os.WriteFile(depJar, []byte("fake jar content"), 0644))

	mockJar := &MockJarTool{
		IsAvailableFunc: func() bool { return true },
	}
	mockProvider := &MockToolProvider{
		JarTool: mockJar,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: tempDir,
		Name:         "test-module",
		Version:      "1.0.0",
	}

	// Execute
	err := builder.buildJar(module, buildDir, "", "com.example.Main", []string{depJar}, "", buildClasses)

	// Verify
	assert.NoError(t, err)
	assert.Len(t, mockJar.CreateCalls, 1)

	call := mockJar.CreateCalls[0]
	expectedJar := filepath.Join(buildDir, "test-module-1.0.0.jar")
	assert.Equal(t, expectedJar, call.JarFile)
	assert.Equal(t, buildClasses, call.BaseDir)
	assert.Equal(t, []string{"."}, call.Files)
	assert.Equal(t, "com.example.Main", call.MainClass)
	assert.Contains(t, call.ClassPath, "dep.jar")
	assert.Equal(t, tempDir, call.WorkDir)
}

func TestBuildJar_NotAvailable(t *testing.T) {
	mockJar := &MockJarTool{
		IsAvailableFunc: func() bool { return false },
	}
	mockProvider := &MockToolProvider{
		JarTool: mockJar,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{}

	err := builder.buildJar(module, "", "", "", nil, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JAR tool not found")
}

func TestRun_Success(t *testing.T) {
	mockRunner := &MockJavaRunner{}
	mockProvider := &MockToolProvider{
		Runner: mockRunner,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: "/test/project",
		Name:         "myapp",
		Version:      "1.0.0",
	}

	progArgs := []string{"arg1", "arg2"}

	// Execute
	err := builder.Run(module, progArgs)

	// Verify
	assert.NoError(t, err)
	assert.Len(t, mockRunner.RunCalls, 1)

	call := mockRunner.RunCalls[0]
	expectedJar := filepath.Join("/test/project", "build", "myapp-1.0.0.jar")
	assert.Equal(t, expectedJar, call.JarFile)
	assert.Equal(t, progArgs, call.ProgramArgs)
	assert.Equal(t, "/test/project", call.WorkDir)
}

func TestRunTest_Success(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	buildDir := filepath.Join(tempDir, "build", "tmp", "classes")
	require.NoError(t, os.MkdirAll(buildDir, 0755))

	mockRunner := &MockJavaRunner{}
	mockProvider := &MockToolProvider{
		Runner: mockRunner,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: tempDir,
		Dependencies: []*project.Dependency{
			{Group: "org.junit.jupiter", Artifact: "junit-jupiter", Version: "5.9.0"},
		},
	}

	// Execute
	builder.RunTest(module)

	// Verify
	assert.Contains(t, logger.Tasks, "Running tests")
	assert.Len(t, mockRunner.RunCalls, 1)

	call := mockRunner.RunCalls[0]
	assert.Equal(t, "org.junit.platform.console.ConsoleLauncher", call.MainClass)
	assert.Contains(t, call.ProgramArgs, "execute")
	assert.Contains(t, call.ProgramArgs, "--scan-classpath")
	assert.Equal(t, tempDir, call.WorkDir)
}

func TestRunTest_NoFrameworkDetected(t *testing.T) {
	logger := &MockBuildLog{}
	builder := NewBuilder(logger)

	module := &project.Module{
		Dependencies: []*project.Dependency{},
	}

	builder.RunTest(module)

	assert.Contains(t, logger.Errors, "Test framework not detected (only junit4 and junit5 are currently supported)")
}

func TestDetectTestFramework(t *testing.T) {
	logger := &MockBuildLog{}
	builder := NewBuilder(logger)

	tests := []struct {
		name         string
		dependencies []*project.Dependency
		expected     string
	}{
		{
			name: "junit4",
			dependencies: []*project.Dependency{
				{Group: "junit", Artifact: "junit", Version: "4.13.2"},
			},
			expected: "junit",
		},
		{
			name: "junit5",
			dependencies: []*project.Dependency{
				{Group: "org.junit.jupiter", Artifact: "junit-jupiter", Version: "5.9.0"},
			},
			expected: "junit",
		},
		{
			name: "no framework",
			dependencies: []*project.Dependency{
				{Group: "org.slf4j", Artifact: "slf4j-api", Version: "1.7.36"},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			module := &project.Module{
				Dependencies: tt.dependencies,
			}
			result := builder.detectTestFramework(module)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBuildDependencies(t *testing.T) {
	logger := &MockBuildLog{}
	builder := NewBuilder(logger)

	// Create a simple module with dependencies
	module := &project.Module{
		Dependencies: []*project.Dependency{
			{
				Group:    "org.example",
				Artifact: "lib1",
				Version:  "1.0.0",
				Path:     "/repo/lib1-1.0.0.jar",
			},
			{
				Group:    "org.example",
				Artifact: "lib2",
				Version:  "2.0.0",
				Path:     "/repo/lib2-2.0.0.jar",
				Transitive: []*project.Dependency{
					{
						Group:    "org.example",
						Artifact: "lib3",
						Version:  "3.0.0",
						Path:     "/repo/lib3-3.0.0.jar",
					},
				},
			},
		},
	}

	// Execute
	deps, err := builder.getBuildDependencies(module)

	// Verify
	assert.NoError(t, err)
	assert.Len(t, deps, 3)
	assert.Contains(t, deps, "/repo/lib1-1.0.0.jar")
	assert.Contains(t, deps, "/repo/lib2-2.0.0.jar")
	assert.Contains(t, deps, "/repo/lib3-3.0.0.jar")
}

func TestBuild_UpToDate(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	buildDir := filepath.Join(tempDir, "build")
	sourceDir := filepath.Join(tempDir, "src")
	require.NoError(t, os.MkdirAll(buildDir, 0755))
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// Create module with consistent content
	moduleContent := []byte("test module content")
	module := &project.Module{
		ModuleDirAbs:    tempDir,
		SourceDirAbs:    sourceDir,
		ResourceDirAbs:  filepath.Join(tempDir, "resources"),
		Name:            "test",
		Version:         "1.0.0",
		ModuleFileBytes: moduleContent,
		Resources:       []string{}, // No resources
	}

	// Calculate the expected hash (same logic as in Build method)
	hasher := sha1.New()
	hasher.Write(moduleContent)
	// No sources or resources, so hash is just from module content
	expectedHash := hex.EncodeToString(hasher.Sum(nil))

	// Write the hash file with the expected hash
	hashFile := filepath.Join(buildDir, buildHashFile)
	require.NoError(t, project.WriteFile(hashFile, expectedHash))

	mockProvider := &MockToolProvider{}
	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	// Execute - should detect up to date
	builder.Build(module)

	// Verify - should see "up to date" task
	assert.Contains(t, logger.Tasks, "up to date")
	// Compiler should not be called
	mockCompiler := mockProvider.GetCompiler().(*MockJavaCompiler)
	assert.Len(t, mockCompiler.CompileCalls, 0)
}

func TestBuild_WithSources(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "src", "main", "java")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// Create a Java source file
	javaFile := filepath.Join(sourceDir, "Main.java")
	require.NoError(t, os.WriteFile(javaFile, []byte("public class Main {}"), 0644))

	// Setup mocks
	mockCompiler := &MockJavaCompiler{
		IsAvailableFunc: func() bool { return true },
		VersionFunc: func() (JavaVersion, error) {
			return JavaVersion{Major: 17}, nil
		},
		CompileFunc: func(args CompileArgs) (CompileResult, error) {
			return CompileResult{Success: true}, nil
		},
	}

	mockJar := &MockJarTool{
		IsAvailableFunc: func() bool { return true },
	}

	mockProvider := &MockToolProvider{
		Compiler: mockCompiler,
		JarTool:  mockJar,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs:    tempDir,
		SourceDirAbs:    filepath.Join(tempDir, "src", "main", "java"),
		ResourceDirAbs:  filepath.Join(tempDir, "src", "main", "resources"),
		Name:            "test-project",
		Version:         "1.0.0",
		ModuleFileBytes: []byte("module content"),
		Resources:       []string{},
	}

	// Execute
	builder.Build(module)

	// Verify
	assert.False(t, logger.failed, "Build should not have failed")
	assert.Len(t, mockCompiler.CompileCalls, 1, "Compiler should have been called once")
	assert.Len(t, mockJar.CreateCalls, 1, "Jar tool should have been called once")

	// Verify jar was created
	jarPath := filepath.Join(tempDir, "build", "test-project-1.0.0.jar")
	assert.Equal(t, jarPath, mockJar.CreateCalls[0].JarFile)
}

func TestBuild_WithResources(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "src", "main", "java")
	resourceDir := filepath.Join(tempDir, "src", "main", "resources")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))
	require.NoError(t, os.MkdirAll(resourceDir, 0755))

	// Create a resource file
	propFile := filepath.Join(resourceDir, "app.properties")
	require.NoError(t, os.WriteFile(propFile, []byte("key=value"), 0644))

	// Setup mocks
	mockJar := &MockJarTool{
		IsAvailableFunc: func() bool { return true },
	}

	mockProvider := &MockToolProvider{
		JarTool: mockJar,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs:    tempDir,
		SourceDirAbs:    sourceDir,
		ResourceDirAbs:  resourceDir,
		Name:            "test-project",
		Version:         "1.0.0",
		ModuleFileBytes: []byte("module content"),
		Resources:       []string{"*.properties"},
	}

	// Execute
	builder.Build(module)

	// Verify
	assert.False(t, logger.failed, "Build should not have failed")

	// Check that the resource file was copied to build/tmp/classes
	expectedDest := filepath.Join(tempDir, "build", "tmp", "classes", "app.properties")
	_, err := os.Stat(expectedDest)
	assert.NoError(t, err, "Resource file should have been copied")
}

func TestBuild_CompilationFailure(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "src")
	require.NoError(t, os.MkdirAll(sourceDir, 0755))

	// Create a Java source file
	javaFile := filepath.Join(sourceDir, "BadCode.java")
	require.NoError(t, os.WriteFile(javaFile, []byte("public class BadCode { syntax error }"), 0644))

	// Setup mocks
	mockCompiler := &MockJavaCompiler{
		IsAvailableFunc: func() bool { return true },
		CompileFunc: func(args CompileArgs) (CompileResult, error) {
			return CompileResult{
				Success:    false,
				ErrorCount: 1,
				Errors: []CompileError{
					{File: "BadCode.java", Line: 1, Column: 23, Message: "syntax error"},
				},
			}, nil
		},
	}

	mockProvider := &MockToolProvider{
		Compiler: mockCompiler,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs:    tempDir,
		SourceDirAbs:    sourceDir,
		ResourceDirAbs:  filepath.Join(tempDir, "resources"),
		Name:            "bad-project",
		Version:         "1.0.0",
		ModuleFileBytes: []byte("module content"),
		Resources:       []string{},
	}

	// Execute
	builder.Build(module)

	// Verify
	assert.True(t, logger.failed, "Build should have failed")
	assert.Contains(t, logger.Errors, "compile java sources: compilation failed with 1 error(s)")

	// Jar should not have been created
	mockJar := mockProvider.GetJarTool().(*MockJarTool)
	assert.Len(t, mockJar.CreateCalls, 0, "Jar tool should not have been called")
}

func TestPublish(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	buildDir := filepath.Join(tempDir, "build")
	require.NoError(t, os.MkdirAll(buildDir, 0755))

	// Create fake jar and pom files
	jarPath := filepath.Join(buildDir, "test-myapp-1.0.0-SNAPSHOT.jar")
	pomPath := filepath.Join(buildDir, "test-myapp-1.0.0-SNAPSHOT.pom")
	require.NoError(t, os.WriteFile(jarPath, []byte("fake jar"), 0644))
	require.NoError(t, os.WriteFile(pomPath, []byte("<project></project>"), 0644))

	logger := &MockBuildLog{}
	builder := NewBuilder(logger)

	module := &project.Module{
		ModuleDirAbs: tempDir,
		Group:        "com.example.test",
		Name:         "test-myapp",
		Version:      "1.0.0-SNAPSHOT",
	}

	// Execute
	err := builder.Publish(module, "https://repo.example.com", "user", "pass")

	// For now, this just tests that it doesn't panic
	// In a real test, we'd mock the maven repository
	assert.NoError(t, err)
}

func TestResolveDependencies_CircularDependency(t *testing.T) {
	logger := &MockBuildLog{}
	builder := NewBuilder(logger)

	// Create modules with circular dependency - already resolved with paths
	dep1 := &project.Dependency{
		Coordinates: "com.example:lib1:1.0",
		Group:       "com.example",
		Artifact:    "lib1",
		Version:     "1.0",
		Path:        "/fake/path/lib1.jar", // Already resolved
		Transitive:  []*project.Dependency{},
	}

	dep2 := &project.Dependency{
		Coordinates: "com.example:lib2:1.0",
		Group:       "com.example",
		Artifact:    "lib2",
		Version:     "1.0",
		Path:        "/fake/path/lib2.jar", // Already resolved
		Transitive:  []*project.Dependency{dep1},
	}

	// Create circular reference
	dep1.Transitive = []*project.Dependency{dep2}

	module := &project.Module{
		Dependencies: []*project.Dependency{dep1},
	}

	// Execute - should handle circular dependency gracefully
	err := builder.ResolveDependencies(module)
	assert.NoError(t, err, "Should handle circular dependencies without error")
}

func TestBuildJar_ExecutableJar(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	buildDir := filepath.Join(tempDir, "build")
	buildClasses := filepath.Join(buildDir, "classes")
	require.NoError(t, os.MkdirAll(buildClasses, 0755))

	// Create dependency jars
	dep1 := filepath.Join(tempDir, "lib1.jar")
	dep2 := filepath.Join(tempDir, "lib2.jar")
	require.NoError(t, os.WriteFile(dep1, []byte("jar1"), 0644))
	require.NoError(t, os.WriteFile(dep2, []byte("jar2"), 0644))

	mockJar := &MockJarTool{
		IsAvailableFunc: func() bool { return true },
	}
	mockProvider := &MockToolProvider{
		JarTool: mockJar,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: tempDir,
		Name:         "executable-app",
		Version:      "1.0.0",
		MainClass:    "com.example.App",
	}

	// Execute
	err := builder.buildJar(module, buildDir, "", "com.example.App", []string{dep1, dep2}, "", buildClasses)

	// Verify
	assert.NoError(t, err)
	assert.Len(t, mockJar.CreateCalls, 1)

	call := mockJar.CreateCalls[0]
	assert.Equal(t, "com.example.App", call.MainClass)
	assert.Contains(t, call.ClassPath, "lib1.jar")
	assert.Contains(t, call.ClassPath, "lib2.jar")

	// Verify dependency jars were copied to build dir
	assert.FileExists(t, filepath.Join(buildDir, "lib1.jar"))
	assert.FileExists(t, filepath.Join(buildDir, "lib2.jar"))
}

func TestRun_Error(t *testing.T) {
	mockRunner := &MockJavaRunner{
		RunFunc: func(args RunArgs) error {
			return errors.New("application crashed")
		},
	}
	mockProvider := &MockToolProvider{
		Runner: mockRunner,
	}

	logger := &MockBuildLog{}
	builder := NewBuilderWithTools(logger, mockProvider)

	module := &project.Module{
		ModuleDirAbs: "/test",
		Name:         "crashy-app",
		Version:      "1.0.0",
	}

	// Execute
	err := builder.Run(module, []string{})

	// Verify
	assert.Error(t, err)
	assert.Equal(t, "application crashed", err.Error())
}

func TestGetBuildDependencies_VersionConflict(t *testing.T) {
	logger := &MockBuildLog{}
	builder := NewBuilder(logger)

	// Create dependencies with version conflict
	module := &project.Module{
		Dependencies: []*project.Dependency{
			{
				Group:    "org.lib",
				Artifact: "common",
				Version:  "1.0.0",
				Path:     "/repo/common-1.0.0.jar",
			},
			{
				Group:    "org.app",
				Artifact: "app",
				Version:  "2.0.0",
				Path:     "/repo/app-2.0.0.jar",
				Transitive: []*project.Dependency{
					{
						Group:    "org.lib",
						Artifact: "common",
						Version:  "2.0.0", // Different version
						Path:     "/repo/common-2.0.0.jar",
					},
				},
			},
		},
	}

	// Execute
	deps, err := builder.getBuildDependencies(module)

	// Verify - should use first version encountered (1.0.0)
	assert.NoError(t, err)
	assert.Contains(t, deps, "/repo/common-1.0.0.jar")
	assert.NotContains(t, deps, "/repo/common-2.0.0.jar")
	assert.Contains(t, deps, "/repo/app-2.0.0.jar")
}
