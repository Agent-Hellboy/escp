package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// TestGetPatterns tests the GetPatterns function for reading ignore patterns
func TestGetPatterns(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock .scpignore file in the temp directory
	scpignorePath := filepath.Join(tmpDir, ".scpignore")
	patternsContent := "*.log\n# Ignore backup files\n*.bak"
	err = ioutil.WriteFile(scpignorePath, []byte(patternsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write .scpignore file: %v", err)
	}

	scp := &SCPManager{}
	patterns, err := scp.GetPatterns(scpignorePath)
	if err != nil {
		t.Fatalf("Error getting patterns: %v", err)
	}

	expectedPatterns := []string{"*.log", "*.bak"}
	if len(patterns) != len(expectedPatterns) {
		t.Fatalf("Expected %d patterns, got %d", len(expectedPatterns), len(patterns))
	}

	for i, pattern := range expectedPatterns {
		if patterns[i] != pattern {
			t.Fatalf("Expected pattern %s, but got %s", pattern, patterns[i])
		}
	}
}

// TestParseDirectory tests the ParseDirectory function for listing files
func TestParseDirectory(t *testing.T) {
	// Create a temporary directory with some files and subdirectories
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files and subdirectories
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	ioutil.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("test"), 0644)
	ioutil.WriteFile(filepath.Join(tmpDir, "file2.log"), []byte("test"), 0644)

	scp := &SCPManager{}
	files, err := scp.ParseDirectory(tmpDir)
	if err != nil {
		t.Fatalf("Error parsing directory: %v", err)
	}

	expectedFiles := []string{
		filepath.Join(tmpDir, "file1.txt"),
		filepath.Join(tmpDir, "file2.log"),
		filepath.Join(tmpDir, "subdir"),
	}

	if len(files) != len(expectedFiles) {
		t.Fatalf("Expected %d files, got %d", len(expectedFiles), len(files))
	}

	for i, expectedFile := range expectedFiles {
		if files[i] != expectedFile {
			t.Fatalf("Expected file %s, but got %s", expectedFile, files[i])
		}
	}
}

// TestFilterFiles tests the FilterFiles function for filtering files based on patterns
func TestFilterFiles(t *testing.T) {
	files := []string{
		"file1.txt",
		"file2.log",
		"file3.bak",
		"subdir/file4.txt",
	}

	patterns := []string{"*.log", "*.bak"}

	scp := &SCPManager{}
	filteredFiles := scp.FilterFiles(files, patterns)

	expectedFilteredFiles := []string{
		"file1.txt",
		"subdir/file4.txt",
	}

	if len(filteredFiles) != len(expectedFilteredFiles) {
		t.Fatalf("Expected %d files after filtering, but got %d", len(expectedFilteredFiles), len(filteredFiles))
	}

	for i, expectedFile := range expectedFilteredFiles {
		if filteredFiles[i] != expectedFile {
			t.Fatalf("Expected file %s, but got %s", expectedFile, filteredFiles[i])
		}
	}
}

// TestIsDirEmpty tests the IsDirEmpty function for checking if a directory is empty
func TestIsDirEmpty(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create an empty subdirectory
	os.Mkdir(filepath.Join(tmpDir, "emptydir"), 0755)

	// Create a non-empty subdirectory
	os.Mkdir(filepath.Join(tmpDir, "nonemptydir"), 0755)
	ioutil.WriteFile(filepath.Join(tmpDir, "nonemptydir", "file.txt"), []byte("test"), 0644)

	scp := &SCPManager{}

	// Check empty directory
	isEmpty, err := scp.IsDirEmpty(filepath.Join(tmpDir, "emptydir"))
	if err != nil {
		t.Fatalf("Error checking if directory is empty: %v", err)
	}
	if !isEmpty {
		t.Fatalf("Expected directory to be empty")
	}

	// Check non-empty directory
	isEmpty, err = scp.IsDirEmpty(filepath.Join(tmpDir, "nonemptydir"))
	if err != nil {
		t.Fatalf("Error checking if directory is empty: %v", err)
	}
	if isEmpty {
		t.Fatalf("Expected directory to be non-empty")
	}
}
