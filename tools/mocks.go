package tools

import (
	"fmt"
	"io"
	"time"
)

// MockJavaCompiler is a mock implementation of JavaCompiler for testing
type MockJavaCompiler struct {
	CompileFunc     func(args CompileArgs) (CompileResult, error)
	VersionFunc     func() (JavaVersion, error)
	IsAvailableFunc func() bool

	// Record of calls
	CompileCalls     []CompileArgs
	VersionCalls     int
	IsAvailableCalls int
}

func (m *MockJavaCompiler) Compile(args CompileArgs) (CompileResult, error) {
	m.CompileCalls = append(m.CompileCalls, args)
	if m.CompileFunc != nil {
		return m.CompileFunc(args)
	}
	return CompileResult{Success: true}, nil
}

func (m *MockJavaCompiler) Version() (JavaVersion, error) {
	m.VersionCalls++
	if m.VersionFunc != nil {
		return m.VersionFunc()
	}
	return JavaVersion{Major: 17, Minor: 0, Patch: 1, Vendor: "Mock"}, nil
}

func (m *MockJavaCompiler) IsAvailable() bool {
	m.IsAvailableCalls++
	if m.IsAvailableFunc != nil {
		return m.IsAvailableFunc()
	}
	return true
}

// MockJarTool is a mock implementation of JarTool for testing
type MockJarTool struct {
	CreateFunc      func(args JarArgs) error
	ExtractFunc     func(jarFile, destDir string) error
	ListFunc        func(jarFile string) ([]string, error)
	UpdateFunc      func(jarFile string, files map[string]string) error
	VersionFunc     func() (JavaVersion, error)
	IsAvailableFunc func() bool

	// Record of calls
	CreateCalls  []JarArgs
	ExtractCalls []struct{ JarFile, DestDir string }
	ListCalls    []string
	UpdateCalls  []struct {
		JarFile string
		Files   map[string]string
	}
	VersionCalls     int
	IsAvailableCalls int
}

func (m *MockJarTool) Create(args JarArgs) error {
	m.CreateCalls = append(m.CreateCalls, args)
	if m.CreateFunc != nil {
		return m.CreateFunc(args)
	}
	return nil
}

func (m *MockJarTool) Extract(jarFile, destDir string) error {
	m.ExtractCalls = append(m.ExtractCalls, struct{ JarFile, DestDir string }{jarFile, destDir})
	if m.ExtractFunc != nil {
		return m.ExtractFunc(jarFile, destDir)
	}
	return nil
}

func (m *MockJarTool) List(jarFile string) ([]string, error) {
	m.ListCalls = append(m.ListCalls, jarFile)
	if m.ListFunc != nil {
		return m.ListFunc(jarFile)
	}
	return []string{"META-INF/", "META-INF/MANIFEST.MF", "com/", "com/example/", "com/example/Main.class"}, nil
}

func (m *MockJarTool) Update(jarFile string, files map[string]string) error {
	m.UpdateCalls = append(m.UpdateCalls, struct {
		JarFile string
		Files   map[string]string
	}{jarFile, files})
	if m.UpdateFunc != nil {
		return m.UpdateFunc(jarFile, files)
	}
	return nil
}

func (m *MockJarTool) Version() (JavaVersion, error) {
	m.VersionCalls++
	if m.VersionFunc != nil {
		return m.VersionFunc()
	}
	return JavaVersion{Major: 17, Minor: 0, Patch: 1, Vendor: "Mock"}, nil
}

func (m *MockJarTool) IsAvailable() bool {
	m.IsAvailableCalls++
	if m.IsAvailableFunc != nil {
		return m.IsAvailableFunc()
	}
	return true
}

// MockJavaRunner is a mock implementation of JavaRunner for testing
type MockJavaRunner struct {
	RunFunc            func(args RunArgs) error
	RunWithTimeoutFunc func(args RunArgs, timeout time.Duration) error
	VersionFunc        func() (JavaVersion, error)
	IsAvailableFunc    func() bool

	// Record of calls
	RunCalls            []RunArgs
	RunWithTimeoutCalls []struct {
		Args    RunArgs
		Timeout time.Duration
	}
	VersionCalls     int
	IsAvailableCalls int
}

func (m *MockJavaRunner) Run(args RunArgs) error {
	m.RunCalls = append(m.RunCalls, args)
	if m.RunFunc != nil {
		return m.RunFunc(args)
	}
	return nil
}

func (m *MockJavaRunner) RunWithTimeout(args RunArgs, timeout time.Duration) error {
	m.RunWithTimeoutCalls = append(m.RunWithTimeoutCalls, struct {
		Args    RunArgs
		Timeout time.Duration
	}{args, timeout})
	if m.RunWithTimeoutFunc != nil {
		return m.RunWithTimeoutFunc(args, timeout)
	}
	return nil
}

func (m *MockJavaRunner) Version() (JavaVersion, error) {
	m.VersionCalls++
	if m.VersionFunc != nil {
		return m.VersionFunc()
	}
	return JavaVersion{Major: 17, Minor: 0, Patch: 1, Vendor: "Mock"}, nil
}

func (m *MockJavaRunner) IsAvailable() bool {
	m.IsAvailableCalls++
	if m.IsAvailableFunc != nil {
		return m.IsAvailableFunc()
	}
	return true
}

// MockToolProvider is a mock implementation of ToolProvider for testing
type MockToolProvider struct {
	Compiler JavaCompiler
	JarTool  JarTool
	Runner   JavaRunner
	JDKInfo  *JDKInfo
	JDKError error
}

func (m *MockToolProvider) GetCompiler() JavaCompiler {
	if m.Compiler != nil {
		return m.Compiler
	}
	return &MockJavaCompiler{}
}

func (m *MockToolProvider) GetJarTool() JarTool {
	if m.JarTool != nil {
		return m.JarTool
	}
	return &MockJarTool{}
}

func (m *MockToolProvider) GetRunner() JavaRunner {
	if m.Runner != nil {
		return m.Runner
	}
	return &MockJavaRunner{}
}

func (m *MockToolProvider) DetectJDK() (*JDKInfo, error) {
	if m.JDKError != nil {
		return nil, m.JDKError
	}
	if m.JDKInfo != nil {
		return m.JDKInfo, nil
	}
	return &JDKInfo{
		Version: JavaVersion{Major: 17, Minor: 0, Patch: 1, Vendor: "Mock"},
		Home:    "/mock/java/home",
		Vendor:  "Mock",
		Arch:    "x64",
		OS:      "mock",
	}, nil
}

// Helper functions for creating pre-configured mocks

// NewSuccessfulCompilerMock creates a mock compiler that always succeeds
func NewSuccessfulCompilerMock() *MockJavaCompiler {
	return &MockJavaCompiler{
		CompileFunc: func(args CompileArgs) (CompileResult, error) {
			return CompileResult{
				Success:      true,
				ErrorCount:   0,
				WarningCount: 0,
			}, nil
		},
	}
}

// NewFailingCompilerMock creates a mock compiler that always fails with the given errors
func NewFailingCompilerMock(errors []CompileError) *MockJavaCompiler {
	return &MockJavaCompiler{
		CompileFunc: func(args CompileArgs) (CompileResult, error) {
			return CompileResult{
				Success:    false,
				ErrorCount: len(errors),
				Errors:     errors,
			}, nil
		},
	}
}

// NewSuccessfulJarToolMock creates a mock jar tool that always succeeds
func NewSuccessfulJarToolMock() *MockJarTool {
	return &MockJarTool{}
}

// NewSuccessfulRunnerMock creates a mock runner that always succeeds
func NewSuccessfulRunnerMock() *MockJavaRunner {
	return &MockJavaRunner{}
}

// CaptureWriter is a simple io.Writer that captures written data
type CaptureWriter struct {
	Data []byte
}

func (w *CaptureWriter) Write(p []byte) (n int, err error) {
	w.Data = append(w.Data, p...)
	return len(p), nil
}

func (w *CaptureWriter) String() string {
	return string(w.Data)
}

// ErrorWriter is an io.Writer that always returns an error
type ErrorWriter struct {
	Err error
}

func (w *ErrorWriter) Write(p []byte) (n int, err error) {
	if w.Err != nil {
		return 0, w.Err
	}
	return 0, fmt.Errorf("write error")
}

// ErrorReader is an io.Reader that always returns an error
type ErrorReader struct {
	Err error
}

func (r *ErrorReader) Read(p []byte) (n int, err error) {
	if r.Err != nil {
		return 0, r.Err
	}
	return 0, io.EOF
}
