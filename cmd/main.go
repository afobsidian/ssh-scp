package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"ssh-scp/internal/config"
	sshclient "ssh-scp/internal/ssh"
	"ssh-scp/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh"
)

// appState represents which screen is shown.
type appState int

const (
	stateConnection appState = iota
	stateMain
	stateHostKeyPrompt
	statePasswordPrompt
)

// passwordResponse carries the user's reply back to the connection goroutine.
type passwordResponse struct {
	Password  string
	Cancelled bool
}

// passwordBridge allows the background connection goroutine to request user
// interaction (password prompts, host-key approval) and wait for the answer.
type passwordBridge struct {
	msgCh      chan tea.Msg          // goroutine → TUI (sends PasswordRequestMsg, hostKeyMsg, connectedMsg)
	responseCh chan passwordResponse // TUI → goroutine (password answers)
	approvalCh chan bool             // TUI → goroutine (host-key approval)
}

// pendingConnection holds connection info during host key verification.
type pendingConnection struct {
	conn     config.Connection
	hostKey  ssh.PublicKey
	hostname string
	remote   net.Addr
}

// AppModel is the root application model.
type AppModel struct {
	state          appState
	width          int
	height         int
	cfg            *config.Config
	sshHosts       []config.SSHHost
	connModel      ui.ConnectionModel
	tabs           []ui.Tab
	activeTab      int
	clients        []*sshclient.Client
	browsers       []ui.FileBrowserModel
	pending        *pendingConnection
	showHelp       bool
	err            string
	bridge         *passwordBridge
	passwordDialog ui.PasswordDialogModel
	editor         *ui.EditorModel
}

func initialModel() AppModel {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		cfg = &config.Config{}
	}
	sshHosts := config.LoadSSHConfig()
	return AppModel{
		state:          stateConnection,
		cfg:            cfg,
		sshHosts:       sshHosts,
		connModel:      ui.NewConnectionModelWithSSH(cfg, sshHosts),
		passwordDialog: ui.NewPasswordDialogModel(),
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
		return m, cmd

	case ui.ConnectMsg:
		// Merge SSH config options for the target host if not already set.
		conn := msg.Conn
		if match := config.MatchSSHHost(m.sshHosts, conn.Host); match != nil {
			log.Printf("[AppModel] matched SSH config host %q for %s", match.Alias, conn.Host)
			if conn.HostKeyAlgorithms == "" {
				conn.HostKeyAlgorithms = match.HostKeyAlgorithms
			}
			if conn.PubkeyAcceptedTypes == "" {
				conn.PubkeyAcceptedTypes = match.PubkeyAcceptedTypes
			}
			if conn.StrictHostKeyChecking == "" {
				conn.StrictHostKeyChecking = match.StrictHostKeyChecking
			}
			if conn.UserKnownHostsFile == "" {
				conn.UserKnownHostsFile = match.UserKnownHostsFile
			}
			if conn.KeyPath == "" && match.IdentityFile != "" {
				conn.KeyPath = match.IdentityFile
			}
			if conn.ProxyJump == "" && match.ProxyJump != "" {
				conn.ProxyJump = match.ProxyJump
			}
		}
		// Create the bridge and start the async connection worker.
		bridge := &passwordBridge{
			msgCh:      make(chan tea.Msg, 1),
			responseCh: make(chan passwordResponse),
			approvalCh: make(chan bool),
		}
		m.bridge = bridge
		m.connModel.SetConnecting(fmt.Sprintf("%s@%s:%s", conn.Username, conn.Host, conn.Port))
		go connectWorker(conn, bridge, m.sshHosts)
		return m, waitForBridgeMsg(bridge)

	case connectedMsg:
		m.bridge = nil
		if msg.err != nil {
			log.Printf("[AppModel] connectedMsg error: %v", msg.err)
			m.connModel.SetError("Connection failed: " + msg.err.Error())
			m.state = stateConnection
			return m, nil
		}
		m.connModel.ClearConnecting()
		log.Printf("[AppModel] connectedMsg success: %s@%s", msg.conn.Username, msg.conn.Host)
		m.cfg.AddRecent(msg.conn)
		if err := config.Save(m.cfg); err != nil {
			log.Printf("[AppModel] failed to save config: %v", err)
		}
		tabTitle := ui.TabTitle(msg.conn.Username, msg.conn.Host, len(m.tabs))
		m.tabs = append(m.tabs, ui.Tab{Title: tabTitle, Connected: true})
		m.clients = append(m.clients, msg.client)

		homeDir := "~"
		if sess, err := msg.client.NewSession(); err == nil {
			if out, err := sess.Output("echo $HOME"); err == nil {
				homeDir = strings.TrimSpace(string(out))
			}
			if err := sess.Close(); err != nil {
				log.Printf("close home-dir session: %v", err)
			}
		}

		localDir, _ := os.Getwd()
		browser := ui.NewFileBrowserModel(msg.client, localDir, homeDir)
		m.browsers = append(m.browsers, browser)
		m.activeTab = len(m.tabs) - 1
		m.state = stateMain

		return m, browser.Init()

	case ui.PasswordRequestMsg:
		log.Printf("[AppModel] password requested for %s@%s: %q", msg.Username, msg.Hostname, msg.Prompt)
		m.passwordDialog.Show(msg.Prompt)
		m.state = statePasswordPrompt
		return m, nil

	case ui.PasswordResponseMsg:
		m.passwordDialog.Hide()
		if m.bridge != nil {
			m.bridge.responseCh <- passwordResponse{
				Password:  msg.Password,
				Cancelled: msg.Cancelled,
			}
			if msg.Cancelled {
				// Still drain the final connectedMsg from the goroutine.
				m.state = stateConnection
			} else {
				m.state = stateConnection
			}
			return m, waitForBridgeMsg(m.bridge)
		}
		m.state = stateConnection
		return m, nil

	case hostKeyMsg:
		log.Printf("[AppModel] hostKeyMsg received for %s", msg.hostname)
		m.pending = &pendingConnection{
			conn:     msg.conn,
			hostKey:  msg.key,
			hostname: msg.hostname,
			remote:   msg.remote,
		}
		m.state = stateHostKeyPrompt
		return m, nil

	case ui.TransferDoneMsg:
		if m.activeTab < len(m.browsers) {
			browser, cmd := m.browsers[m.activeTab].Update(msg)
			m.browsers[m.activeTab] = browser
			return m, cmd
		}

	case ui.OpenEditorMsg:
		log.Printf("[AppModel] OpenEditorMsg: path=%s remote=%v", msg.Path, msg.IsRemote)
		if msg.IsRemote {
			if m.activeTab < len(m.clients) {
				client := m.clients[m.activeTab]
				path := msg.Path
				return m, func() tea.Msg {
					content, err := client.ReadFile(path)
					if err != nil {
						return ui.EditorContentLoadedMsg{Err: err}
					}
					return ui.EditorContentLoadedMsg{Path: path, Content: content, IsRemote: true}
				}
			}
		} else {
			path := msg.Path
			return m, func() tea.Msg {
				data, err := os.ReadFile(path)
				if err != nil {
					return ui.EditorContentLoadedMsg{Err: err}
				}
				return ui.EditorContentLoadedMsg{Path: path, Content: string(data), IsRemote: false}
			}
		}

	case ui.EditorContentLoadedMsg:
		if msg.Err != nil {
			log.Printf("[AppModel] editor load error: %v", msg.Err)
			m.err = "Failed to open file: " + msg.Err.Error()
			return m, nil
		}
		log.Printf("[AppModel] editor loaded: %s (%d bytes)", msg.Path, len(msg.Content))
		editor := ui.NewEditorModel(msg.Path, msg.IsRemote, msg.Content)
		m.editor = &editor
		return m, nil

	case ui.EditorSaveMsg:
		log.Printf("[AppModel] EditorSaveMsg: path=%s remote=%v", msg.Path, msg.IsRemote)
		if msg.IsRemote {
			if m.activeTab < len(m.clients) {
				client := m.clients[m.activeTab]
				path := msg.Path
				content := msg.Content
				return m, func() tea.Msg {
					err := client.WriteFile(path, content)
					return ui.EditorSaveDoneMsg{Err: err}
				}
			}
		} else {
			path := msg.Path
			content := msg.Content
			return m, func() tea.Msg {
				err := os.WriteFile(path, []byte(content), 0o644)
				return ui.EditorSaveDoneMsg{Err: err}
			}
		}

	case ui.EditorSaveDoneMsg:
		if m.editor != nil {
			editor, cmd := m.editor.Update(msg)
			m.editor = &editor
			return m, cmd
		}

	case ui.EditorCloseMsg:
		log.Printf("[AppModel] EditorCloseMsg")
		m.editor = nil
		if m.activeTab < len(m.browsers) {
			m.browsers[m.activeTab].RefreshLocal()
			return m, m.browsers[m.activeTab].RefreshRemoteCmd()
		}
		return m, nil

	case tea.KeyMsg:
		log.Printf("[AppModel] key: type=%d string=%q runes=%v alt=%v state=%d",
			msg.Type, msg.String(), msg.Runes, msg.Alt, m.state)

		// Password dialog captures all keys when visible.
		if m.state == statePasswordPrompt {
			if msg.Type == tea.KeyCtrlC {
				m.cleanup()
				return m, tea.Quit
			}
			dlg, cmd := m.passwordDialog.Update(msg)
			m.passwordDialog = dlg
			return m, cmd
		}

		// Editor captures all keys when active (except Ctrl+C).
		if m.state == stateMain && m.editor != nil {
			if msg.Type == tea.KeyCtrlC {
				m.cleanup()
				return m, tea.Quit
			}
			editor, cmd := m.editor.Update(msg)
			m.editor = &editor
			return m, cmd
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			m.cleanup()
			return m, tea.Quit

		case tea.KeyEnter:
			log.Printf("[AppModel] Enter pressed, state=%d pending=%v", m.state, m.pending != nil)
			if m.state == stateHostKeyPrompt && m.pending != nil {
				pending := m.pending
				m.pending = nil
				m.state = stateConnection
				// Accept the host key and tell the waiting goroutine.
				fp := fingerprintSHA256(pending.hostKey)
				acceptedHosts[pending.hostname] = fp
				if m.bridge != nil {
					m.bridge.approvalCh <- true
					return m, waitForBridgeMsg(m.bridge)
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "?":
			if m.state == stateMain {
				m.showHelp = !m.showHelp
				return m, nil
			}

		case "ctrl+n":
			if m.state == stateMain {
				m.state = stateConnection
				m.connModel = ui.NewConnectionModelWithSSH(m.cfg, m.sshHosts)
				return m, m.connModel.Init()
			}

		case "ctrl+t":
			if m.state == stateMain && len(m.tabs) > 1 {
				m.activeTab = (m.activeTab + 1) % len(m.tabs)
				return m, nil
			}

		case "ctrl+w":
			if m.state == stateMain && len(m.tabs) > 0 {
				m.closeTab(m.activeTab)
				if len(m.tabs) == 0 {
					m.state = stateConnection
					m.connModel = ui.NewConnectionModelWithSSH(m.cfg, m.sshHosts)
					return m, m.connModel.Init()
				}
				return m, nil
			}

		case "n", "N":
			if m.state == stateHostKeyPrompt {
				m.pending = nil
				m.state = stateConnection
				m.connModel.SetError("Connection aborted: host key rejected")
				if m.bridge != nil {
					m.bridge.approvalCh <- false
					return m, waitForBridgeMsg(m.bridge)
				}
				return m, nil
			}
		}

		if m.state == stateMain && !m.showHelp {
			if m.activeTab < len(m.browsers) {
				browser, cmd := m.browsers[m.activeTab].Update(msg)
				m.browsers[m.activeTab] = browser
				return m, cmd
			}
		}

		if m.state == stateConnection {
			log.Printf("[AppModel] forwarding key to ConnectionModel: type=%d string=%q", msg.Type, msg.String())
			newConn, cmd := m.connModel.Update(msg)
			m.connModel = newConn.(ui.ConnectionModel)
			if cmd != nil {
				log.Printf("[AppModel] ConnectionModel returned a command")
			}
			return m, cmd
		}
	}

	if m.state == stateConnection {
		newConn, cmd := m.connModel.Update(msg)
		m.connModel = newConn.(ui.ConnectionModel)
		return m, cmd
	}

	// Forward unhandled messages (e.g. remoteFilesMsg) to the active browser.
	if m.state == stateMain && m.activeTab < len(m.browsers) {
		browser, cmd := m.browsers[m.activeTab].Update(msg)
		m.browsers[m.activeTab] = browser
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
	case statePasswordPrompt:
		// Draw the connection screen underneath with the dialog on top.
		bg := m.connModel.View()
		overlay := m.passwordDialog.View(m.width, m.height)
		if overlay != "" {
			return overlay
		}
		return bg
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
	if m.editor != nil {
		browserHeight := m.height - 4
		m.editor.SetDimensions(m.width, browserHeight)
		body = m.editor.View()
	} else if m.activeTab < len(m.browsers) {
		browserHeight := m.height - 4 // tab bar + status line
		m.browsers[m.activeTab].SetDimensions(m.width, browserHeight)
		body = m.browsers[m.activeTab].View()
	}

	var errLine string
	if m.err != "" {
		errLine = "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5555")).Render(m.err)
	}

	statusLine := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555")).
		Render(" Ctrl+T: next tab • Ctrl+N: new tab • Ctrl+W: close tab • ?: help • Ctrl+C: quit" + errLine)

	return lipgloss.JoinVertical(lipgloss.Left, tabBar, body, statusLine)
}

func (m *AppModel) cleanup() {
	for _, c := range m.clients {
		if c != nil {
			if err := c.Close(); err != nil {
				log.Printf("close client: %v", err)
			}
		}
	}
}

func (m *AppModel) closeTab(idx int) {
	if idx < len(m.clients) && m.clients[idx] != nil {
		if err := m.clients[idx].Close(); err != nil {
			log.Printf("close tab client: %v", err)
		}
	}
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
	if idx < len(m.clients) {
		m.clients = append(m.clients[:idx], m.clients[idx+1:]...)
	}
	if idx < len(m.browsers) {
		m.browsers = append(m.browsers[:idx], m.browsers[idx+1:]...)
	}
	if m.activeTab >= len(m.tabs) && m.activeTab > 0 {
		m.activeTab = len(m.tabs) - 1
	}
}

// waitForBridgeMsg returns a tea.Cmd that blocks until the connection
// goroutine sends a message on the bridge (password request, host-key
// prompt, or final connectedMsg).
func waitForBridgeMsg(bridge *passwordBridge) tea.Cmd {
	return func() tea.Msg {
		return <-bridge.msgCh
	}
}

// buildInteractiveAuthMethods assembles SSH auth methods that use the bridge
// for any interactive challenges (password, keyboard-interactive).
// Key-based and agent-based auth are tried first without user interaction.
func buildInteractiveAuthMethods(conn config.Connection, bridge *passwordBridge, displayHost string) []ssh.AuthMethod {
	var methods []ssh.AuthMethod
	username := conn.Username

	// 1. Explicit key file.
	if conn.KeyPath != "" {
		am, err := sshclient.PubKeyAuth(conn.KeyPath)
		if err == nil {
			methods = append(methods, am)
		}
	}

	// 2. SSH agent.
	if am, err := sshclient.AgentAuth(); err == nil {
		methods = append(methods, am)
	}

	// 3. Default key paths (skip any already added).
	for _, kp := range sshclient.DefaultKeyPaths() {
		if kp == conn.KeyPath {
			continue
		}
		if am, err := sshclient.PubKeyAuth(kp); err == nil {
			methods = append(methods, am)
		}
	}

	// 4. Password callback — prompts the user interactively via the bridge.
	methods = append(methods, sshclient.PasswordCallbackAuth(func() (string, error) {
		bridge.msgCh <- ui.PasswordRequestMsg{
			Prompt:   fmt.Sprintf("Password for %s@%s:", username, displayHost),
			Hostname: displayHost,
			Username: username,
		}
		resp := <-bridge.responseCh
		if resp.Cancelled {
			return "", fmt.Errorf("authentication cancelled by user")
		}
		return resp.Password, nil
	}))

	// 5. Keyboard-interactive — the server sends prompts (may include 2FA).
	methods = append(methods, sshclient.KeyboardInteractiveAuth(
		func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			if len(questions) == 0 {
				return nil, nil
			}
			answers := make([]string, len(questions))
			for i, q := range questions {
				prompt := q
				if prompt == "" {
					prompt = fmt.Sprintf("Authentication for %s@%s:", username, displayHost)
				}
				bridge.msgCh <- ui.PasswordRequestMsg{
					Prompt:   prompt,
					Hostname: displayHost,
					Username: username,
				}
				resp := <-bridge.responseCh
				if resp.Cancelled {
					return nil, fmt.Errorf("authentication cancelled by user")
				}
				answers[i] = resp.Password
			}
			return answers, nil
		},
	))

	return methods
}

// makeInteractiveHKCallback returns an ssh.HostKeyCallback that uses the
// bridge to ask the user whether to accept an unknown host key. The callback
// blocks until the user responds.
func makeInteractiveHKCallback(bridge *passwordBridge, conn config.Connection) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fp := fingerprintSHA256(key)
		if acceptedHosts[hostname] == fp {
			return nil
		}

		// Check StrictHostKeyChecking=no (already handled in ConnectOptions,
		// but the callback still runs before opts are applied).
		if strings.EqualFold(conn.StrictHostKeyChecking, "no") {
			acceptedHosts[hostname] = fp
			return nil
		}

		bridge.msgCh <- hostKeyMsg{
			conn:     conn,
			key:      key,
			hostname: hostname,
			remote:   remote,
		}
		approved := <-bridge.approvalCh
		if !approved {
			return fmt.Errorf("host key rejected by user")
		}
		return nil
	}
}

// makeConnectOptions builds SSH connect options from a connection's config fields.
func makeConnectOptions(conn config.Connection) *sshclient.ConnectOptions {
	opts := &sshclient.ConnectOptions{
		HostKeyAlgorithms:     conn.HostKeyAlgorithms,
		PubkeyAcceptedTypes:   conn.PubkeyAcceptedTypes,
		StrictHostKeyChecking: conn.StrictHostKeyChecking,
		UserKnownHostsFile:    conn.UserKnownHostsFile,
	}
	log.Printf("[connectOpts] HostKeyAlgorithms=%q StrictHostKeyChecking=%q PubkeyAcceptedTypes=%q",
		opts.HostKeyAlgorithms, opts.StrictHostKeyChecking, opts.PubkeyAcceptedTypes)
	return opts
}

// parseJumpSpec parses a ProxyJump value into host, port, username.
// Supported formats: host, host:port, user@host, user@host:port, or
// an SSH config alias (resolved via sshHosts).
func parseJumpSpec(spec string, sshHosts []config.SSHHost) config.Connection {
	spec = strings.TrimSpace(spec)

	// First try to resolve as an SSH config alias.
	if match := config.MatchSSHHost(sshHosts, spec); match != nil {
		c := match.ToConnection()
		if c.Port == "" {
			c.Port = "22"
		}
		return c
	}

	var user, host, port string

	if at := strings.Index(spec, "@"); at >= 0 {
		user = spec[:at]
		spec = spec[at+1:]
	}

	// Handle [IPv6]:port or host:port.
	if strings.HasPrefix(spec, "[") {
		if end := strings.Index(spec, "]:"); end >= 0 {
			host = spec[1:end]
			port = spec[end+2:]
		} else {
			host = strings.Trim(spec, "[]")
		}
	} else if colon := strings.LastIndex(spec, ":"); colon >= 0 {
		host = spec[:colon]
		port = spec[colon+1:]
	} else {
		host = spec
	}

	if port == "" {
		port = "22"
	}

	return config.Connection{
		Name:     fmt.Sprintf("jump:%s", host),
		Host:     host,
		Port:     port,
		Username: user,
	}
}

// connectWorker runs in a background goroutine and performs the full SSH
// connection (optionally through a jump host), using the bridge for any
// interactive prompts. The final result (success or error) is sent on
// bridge.msgCh as a connectedMsg.
func connectWorker(conn config.Connection, bridge *passwordBridge, sshHosts []config.SSHHost) {
	destAuth := buildInteractiveAuthMethods(conn, bridge, conn.Host)
	destHKCb := makeInteractiveHKCallback(bridge, conn)
	destOpts := makeConnectOptions(conn)

	if conn.ProxyJump != "" {
		log.Printf("[connectWorker] using ProxyJump %q for %s", conn.ProxyJump, conn.Host)

		jumpConn := parseJumpSpec(conn.ProxyJump, sshHosts)
		// Default jump username to destination username if not specified.
		if jumpConn.Username == "" {
			jumpConn.Username = conn.Username
		}

		// Merge SSH config options for the jump host.
		if match := config.MatchSSHHost(sshHosts, jumpConn.Host); match != nil {
			if jumpConn.HostKeyAlgorithms == "" {
				jumpConn.HostKeyAlgorithms = match.HostKeyAlgorithms
			}
			if jumpConn.PubkeyAcceptedTypes == "" {
				jumpConn.PubkeyAcceptedTypes = match.PubkeyAcceptedTypes
			}
			if jumpConn.StrictHostKeyChecking == "" {
				jumpConn.StrictHostKeyChecking = match.StrictHostKeyChecking
			}
			if jumpConn.UserKnownHostsFile == "" {
				jumpConn.UserKnownHostsFile = match.UserKnownHostsFile
			}
			if jumpConn.KeyPath == "" && match.IdentityFile != "" {
				jumpConn.KeyPath = match.IdentityFile
			}
		}

		jumpAuth := buildInteractiveAuthMethods(jumpConn, bridge, jumpConn.Host)
		jumpHKCb := makeInteractiveHKCallback(bridge, jumpConn)
		jumpOpts := makeConnectOptions(jumpConn)

		log.Printf("[connectWorker] connecting to jump host %s@%s:%s", jumpConn.Username, jumpConn.Host, jumpConn.Port)
		jumpClient, err := sshclient.New(jumpConn.Host, jumpConn.Port, jumpConn.Username, jumpAuth, jumpHKCb, jumpOpts)
		if err != nil {
			bridge.msgCh <- connectedMsg{err: fmt.Errorf("jump host %s: %w", jumpConn.Host, err), conn: conn}
			return
		}

		log.Printf("[connectWorker] dialling %s@%s:%s via jump host", conn.Username, conn.Host, conn.Port)
		client, err := sshclient.NewViaJump(jumpClient.SSHClient(), conn.Host, conn.Port, conn.Username, destAuth, destHKCb, destOpts)
		if err != nil {
			_ = jumpClient.Close()
			bridge.msgCh <- connectedMsg{err: fmt.Errorf("destination via jump: %w", err), conn: conn}
			return
		}
		bridge.msgCh <- connectedMsg{client: client, conn: conn}
		return
	}

	// Direct connection (no jump host).
	log.Printf("[connectWorker] direct connection to %s@%s:%s", conn.Username, conn.Host, conn.Port)
	client, err := sshclient.New(conn.Host, conn.Port, conn.Username, destAuth, destHKCb, destOpts)
	if err != nil {
		bridge.msgCh <- connectedMsg{err: err, conn: conn}
		return
	}
	bridge.msgCh <- connectedMsg{client: client, conn: conn}
}

// fingerprintSHA256 computes the SHA256 fingerprint of a host key,
// matching the modern OpenSSH fingerprint format (e.g. "SHA256:...").
func fingerprintSHA256(key ssh.PublicKey) string {
	return ssh.FingerprintSHA256(key)
}

// logPath returns the path for the debug log file.
// When running from the project directory (go run / ./bin/ssh-scp), logs go
// to .logs/debug.log.  When installed (e.g. /usr/local/bin), logs go to
// ~/.local/state/ssh-scp/debug.log following XDG conventions.
func logPath() string {
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		cwd, _ := os.Getwd()
		// Treat it as a local run when the binary lives under the working directory
		// (e.g. ./bin/ssh-scp) or in a go tmp build directory.
		if strings.HasPrefix(exeDir, cwd) || strings.Contains(exeDir, "go-build") {
			dir := filepath.Join(cwd, ".logs")
			_ = os.MkdirAll(dir, 0o755)
			return filepath.Join(dir, "debug.log")
		}
	}
	// Installed: use XDG state directory.
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, _ := os.UserHomeDir()
		stateDir = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(stateDir, "ssh-scp")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "debug.log")
}

func main() {
	// Set up debug log file.
	f, err := tea.LogToFile(logPath(), "debug")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not open debug log:", err)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()
	log.Printf("=== ssh-scp starting (log: %s) ===", logPath())

	model := initialModel()
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
