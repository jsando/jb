package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type SourceFileInfo struct {
	Info os.FileInfo
	Path string // path relative to module root
}

func FindFilesBySuffixR(dir, suffix string) ([]SourceFileInfo, error) {
	var sources []SourceFileInfo
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() == "build" {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(info.Name())) == suffix {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			sources = append(sources, SourceFileInfo{Info: info, Path: relPath})
		}
		return nil
	})
	return sources, err
}

type FoundFileInfo struct {
	Dir  string      // directory that was searched to find Path
	Path string      // absolute path ... convert to relative using Dir
	Info os.FileInfo //
}

func FindFilesByGlob(dir string, includes []string) ([]FoundFileInfo, error) {
	var sources []FoundFileInfo
	for _, include := range includes {
		srcPattern := filepath.Join(dir, include)
		matchingFiles, err := filepath.Glob(srcPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pattern %s: %w", srcPattern, err)
		}
		if len(matchingFiles) == 0 {
			return nil, fmt.Errorf("no embeds found matching %s", srcPattern)
		}
		for _, src := range matchingFiles {
			absPath, err := filepath.Abs(src)
			if err != nil {
				return nil, fmt.Errorf("failed to get absolute path for %s: %w", src, err)
			}
			info, err := os.Stat(absPath)
			if err != nil {
				return nil, fmt.Errorf("failed to stat file %s: %w", absPath, err)
			}
			sources = append(sources, FoundFileInfo{Info: info, Dir: dir, Path: absPath})
		}
	}
	return sources, nil
}

func WriteFile(filePath, content string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(content)
	return err
}

// ReadFileAsString reads the entire content of a file and returns it as a string
// Returns empty string if file not found
func ReadFileAsString(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read file %s: %w", filename, err)
	}
	return string(data), nil
}

func CopyFile(src, dst string) error {
	// Open the source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create the destination file
	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// Copy the contents from source to destination
	_, err = io.Copy(destinationFile, sourceFile)
	return err
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	b := !info.IsDir()
	return b
}
