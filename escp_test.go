package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	patternsContent := "*.log\n# Ignore backup files\n*.bak\nsecret/*"
	err = ioutil.WriteFile(scpignorePath, []byte(patternsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write .scpignore file: %v", err)
	}

	scp := &SCPManager{}
	patterns, err := scp.GetPatterns(scpignorePath)
	if err != nil {
		t.Fatalf("Error getting patterns: %v", err)
	}

	expectedPatterns := []string{"*.log", "*.bak", "secret/*"}
	if len(patterns) != len(expectedPatterns) {
		t.Fatalf("Expected %d patterns, got %d", len(expectedPatterns), len(patterns))
	}

	for i, pattern := range expectedPatterns {
		if patterns[i] != pattern {
			t.Fatalf("Expected pattern %s, but got %s", pattern, patterns[i])
		}
	}
}

func TestGetPatternsMissingFile(t *testing.T) {
	scp := &SCPManager{}
	patterns, err := scp.GetPatterns(filepath.Join(t.TempDir(), ".scpignore"))
	if err != nil {
		t.Fatalf("Expected missing ignore file to be allowed, got %v", err)
	}
	if patterns != nil {
		t.Fatalf("Expected nil patterns for missing ignore file, got %v", patterns)
	}
}

func TestParseDestinationWithUserHostAndPath(t *testing.T) {
	remote, err := ParseDestination("deploy@example.com:/var/www/app")
	if err != nil {
		t.Fatalf("ParseDestination returned error: %v", err)
	}

	if remote.User != "deploy" {
		t.Fatalf("Expected user deploy, got %q", remote.User)
	}
	if remote.Host != "example.com" {
		t.Fatalf("Expected host example.com, got %q", remote.Host)
	}
	if remote.Port != "22" {
		t.Fatalf("Expected default port 22, got %q", remote.Port)
	}
	if remote.Path != "/var/www/app" {
		t.Fatalf("Expected remote path /var/www/app, got %q", remote.Path)
	}
}

func TestParseDestinationWithPort(t *testing.T) {
	remote, err := ParseDestination("deploy@example.com:2222:/srv/app")
	if err != nil {
		t.Fatalf("ParseDestination returned error: %v", err)
	}

	if remote.Host != "example.com" {
		t.Fatalf("Expected host example.com, got %q", remote.Host)
	}
	if remote.Port != "2222" {
		t.Fatalf("Expected port 2222, got %q", remote.Port)
	}
	if remote.Path != "/srv/app" {
		t.Fatalf("Expected remote path /srv/app, got %q", remote.Path)
	}
}

func TestParseDestinationRejectsInvalidFormat(t *testing.T) {
	if _, err := ParseDestination("/tmp/local/path"); err == nil {
		t.Fatalf("Expected invalid destination format to fail")
	}
}

func TestParseDestinationRejectsInvalidPort(t *testing.T) {
	if _, err := ParseDestination("deploy@example.com:ssh:/srv/app"); err == nil {
		t.Fatalf("Expected non-numeric port to fail")
	}
}

func TestCopyFilesRejectsEmptyList(t *testing.T) {
	scp := &SCPManager{}
	if err := scp.CopyFiles(nil, "deploy@example.com:/srv/app"); err == nil {
		t.Fatalf("Expected empty copy list to fail")
	}
}

func TestCopyFilesRejectsInvalidDestinationBeforeSSH(t *testing.T) {
	scp := &SCPManager{}
	if err := scp.CopyFiles([]string{"escp.go"}, "not-a-destination"); err == nil {
		t.Fatalf("Expected invalid destination to fail")
	}
}

func TestAuthMethodsRejectsMissingKeysAndAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ESCP_SSH_PASSWORD", "")

	if _, err := authMethods(); err == nil {
		t.Fatalf("Expected missing SSH auth methods to fail")
	}
}

func TestAuthMethodsAcceptsPassword(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ESCP_SSH_PASSWORD", "secret")

	if _, err := authMethods(); err != nil {
		t.Fatalf("Expected password auth method to be accepted, got %v", err)
	}
}

func TestHostKeyCallbackRejectsMissingKnownHosts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := hostKeyCallback(); err == nil {
		t.Fatalf("Expected missing known_hosts to fail")
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

func TestGitIgnoreMatcherDirectoryPattern(t *testing.T) {
	matcher := NewGitIgnoreMatcher([]string{"secret/"}, ".")
	if !matcher.ShouldIgnore("project/secret/api.key") {
		t.Fatalf("Expected secret directory pattern to ignore nested file")
	}
	if matcher.ShouldIgnore("project/src/api.key") {
		t.Fatalf("Did not expect unrelated path to be ignored")
	}
}

func TestGitIgnoreMatcherInvalidPatternDoesNotIgnore(t *testing.T) {
	matcher := NewGitIgnoreMatcher([]string{"["}, ".")
	if matcher.ShouldIgnore("file.txt") {
		t.Fatalf("Invalid pattern should not ignore files")
	}
}

func TestFilterFilesUsesGitIgnoreNegation(t *testing.T) {
	files := []string{
		"app.log",
		"keep.log",
		"notes.txt",
	}
	patterns := []string{
		"*.log",
		"!keep.log",
	}

	scp := &SCPManager{}
	filteredFiles := scp.FilterFiles(files, patterns)

	expectedFilteredFiles := []string{
		"keep.log",
		"notes.txt",
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

func TestFilterFilesUsesGitIgnoreAnchoredPatterns(t *testing.T) {
	files := []string{
		"build/app",
		"src/build/app",
		"src/main.go",
	}
	patterns := []string{"/build"}

	scp := &SCPManager{}
	filteredFiles := scp.FilterFiles(files, patterns)

	expectedFilteredFiles := []string{
		"src/build/app",
		"src/main.go",
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

func TestFilterFilesUsesGitIgnoreDoubleStar(t *testing.T) {
	files := []string{
		"generated/root.pb.go",
		"src/generated/api.pb.go",
		"src/api.go",
	}
	patterns := []string{"**/generated/**"}

	scp := &SCPManager{}
	filteredFiles := scp.FilterFiles(files, patterns)

	expectedFilteredFiles := []string{
		"src/api.go",
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

func TestGitIgnoreMatcherTreatsInlineHashAsLiteral(t *testing.T) {
	matcher := NewGitIgnoreMatcher([]string{"file#name.txt"}, ".")
	if !matcher.ShouldIgnore("file#name.txt") {
		t.Fatalf("Expected inline hash to be treated as a literal pattern character")
	}
	if matcher.ShouldIgnore("file") {
		t.Fatalf("Did not expect inline hash pattern to ignore a shorter filename")
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

func TestRemoteJoin(t *testing.T) {
	got := remoteJoin("/srv/app/", "/nested/file.txt")
	if got != "/srv/app/nested/file.txt" {
		t.Fatalf("Expected normalized remote path, got %q", got)
	}
}

func TestCommonBaseDir(t *testing.T) {
	files := []string{
		filepath.Join("project", "src", "main.go"),
		filepath.Join("project", "assets", "logo.png"),
		filepath.Join("project", "empty"),
	}

	got := commonBaseDir(files)
	if got != "project" {
		t.Fatalf("Expected common base project, got %q", got)
	}
}

func TestRemoteDir(t *testing.T) {
	if got := remoteDir("file.txt"); got != "." {
		t.Fatalf("Expected current directory for bare filename, got %q", got)
	}
	if got := remoteDir("/srv/app/file.txt"); got != "/srv/app" {
		t.Fatalf("Expected parent directory, got %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	got := shellQuote("it's fine")
	if got != "'it'\\''s fine'" {
		t.Fatalf("Expected shell escaped quote, got %q", got)
	}
}

func TestCheckSCPAckSuccess(t *testing.T) {
	if err := checkSCPAck(strings.NewReader("\x00")); err != nil {
		t.Fatalf("Expected successful acknowledgement, got %v", err)
	}
}

func TestCheckSCPAckRemoteError(t *testing.T) {
	err := checkSCPAck(strings.NewReader("\x01permission denied\n"))
	if err == nil {
		t.Fatalf("Expected remote error acknowledgement to fail")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("Expected remote error message in failure, got %v", err)
	}
}

func TestCheckSCPAckUnexpectedByte(t *testing.T) {
	err := checkSCPAck(strings.NewReader("x"))
	if err == nil {
		t.Fatalf("Expected unexpected acknowledgement byte to fail")
	}
}

func TestCheckSCPAckReadFailure(t *testing.T) {
	err := checkSCPAck(errReader{})
	if err == nil {
		t.Fatalf("Expected read failure")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestE2ERemoteCopyWithGitIgnore(t *testing.T) {
	rawURL := os.Getenv("ESCP_E2E_SSH_URL")
	if rawURL == "" {
		t.Skip("set ESCP_E2E_SSH_URL=username:password@host[:port] to run remote E2E test")
	}

	remote, password, err := parseE2ESSHURL(rawURL)
	if err != nil {
		t.Fatalf("Invalid ESCP_E2E_SSH_URL: %v", err)
	}

	homeDir := t.TempDir()
	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("Create test .ssh directory: %v", err)
	}

	knownHosts, err := exec.Command("ssh-keyscan", "-p", remote.Port, remote.Host).Output()
	if err != nil {
		t.Fatalf("Scan remote host key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "known_hosts"), knownHosts, 0600); err != nil {
		t.Fatalf("Write test known_hosts: %v", err)
	}

	t.Setenv("HOME", homeDir)
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("ESCP_SSH_PASSWORD", password)

	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, ".scpignore"), "*.log\n!keep.log\nignored/\n**/generated/**\n")
	writeTestFile(t, filepath.Join(sourceDir, "hello.txt"), "hello\n")
	writeTestFile(t, filepath.Join(sourceDir, "keep.log"), "keep\n")
	writeTestFile(t, filepath.Join(sourceDir, "app.log"), "ignore\n")
	writeTestFile(t, filepath.Join(sourceDir, "subdir", "nested.txt"), "nested\n")
	writeTestFile(t, filepath.Join(sourceDir, "ignored", "skip.txt"), "ignore\n")
	writeTestFile(t, filepath.Join(sourceDir, "generated", "root.pb.go"), "ignore\n")

	remoteBase := fmt.Sprintf("/tmp/escp-e2e-%d", time.Now().UnixNano())
	remote.Path = remoteBase

	client, err := NewSSHClient(remote)
	if err != nil {
		t.Fatalf("Connect to remote server: %v", err)
	}
	defer client.Close()
	defer runRemoteCommand(client, "rm -rf "+shellQuote(remoteBase))

	scp := &SCPManager{}
	files, err := scp.ParseDirectory(sourceDir)
	if err != nil {
		t.Fatalf("Parse source directory: %v", err)
	}
	patterns, err := scp.GetPatterns(filepath.Join(sourceDir, ".scpignore"))
	if err != nil {
		t.Fatalf("Read .scpignore: %v", err)
	}
	files = scp.FilterFiles(files, patterns)

	if err := scp.CopyFiles(files, remoteDestination(remote)); err != nil {
		t.Fatalf("Copy files to remote server: %v", err)
	}

	verifyCommand := strings.Join([]string{
		"test -f " + shellQuote(remoteJoin(remoteBase, ".scpignore")),
		"test -f " + shellQuote(remoteJoin(remoteBase, "hello.txt")),
		"test -f " + shellQuote(remoteJoin(remoteBase, "keep.log")),
		"test -f " + shellQuote(remoteJoin(remoteBase, "subdir/nested.txt")),
		"test ! -e " + shellQuote(remoteJoin(remoteBase, "app.log")),
		"test ! -e " + shellQuote(remoteJoin(remoteBase, "ignored/skip.txt")),
		"test ! -e " + shellQuote(remoteJoin(remoteBase, "generated/root.pb.go")),
	}, " && ")

	if err := runRemoteCommand(client, verifyCommand); err != nil {
		t.Fatalf("Verify remote copy result: %v", err)
	}
}

func parseE2ESSHURL(raw string) (RemoteDestination, string, error) {
	var remote RemoteDestination

	at := strings.LastIndex(raw, "@")
	if at <= 0 || at == len(raw)-1 {
		return remote, "", fmt.Errorf("must use username:password@host[:port]")
	}

	userInfo := raw[:at]
	hostPort := raw[at+1:]

	userName, password, ok := strings.Cut(userInfo, ":")
	if !ok || userName == "" || password == "" {
		return remote, "", fmt.Errorf("must include username and password before @")
	}

	remote.User = userName
	remote.Port = "22"

	if host, port, err := net.SplitHostPort(hostPort); err == nil {
		remote.Host = strings.Trim(host, "[]")
		remote.Port = port
	} else if host, port, ok := strings.Cut(hostPort, ":"); ok {
		remote.Host = host
		remote.Port = port
	} else {
		remote.Host = hostPort
	}

	if remote.Host == "" {
		return remote, "", fmt.Errorf("host cannot be empty")
	}
	if remote.Port == "" {
		return remote, "", fmt.Errorf("port cannot be empty")
	}

	return remote, password, nil
}

func remoteDestination(remote RemoteDestination) string {
	host := remote.Host
	if remote.Port != "" && remote.Port != "22" {
		host = net.JoinHostPort(remote.Host, remote.Port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return remote.User + "@" + host + ":" + remote.Path
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("Create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Write %s: %v", path, err)
	}
}
