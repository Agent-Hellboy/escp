package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PatternMatcher is the interface for handling pattern matching
type PatternMatcher interface {
	ShouldIgnore(path string) bool
}

// FileManager is the interface for handling file-related operations
type FileManager interface {
	GetPatterns(filename string) ([]string, error)
	FilterFiles(files []string, patterns []string) []string
	ParseDirectory(source string) ([]string, error)
	IsDirEmpty(path string) (bool, error)
	CopyFiles(files []string, destination string) error
}

// SCPManager implements FileManager and manages the SCP file transfers
type SCPManager struct{}

// GetPatterns reads the .scpignore file and returns the patterns
func (scp *SCPManager) GetPatterns(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	patterns := []string{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	return patterns, nil
}

// FilterFiles filters out files that match the patterns from the provided list of files
func (scp *SCPManager) FilterFiles(files []string, patterns []string) []string {
	var filtered []string
	matcher := &SimplePatternMatcher{Patterns: patterns}
	for _, file := range files {
		if !matcher.ShouldIgnore(file) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// ParseDirectory walks the directory and collects files or empty directories to copy
func (scp *SCPManager) ParseDirectory(source string) ([]string, error) {
	var toCopy []string
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			toCopy = append(toCopy, path)
			return nil
		}

		isEmpty, err := scp.IsDirEmpty(path)
		if err != nil {
			return err
		}

		if isEmpty {
			toCopy = append(toCopy, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error walking directory: %v", err)
	}

	return toCopy, nil
}

// IsDirEmpty checks if a directory is empty (to detect if it's a leaf directory)
func (scp *SCPManager) IsDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Try to read one entry
	if err == nil {
		return false, nil // Directory has entries, it's not empty
	}
	if err == io.EOF {
		return true, nil // Directory is empty
	}

	return false, err
}

// CopyFiles copies the filtered files to the destination via SCP
func (scp *SCPManager) CopyFiles(files []string, destination string) error {
	if len(files) == 0 {
		return fmt.Errorf("no files to copy")
	}

	fmt.Println("Files to copy:", files)
	args := append([]string{"-r"}, files...)
	args = append(args, destination)

	cmd := exec.Command("scp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error copying files: %s", err)
	}
	fmt.Printf("Output:\n%s\n", output)
	return nil
}

// SimplePatternMatcher implements PatternMatcher and performs the pattern matching
type SimplePatternMatcher struct {
	Patterns []string
}

// ShouldIgnore checks if a file or directory matches any of the ignore patterns
func (pm *SimplePatternMatcher) ShouldIgnore(path string) bool {
	for _, pattern := range pm.Patterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err != nil {
			fmt.Printf("Error matching pattern: %v\n", err)
			return false
		}
		if matched {
			return true
		}

		// Handle patterns that should match subdirectories or path fragments
		if strings.HasSuffix(pattern, "*") {
			pattern = strings.TrimSuffix(pattern, "*")
		}
		if strings.HasPrefix(pattern, "*") {
			pattern = strings.TrimPrefix(pattern, "*")
		}
		index := strings.Index(path, pattern)
		if index != -1 {
			if index == 0 {
				if len(pattern) == len(path) || path[len(pattern)] == '/' {
					return true
				}
			}

			if index > 0 && path[index-1] == '/' {
				endIndex := index + len(pattern)
				if endIndex == len(path) || path[endIndex-1] == '/' {
					return true
				}
			}
		}
	}
	return false
}

// Main program entry
func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <source> <destination>")
		return
	}

	currentDir, _ := os.Getwd()
	source := os.Args[1]
	destination := os.Args[2]

	scp := &SCPManager{}

	// Read ignore patterns from the .scpignore file
	patterns, err := scp.GetPatterns(filepath.Join(currentDir, ".scpignore"))
	if err != nil {
		log.Fatalf("Error reading patterns: %v", err)
	}

	// Parse the source directory and collect files and directories to copy
	toCopy, err := scp.ParseDirectory(source)
	if err != nil {
		log.Fatalf("Error parsing directory: %v", err)
	}

	// Filter files based on the patterns in the .scpignore file
	toCopy = scp.FilterFiles(toCopy, patterns)

	if len(toCopy) == 0 {
		fmt.Println("No files to copy based on the .scpignore patterns.")
		return
	}

	// Copy files to the destination via SCP
	if err := scp.CopyFiles(toCopy, destination); err != nil {
		log.Fatalf("Error during file transfer: %v", err)
	}
}
