package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	sshclient "sshtui/internal/ssh"
)

// TransferMsg is sent to request a file transfer.
type TransferMsg struct {
	LocalPath  string
	RemotePath string
	Upload     bool
}

// TransferDoneMsg is sent when a transfer completes.
type TransferDoneMsg struct {
	Err error
}

// RefreshRemoteMsg requests a refresh of the remote file list.
type RefreshRemoteMsg struct{}

type panelFocus int

const (
	panelLocal panelFocus = iota
	panelRemote
)

// FileBrowserModel manages the dual-panel file browser.
type FileBrowserModel struct {
	localDir    string
	localFiles  []os.FileInfo
	localCursor int
	localScroll int

	remoteDir    string
	remoteFiles  []sshclient.RemoteFile
	remoteCursor int
	remoteScroll int

	focus            panelFocus
	width            int
	height           int
	transferring     bool
	transferProgress string
	statusMsg        string
	client           *sshclient.Client
}

// NewFileBrowserModel creates a new file browser model.
func NewFileBrowserModel(client *sshclient.Client, localDir, remoteDir string) FileBrowserModel {
	m := FileBrowserModel{
		localDir:  localDir,
		remoteDir: remoteDir,
		client:    client,
		focus:     panelLocal,
	}
	m.refreshLocal()
	return m
}

// SetDimensions sets the width and height for the file browser.
func (m *FileBrowserModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
}

func (m *FileBrowserModel) refreshLocal() {
	entries, err := os.ReadDir(m.localDir)
	if err != nil {
		m.localFiles = nil
		return
	}
	files := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err == nil {
			files = append(files, info)
		}
	}
	m.localFiles = files
	if m.localCursor >= len(m.localFiles) {
		m.localCursor = 0
	}
}

func refreshRemoteCmd(client *sshclient.Client, dir string) tea.Cmd {
	return func() tea.Msg {
		files, err := client.ListDir(dir)
		if err != nil {
			return remoteFilesMsg{files: nil, err: err}
		}
		return remoteFilesMsg{files: files}
	}
}

type remoteFilesMsg struct {
	files []sshclient.RemoteFile
	err   error
}

func (m FileBrowserModel) Init() tea.Cmd {
	return refreshRemoteCmd(m.client, m.remoteDir)
}

func (m FileBrowserModel) Update(msg tea.Msg) (FileBrowserModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case remoteFilesMsg:
		if msg.err == nil {
			m.remoteFiles = msg.files
			if m.remoteCursor >= len(m.remoteFiles) {
				m.remoteCursor = 0
			}
		} else {
			m.statusMsg = "Error: " + msg.err.Error()
		}

	case TransferDoneMsg:
		m.transferring = false
		if msg.Err != nil {
			m.statusMsg = "Transfer failed: " + msg.Err.Error()
		} else {
			m.statusMsg = "Transfer complete!"
			m.refreshLocal()
			return m, refreshRemoteCmd(m.client, m.remoteDir)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			if m.focus == panelLocal {
				m.focus = panelRemote
			} else {
				m.focus = panelLocal
			}

		case "up", "k":
			if m.focus == panelLocal && m.localCursor > 0 {
				m.localCursor--
			} else if m.focus == panelRemote && m.remoteCursor > 0 {
				m.remoteCursor--
			}

		case "down", "j":
			if m.focus == panelLocal && m.localCursor < len(m.localFiles)-1 {
				m.localCursor++
			} else if m.focus == panelRemote && m.remoteCursor < len(m.remoteFiles)-1 {
				m.remoteCursor++
			}

		case "enter":
			if m.focus == panelLocal && len(m.localFiles) > 0 {
				f := m.localFiles[m.localCursor]
				if f.IsDir() {
					m.localDir = filepath.Join(m.localDir, f.Name())
					m.localCursor = 0
					m.refreshLocal()
				}
			} else if m.focus == panelRemote && len(m.remoteFiles) > 0 {
				f := m.remoteFiles[m.remoteCursor]
				if f.IsDir {
					m.remoteDir = joinRemotePath(m.remoteDir, f.Name)
					m.remoteCursor = 0
					return m, refreshRemoteCmd(m.client, m.remoteDir)
				}
			}

		case "backspace":
			if m.focus == panelLocal {
				parent := filepath.Dir(m.localDir)
				if parent != m.localDir {
					m.localDir = parent
					m.localCursor = 0
					m.refreshLocal()
				}
			} else {
				parts := strings.Split(strings.TrimRight(m.remoteDir, "/"), "/")
				if len(parts) > 1 {
					m.remoteDir = strings.Join(parts[:len(parts)-1], "/")
					if m.remoteDir == "" {
						m.remoteDir = "/"
					}
					m.remoteCursor = 0
					return m, refreshRemoteCmd(m.client, m.remoteDir)
				}
			}

		case "ctrl+u":
			if !m.transferring && len(m.localFiles) > 0 {
				f := m.localFiles[m.localCursor]
				if !f.IsDir() {
					localPath := filepath.Join(m.localDir, f.Name())
					remotePath := joinRemotePath(m.remoteDir, f.Name())
					m.transferring = true
					m.statusMsg = "Uploading " + f.Name() + "..."
					client := m.client
					return m, func() tea.Msg {
						err := client.UploadFile(localPath, remotePath)
						return TransferDoneMsg{Err: err}
					}
				}
			}

		case "T":
			// Context-aware transfer: upload if local panel focused, download if remote panel focused.
			if !m.transferring {
				if m.focus == panelLocal && len(m.localFiles) > 0 {
					f := m.localFiles[m.localCursor]
					if !f.IsDir() {
						localPath := filepath.Join(m.localDir, f.Name())
						remotePath := joinRemotePath(m.remoteDir, f.Name())
						m.transferring = true
						m.statusMsg = "Uploading " + f.Name() + "..."
						client := m.client
						return m, func() tea.Msg {
							err := client.UploadFile(localPath, remotePath)
							return TransferDoneMsg{Err: err}
						}
					}
				} else if m.focus == panelRemote && len(m.remoteFiles) > 0 {
					f := m.remoteFiles[m.remoteCursor]
					if !f.IsDir {
						remotePath := joinRemotePath(m.remoteDir, f.Name)
						m.transferring = true
						m.statusMsg = "Downloading " + f.Name + "..."
						client := m.client
						localDir := m.localDir
						return m, func() tea.Msg {
							err := client.DownloadFile(remotePath, localDir)
							return TransferDoneMsg{Err: err}
						}
					}
				}
			}

		case "ctrl+d":
			if !m.transferring && len(m.remoteFiles) > 0 {
				f := m.remoteFiles[m.remoteCursor]
				if !f.IsDir {
					remotePath := joinRemotePath(m.remoteDir, f.Name)
					m.transferring = true
					m.statusMsg = "Downloading " + f.Name + "..."
					client := m.client
					localDir := m.localDir
					return m, func() tea.Msg {
						err := client.DownloadFile(remotePath, localDir)
						return TransferDoneMsg{Err: err}
					}
				}
			}
		}
	}
	return m, nil
}

var (
	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444")).
		Padding(0, 1)

	activePanelStyle = panelStyle.
		BorderForeground(lipgloss.Color("#7D56F4"))

	fileSelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FFFFFF"))

	dirStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#56D1F4")).
		Bold(true)

	fileStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC"))

	headerStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#AAAAAA")).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#444444"))

	statusBarStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)
)

func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.1fG", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1fM", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1fK", float64(size)/KB)
	default:
		return fmt.Sprintf("%dB", size)
	}
}

func (m FileBrowserModel) renderLocalPanel(panelWidth, panelHeight int) string {
	active := m.focus == panelLocal
	style := panelStyle
	if active {
		style = activePanelStyle
	}

	header := headerStyle.Width(panelWidth - 4).Render(
		fmt.Sprintf("Local: %s", truncatePath(m.localDir, panelWidth-10)),
	)

	var rows []string
	visibleHeight := panelHeight - 4
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	start := 0
	if m.localCursor >= visibleHeight {
		start = m.localCursor - visibleHeight + 1
	}

	for i := start; i < len(m.localFiles) && i < start+visibleHeight; i++ {
		f := m.localFiles[i]
		name := f.Name()
		size := formatSize(f.Size())
		modTime := f.ModTime().Format("2006-01-02")
		perm := f.Mode().String()
		permShort := perm
		if len(permShort) > 3 {
			permShort = permShort[:3]
		}

		var line string
		if f.IsDir() {
			line = dirStyle.Render("▸ " + truncate(name, 20) + "/")
		} else {
			line = fileStyle.Render(fmt.Sprintf("%-3s %-20s %6s  %s", permShort, truncate(name, 20), size, modTime))
		}

		if i == m.localCursor {
			line = fileSelectedStyle.Width(panelWidth - 4).Render(line)
		}
		rows = append(rows, line)
	}

	body := strings.Join(rows, "\n")
	content := header + "\n" + body
	return style.Width(panelWidth).Height(panelHeight).Render(content)
}

func (m FileBrowserModel) renderRemotePanel(panelWidth, panelHeight int) string {
	active := m.focus == panelRemote
	style := panelStyle
	if active {
		style = activePanelStyle
	}

	header := headerStyle.Width(panelWidth - 4).Render(
		fmt.Sprintf("Remote: %s", truncatePath(m.remoteDir, panelWidth-10)),
	)

	var rows []string
	visibleHeight := panelHeight - 4
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	start := 0
	if m.remoteCursor >= visibleHeight {
		start = m.remoteCursor - visibleHeight + 1
	}

	for i := start; i < len(m.remoteFiles) && i < start+visibleHeight; i++ {
		f := m.remoteFiles[i]
		size := formatSize(f.Size)
		modTime := f.ModTime.Format("2006-01-02")
		perm := f.Mode.String()
		if len(perm) > 3 {
			perm = perm[:3]
		}

		var line string
		if f.IsDir {
			line = dirStyle.Render("▸ " + truncate(f.Name, 20) + "/")
		} else {
			line = fileStyle.Render(fmt.Sprintf("%-3s %-20s %6s  %s", perm, truncate(f.Name, 20), size, modTime))
		}

		if i == m.remoteCursor {
			line = fileSelectedStyle.Width(panelWidth - 4).Render(line)
		}
		rows = append(rows, line)
	}

	body := strings.Join(rows, "\n")
	content := header + "\n" + body
	return style.Width(panelWidth).Height(panelHeight).Render(content)
}

func (m FileBrowserModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	panelWidth := m.width/2 - 2
	panelHeight := m.height - 4
	if panelHeight < 4 {
		panelHeight = 4
	}

	local := m.renderLocalPanel(panelWidth, panelHeight)
	remote := m.renderRemotePanel(panelWidth, panelHeight)
	panels := lipgloss.JoinHorizontal(lipgloss.Top, local, remote)

	statusLine := statusBarStyle.Render(
		fmt.Sprintf(" Tab: switch panels • Ctrl+U: upload • Ctrl+D: download • Enter: enter dir • Backspace: up | %s", m.statusMsg),
	)

	return lipgloss.JoinVertical(lipgloss.Left, panels, statusLine)
}

// SelectedLocalFile returns the full path of the currently selected local file.
func (m FileBrowserModel) SelectedLocalFile() string {
	if len(m.localFiles) == 0 {
		return ""
	}
	return filepath.Join(m.localDir, m.localFiles[m.localCursor].Name())
}

// SelectedRemoteFile returns the remote path of the currently selected remote file.
func (m FileBrowserModel) SelectedRemoteFile() string {
	if len(m.remoteFiles) == 0 {
		return ""
	}
	return joinRemotePath(m.remoteDir, m.remoteFiles[m.remoteCursor].Name)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func truncatePath(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n+1:]
}

// joinRemotePath joins a remote directory and a filename, avoiding double slashes.
func joinRemotePath(dir, name string) string {
	dir = strings.TrimRight(dir, "/")
	if dir == "" {
		return "/" + name
	}
	return dir + "/" + name
}
