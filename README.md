# escp

`escp` copies files over SSH like `scp`, but skips files matched by a `.scpignore` file.

[![codecov](https://codecov.io/gh/Agent-Hellboy/escp/graph/badge.svg?token=596VLH7OJR)](https://codecov.io/gh/Agent-Hellboy/escp)

## Features

- `.scpignore` support with `.gitignore`-style patterns.
- Native SSH transfer using `golang.org/x/crypto/ssh`.
- Copies files and empty directories.

## Install

Requires Go 1.25 or newer.

```bash
git clone https://github.com/Agent-Hellboy/escp.git
cd escp
go build -o escp
```

## Usage

Create `.scpignore` in the directory where you run `escp`:

```gitignore
*.log
temp/
/build
!keep.log
```

Copy a directory:

```bash
./escp ./project user@host:/remote/path
```

Use a custom SSH port:

```bash
./escp ./project user@host:2222:/remote/path
```

SSH auth uses your agent, `ESCP_SSH_PASSWORD`, or unencrypted default keys in `~/.ssh`. The remote host must exist in `~/.ssh/known_hosts`.

## Test

```bash
go test ./...
```

Remote E2E tests are opt-in:

```bash
ESCP_E2E_SSH_URL='user:password@host[:port]' go test -v ./...
```

## Contributions

This project is still rough. Please test it and contribute fixes.

## License

MIT
