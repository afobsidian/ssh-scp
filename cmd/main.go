package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh"
	"sshtui/internal/config"
	sshclient "sshtui/internal/ssh"
	"sshtui/internal/ui"
)

// prog is the global bubbletea program reference used by terminal writers.
var prog *tea.Program

// appState represents which screen is shown.
type appState int

const (
	stateConnection appState = iota
	stateMain
	stateHostKeyPrompt
)

// focusPane tracks which pane has keyboard focus in the main view.
type focusPane int

const (
	paneTerminal focusPane = iota
	paneFileBrowser
)

// pendingConnection holds connection info during host key verification.
type pendingConnection struct {
	conn     config.Connection
	hostKey  ssh.PublicKey
	hostname string
	remote   net.Addr
}

// AppModel is the root application model.
type AppModel struct {
	state     appState
	width     int
	height    int
	cfg       *config.Config
	connModel ui.ConnectionModel
	tabs      []ui.Tab
	activeTab int
	clients   []*sshclient.Client
	terminals []*ui.TerminalModel
	browsers  []ui.FileBrowserModel
	focus     focusPane
	pending   *pendingConnection
	showHelp  bool
	err       string
}

func initialModel() AppModel {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	return AppModel{
		state:     stateConnection,
		cfg:       cfg,
		connModel: ui.NewConnectionModel(cfg),
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.connModel.Init()
}

// hostKeyMsg is sent when a host key needs user verification.
type hostKeyMsg struct {
	conn     config.Connection
	key      ssh.PublicKey
	hostname string
	remote   net.Addr
}

// connectedMsg is sent when an SSH connection attempt completes.
type connectedMsg struct {
	client *sshclient.Client
	conn   config.Connection
	err    error
}

// acceptedHosts stores fingerprints of host keys the user has accepted.
var acceptedHosts = map[string]string{}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		newConn, cmd := m.connModel.Update(msg)
		m.connModel = newConn.(ui.ConnectionModel)
		for i := range m.browsers {
			browser, _ := m.browsers[i].Update(msg)
			m.browsers[i] = browser
		}
		for _, t := range m.terminals {
			if t != nil {
				t.Resize(m.width, m.height/2)
			}
		}
		return m, cmd

	case ui.ConnectMsg:
		return m, connectCmd(msg.Conn)

	case connectedMsg:
		if msg.err != nil {
			m.err = "Connection failed: " + msg.err.Error()
			m.state = stateConnection
			return m, nil
		}
		tabTitle := ui.TabTitle(msg.conn.Username, msg.conn.Host, len(m.tabs))
		m.tabs = append(m.tabs, ui.Tab{Title: tabTitle, Connected: true})
		m.clients = append(m.clients, msg.client)
		terminal := ui.NewTerminalModel(msg.client)
		terminal.SetProgram(prog)
		m.terminals = append(m.terminals, terminal)

		homeDir := "~"
		if sess, err := msg.client.NewSession(); err == nil {
			if out, err := sess.Output("echo $HOME"); err == nil {
				homeDir = strings.TrimSpace(string(out))
			}
			sess.Close()
		}

		localDir, _ := os.Getwd()
		browser := ui.NewFileBrowserModel(msg.client, localDir, homeDir)
		m.browsers = append(m.browsers, browser)
		m.activeTab = len(m.tabs) - 1
		m.state = stateMain

		var cmds []tea.Cmd
		t := terminal
		cmds = append(cmds, func() tea.Msg {
			if err := t.StartSession(); err != nil {
				return ui.TerminalOutputMsg{Data: []byte("Error starting terminal: " + err.Error() + "\r\n")}
			}
			return nil
		})
		cmds = append(cmds, browser.Init())
		return m, tea.Batch(cmds...)

	case hostKeyMsg:
		m.pending = &pendingConnection{
			conn:     msg.conn,
			hostKey:  msg.key,
			hostname: msg.hostname,
			remote:   msg.remote,
		}
		m.state = stateHostKeyPrompt
		return m, nil

	case ui.TerminalOutputMsg:
		if m.activeTab < len(m.terminals) && m.terminals[m.activeTab] != nil {
			m.terminals[m.activeTab].AppendOutput(msg.Data)
		}
		return m, nil

	case ui.TransferDoneMsg:
		if m.activeTab < len(m.browsers) {
			browser, cmd := m.browsers[m.activeTab].Update(msg)
			m.browsers[m.activeTab] = browser
			return m, cmd
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cleanup()
			return m, tea.Quit

		case "?":
			if m.state == stateMain {
				m.showHelp = !m.showHelp
				return m, nil
			}

		case "ctrl+n":
			if m.state == stateMain {
				m.state = stateConnection
				m.connModel = ui.NewConnectionModel(m.cfg)
				return m, m.connModel.Init()
			}

		case "ctrl+w":
			if m.state == stateMain && len(m.tabs) > 0 {
				m.closeTab(m.activeTab)
				if len(m.tabs) == 0 {
					m.state = stateConnection
					m.connModel = ui.NewConnectionModel(m.cfg)
					return m, m.connModel.Init()
				}
				return m, nil
			}

		case "ctrl+t":
			if m.state == stateMain {
				if m.focus == paneTerminal {
					m.focus = paneFileBrowser
				} else {
					m.focus = paneTerminal
				}
				return m, nil
			}

		case "enter":
			if m.state == stateHostKeyPrompt && m.pending != nil {
				pending := m.pending
				m.pending = nil
				m.state = stateConnection
				return m, connectWithAcceptedKey(pending)
			}

		case "n", "N":
			if m.state == stateHostKeyPrompt {
				m.pending = nil
				m.state = stateConnection
				m.err = "Connection aborted: host key rejected"
				return m, nil
			}
		}

		if m.state == stateMain && !m.showHelp {
			if m.focus == paneTerminal {
				if m.activeTab < len(m.terminals) && m.terminals[m.activeTab] != nil {
					_ = m.terminals[m.activeTab].Write(keyToBytes(msg))
				}
				return m, nil
			} else {
				if m.activeTab < len(m.browsers) {
					browser, cmd := m.browsers[m.activeTab].Update(msg)
					m.browsers[m.activeTab] = browser
					return m, cmd
				}
			}
		}

		if m.state == stateConnection {
			newConn, cmd := m.connModel.Update(msg)
			m.connModel = newConn.(ui.ConnectionModel)
			return m, cmd
		}
	}

	if m.state == stateConnection {
		newConn, cmd := m.connModel.Update(msg)
		m.connModel = newConn.(ui.ConnectionModel)
		return m, cmd
	}

	return m, nil
}

func (m AppModel) View() string {
	if m.showHelp {
		return ui.RenderHelp(m.width, m.height)
	}

	switch m.state {
	case stateConnection:
		return m.connModel.View()
	case stateHostKeyPrompt:
		return m.renderHostKeyPrompt()
	case stateMain:
		return m.renderMain()
	}
	return ""
}

func (m AppModel) renderHostKeyPrompt() string {
	if m.pending == nil {
		return ""
	}
	fp := fingerprintSHA256(m.pending.hostKey)
	prompt := fmt.Sprintf(
		"The authenticity of host '%s' can't be established.\n\nHost key fingerprint:\n  %s\n\nDo you want to continue? (Enter=yes, n=no)",
		m.pending.hostname, fp,
	)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF9500")).
		Padding(1, 3).
		Render(prompt)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m AppModel) renderMain() string {
	if m.width == 0 {
		return "Initializing..."
	}

	tabBar := ui.RenderTabBar(m.tabs, m.activeTab, m.width)

	var body string
	if m.activeTab < len(m.terminals) {
		termHeight := m.height / 2
		browserHeight := m.height - termHeight - 3

		termView := m.terminals[m.activeTab].RenderTerminal(
			m.focus == paneTerminal, m.width, termHeight,
		)

		var browserView string
		if m.activeTab < len(m.browsers) {
			m.browsers[m.activeTab].SetDimensions(m.width, browserHeight)
			browserView = m.browsers[m.activeTab].View()
		}

		body = lipgloss.JoinVertical(lipgloss.Left, termView, browserView)
	}

	var errLine string
	if m.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Render(m.err)
	}

	statusLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		Render(" Ctrl+T: toggle focus • Ctrl+N: new tab • Ctrl+W: close tab • ?: help • Ctrl+C: quit" + errLine)

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, statusLine)
}

func (m *AppModel) cleanup() {
	for _, t := range m.terminals {
		if t != nil {
			t.Close()
		}
	}
	for _, c := range m.clients {
		if c != nil {
			c.Close()
		}
	}
}

func (m *AppModel) closeTab(idx int) {
	if idx < len(m.terminals) && m.terminals[idx] != nil {
		m.terminals[idx].Close()
	}
	if idx < len(m.clients) && m.clients[idx] != nil {
		m.clients[idx].Close()
	}
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
	if idx < len(m.clients) {
		m.clients = append(m.clients[:idx], m.clients[idx+1:]...)
	}
	if idx < len(m.terminals) {
		m.terminals = append(m.terminals[:idx], m.terminals[idx+1:]...)
	}
	if idx < len(m.browsers) {
		m.browsers = append(m.browsers[:idx], m.browsers[idx+1:]...)
	}
	if m.activeTab >= len(m.tabs) && m.activeTab > 0 {
		m.activeTab = len(m.tabs) - 1
	}
}

// hostKeyPendingError is returned when host key verification needs user confirmation.
type hostKeyPendingError struct {
	hostname string
	remote   net.Addr
	key      ssh.PublicKey
	conn     config.Connection
}

func (e *hostKeyPendingError) Error() string {
	return "host key verification required for " + e.hostname
}

func connectCmd(conn config.Connection) tea.Cmd {
	return func() tea.Msg {
		var authMethods []ssh.AuthMethod
		if conn.KeyPath != "" {
			am, err := sshclient.PubKeyAuth(conn.KeyPath)
			if err == nil {
				authMethods = append(authMethods, am)
			}
		}
		if conn.Password != "" {
			authMethods = append(authMethods, sshclient.PasswordAuth(conn.Password))
		}
		if len(authMethods) == 0 {
			return connectedMsg{err: fmt.Errorf("no auth method provided"), conn: conn}
		}

		hostKeyCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fp := fingerprintSHA256(key)
			if acceptedHosts[hostname] == fp {
				return nil
			}
			return &hostKeyPendingError{
				hostname: hostname,
				remote:   remote,
				key:      key,
				conn:     conn,
			}
		}

		client, err := sshclient.New(conn.Host, conn.Port, conn.Username, authMethods, hostKeyCallback)
		if err != nil {
			if hkErr, ok := err.(*hostKeyPendingError); ok {
				return hostKeyMsg{
					conn:     hkErr.conn,
					key:      hkErr.key,
					hostname: hkErr.hostname,
					remote:   hkErr.remote,
				}
			}
			return connectedMsg{err: err, conn: conn}
		}
		return connectedMsg{client: client, conn: conn}
	}
}

func connectWithAcceptedKey(pending *pendingConnection) tea.Cmd {
	return func() tea.Msg {
		conn := pending.conn
		fp := fingerprintSHA256(pending.hostKey)
		acceptedHosts[pending.hostname] = fp

		var authMethods []ssh.AuthMethod
		if conn.KeyPath != "" {
			am, err := sshclient.PubKeyAuth(conn.KeyPath)
			if err == nil {
				authMethods = append(authMethods, am)
			}
		}
		if conn.Password != "" {
			authMethods = append(authMethods, sshclient.PasswordAuth(conn.Password))
		}

		hkCallback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			kfp := fingerprintSHA256(key)
			if acceptedHosts[hostname] == kfp {
				return nil
			}
			return fmt.Errorf("host key changed")
		}

		client, err := sshclient.New(conn.Host, conn.Port, conn.Username, authMethods, hkCallback)
		if err != nil {
			return connectedMsg{err: err, conn: conn}
		}
		return connectedMsg{client: client, conn: conn}
	}
}

// fingerprintSHA256 computes the SHA256 fingerprint of a host key,
// matching the modern OpenSSH fingerprint format (e.g. "SHA256:...").
func fingerprintSHA256(key ssh.PublicKey) string {
	return ssh.FingerprintSHA256(key)
}

// keyToBytes converts a bubbletea KeyMsg to the ANSI byte sequence for the SSH terminal.
func keyToBytes(msg tea.KeyMsg) []byte {
	// Map special keys to their ANSI escape sequences.
	switch msg.Type {
	case tea.KeyEnter:
		return []byte{'\r'}
	case tea.KeyBackspace:
		return []byte{127}
	case tea.KeyDelete:
		return []byte{'\x1b', '[', '3', '~'}
	case tea.KeyTab:
		return []byte{'\t'}
	case tea.KeySpace:
		return []byte{' '}
	case tea.KeyEscape:
		return []byte{'\x1b'}
	case tea.KeyCtrlC:
		return []byte{3}
	case tea.KeyCtrlD:
		return []byte{4}
	case tea.KeyCtrlZ:
		return []byte{26}
	case tea.KeyCtrlA:
		return []byte{1}
	case tea.KeyCtrlE:
		return []byte{5}
	case tea.KeyCtrlK:
		return []byte{11}
	case tea.KeyCtrlU:
		return []byte{21}
	case tea.KeyCtrlW:
		return []byte{23}
	case tea.KeyUp:
		return []byte{'\x1b', '[', 'A'}
	case tea.KeyDown:
		return []byte{'\x1b', '[', 'B'}
	case tea.KeyRight:
		return []byte{'\x1b', '[', 'C'}
	case tea.KeyLeft:
		return []byte{'\x1b', '[', 'D'}
	case tea.KeyHome:
		return []byte{'\x1b', '[', 'H'}
	case tea.KeyEnd:
		return []byte{'\x1b', '[', 'F'}
	case tea.KeyPgUp:
		return []byte{'\x1b', '[', '5', '~'}
	case tea.KeyPgDown:
		return []byte{'\x1b', '[', '6', '~'}
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	default:
		// For any other key, send the raw string representation.
		return []byte(msg.String())
	}
}

func main() {
	model := initialModel()
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	prog = p
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
