# sshtui

A terminal user interface (TUI) for interactive SSH terminal access and SCP file transfers, built in Go. Designed for servers that lack SFTP support.

![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)

## Features

- **Interactive SSH terminal** — Full PTY session with xterm-256color support
- **Dual-pane file browser** — Side-by-side local and remote file navigation
- **SCP file transfers** — Upload and download files without SFTP
- **Tabbed connections** — Multiple SSH sessions in separate tabs
- **Host key verification** — SHA256 fingerprint prompt on first connect
- **Recent connections** — Automatically saved and restored between sessions
- **Keyboard-driven** — No mouse required; full keyboard navigation

## Requirements

- Go 1.24 or later

## Installation

```sh
# Clone and build
git clone <repo-url> && cd ssh-scp
go build -o sshtui ./cmd/main.go

# Or run directly
go run ./cmd/main.go
```

## Quick Start

1. Launch: `./sshtui`
2. Fill in the connection form (host, port, username, password or SSH key path)
3. Press **Enter** to connect
4. If prompted, verify the host key fingerprint and press **Enter** to accept
5. You're in — the terminal is focused by default

## Key Bindings

| Key         | Action                                                            |
| ----------- | ----------------------------------------------------------------- |
| `Ctrl+T`    | Toggle focus between terminal and file browser                    |
| `Ctrl+U`    | Upload selected local file to remote directory                    |
| `Ctrl+D`    | Download selected remote file to local directory                  |
| `Ctrl+N`    | Open a new connection tab                                         |
| `Ctrl+W`    | Close the current tab                                             |
| `Tab`       | Switch between local and remote file panels                       |
| `Enter`     | Navigate into a directory                                         |
| `Backspace` | Go up one directory                                               |
| `T`         | Context-aware transfer (upload or download based on active panel) |
| `?`         | Toggle help overlay                                               |
| `Ctrl+C`    | Quit                                                              |

## Authentication

sshtui supports two authentication methods:

- **Password** — Enter in the connection form
- **SSH key** — Provide the path to your private key file (e.g., `~/.ssh/id_rsa`)

Both can be provided simultaneously; key auth is attempted first.

## Configuration

Recent connections are stored in:

```text
~/.config/sshtui/connections.json
```

Up to 10 recent connections are saved automatically. Passwords are stored in plaintext in this file — use SSH key authentication for sensitive environments.

## Project Structure

```text
cmd/main.go              # Application entry point and root model
internal/
  config/config.go       # Connection persistence (~/.config/sshtui/)
  ssh/client.go          # SSH client, PTY, SCP transfers, remote ls parsing
  ui/
    connection.go        # Connection form screen
    terminal.go          # Interactive SSH terminal view
    filebrowser.go       # Dual-pane local/remote file browser
    tabs.go              # Tab bar rendering
    help.go              # Help overlay
```

## Documentation

- [User Guide](docs/user_guide.md) — Detailed usage instructions
- [Software Architecture](docs/software_architecture.md) — Technical design and internals

## License

See [LICENSE](LICENSE) for details.
