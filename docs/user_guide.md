# User Guide

## Getting Started

### Building

```sh
go build -o sshtui ./cmd/main.go
```

### Running

```sh
./sshtui
# or run directly without building:
go run ./cmd/main.go
```

The application starts in fullscreen alternate-screen mode with mouse support enabled.

## Connecting to a Server

When sshtui launches, you see the connection form. Fill in the fields using **Tab** or **arrow keys** to navigate between them:

| Field    | Description                                      | Default    |
| -------- | ------------------------------------------------ | ---------- |
| Host     | Hostname or IP address                           | (required) |
| Port     | SSH port                                         | 22         |
| Username | SSH username                                     | (required) |
| Password | Password for password auth                       | (optional) |
| SSH Key  | Path to private key file (e.g., `~/.ssh/id_rsa`) | (optional) |

Press **Enter** to connect. At least one authentication method (password or SSH key) must be provided.

### Recent Connections

If you've connected before, a "Recent Connections" list appears to the right of the form. Up to 10 recent connections are stored automatically in `~/.config/sshtui/connections.json`.

### Host Key Verification

On first connection to a new server, sshtui displays the host's SHA256 fingerprint:

```text
The authenticity of host 'example.com' can't be established.

Host key fingerprint:
  SHA256:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

Do you want to continue? (Enter=yes, n=no)
```

- Press **Enter** to accept and continue connecting
- Press **n** to reject and return to the connection form

Accepted host keys are remembered for the duration of the session (they are not persisted to disk).

## Main Interface

After connecting, the screen is split into three sections:

```text
┌─────────────────────────────────────────┐
│ ● user@host                          +  │  ← Tab bar
├─────────────────────────────────────────┤
│                                         │
│  SSH Terminal                           │  ← Upper half
│                                         │
├────────────────────┬────────────────────┤
│  Local Files       │  Remote Files      │  ← Lower half
│  /home/user/...    │  /home/remote/...  │
│  > file1.txt       │  > file2.txt       │
│    folder/         │    folder/         │
└────────────────────┴────────────────────┘
  Status bar with key hints
```

### Focus

The interface has two focus zones:

- **Terminal** (default) — Keystrokes are sent directly to the remote shell
- **File browser** — Keystrokes control the file browser panels

Press **Ctrl+T** to toggle between them. The active pane is highlighted with a purple border.

When the terminal is focused, it behaves like a normal SSH session — you can run commands, use tab completion, navigate with arrow keys, and use Ctrl shortcuts (Ctrl+C, Ctrl+D, Ctrl+Z, etc.).

## File Browser

The file browser has two panels side by side: **local** (left) and **remote** (right).

### Navigation

| Key          | Action                                       |
| ------------ | -------------------------------------------- |
| `Tab`        | Switch focus between local and remote panels |
| `Up` / `k`   | Move cursor up                               |
| `Down` / `j` | Move cursor down                             |
| `Enter`      | Enter the selected directory                 |
| `Backspace`  | Go to parent directory                       |

The active panel has a purple border. The current directory path is shown in the panel header.

Directories are displayed with a `▸` prefix and a trailing `/` in cyan. Files show permissions, name, size, and modification date.

### File Transfers

sshtui uses the SCP protocol for file transfers (not SFTP). Transfers operate on the currently selected file and the opposite panel's directory.

| Key      | Action                                                   | Condition                                                         |
| -------- | -------------------------------------------------------- | ----------------------------------------------------------------- |
| `Ctrl+U` | Upload selected local file to current remote directory   | Always uploads                                                    |
| `Ctrl+D` | Download selected remote file to current local directory | Always downloads                                                  |
| `T`      | Context-aware transfer                                   | Uploads if local panel focused, downloads if remote panel focused |

During a transfer, the status bar shows progress. After completion, both panels refresh automatically.

**Note:** Only individual files can be transferred — directory transfers are not supported.

## Tabs

sshtui supports multiple simultaneous SSH connections, each in its own tab.

| Key      | Action                                                 |
| -------- | ------------------------------------------------------ |
| `Ctrl+N` | Open a new connection tab (returns to connection form) |
| `Ctrl+W` | Close the current tab (disconnects SSH session)        |

Each tab maintains its own independent terminal session and file browser state. The tab bar at the top shows all connections — a filled dot (`●`) indicates a connected tab.

Closing the last tab returns to the connection form.

## Help Overlay

Press **?** to toggle a help overlay showing all key bindings. Press **?** again to dismiss it. The help overlay is only available in the main view (not on the connection form).

## Complete Key Binding Reference

### Connection Form

| Key                | Action         |
| ------------------ | -------------- |
| `Tab` / `Down`     | Next field     |
| `Shift+Tab` / `Up` | Previous field |
| `Enter`            | Connect        |
| `Ctrl+C` / `Esc`   | Quit           |

### Main View — Global

| Key      | Action                                |
| -------- | ------------------------------------- |
| `Ctrl+T` | Toggle focus: terminal ↔ file browser |
| `Ctrl+N` | New connection tab                    |
| `Ctrl+W` | Close current tab                     |
| `?`      | Toggle help overlay                   |
| `Ctrl+C` | Quit (closes all connections)         |

### Main View — File Browser (when focused)

| Key          | Action                                 |
| ------------ | -------------------------------------- |
| `Tab`        | Switch between local and remote panels |
| `Up` / `k`   | Move cursor up                         |
| `Down` / `j` | Move cursor down                       |
| `Enter`      | Enter directory                        |
| `Backspace`  | Go to parent directory                 |
| `Ctrl+U`     | Upload selected local file             |
| `Ctrl+D`     | Download selected remote file          |
| `T`          | Context-aware transfer                 |

### Main View — Terminal (when focused)

All keystrokes are forwarded to the remote shell as ANSI sequences. Standard terminal shortcuts work as expected (Ctrl+C, Ctrl+D, Ctrl+Z, arrow keys, etc.).

## Configuration

### Config File Location

```text
~/.config/sshtui/connections.json
```

### Config Format

```json
{
  "recent_connections": [
    {
      "name": "user@example.com",
      "host": "example.com",
      "port": "22",
      "username": "user",
      "password": "...",
      "key_path": "/home/user/.ssh/id_rsa"
    }
  ]
}
```

The config file is created automatically on first connection. Connections are deduplicated by host + port + username.

### Security Note

Passwords are stored in plaintext in the config file. For sensitive environments, use SSH key authentication and leave the password field empty.

## Troubleshooting

### "No auth method provided"

You must supply at least one of: password or SSH key path. Check that the SSH key path is correct and the file exists.

### Connection timeout

The SSH connection has a 10-second timeout. Verify the host is reachable and the port is correct.

### Remote file listing shows nothing

Remote directory listing uses `ls -la` over SSH. If the remote shell has unusual `ls` output formatting, files may not parse correctly.

### Terminal output looks garbled

The PTY is set to `xterm-256color`. If the remote server doesn't support this terminal type, set the `TERM` environment variable after connecting:

```sh
export TERM=xterm
```
