# escp

`escp` (Enhanced SCP) is a tool designed to work like `scp` but with the added capability of automatically ignoring files or folders based on specified patterns (similar to `.gitignore`). It is especially useful for uploading files to remote servers like EC2, while excluding certain files or directories that you don't want to upload.

[![codecov](https://codecov.io/gh/Agent-Hellboy/escp/graph/badge.svg?token=596VLH7OJR)](https://codecov.io/gh/Agent-Hellboy/escp)

## Why `escp`?

While working with `scp` to upload files to EC2, I wanted a way to automate the process and ignore files or folders of specific patterns (e.g., log files, temporary files, or large directories). With `escp`, you can define a `.scpignore` file, similar to `.gitignore`, to list patterns of files or directories to exclude during the `scp` upload.

## Features

- **Git-style exclusion**: Define file and directory patterns in a `.scpignore` file using `.gitignore`-style syntax, including negation, anchored patterns, and `**`.
- **Native SSH transport**: Connects with `golang.org/x/crypto/ssh` instead of shelling out to the local `scp` binary.


## Installation

To build the binary for `escp`, follow these steps:

1. Clone the repository:
    ```bash
    git clone https://github.com/Agent-Hellboy/escp.git
    cd escp
    ```

2. Build the binary with Go 1.25 or newer:
    ```bash
    go build -o escp
    ```

3. (Optional) Move the binary to your `$PATH` for easier access:
    ```bash
    mv escp /usr/local/bin/
    ```

## Usage

1. Create a `.scpignore` file in the root of your project (same directory where you're running `escp`) and add the patterns you want to ignore. Example `.scpignore`:
    ```plaintext
    # Ignore all log files
    *.log

    # Ignore everything in the temp directory
    temp/

    # Ignore key files directly under secret
    secret/*.key

    # Keep one log file even though *.log is ignored
    !keep.log
    ```

2. Make sure SSH authentication is available. `escp` uses `SSH_AUTH_SOCK` when an agent is running, and also tries unencrypted `~/.ssh/id_ed25519`, `~/.ssh/id_rsa`, and `~/.ssh/id_ecdsa` keys. The destination host must be present in `~/.ssh/known_hosts`.

    If you do not already have a key that can log in to the server, generate one locally:
    ```bash
    ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519 -C "escp"
    ```

    Add the generated public key to the remote user's authorized keys on the server:
    ```bash
    cat ~/.ssh/id_ed25519.pub
    ```

    Copy that output into the server file for the user you will connect as. For root, that file is:
    ```bash
    /root/.ssh/authorized_keys
    ```

    The server-side permissions should be:
    ```bash
    chmod 700 /root/.ssh
    chmod 600 /root/.ssh/authorized_keys
    ```

    If you use a non-default key path, add it to your local SSH agent before running `escp`:
    ```bash
    ssh-add ~/.ssh/your_key
    ```

    Test SSH directly first:
    ```bash
    ssh root@your-server
    ```

3. Use `escp` to copy files just like you would with `scp`. The tool will automatically read the `.scpignore` file and skip files that match the patterns:
    ```bash
    ./escp source_directory/ user@ec2:/path/to/destination
    ```

4. `escp` uploads the filtered files and empty directories to the destination path.

## Example

Let's say you have the following directory structure:

```
project/
├── .scpignore
├── build/
│   ├── artifact.log
│   ├── binary
├── secret/
│   └── api.key
└── src/
    ├── main.go
    └── util.go
```

And the `.scpignore` file contains:
```plaintext
*.log
secret/
```

When you run `escp`:

```bash
./escp project/ user@ec2:/path/to/destination
```

The following files will be uploaded:
- `project/src/main.go`
- `project/src/util.go`

The following files will be ignored:
- `project/build/artifact.log`
- `project/secret/api.key`


## Debugging SSH Authentication

If normal SSH works but `escp` cannot authenticate, check whether SSH works without an interactive password or passphrase prompt:

```bash
ssh -o BatchMode=yes root@your-server true
```

Inspect the SSH configuration and identity files OpenSSH will try:

```bash
ssh -G root@your-server
```

Check whether your SSH agent is reachable and has loaded keys:

```bash
ssh-add -l
```

Run a verbose SSH probe to see the exact key OpenSSH accepts:

```bash
ssh -vvv -o BatchMode=yes -o ConnectTimeout=10 root@your-server true
```

Test `escp` with the SSH agent disabled to confirm fallback to default keys such as `~/.ssh/id_ed25519`:

```bash
env -u SSH_AUTH_SOCK ./escp ./project root@your-server:/tmp/escp-test-default-key
```

Verify the copied files on the server:

```bash
ssh -o BatchMode=yes root@your-server find /tmp/escp-test-default-key -maxdepth 3 -type f -print
```

## Remote E2E Testing

The unit test suite includes an opt-in remote E2E test. It is skipped unless `ESCP_E2E_SSH_URL` is set.

Use this format:

```bash
ESCP_E2E_SSH_URL='root:password@your-server'
```

For a custom SSH port:

```bash
ESCP_E2E_SSH_URL='root:password@your-server:2222'
```

Then run:

```bash
go test -v ./...
```

The E2E test creates a temporary source tree with a `.scpignore`, copies it to `/tmp/escp-e2e-*` on the server, verifies that ignored files were not uploaded, and removes the remote test directory afterward.

In GitHub Actions, add a repository secret named `ESCP_E2E_SSH_URL` with the same `username:password@host[:port]` format. If the secret is missing, CI still runs unit tests and skips the remote E2E test.

## Improving `escp`

1. **Performance Enhancements**:
   - Add support for parallel file uploads to speed up large directory transfers.
   - Implement incremental uploads, so only files that have changed are copied.

2. **Error Handling and Logging**:
   - Add a verbose mode (`-v`) to log detailed information during file transfers.
   - Enhance error handling by retrying failed transfers and improving error messages.

3. **Cross-Platform Support**:
   - Extend support for Windows environments using WSL (Windows Subsystem for Linux) or native support.

4. **Multiple Ignore Files**:
   - Support multiple ignore files such as `.scpignore` and `.escpignore`. Allow custom ignore files with a command-line argument.

## Testing `escp`

1. **Pattern Matching Test Cases**:
   - Test common Git-style patterns (`*.log`, `subdir/`, `subdir/*`, `/build`, `**/generated/**`, `!keep.log`).
   - Check that deeply nested directories, anchored patterns, negation, and literal `#` characters are handled correctly.

2. **Unit Tests**:
   - Write Go unit tests for key functions like `shouldIgnore`, `filterFiles`, and `parseDirectory`.

3. **Integration Tests**:
   - Run integration tests to ensure `escp` works as expected across different directory structures and file patterns.

4. **Git Ignore Semantics**:
   - `escp` uses `github.com/go-git/go-git/v5/plumbing/format/gitignore` for `.gitignore`-style matching. Add regression tests from Git ignore edge cases when changing ignore behavior.


## Contributions

I am 100% sure this broken , please test it and try to contribute if possible

## License

This project is licensed under the MIT License.
