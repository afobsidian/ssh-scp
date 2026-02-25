# Software Architecture

## Overview

sshtui is a Go terminal application built on the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework, which implements the Elm architecture (Model → Update → View). The application provides interactive SSH terminal access and SCP-based file transfers through a keyboard-driven TUI.

## High-Level Design

```text
┌─────────────────────────────────────────────────────┐
│                    cmd/main.go                       │
│                    AppModel                          │
│  ┌──────────────┬──────────────┬──────────────────┐  │
│  │ stateConn    │ stateHostKey │ stateMain        │  │
│  │              │              │ ┌──────────────┐ │  │
│  │ Connection   │ Host Key     │ │ Tab Bar      │ │  │
│  │ Model        │ Prompt       │ ├──────────────┤ │  │
│  │              │              │ │ Terminal     │ │  │
│  │              │              │ │ Model        │ │  │
│  │              │              │ ├──────────────┤ │  │
│  │              │              │ │ FileBrowser  │ │  │
│  │              │              │ │ Model        │ │  │
│  │              │              │ └──────────────┘ │  │
│  └──────────────┴──────────────┴──────────────────┘  │
└──────────────────────┬──────────────────────────────┘
                       │
         ┌─────────────┼─────────────┐
         │             │             │
    internal/ssh   internal/ui   internal/config
    (SSH+SCP)      (sub-models)  (persistence)
```

## Application States

`AppModel` transitions through three states via the `appState` enum:

| State           | Enum                 | Description                                                                                  |
| --------------- | -------------------- | -------------------------------------------------------------------------------------------- |
| Connection form | `stateConnection`    | Text inputs for host, port, username, password, SSH key path. Shows recent connections list. |
| Host key prompt | `stateHostKeyPrompt` | Displays SHA256 fingerprint for unknown hosts. User accepts (Enter) or rejects (n).          |
| Main view       | `stateMain`          | Split layout: tab bar at top, terminal in upper half, dual-pane file browser in lower half.  |

State transitions:

```text
stateConnection ──ConnectMsg──► connectCmd() ──┬──connectedMsg──► stateMain
                                               │
                                               └──hostKeyMsg──► stateHostKeyPrompt
                                                                    │
                                                        Enter ──────┘ (retry with accepted key)
```

## Package Responsibilities

### `cmd/main.go` — Root Model & Orchestration

The `AppModel` struct is the top-level Bubble Tea model. It owns:

- **Screen state** (`appState`) — which screen is active
- **Focus state** (`focusPane`) — whether keystrokes go to the terminal or file browser
- **Parallel slices** for per-tab data: `tabs[]`, `clients[]`, `terminals[]`, `browsers[]`
- **Message routing** — dispatches messages to sub-models or handles them directly
- **SSH connection lifecycle** — `connectCmd()` and `connectWithAcceptedKey()` produce `tea.Cmd` closures
- **Key-to-ANSI translation** — `keyToBytes()` maps Bubble Tea key events to byte sequences for SSH stdin

Global state: A package-level `prog *tea.Program` is set in `main()` and passed to `TerminalModel` via `SetProgram()`. This is the only global mutable state in the application.

### `internal/ssh` — SSH Client Wrapper

`Client` wraps `golang.org/x/crypto/ssh` and provides:

- **Connection** — `New()` dials TCP with a 10-second timeout, supports password and public key auth
- **PTY sessions** — `StartTerminal()` requests an `xterm-256color` PTY and starts a shell
- **Terminal resize** — `ResizePty()` sends window-change requests
- **Remote directory listing** — `ListDir()` runs `ls -la` over SSH and parses the output (no SFTP dependency)
- **File transfers** — `UploadFile()` and `DownloadFile()` use `go-scp` (SCP protocol over the existing SSH connection)

Path safety: `shellQuote()` wraps remote paths in single quotes with proper escaping to prevent shell injection.

### `internal/ui` — UI Sub-Models

Each file owns one logical UI component:

| File             | Model/Function     | Purpose                                                      |
| ---------------- | ------------------ | ------------------------------------------------------------ |
| `connection.go`  | `ConnectionModel`  | Form with 5 text inputs + recent connections list            |
| `terminal.go`    | `TerminalModel`    | SSH PTY I/O buffer with 100KB cap (trims to 50KB)            |
| `filebrowser.go` | `FileBrowserModel` | Dual-pane (local/remote) file browser with cursor navigation |
| `tabs.go`        | `RenderTabBar()`   | Renders the tab bar with active/inactive styling             |
| `help.go`        | `RenderHelp()`     | Centered help overlay with key binding reference             |

### `internal/config` — Persistence

Manages `~/.config/sshtui/connections.json`:

- `Load()` / `Save()` — JSON serialization with `0600` file permissions
- `AddRecent()` — Upserts connections, caps at 10 entries
- Config directory created with `0700` permissions on first save

## Message Flow

All async operations in the application use Bubble Tea's command pattern. No blocking I/O occurs in `Update()`.

### SSH Connection Flow

```text
1. User fills form → presses Enter
2. ConnectionModel.Update() returns tea.Cmd producing ConnectMsg
3. AppModel.Update() receives ConnectMsg → returns connectCmd()
4. connectCmd() runs in goroutine:
   a. Builds auth methods (key first, then password)
   b. Dials SSH with hostKeyCallback
   c. If host key unknown → returns hostKeyMsg → shows prompt
   d. If success → returns connectedMsg
5. On connectedMsg:
   a. Creates new Tab, Client, TerminalModel, FileBrowserModel
   b. Starts PTY session (async)
   c. Requests remote directory listing (async)
```

### Terminal I/O Flow

```text
Input:  KeyMsg → keyToBytes() → TerminalModel.Write() → ssh.Session.StdinPipe
Output: ssh.Session.Stdout → terminalWriter.Write() → tea.Program.Send(TerminalOutputMsg)
        → AppModel.Update() → TerminalModel.AppendOutput() → buffer
```

`terminalWriter` is the only component that calls `tea.Program.Send()` directly, pushing output from the SSH goroutine into the Bubble Tea event loop.

### File Transfer Flow

```text
1. User presses Ctrl+U (upload) or Ctrl+D (download) or T (context-aware)
2. FileBrowserModel.Update() returns tea.Cmd closure calling client.UploadFile/DownloadFile
3. Closure runs in goroutine, returns TransferDoneMsg
4. AppModel routes TransferDoneMsg → FileBrowserModel.Update()
5. On success: refreshes local files, triggers remote dir refresh via refreshRemoteCmd()
```

## Focus & Input Routing

The `focus` field on `AppModel` determines keystroke routing in `stateMain`:

- **`paneTerminal`** — Keys are converted via `keyToBytes()` and written to SSH stdin. The terminal receives raw ANSI sequences.
- **`paneFileBrowser`** — Keys are dispatched to `FileBrowserModel.Update()` for cursor movement, directory navigation, and transfer commands.

`Ctrl+T` toggles between panes. Within the file browser, `Tab` switches between local and remote panels.

## Tab Management

Each connection creates a tab with isolated state:

```text
tabs[i]      → Tab{Title, Connected}     (display metadata)
clients[i]   → *sshclient.Client         (SSH connection)
terminals[i] → *TerminalModel            (PTY session + output buffer)
browsers[i]  → FileBrowserModel           (local/remote file state)
```

These parallel slices are indexed by `activeTab`. Closing a tab (`Ctrl+W`) removes entries from all four slices and cleans up the SSH session and client connection.

## Styling

All UI styling uses [Lip Gloss](https://github.com/charmbracelet/lipgloss). Styles are declared as package-level `var` blocks colocated with the views that use them.

- **Accent color**: `#7D56F4` (purple) — used for active borders, tab highlights, selected items
- **Inactive borders**: `#444444`
- **Text colors**: `#888888` (secondary), `#CCCCCC` (file names), `#56D1F4` (directories)
- **Error color**: `#FF5555`

## Dependencies

| Package                   | Version | Purpose                                       |
| ------------------------- | ------- | --------------------------------------------- |
| `charmbracelet/bubbletea` | v1.3.4  | TUI framework (Elm architecture)              |
| `charmbracelet/bubbles`   | v0.20.0 | Pre-built TUI components (text inputs, lists) |
| `charmbracelet/lipgloss`  | v1.1.0  | Terminal styling and layout                   |
| `golang.org/x/crypto`     | v0.35.0 | SSH protocol implementation                   |
| `bramvdbogaerde/go-scp`   | v1.5.0  | SCP file transfer over SSH                    |

## Known Limitations

- **No SFTP** — By design; the app targets servers where SFTP is unavailable
- **No host key persistence** — Accepted host keys are stored in-memory (`acceptedHosts` map) and lost on restart
- **No test suite** — No unit or integration tests exist yet
- **Password stored in plaintext** — Recent connections config stores passwords without encryption
- **Remote listing via `ls -la`** — Parsing shell output is fragile compared to SFTP directory enumeration
- **Global `tea.Program`** — The `prog` variable is package-level mutable state in `cmd/main.go`
