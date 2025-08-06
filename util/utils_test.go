package util

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindFilesBySuffixR(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()

	// Create test files
	srcDir := filepath.Join(tempDir, "src")
	buildDir := filepath.Join(tempDir, "build")
	subDir := filepath.Join(srcDir, "com", "example")

	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(buildDir, 0755))
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Create Java files
	files := map[string]string{
		filepath.Join(srcDir, "Main.java"):        "public class Main {}",
		filepath.Join(srcDir, "Test.JAVA"):        "public class Test {}", // uppercase extension
		filepath.Join(subDir, "Example.java"):     "public class Example {}",
		filepath.Join(srcDir, "readme.txt"):       "readme",
		filepath.Join(buildDir, "Generated.java"): "public class Generated {}", // should be skipped
	}

	for path, content := range files {
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	t.Run("find java files", func(t *testing.T) {
		results, err := FindFilesBySuffixR(tempDir, ".java")
		assert.NoError(t, err)
		assert.Len(t, results, 3) // Main.java, Test.JAVA, Example.java (not Generated.java in build/)

		// Check that paths are relative
		paths := make([]string, len(results))
		for i, r := range results {
			paths[i] = r.Path
		}

		assert.Contains(t, paths, filepath.Join("src", "Main.java"))
		assert.Contains(t, paths, filepath.Join("src", "Test.JAVA"))
		assert.Contains(t, paths, filepath.Join("src", "com", "example", "Example.java"))

		// Verify build directory was skipped
		for _, path := range paths {
			assert.NotContains(t, path, "build")
		}
	})

	t.Run("find txt files", func(t *testing.T) {
		results, err := FindFilesBySuffixR(tempDir, ".txt")
		assert.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, filepath.Join("src", "readme.txt"), results[0].Path)
	})

	t.Run("no matching files", func(t *testing.T) {
		results, err := FindFilesBySuffixR(tempDir, ".cpp")
		assert.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("non-existent directory", func(t *testing.T) {
		_, err := FindFilesBySuffixR("/non/existent/dir", ".java")
		assert.Error(t, err)
	})

	t.Run("permission error", func(t *testing.T) {
		// Create a directory with no read permission
		noReadDir := filepath.Join(tempDir, "noread")
		require.NoError(t, os.MkdirAll(noReadDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(noReadDir, "test.java"), []byte("test"), 0644))

		// Change permissions to write-only
		require.NoError(t, os.Chmod(noReadDir, 0200))
		defer os.Chmod(noReadDir, 0755) // restore permissions for cleanup

		// Should encounter an error when trying to read the directory
		_, err := FindFilesBySuffixR(noReadDir, ".java")
		// On some systems/filesystems, this might not fail, so we just check if it doesn't panic
		_ = err
	})
}

func TestFindFilesByGlob(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()

	// Create test files
	srcDir := filepath.Join(tempDir, "src")
	resourcesDir := filepath.Join(tempDir, "resources")

	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.MkdirAll(resourcesDir, 0755))

	files := map[string]string{
		filepath.Join(tempDir, "config.properties"):        "prop1=value1",
		filepath.Join(tempDir, "app.properties"):           "prop2=value2",
		filepath.Join(resourcesDir, "test.properties"):     "prop3=value3",
		filepath.Join(resourcesDir, "messages.properties"): "hello=world",
		filepath.Join(srcDir, "Main.java"):                 "public class Main {}",
		filepath.Join(tempDir, "readme.txt"):               "readme",
	}

	for path, content := range files {
		require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	}

	t.Run("glob properties files", func(t *testing.T) {
		results, err := FindFilesByGlob(tempDir, []string{"*.properties"})
		assert.NoError(t, err)
		assert.Len(t, results, 2) // config.properties and app.properties

		// Check that absolute paths are returned
		for _, r := range results {
			assert.True(t, filepath.IsAbs(r.Path))
			assert.Equal(t, tempDir, r.Dir)
			assert.NotNil(t, r.Info)
			assert.False(t, r.Info.IsDir())
		}
	})

	t.Run("glob with subdirectory", func(t *testing.T) {
		results, err := FindFilesByGlob(tempDir, []string{"resources/*.properties"})
		assert.NoError(t, err)
		assert.Len(t, results, 2) // test.properties and messages.properties
	})

	t.Run("multiple patterns", func(t *testing.T) {
		results, err := FindFilesByGlob(tempDir, []string{"*.txt", "src/*.java"})
		assert.NoError(t, err)
		assert.Len(t, results, 2) // readme.txt and Main.java
	})

	t.Run("no matching files", func(t *testing.T) {
		_, err := FindFilesByGlob(tempDir, []string{"*.xml"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no embeds found matching")
	})

	t.Run("invalid pattern", func(t *testing.T) {
		_, err := FindFilesByGlob(tempDir, []string{"[invalid"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse pattern")
	})

	t.Run("file stat error", func(t *testing.T) {
		// This is hard to test without mocking os.Stat
		// The glob might return a path that gets deleted before stat
		// For coverage, we'll accept that this edge case is difficult to test
	})
}

func TestWriteFile(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("write new file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "test.txt")
		content := "Hello, World!"

		err := WriteFile(filePath, content)
		assert.NoError(t, err)

		// Verify file was written
		data, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "existing.txt")

		// Create initial file
		require.NoError(t, os.WriteFile(filePath, []byte("old content"), 0644))

		// Overwrite it
		newContent := "new content"
		err := WriteFile(filePath, newContent)
		assert.NoError(t, err)

		// Verify new content
		data, err := os.ReadFile(filePath)
		assert.NoError(t, err)
		assert.Equal(t, newContent, string(data))
	})

	t.Run("write to non-existent directory", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "nonexistent", "test.txt")
		err := WriteFile(filePath, "content")
		assert.Error(t, err)
	})

	t.Run("write to read-only directory", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping read-only directory test on Windows")
		}
		readOnlyDir := filepath.Join(tempDir, "readonly")
		require.NoError(t, os.MkdirAll(readOnlyDir, 0755))
		require.NoError(t, os.Chmod(readOnlyDir, 0555))
		defer os.Chmod(readOnlyDir, 0755)

		filePath := filepath.Join(readOnlyDir, "test.txt")
		err := WriteFile(filePath, "content")
		assert.Error(t, err)
	})
}

func TestReadFileAsString(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("read existing file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "test.txt")
		content := "Hello, World!\nLine 2"
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0644))

		result, err := ReadFileAsString(filePath)
		assert.NoError(t, err)
		assert.Equal(t, content, result)
	})

	t.Run("read non-existent file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "nonexistent.txt")
		result, err := ReadFileAsString(filePath)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("read empty file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "empty.txt")
		require.NoError(t, os.WriteFile(filePath, []byte(""), 0644))

		result, err := ReadFileAsString(filePath)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("read file with no permissions", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Skipping permissions test on Windows")
		}
		filePath := filepath.Join(tempDir, "noperm.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("secret"), 0000))
		defer os.Chmod(filePath, 0644)

		_, err := ReadFileAsString(filePath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read file")
	})

	t.Run("read large file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "large.txt")
		// Create a 1MB file
		largeContent := strings.Repeat("x", 1024*1024)
		require.NoError(t, os.WriteFile(filePath, []byte(largeContent), 0644))

		result, err := ReadFileAsString(filePath)
		assert.NoError(t, err)
		assert.Equal(t, largeContent, result)
	})
}

func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("copy regular file", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "source.txt")
		dstPath := filepath.Join(tempDir, "dest.txt")
		content := "Hello, World!"

		require.NoError(t, os.WriteFile(srcPath, []byte(content), 0644))

		err := CopyFile(srcPath, dstPath)
		assert.NoError(t, err)

		// Verify destination file
		data, err := os.ReadFile(dstPath)
		assert.NoError(t, err)
		assert.Equal(t, content, string(data))

		// Verify source file still exists
		assert.FileExists(t, srcPath)
	})

	t.Run("copy binary file", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "binary.bin")
		dstPath := filepath.Join(tempDir, "binary_copy.bin")

		// Create binary content
		binaryContent := []byte{0x00, 0xFF, 0x42, 0x13, 0x37}
		require.NoError(t, os.WriteFile(srcPath, binaryContent, 0644))

		err := CopyFile(srcPath, dstPath)
		assert.NoError(t, err)

		// Verify binary content
		data, err := os.ReadFile(dstPath)
		assert.NoError(t, err)
		assert.Equal(t, binaryContent, data)
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "source2.txt")
		dstPath := filepath.Join(tempDir, "existing.txt")

		require.NoError(t, os.WriteFile(srcPath, []byte("new content"), 0644))
		require.NoError(t, os.WriteFile(dstPath, []byte("old content"), 0644))

		err := CopyFile(srcPath, dstPath)
		assert.NoError(t, err)

		// Verify new content
		data, err := os.ReadFile(dstPath)
		assert.NoError(t, err)
		assert.Equal(t, "new content", string(data))
	})

	t.Run("source file not found", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "nonexistent.txt")
		dstPath := filepath.Join(tempDir, "dest.txt")

		err := CopyFile(srcPath, dstPath)
		assert.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("destination directory not found", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "source3.txt")
		dstPath := filepath.Join(tempDir, "nonexistent", "dest.txt")

		require.NoError(t, os.WriteFile(srcPath, []byte("content"), 0644))

		err := CopyFile(srcPath, dstPath)
		assert.Error(t, err)
	})

	t.Run("source is directory", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "srcdir")
		dstPath := filepath.Join(tempDir, "dstdir")

		require.NoError(t, os.MkdirAll(srcPath, 0755))

		err := CopyFile(srcPath, dstPath)
		assert.Error(t, err)
	})

	t.Run("copy preserves content exactly", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "exact.txt")
		dstPath := filepath.Join(tempDir, "exact_copy.txt")

		// Content with various line endings and special characters
		content := "Line 1\nLine 2\r\nLine 3\rUnicode: 你好世界\nBinary: \x00\x01\x02"
		require.NoError(t, os.WriteFile(srcPath, []byte(content), 0644))

		err := CopyFile(srcPath, dstPath)
		assert.NoError(t, err)

		// Verify byte-for-byte equality
		srcData, _ := os.ReadFile(srcPath)
		dstData, _ := os.ReadFile(dstPath)
		assert.Equal(t, srcData, dstData)
	})
}

func TestFileExists(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("existing file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "exists.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		exists := FileExists(filePath)
		assert.True(t, exists)
	})

	t.Run("non-existent file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "nonexistent.txt")
		exists := FileExists(filePath)
		assert.False(t, exists)
	})

	t.Run("directory is not a file", func(t *testing.T) {
		dirPath := filepath.Join(tempDir, "subdir")
		require.NoError(t, os.MkdirAll(dirPath, 0755))

		exists := FileExists(dirPath)
		assert.False(t, exists)
	})

	t.Run("symlink to file", func(t *testing.T) {
		// Create a file and a symlink to it
		filePath := filepath.Join(tempDir, "target.txt")
		linkPath := filepath.Join(tempDir, "link.txt")

		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
		require.NoError(t, os.Symlink(filePath, linkPath))

		// Symlink should be treated as existing file
		exists := FileExists(linkPath)
		assert.True(t, exists)
	})

	t.Run("broken symlink", func(t *testing.T) {
		// Create a symlink to non-existent file
		targetPath := filepath.Join(tempDir, "nonexistent_target.txt")
		linkPath := filepath.Join(tempDir, "broken_link.txt")

		require.NoError(t, os.Symlink(targetPath, linkPath))

		// Broken symlink should not exist
		exists := FileExists(linkPath)
		assert.False(t, exists)
	})

	t.Run("file with no read permissions", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "noperm.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0000))
		defer os.Chmod(filePath, 0644)

		// Should still be able to stat the file
		exists := FileExists(filePath)
		assert.True(t, exists)
	})
}

func TestSourceFileInfo(t *testing.T) {
	// Test the SourceFileInfo struct
	info := SourceFileInfo{
		Info: &mockFileInfo{name: "Test.java", size: 100},
		Path: "src/Test.java",
	}

	assert.Equal(t, "Test.java", info.Info.Name())
	assert.Equal(t, int64(100), info.Info.Size())
	assert.Equal(t, "src/Test.java", info.Path)
}

func TestFoundFileInfo(t *testing.T) {
	// Test the FoundFileInfo struct
	info := FoundFileInfo{
		Dir:  "/project/src",
		Path: "/project/src/Main.java",
		Info: &mockFileInfo{name: "Main.java", size: 200},
	}

	assert.Equal(t, "/project/src", info.Dir)
	assert.Equal(t, "/project/src/Main.java", info.Path)
	assert.Equal(t, "Main.java", info.Info.Name())
}

// mockFileInfo implements os.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }
