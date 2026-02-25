# Copilot Instructions — ssh-scp (ssh-scp)

## Project Overview

A Go TUI application for interactive SSH terminal access and SCP file transfers, targeting servers without SFTP support. Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm architecture) with [Lip Gloss](https://github.com/charmbracelet/lipgloss) styling.

## Architecture

The app follows Bubble Tea's **Model → Update → View** loop. The root model lives in `cmd/main.go` (`AppModel`) and orchestrates three screens via `appState`: connection form, host-key verification prompt, and the main split view.

### Key components

| Package           | Responsibility                                                                                                                                                      |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cmd/main.go`     | Root model, app states, message routing, SSH connection lifecycle, key-to-ANSI mapping                                                                              |
| `internal/config` | JSON config at `~/.config/ssh-scp/connections.json` — load/save recent connections (max 10)                                                                         |
| `internal/ssh`    | SSH client wrapper — PTY sessions, `ls -la` parsing for remote dir listing, SCP upload/download via `go-scp`                                                        |
| `internal/ui`     | Bubble Tea sub-models: `ConnectionModel` (form + recent list), `TerminalModel` (PTY I/O buffer), `FileBrowserModel` (dual-pane local/remote), tab bar, help overlay |

### Data flow

1. `ConnectionModel` emits `ConnectMsg` → `cmd/main.go` runs `connectCmd` (async SSH dial)
2. Host key verification uses `hostKeyPendingError` to pause connection and show a prompt
3. `connectedMsg` triggers creation of `TerminalModel` + `FileBrowserModel` per tab
4. Terminal output flows: SSH session → `terminalWriter` → `tea.Program.Send(TerminalOutputMsg)` → `AppendOutput` buffer
5. File transfers: `FileBrowserModel` invokes `client.UploadFile`/`DownloadFile` via async `tea.Cmd`, emits `TransferDoneMsg`

### Focus & input routing

`AppModel.focus` (`paneTerminal` | `paneFileBrowser`) controls where keystrokes go. When terminal is focused, keys are converted to ANSI bytes via `keyToBytes()` and written directly to the SSH stdin pipe. Ctrl+T toggles focus.

## Build & Run

```sh
go build -o ./bin/ssh-scp ./cmd/main.go   # build
go run ./cmd/main.go                # run directly
```

Module name is `ssh-scp` (see `go.mod`). No test suite exists yet.

## Conventions & Patterns

- **Message-driven architecture**: All async operations (connecting, file transfers, remote dir listing) return `tea.Cmd` functions that produce typed messages. Never perform blocking I/O inside `Update`.
- **One file per UI concern**: Each `internal/ui/*.go` file owns one sub-model or rendering helper. Keep this separation when adding new UI components.
- **Sub-models don't own `tea.Program`**: Only `TerminalModel` holds a `*tea.Program` reference (set via `SetProgram`) for pushing output. Other models communicate via returned `tea.Cmd`/messages.
- **Styles are colocated**: Lip Gloss styles are declared as package-level `var` blocks in the same file as the view that uses them. The accent color is `#7D56F4`.
- **Remote path handling**: Use `joinRemotePath()` and `shellQuote()` for remote paths — never use `filepath.Join` for remote paths (it uses OS-specific separators). Shell injection is mitigated via single-quote escaping in `shellQuote`.
- **Tab-per-connection model**: Each SSH connection gets its own tab with independent `Client`, `TerminalModel`, and `FileBrowserModel` instances, stored as parallel slices in `AppModel`.
- **Terminal buffer management**: `TerminalModel` caps its buffer at 100KB, trimming to the last 50KB when exceeded.

## Adding a New Feature

1. Define a new message type (e.g., `type MyActionMsg struct{...}`)
2. Handle it in `AppModel.Update` in `cmd/main.go` for routing, or in the relevant sub-model's `Update`
3. If it needs async work, return a `tea.Cmd` closure that produces the result message
4. Add any new UI rendering in the appropriate `internal/ui/*.go` file
