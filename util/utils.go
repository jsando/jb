package util

import (
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

func WriteFile(filePath, content string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(content)
	return err
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
