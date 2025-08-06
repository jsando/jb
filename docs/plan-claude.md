# Strategic Testing Plan for jb

## Overview

For a build tool to be fun to use, it needs to be very thoroughly tested. This document outlines a comprehensive strategy for improving jb's testing infrastructure and cross-platform reliability.

## Current State
- Limited unit tests (only in `builder` and `maven` packages)
- Integration tests exist but are basic (in `tests/` directory)
- No cross-platform testing
- No JDK version matrix testing
- Limited error handling and reporting

## Key Challenges
1. **External Dependencies**: javac, jar, java, Maven repositories
2. **Platform Variations**: Path handling, file separators, executable extensions
3. **JDK Version Differences**: Different flags, behaviors, and features across Java 8, 11, 17, 21, etc.

## Recommended Strategy

### 1. Testing Architecture
```
jb/
├── internal_test/     # Unit tests with mocks
├── integration/       # Integration test suite (separate module)
│   ├── fixtures/      # Test projects
│   ├── scenarios/     # Test scenarios
│   └── matrix/        # Platform/JDK specific tests
└── .github/
    └── workflows/     # CI/CD matrix testing
```

**Rationale**: Keep integration tests as a subfolder rather than separate repo to:
- Maintain version synchronization
- Simplify contribution process
- Enable atomic commits that include both code and test changes

### 2. Abstraction Layer for External Tools

Create interfaces for external tool interaction:

```go
type JavaCompiler interface {
    Compile(args CompileArgs) (CompileResult, error)
    Version() (JavaVersion, error)
}

type JarTool interface {
    Create(args JarArgs) error
    Extract(jar, dest string) error
    List(jar string) ([]string, error)
}

type JavaRunner interface {
    Run(args RunArgs) error
    Version() (JavaVersion, error)
}
```

This enables:
- Unit testing with mocks
- Platform-specific implementations
- Better error handling and normalization
- Version-specific behavior handling

### 3. CI/CD Matrix Strategy

Start with GitHub Actions matrix:
```yaml
strategy:
  matrix:
    os: [ubuntu-latest, windows-latest, macos-latest]
    java: [8, 11, 17, 21]  # LTS versions
```

### 5. Phased Implementation Plan

#### Completed Work

**Abstraction Layer (December 2024)**
- Created `tools` package with interfaces for JavaCompiler, JarTool, JavaRunner, and ToolProvider
- Implemented default implementations that wrap system commands
- Added mock implementations for testing
- Updated Java builder to use abstractions instead of direct exec.Command calls
- Improved error handling with structured CompileResult containing parsed errors/warnings
- Added platform-specific helpers and JDK detection
- All existing tests pass with the new implementation

**GitHub Actions CI/CD (December 2024)**
- Created comprehensive CI workflow with matrix testing
- Tests run on Ubuntu, Windows, and macOS
- Tests run against Java LTS versions: 8, 11, 17, 21
- Added linting (go fmt, go vet, staticcheck)
- Added test coverage reporting
- Created release workflow for automated binary builds
- Integration tests run actual jb builds on all platforms

#### Phase 1: Foundation (2-3 weeks)
- ✅ Create abstraction interfaces for external tools (COMPLETED)
- ✅ Add comprehensive unit tests for existing code
- ✅ Set up basic CI/CD pipeline (COMPLETED)

#### Phase 2: Integration Testing (3-4 weeks)
- Design integration test framework
- Create test fixtures for various project types
- Implement JDK version detection and compatibility
- Add Windows/Linux/macOS specific tests

#### Phase 3: Enhanced Diagnostics (2-3 weeks)
- Create progress indicators
- Add verbose/debug logging options

#### Phase 4: Advanced Features (ongoing)
- Parallel builds
- Incremental compilation
- Build caching
- Performance profiling

## Immediate Next Steps

1. ✅ **Create the abstraction layer** - This unblocks both unit testing and cross-platform support (COMPLETED)
2. ✅ **Set up GitHub Actions** with a basic matrix for the current tests (COMPLETED)
3. **Add unit tests** for the existing packages using the new abstractions

## Key Design Decisions

1. **Integration tests in subfolder** - Easier to maintain and coordinate
2. **Start with LTS Java versions** - Cover 8, 11, 17, 21 initially
3. **Use GitHub Actions** - Free for open source, good matrix support
4. **Mock external tools for unit tests** - But integration tests use real tools

## Success Metrics

- Unit test coverage > 80%
- Integration tests pass on all supported platforms
- Build times remain fast (< 1s for small projects)
- Zero platform-specific bugs in common scenarios

## Long-term Vision

jb should be the most reliable and user-friendly Java build tool, with:
- Rock-solid cross-platform support
- Fast builds that "just work"
- Comprehensive test coverage ensuring stability
- Clear documentation and helpful diagnostics

This approach balances immediate needs (better testing) with long-term goals (cross-platform reliability) while keeping the tool's simplicity philosophy intact.