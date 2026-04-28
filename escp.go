package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
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

// RemoteDestination contains the SSH endpoint and remote copy path.
type RemoteDestination struct {
	User string
	Host string
	Port string
	Path string
}

// GetPatterns reads the .scpignore file and returns the patterns
func (scp *SCPManager) GetPatterns(filename string) ([]string, error) {
	// Check if the .scpignore file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		// If the file does not exist, return an empty slice (no patterns to ignore)
		return nil, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	patterns := []string{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		// Git ignore semantics treat only lines starting with # as comments.
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
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
	if len(files) == 0 {
		return nil
	}

	var filtered []string
	matcher := NewGitIgnoreMatcher(patterns, commonBaseDir(files))
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

	remote, err := ParseDestination(destination)
	if err != nil {
		return err
	}

	client, err := NewSSHClient(remote)
	if err != nil {
		return err
	}
	defer client.Close()

	baseDir := commonBaseDir(files)
	for _, file := range files {
		rel, err := filepath.Rel(baseDir, file)
		if err != nil {
			return fmt.Errorf("calculate relative path for %q: %w", file, err)
		}
		if strings.HasPrefix(rel, "..") {
			rel = filepath.Base(file)
		}

		remotePath := remoteJoin(remote.Path, filepath.ToSlash(rel))
		info, err := os.Stat(file)
		if err != nil {
			return fmt.Errorf("stat %q: %w", file, err)
		}

		if info.IsDir() {
			if err := runRemoteCommand(client, "mkdir -p "+shellQuote(remotePath)); err != nil {
				return err
			}
			continue
		}

		if err := copyFile(client, file, remotePath, info); err != nil {
			return err
		}
	}

	return nil
}

// ParseDestination parses scp-style destinations such as user@example.com:/tmp/app.
func ParseDestination(destination string) (RemoteDestination, error) {
	var remote RemoteDestination
	separator := strings.LastIndex(destination, ":")
	if separator <= 0 || separator == len(destination)-1 {
		return remote, fmt.Errorf("destination must use scp format [user@]host:path")
	}

	target := destination[:separator]
	remote.Path = destination[separator+1:]
	if remote.Path == "" {
		return remote, fmt.Errorf("remote path cannot be empty")
	}

	if at := strings.LastIndex(target, "@"); at >= 0 {
		remote.User = target[:at]
		target = target[at+1:]
	}
	if remote.User == "" {
		currentUser, err := user.Current()
		if err != nil || currentUser.Username == "" {
			return remote, fmt.Errorf("destination user is required when the local user cannot be determined")
		}
		remote.User = currentUser.Username
	}

	host, port, err := net.SplitHostPort(target)
	if err == nil {
		remote.Host = strings.Trim(host, "[]")
		remote.Port = port
	} else {
		remote.Host = strings.Trim(target, "[]")
		remote.Port = "22"
	}

	if remote.Host == "" {
		return remote, fmt.Errorf("destination host cannot be empty")
	}
	if _, err := strconv.Atoi(remote.Port); err != nil {
		return remote, fmt.Errorf("destination port must be numeric: %w", err)
	}

	return remote, nil
}

// NewSSHClient creates an SSH client using ssh-agent and common private key files.
func NewSSHClient(remote RemoteDestination) (*ssh.Client, error) {
	authMethods, err := authMethods()
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := hostKeyCallback()
	if err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User:            remote.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(remote.Host, remote.Port), config)
	if err != nil {
		return nil, fmt.Errorf("connect to %s@%s:%s: %w", remote.User, remote.Host, remote.Port, err)
	}
	return client, nil
}

func authMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if socket := os.Getenv("SSH_AUTH_SOCK"); socket != "" {
		conn, err := net.Dial("unix", socket)
		if err == nil {
			signers, signersErr := agent.NewClient(conn).Signers()
			if signersErr == nil && len(signers) > 0 {
				methods = append(methods, ssh.PublicKeys(signers...))
			}
		}
	}

	if password := os.Getenv("ESCP_SSH_PASSWORD"); password != "" {
		methods = append(methods,
			ssh.Password(password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					answers[i] = password
				}
				return answers, nil
			}),
		)
	}

	home, err := os.UserHomeDir()
	if err == nil {
		for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
			keyPath := filepath.Join(home, ".ssh", name)
			key, readErr := os.ReadFile(keyPath)
			if readErr != nil {
				continue
			}
			signer, parseErr := ssh.ParsePrivateKey(key)
			if parseErr == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods found; start ssh-agent or add an unencrypted key in ~/.ssh")
	}
	return methods, nil
}

func hostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory for known_hosts: %w", err)
	}

	callback, err := knownhosts.New(filepath.Join(home, ".ssh", "known_hosts"))
	if err != nil {
		return nil, fmt.Errorf("load ~/.ssh/known_hosts: %w", err)
	}
	return callback, nil
}

func copyFile(client *ssh.Client, localPath, remotePath string, info os.FileInfo) error {
	if err := runRemoteCommand(client, "mkdir -p "+shellQuote(remoteDir(remotePath))); err != nil {
		return err
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %q: %w", localPath, err)
	}
	defer file.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("open remote stdin: %w", err)
	}
	defer stdin.Close()
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open remote stdout: %w", err)
	}

	if err := session.Start("scp -t " + shellQuote(remoteDir(remotePath))); err != nil {
		return fmt.Errorf("start remote scp: %w", err)
	}
	if err := checkSCPAck(stdout); err != nil {
		return err
	}

	mode := info.Mode().Perm()
	if _, err := fmt.Fprintf(stdin, "C%04o %d %s\n", mode, info.Size(), filepath.Base(remotePath)); err != nil {
		return fmt.Errorf("send SCP header: %w", err)
	}
	if err := checkSCPAck(stdout); err != nil {
		return err
	}
	if _, err := io.Copy(stdin, file); err != nil {
		return fmt.Errorf("send %q: %w", localPath, err)
	}
	if _, err := fmt.Fprint(stdin, "\x00"); err != nil {
		return fmt.Errorf("finish SCP transfer: %w", err)
	}
	if err := checkSCPAck(stdout); err != nil {
		return err
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("close remote stdin: %w", err)
	}
	if err := session.Wait(); err != nil {
		return fmt.Errorf("remote scp failed for %q: %w", localPath, err)
	}

	return nil
}

func checkSCPAck(reader io.Reader) error {
	var response [1]byte
	if _, err := io.ReadFull(reader, response[:]); err != nil {
		return fmt.Errorf("read SCP acknowledgement: %w", err)
	}
	switch response[0] {
	case 0:
		return nil
	case 1, 2:
		message, _ := bufio.NewReader(reader).ReadString('\n')
		return fmt.Errorf("remote scp rejected transfer: %s", strings.TrimSpace(message))
	default:
		return fmt.Errorf("unexpected SCP acknowledgement byte: %d", response[0])
	}
}

func runRemoteCommand(client *ssh.Client, command string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer session.Close()

	if output, err := session.CombinedOutput(command); err != nil {
		return fmt.Errorf("remote command failed: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return nil
}

func commonBaseDir(files []string) string {
	base := filepath.Dir(files[0])
	for _, file := range files[1:] {
		dir := filepath.Dir(file)
		for !samePathOrChild(dir, base) {
			next := filepath.Dir(base)
			if next == base {
				return base
			}
			base = next
		}
	}
	return base
}

func samePathOrChild(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && (rel == "." || !strings.HasPrefix(rel, ".."))
}

func remoteJoin(base, rel string) string {
	if base == "" || base == "." {
		return rel
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(rel, "/")
}

func remoteDir(path string) string {
	dir := filepath.ToSlash(filepath.Dir(path))
	if dir == "." {
		return "."
	}
	return dir
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// GitIgnoreMatcher implements PatternMatcher using Git-style ignore semantics.
type GitIgnoreMatcher struct {
	matcher gitignore.Matcher
	baseDir string
}

// NewGitIgnoreMatcher builds a matcher for patterns from the source root.
func NewGitIgnoreMatcher(patterns []string, baseDir string) *GitIgnoreMatcher {
	parsed := make([]gitignore.Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		parsed = append(parsed, gitignore.ParsePattern(pattern, nil))
	}
	return &GitIgnoreMatcher{
		matcher: gitignore.NewMatcher(parsed),
		baseDir: baseDir,
	}
}

// ShouldIgnore checks if a file or directory matches the Git-style ignore patterns.
func (pm *GitIgnoreMatcher) ShouldIgnore(path string) bool {
	rel := path
	if pm.baseDir != "" {
		if relativePath, err := filepath.Rel(pm.baseDir, path); err == nil {
			rel = relativePath
		}
	}

	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." {
		return false
	}

	info, err := os.Stat(path)
	isDir := err == nil && info.IsDir()
	return pm.matcher.Match(strings.Split(rel, "/"), isDir)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run main.go <source> <destination>")
		return
	}

	currentDir, _ := os.Getwd()
	source := os.Args[1]
	destination := os.Args[2]

	scp := &SCPManager{}

	// Try to get patterns from .scpignore file, but proceed even if it's not present
	patterns, err := scp.GetPatterns(filepath.Join(currentDir, ".scpignore"))
	if err != nil {
		log.Fatalf("Error reading patterns: %v", err)
	}

	// Parse the source directory and collect files and directories to copy
	toCopy, err := scp.ParseDirectory(source)
	if err != nil {
		log.Fatalf("Error parsing directory: %v", err)
	}

	// If patterns are found in .scpignore, filter files; otherwise, copy everything
	if len(patterns) > 0 {
		toCopy = scp.FilterFiles(toCopy, patterns)
	} else {
		fmt.Println(".scpignore not found, copying all files.")
	}

	if len(toCopy) == 0 {
		fmt.Println("No files to copy based on the .scpignore patterns.")
		return
	}

	// Copy files to the destination via SCP
	if err := scp.CopyFiles(toCopy, destination); err != nil {
		log.Fatalf("Error during file transfer: %v", err)
	}
}
