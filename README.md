# escp

`escp` (Enhanced SCP) is a tool designed to work like `scp` but with the added capability of automatically ignoring files or folders based on specified patterns (similar to `.gitignore`). It is especially useful for uploading files to remote servers like EC2, while excluding certain files or directories that you don't want to upload.

[![codecov](https://codecov.io/gh/Agent-Hellboy/escp/graph/badge.svg?token=596VLH7OJR)](https://codecov.io/gh/Agent-Hellboy/escp)

## Why `escp`?

While working with `scp` to upload files to EC2, I wanted a way to automate the process and ignore files or folders of specific patterns (e.g., log files, temporary files, or large directories). With `escp`, you can define a `.scpignore` file, similar to `.gitignore`, to list patterns of files or directories to exclude during the `scp` upload.

## Features

- **Pattern-based exclusion**: Define file and directory patterns in a `.scpignore` file to automatically exclude them from being uploaded.


## Installation

To build the binary for `escp`, follow these steps:

1. Clone the repository:
    ```bash
    git clone https://github.com/Agent-Hellboy/escp.git
    cd escp
    ```

2. Build the binary:
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
    *.log          # Ignore all log files
    temp/          # Ignore everything in the temp directory
    secret/*.key   # Ignore all key files in the secret directory
    ```

2. Use `escp` to copy files just like you would with `scp`. The tool will automatically read the `.scpignore` file and skip files that match the patterns:
    ```bash
    ./escp source_directory/ user@ec2:/path/to/destination
    ```

3. `escp` will print the list of files being copied and will exclude those specified in the `.scpignore` file.

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
   - Test common patterns (`*.log`, `subdir/`, `subdir/*`).
   - Check that deeply nested directories are correctly ignored.

2. **Unit Tests**:
   - Write Go unit tests for key functions like `shouldIgnore`, `filterFiles`, and `parseDirectory`.

3. **Integration Tests**:
   - Run integration tests to ensure `escp` works as expected across different directory structures and file patterns.

4. **Use `.gitignore` Test Suite for Inspiration**:
   - Git’s `.gitignore` test suite can serve as a great reference. Build a similar test suite to ensure `escp` reliably ignores files and directories based on the defined patterns.


## Contributions

I am 100% sure this broken , please test it and try to contribute if possible

## License

This project is licensed under the MIT License.
