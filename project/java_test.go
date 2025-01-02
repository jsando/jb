package project

import (
	"archive/zip"
	"encoding/xml"
	"github.com/jsando/jb/artifact"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func loadModule(path string) (*Module, error) {
	path, err := filepath.Abs(path)
	loader := NewModuleLoader()
	module, err := loader.GetModule(path)
	if err != nil {
		return nil, err
	}
	return module, nil
}

func TestJavaBuilder_Build(t *testing.T) {
	tests := []struct {
		path          string
		expectedError bool
	}{
		{
			path:          "../tests/nodeps",
			expectedError: false,
		},
		{
			path:          "../tests/refs/main",
			expectedError: false,
		},
		{
			path:          "../tests/simpledeps",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			module, err := loadModule(tt.path)
			assert.NoError(t, err)
			err = module.Clean()
			assert.NoError(t, err)
			err = module.Build()
			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJavaBuilder_Build_WithEmbeds(t *testing.T) {
	module, err := loadModule("../tests/embed/test1")
	assert.NoError(t, err)
	err = module.Clean()
	assert.NoError(t, err)
	err = module.Build()
	assert.NoError(t, err)
	jarPath := filepath.Join("..", "tests", "embed", "test1", "build", "test1-1.0.jar")
	verifyJarContents(t, jarPath, []string{
		"META-INF/",
		"META-INF/MANIFEST.MF",
		"file1",
		"file2",
		"text1.txt",
		"text2.txt",
		"morefiles/",
		"morefiles/subdir/",
		"morefiles/subdir/file1",
		"morefiles/subdir/file2",
		"morefiles/subdir/file3",
	})
	// parse the pom file and verify its contents
	pomPath := filepath.Join("..", "tests", "embed", "test1", "build", "test1-1.0.pom")
	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		t.Fatalf("POM file does not exist: %v", pomPath)
	}
	file, err := os.Open(pomPath)
	if err != nil {
		t.Fatalf("Failed to open POM file: %v", err)
	}
	defer file.Close()
	decoder := xml.NewDecoder(file)
	pom := artifact.POM{}
	if err := decoder.Decode(&pom); err != nil {
		t.Fatalf("Failed to parse POM file: %v", err)
	}
	assert.Equal(t, "jar", pom.Packaging)
	assert.Equal(t, "1.0", pom.Version)
	assert.Equal(t, "com.example", pom.GroupID)
}

func verifyJarContents(t *testing.T, jarPath string, files []string) {
	// Open the JAR file as a ZIP archive
	zipReader, err := zip.OpenReader(jarPath)
	if err != nil {
		t.Fatalf("Failed to open JAR file: %v", err)
	}
	defer zipReader.Close()

	// Expected files/folders in the JAR
	expectedFiles := make(map[string]bool)
	for _, file := range files {
		expectedFiles[file] = false
	}

	// Traverse the files in the archive
	for _, file := range zipReader.File {
		if _, exists := expectedFiles[file.Name]; exists {
			expectedFiles[file.Name] = true
		}
	}

	// Verify all expected files/folders were found
	for file, found := range expectedFiles {
		if !found {
			t.Errorf("Expected file/folder %s not found in JAR", file)
		}
	}

	// Optionally, ensure there are no unexpected files
	for _, file := range zipReader.File {
		if _, exists := expectedFiles[file.Name]; !exists {
			t.Logf("Unexpected file found in JAR: %s", file.Name)
		}
	}

	// Check we got exact matches for better assertions
	if t.Failed() {
		t.Fatalf("Some files/folders are missing or unexpected in the JAR")
	}
}
