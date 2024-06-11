package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// create a function which will parse a file and return all strings which are patterns from .scpignore
func getPatterns(filename string) []string {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	if data == nil {
		return nil
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	// get all patterns
	patterns := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	return patterns
}

func parseDirectory(directory string) []string {
	// get all files and directories in the specified directory
	var filesAndDirs []string
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		filesAndDirs = append(filesAndDirs, path)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	return filesAndDirs
}

func match(patterns []string, fileOrDir string) bool {
	// check if filename matches any of the patterns
	for _, pattern := range patterns {
		regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
		regexPattern = strings.ReplaceAll(regexPattern, `\*`, ".*")
		matched, err := regexp.MatchString(regexPattern, fileOrDir)
		if err != nil {
			log.Fatal(err)
		}
		if matched {
			return true
		}
	}
	return false
}

func main() {
	// Validate the number of arguments
	if len(os.Args) < 3 {
		log.Fatal("Usage: go run main.go <source> <destination>")
	}

	// Get the source and destination directories from the arguments
	source := os.Args[1]
	destination := os.Args[2]

	// get all files and directories in the source directory
	filesAndDirs := parseDirectory(source)

	// get all patterns from .scpignore
	patterns := getPatterns(filepath.Join(source, ".scpignore"))

	// check if files and folders match the pattern in the scpignore
	var toCopy []string
	for _, fileOrDir := range filesAndDirs {
		// Normalize fileOrDir to remove the leading source directory path
		normalizedFileOrDir := strings.TrimPrefix(fileOrDir, source)
		normalizedFileOrDir = strings.TrimPrefix(normalizedFileOrDir, string(filepath.Separator))

		if !match(patterns, normalizedFileOrDir) {
			toCopy = append(toCopy, fileOrDir)
		}
	}

	// do scp of all files and directories not matched
	for _, item := range toCopy {
		cmd := exec.Command("scp", "-r", item, destination)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Error copying %s: %s\n", item, err)
		}
		fmt.Printf("Output for %s:\n%s\n", item, output)
	}
}
