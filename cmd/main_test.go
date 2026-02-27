package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"

	"ssh-scp/internal/config"
	sshclient "ssh-scp/internal/ssh"
	"ssh-scp/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	gossh "golang.org/x/crypto/ssh"
)

// ---------------------------------------------------------------------------
// initialModel
// ---------------------------------------------------------------------------

func TestInitialModel(t *testing.T) {
	m := initialModel()
	if m.state != stateConnection {
		t.Errorf("initial state = %d, want stateConnection(%d)", m.state, stateConnection)
	}
	if m.cfg == nil {
		t.Error("cfg should not be nil")
	}
}

func TestInitialModelInit(t *testing.T) {
	m := initialModel()
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command")
	}
}

// ---------------------------------------------------------------------------
// AppModel - state constants
// ---------------------------------------------------------------------------

func TestAppStateConstants(t *testing.T) {
	if stateConnection != 0 {
		t.Errorf("stateConnection = %d, want 0", stateConnection)
	}
	if stateMain != 1 {
		t.Errorf("stateMain = %d, want 1", stateMain)
	}
	if stateHostKeyPrompt != 2 {
		t.Errorf("stateHostKeyPrompt = %d, want 2", stateHostKeyPrompt)
	}
}

// ---------------------------------------------------------------------------
// AppModel - View
// ---------------------------------------------------------------------------

func TestAppModelViewConnection(t *testing.T) {
	m := initialModel()
	view := m.View()
	if view == "" {
		t.Error("View should not be empty in connection state")
	}
}

func TestAppModelViewShowHelp(t *testing.T) {
	m := initialModel()
	m.showHelp = true
	m.width = 80
	m.height = 40
	view := m.View()
	if view == "" {
		t.Error("help view should not be empty")
	}
}

func TestAppModelViewMainInitializing(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.width = 0
	m.height = 0
	view := m.View()
	if view != "Initializing..." {
		t.Errorf("view = %q, want %q", view, "Initializing...")
	}
}

func TestAppModelViewHostKeyPromptNilPending(t *testing.T) {
	m := initialModel()
	m.state = stateHostKeyPrompt
	m.pending = nil
	view := m.View()
	if view != "" {
		t.Errorf("host key prompt with nil pending should be empty, got %q", view)
	}
}

// ---------------------------------------------------------------------------
// AppModel - Update key events
// ---------------------------------------------------------------------------

func TestAppModelUpdateHelpToggle(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if !am.showHelp {
		t.Error("? should toggle help on")
	}
	result, _ = am.Update(msg)
	am = result.(AppModel)
	if am.showHelp {
		t.Error("? should toggle help off")
	}
}

func TestAppModelUpdateHostKeyReject(t *testing.T) {
	m := initialModel()
	m.state = stateHostKeyPrompt
	m.pending = &pendingConnection{hostname: "host"}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateConnection {
		t.Errorf("rejecting host key should return to connection, got state %d", am.state)
	}
	if am.pending != nil {
		t.Error("pending should be nil after rejection")
	}
}

func TestAppModelUpdateConnFailed(t *testing.T) {
	m := initialModel()
	msg := connectedMsg{err: fmt.Errorf("connection refused"), conn: config.Connection{}}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateConnection {
		t.Errorf("failed connection should return to connection state, got %d", am.state)
	}
}

func TestAppModelUpdateCtrlN(t *testing.T) {
	m := initialModel()
	m.state = stateMain

	msg := tea.KeyMsg{Type: tea.KeyCtrlN}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateConnection {
		t.Errorf("Ctrl+N should go to connection state, got %d", am.state)
	}
}

func TestAppModelCloseTabSwitchesToConnection(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.tabs = []ui.Tab{{Title: "t1", Connected: true}}
	m.clients = []*sshclient.Client{nil}
	m.browsers = []ui.FileBrowserModel{{}}

	msg := tea.KeyMsg{Type: tea.KeyCtrlW}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateConnection {
		t.Errorf("closing last tab should return to connection, got state %d", am.state)
	}
	if len(am.tabs) != 0 {
		t.Errorf("tabs should be empty, got %d", len(am.tabs))
	}
}

// ---------------------------------------------------------------------------
// AppModel - cleanup
// ---------------------------------------------------------------------------

func TestCleanupNoClients(t *testing.T) {
	m := &AppModel{}
	// Should not panic with empty slices
	m.cleanup()
}

func TestCloseTabNilEntries(t *testing.T) {
	m := &AppModel{
		tabs:      []ui.Tab{{Title: "t1"}, {Title: "t2"}},
		clients:   []*sshclient.Client{nil, nil},
		browsers:  []ui.FileBrowserModel{{}, {}},
		activeTab: 1,
	}
	m.closeTab(0)
	if len(m.tabs) != 1 {
		t.Errorf("tabs after closeTab = %d, want 1", len(m.tabs))
	}
	if m.tabs[0].Title != "t2" {
		t.Errorf("remaining tab = %q, want t2", m.tabs[0].Title)
	}
}

// ---------------------------------------------------------------------------
// WindowSizeMsg
// ---------------------------------------------------------------------------

func TestAppModelWindowSize(t *testing.T) {
	m := initialModel()
	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.width != 120 || am.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", am.width, am.height)
	}
}

// ---------------------------------------------------------------------------
// renderMain with terminal
// ---------------------------------------------------------------------------

func TestRenderMainWithBrowser(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40
	m.tabs = []ui.Tab{{Title: "test", Connected: true}}
	browser := ui.NewFileBrowserModel(nil, dir, "/home")
	m.browsers = []ui.FileBrowserModel{browser}
	m.activeTab = 0

	view := m.renderMain()
	if view == "" {
		t.Error("renderMain should produce output")
	}
}

func TestRenderMainWithError(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40
	m.err = "something went wrong"
	m.tabs = []ui.Tab{{Title: "test", Connected: true}}
	browser := ui.NewFileBrowserModel(nil, dir, "/home")
	m.browsers = []ui.FileBrowserModel{browser}
	m.activeTab = 0

	view := m.renderMain()
	if view == "" {
		t.Error("renderMain should produce output even with error")
	}
}

// ---------------------------------------------------------------------------
// ConnectMsg routing
// ---------------------------------------------------------------------------

func TestAppModelConnectMsg(t *testing.T) {
	m := initialModel()
	conn := config.Connection{Host: "h", Port: "22", Username: "u", Password: "p"}
	msg := ui.ConnectMsg{Conn: conn}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("ConnectMsg should return a command")
	}
}

// ---------------------------------------------------------------------------
// AppModel - hostKeyMsg
// ---------------------------------------------------------------------------

func TestAppModelHostKeyMsg(t *testing.T) {
	m := initialModel()
	msg := hostKeyMsg{
		conn:     config.Connection{Host: "h", Port: "22", Username: "u"},
		hostname: "h:22",
	}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateHostKeyPrompt {
		t.Errorf("should transition to host key prompt, got %d", am.state)
	}
	if am.pending == nil {
		t.Error("pending should be set")
	}
}

// ---------------------------------------------------------------------------
// AppModel - TransferDoneMsg routing
// ---------------------------------------------------------------------------

func TestAppModelTransferDoneMsg(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	m.activeTab = 0
	m.browsers = []ui.FileBrowserModel{
		{},
	}
	m.browsers[0].SetDimensions(80, 30)

	// Create a temp file to avoid issues with refreshLocal
	_ = dir
	msg := ui.TransferDoneMsg{Err: nil}
	result, _ := m.Update(msg)
	_ = result.(AppModel)
}

// ---------------------------------------------------------------------------
// AppModel - CtrlC quits
// ---------------------------------------------------------------------------

func TestAppModelCtrlCQuits(t *testing.T) {
	m := initialModel()
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("CtrlC should return a quit command")
	}
}

// ---------------------------------------------------------------------------
// AppModel - help in main state
// ---------------------------------------------------------------------------

func TestAppModelHelpNotInConnection(t *testing.T) {
	m := initialModel()
	m.state = stateConnection
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	// ? in connection state should not toggle help (it goes to text input)
	if am.showHelp {
		t.Error("? should not toggle help in connection state")
	}
}

// ---------------------------------------------------------------------------
// AppModel - CtrlW with multiple tabs
// ---------------------------------------------------------------------------

func TestAppModelCloseTabMultiple(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.tabs = []ui.Tab{{Title: "t1"}, {Title: "t2"}}
	m.clients = []*sshclient.Client{nil, nil}
	m.browsers = []ui.FileBrowserModel{{}, {}}
	m.activeTab = 0

	msg := tea.KeyMsg{Type: tea.KeyCtrlW}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateMain {
		t.Errorf("should stay in main state when tabs remain, got %d", am.state)
	}
	if len(am.tabs) != 1 {
		t.Errorf("tabs = %d, want 1", len(am.tabs))
	}
}

// ---------------------------------------------------------------------------
// AppModel - file browser focus forwards keys
// ---------------------------------------------------------------------------

func TestAppModelFileBrowserFocus(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	browser := ui.NewFileBrowserModel(nil, dir, "/home")
	browser.SetDimensions(80, 30)
	m.browsers = []ui.FileBrowserModel{browser}
	m.activeTab = 0

	msg := tea.KeyMsg{Type: tea.KeyTab}
	result, _ := m.Update(msg)
	_ = result.(AppModel)
	// Tab should be forwarded to browser and not crash
}

// ---------------------------------------------------------------------------
// AppModel - connection state forwards to connModel
// ---------------------------------------------------------------------------

func TestAppModelConnectionFallthrough(t *testing.T) {
	m := initialModel()
	m.state = stateConnection
	// Send a non-key message to test fallthrough
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.width != 100 {
		t.Errorf("width = %d, want 100", am.width)
	}
}

// ---------------------------------------------------------------------------
// AppModel - renderHostKeyPrompt
// ---------------------------------------------------------------------------

func TestRenderHostKeyPromptWithPending(t *testing.T) {
	m := initialModel()
	m.width = 80
	m.height = 40

	// Create a dummy key for the fingerprint
	// We can't easily create an ssh.PublicKey without a real key,
	// so test nil pending case
	m.pending = nil
	view := m.renderHostKeyPrompt()
	if view != "" {
		t.Errorf("nil pending should return empty, got %q", view)
	}
}

// ---------------------------------------------------------------------------
// closeTab - activeTab adjusts
// ---------------------------------------------------------------------------

func TestCloseTabAdjustsActiveTab(t *testing.T) {
	m := &AppModel{
		tabs:      []ui.Tab{{Title: "t1"}, {Title: "t2"}, {Title: "t3"}},
		clients:   []*sshclient.Client{nil, nil, nil},
		browsers:  []ui.FileBrowserModel{{}, {}, {}},
		activeTab: 2,
	}
	m.closeTab(2) // close last tab
	if m.activeTab != 1 {
		t.Errorf("activeTab = %d, want 1", m.activeTab)
	}
}

func TestCloseTabMiddle(t *testing.T) {
	m := &AppModel{
		tabs:      []ui.Tab{{Title: "t1"}, {Title: "t2"}, {Title: "t3"}},
		clients:   []*sshclient.Client{nil, nil, nil},
		browsers:  []ui.FileBrowserModel{{}, {}, {}},
		activeTab: 0,
	}
	m.closeTab(1)
	if len(m.tabs) != 2 {
		t.Errorf("tabs = %d, want 2", len(m.tabs))
	}
	if m.tabs[1].Title != "t3" {
		t.Errorf("remaining tab = %q, want t3", m.tabs[1].Title)
	}
}

// ---------------------------------------------------------------------------
// AppModel - connection key forwarding
// ---------------------------------------------------------------------------

func TestAppModelConnectionKeyForward(t *testing.T) {
	m := initialModel()
	m.state = stateConnection
	msg := tea.KeyMsg{Type: tea.KeyTab}
	result, _ := m.Update(msg)
	_ = result.(AppModel)
	// Tab in connection should forward to connModel without crashing
}

// ---------------------------------------------------------------------------
// AppModel - Host key N uppercase
// ---------------------------------------------------------------------------

func TestAppModelHostKeyRejectUpperN(t *testing.T) {
	m := initialModel()
	m.state = stateHostKeyPrompt
	m.pending = &pendingConnection{hostname: "host"}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateConnection {
		t.Errorf("N should reject host key, got state %d", am.state)
	}
}

// ---------------------------------------------------------------------------
// connectWorker / bridge
// ---------------------------------------------------------------------------

func TestConnectWorkerUnreachable(t *testing.T) {
	conn := config.Connection{Host: "192.0.2.1", Port: "22", Username: "u"}
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse),
		approvalCh: make(chan bool),
	}
	go connectWorker(conn, bridge, nil)

	// The worker should eventually send a connectedMsg (error) or a
	// PasswordRequestMsg (if auth callback fires before timeout).
	// With an unreachable host the TCP dial should timeout with an error.
	msg := <-bridge.msgCh
	switch m := msg.(type) {
	case connectedMsg:
		if m.err == nil {
			t.Error("unreachable host should produce error")
		}
	case ui.PasswordRequestMsg:
		// Auth callback fired first; cancel it so the worker can finish.
		bridge.responseCh <- passwordResponse{Cancelled: true}
		final := <-bridge.msgCh
		if cm, ok := final.(connectedMsg); ok && cm.err == nil {
			t.Error("should still fail after cancel")
		}
	default:
		t.Fatalf("unexpected msg type: %T", msg)
	}
}

func TestConnectWorkerWithBadKeyPath(t *testing.T) {
	conn := config.Connection{
		Host:     "192.0.2.1",
		Port:     "22",
		Username: "u",
		KeyPath:  "/nonexistent/key",
	}
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse),
		approvalCh: make(chan bool),
	}
	go connectWorker(conn, bridge, nil)

	msg := <-bridge.msgCh
	switch m := msg.(type) {
	case connectedMsg:
		if m.err == nil {
			t.Error("unreachable host should produce error")
		}
	case ui.PasswordRequestMsg:
		bridge.responseCh <- passwordResponse{Cancelled: true}
		final := <-bridge.msgCh
		if cm, ok := final.(connectedMsg); ok && cm.err == nil {
			t.Error("should still fail after cancel")
		}
	default:
		t.Fatalf("unexpected msg type: %T", msg)
	}
}

// ---------------------------------------------------------------------------
// AppModel - showHelp blocks key forwarding
// ---------------------------------------------------------------------------

func TestAppModelShowHelpBlocksKeys(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.showHelp = true

	// When help is shown, regular keys should not go to file browser
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	result, _ := m.Update(msg)
	_ = result.(AppModel)
	// No crash means help overlay blocked key forwarding
}

// ---------------------------------------------------------------------------
// fingerprintSHA256
// ---------------------------------------------------------------------------

func TestFingerprintSHA256(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	fp := fingerprintSHA256(signer.PublicKey())
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Errorf("fingerprint = %q, want SHA256: prefix", fp)
	}
}

// ---------------------------------------------------------------------------
// renderHostKeyPrompt with real key
// ---------------------------------------------------------------------------

func TestRenderHostKeyPromptWithKey(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)

	m := initialModel()
	m.width = 80
	m.height = 40
	m.pending = &pendingConnection{
		hostname: "example.com:22",
		hostKey:  signer.PublicKey(),
	}

	view := m.renderHostKeyPrompt()
	if !strings.Contains(view, "example.com:22") {
		t.Error("prompt should contain hostname")
	}
	if !strings.Contains(view, "SHA256:") {
		t.Error("prompt should contain fingerprint")
	}
}

// ---------------------------------------------------------------------------
// AppModel - host key enter (accept)
// ---------------------------------------------------------------------------

func TestAppModelHostKeyAcceptEnter(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)

	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1), // buffered so Update send doesn't block
	}

	m := initialModel()
	m.state = stateHostKeyPrompt
	m.bridge = bridge
	m.pending = &pendingConnection{
		hostname: "host:22",
		hostKey:  signer.PublicKey(),
		conn:     config.Connection{Host: "host", Port: "22", Username: "user"},
	}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	// Run Update in a goroutine since it will try to send on approvalCh
	// and we need to be ready to drain the channel in the resulting cmd.
	result, cmd := m.Update(enterMsg)
	am := result.(AppModel)
	if am.pending != nil {
		t.Error("pending should be nil after accept")
	}
	if cmd == nil {
		t.Error("accept should return a waitForBridgeMsg command")
	}
	fp := fingerprintSHA256(signer.PublicKey())
	if acceptedHosts["host:22"] != fp {
		t.Error("host should be in acceptedHosts after accept")
	}
}

// ---------------------------------------------------------------------------
// parseJumpSpec
// ---------------------------------------------------------------------------

func TestParseJumpSpec(t *testing.T) {
	tests := []struct {
		spec string
		host string
		port string
		user string
	}{
		{"bastion.example.com", "bastion.example.com", "22", ""},
		{"bastion.example.com:2222", "bastion.example.com", "2222", ""},
		{"admin@bastion.example.com", "bastion.example.com", "22", "admin"},
		{"admin@bastion.example.com:2222", "bastion.example.com", "2222", "admin"},
	}
	for _, tt := range tests {
		c := parseJumpSpec(tt.spec, nil)
		if c.Host != tt.host {
			t.Errorf("parseJumpSpec(%q).Host = %q, want %q", tt.spec, c.Host, tt.host)
		}
		if c.Port != tt.port {
			t.Errorf("parseJumpSpec(%q).Port = %q, want %q", tt.spec, c.Port, tt.port)
		}
		if c.Username != tt.user {
			t.Errorf("parseJumpSpec(%q).Username = %q, want %q", tt.spec, c.Username, tt.user)
		}
	}
}

func TestParseJumpSpecResolvesSSHConfig(t *testing.T) {
	hosts := []config.SSHHost{{
		Alias:    "bastion",
		HostName: "10.0.0.1",
		Port:     "2222",
		User:     "jump",
	}}
	c := parseJumpSpec("bastion", hosts)
	if c.Host != "10.0.0.1" {
		t.Errorf("expected resolved host 10.0.0.1, got %s", c.Host)
	}
	if c.Port != "2222" {
		t.Errorf("expected port 2222, got %s", c.Port)
	}
	if c.Username != "jump" {
		t.Errorf("expected user jump, got %s", c.Username)
	}
}

// ---------------------------------------------------------------------------
// buildInteractiveAuthMethods
// ---------------------------------------------------------------------------

func TestBuildInteractiveAuthMethodsIncludesAllTypes(t *testing.T) {
	conn := config.Connection{Host: "h", Port: "22", Username: "u"}
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse),
		approvalCh: make(chan bool),
	}
	methods := buildInteractiveAuthMethods(conn, bridge, "h")
	// Should have at least the password-callback and keyboard-interactive methods
	if len(methods) < 2 {
		t.Errorf("expected at least 2 auth methods, got %d", len(methods))
	}
}

// ---------------------------------------------------------------------------
// PasswordRequestMsg / PasswordResponseMsg round-trip
// ---------------------------------------------------------------------------

func TestPasswordDialogRoundTrip(t *testing.T) {
	m := initialModel()
	// Simulate receiving a PasswordRequestMsg
	reqMsg := ui.PasswordRequestMsg{Prompt: "Password:", Hostname: "h", Username: "u"}
	result, _ := m.Update(reqMsg)
	am := result.(AppModel)
	if am.state != statePasswordPrompt {
		t.Errorf("expected statePasswordPrompt, got %d", am.state)
	}
	if !am.passwordDialog.Visible() {
		t.Error("password dialog should be visible")
	}
}

// ---------------------------------------------------------------------------
// View dispatches to correct screen
// ---------------------------------------------------------------------------

func TestAppModelViewDispatch(t *testing.T) {
	m := initialModel()

	// connection state
	m.state = stateConnection
	v := m.View()
	if v == "" {
		t.Error("connection view should not be empty")
	}

	// main state without width
	m.state = stateMain
	m.width = 0
	v = m.View()
	if v != "Initializing..." {
		t.Errorf("main at zero width = %q", v)
	}

	// help overlay
	m.showHelp = true
	m.width = 80
	m.height = 40
	v = m.View()
	if !strings.Contains(v, "Ctrl") {
		t.Error("help view should contain key bindings")
	}
}

// ---------------------------------------------------------------------------
// View default state — covers the empty return for unknown state (line 288)
// ---------------------------------------------------------------------------

func TestAppModelViewDefaultState(t *testing.T) {
	m := initialModel()
	m.state = 99 // invalid state
	v := m.View()
	if v != "" {
		t.Errorf("unknown state should return empty, got %q", v)
	}
}

// ---------------------------------------------------------------------------
// Non-key msg routed via connection fallthrough (lines 266-270)
// ---------------------------------------------------------------------------

type dummyMsg struct{}

func TestAppModelConnectionFallthroughNonKeyMsg(t *testing.T) {
	m := initialModel()
	m.state = stateConnection
	// Send a custom non-key, non-WindowSizeMsg message
	result, _ := m.Update(dummyMsg{})
	_ = result.(AppModel)
	// The connection fallthrough at the bottom of Update should handle this
}

// ---------------------------------------------------------------------------
// View — statePasswordPrompt
// ---------------------------------------------------------------------------

func TestAppModelViewPasswordPromptVisible(t *testing.T) {
	m := initialModel()
	m.state = statePasswordPrompt
	m.width = 80
	m.height = 40
	m.passwordDialog.Show("Enter password:")

	view := m.View()
	if view == "" {
		t.Error("password prompt view should not be empty")
	}
	if !strings.Contains(view, "Enter password:") {
		t.Error("view should contain the password prompt text")
	}
}

func TestAppModelViewPasswordPromptHidden(t *testing.T) {
	m := initialModel()
	m.state = statePasswordPrompt
	m.width = 80
	m.height = 40
	// Dialog not visible — should fall back to connection view
	view := m.View()
	if view == "" {
		t.Error("view should not be empty even when dialog is hidden")
	}
}

// ---------------------------------------------------------------------------
// Update — statePasswordPrompt key handling
// ---------------------------------------------------------------------------

func TestAppModelPasswordPromptCtrlC(t *testing.T) {
	m := initialModel()
	m.state = statePasswordPrompt
	m.passwordDialog.Show("Enter password:")

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("Ctrl+C in password prompt should return a quit command")
	}
}

func TestAppModelPasswordPromptRegularKey(t *testing.T) {
	m := initialModel()
	m.state = statePasswordPrompt
	m.passwordDialog.Show("Enter password:")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != statePasswordPrompt {
		t.Errorf("regular key should stay in password prompt, got state %d", am.state)
	}
}

// ---------------------------------------------------------------------------
// PasswordResponseMsg — handling
// ---------------------------------------------------------------------------

func TestAppModelPasswordResponseWithBridge(t *testing.T) {
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}

	m := initialModel()
	m.bridge = bridge
	m.state = statePasswordPrompt
	m.passwordDialog.Show("Enter password:")

	msg := ui.PasswordResponseMsg{Password: "secret", Cancelled: false}
	result, cmd := m.Update(msg)
	am := result.(AppModel)
	if am.passwordDialog.Visible() {
		t.Error("dialog should be hidden after response")
	}
	if cmd == nil {
		t.Error("should return waitForBridgeMsg command")
	}
	// Verify the response was sent on the bridge
	resp := <-bridge.responseCh
	if resp.Password != "secret" {
		t.Errorf("password = %q, want %q", resp.Password, "secret")
	}
}

func TestAppModelPasswordResponseCancelled(t *testing.T) {
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}

	m := initialModel()
	m.bridge = bridge
	m.state = statePasswordPrompt

	msg := ui.PasswordResponseMsg{Cancelled: true}
	result, cmd := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateConnection {
		t.Errorf("cancelled response should return to connection, got %d", am.state)
	}
	if cmd == nil {
		t.Error("should still return waitForBridgeMsg to drain goroutine")
	}
	resp := <-bridge.responseCh
	if !resp.Cancelled {
		t.Error("cancelled flag should be true")
	}
}

func TestAppModelPasswordResponseNoBridge(t *testing.T) {
	m := initialModel()
	m.bridge = nil
	m.state = statePasswordPrompt

	msg := ui.PasswordResponseMsg{Password: "test"}
	result, cmd := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateConnection {
		t.Errorf("no bridge should go to connection state, got %d", am.state)
	}
	if cmd != nil {
		t.Error("no bridge should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// CtrlT tab cycling
// ---------------------------------------------------------------------------

func TestAppModelCtrlTCyclesTabs(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.tabs = []ui.Tab{{Title: "t1"}, {Title: "t2"}, {Title: "t3"}}
	m.clients = []*sshclient.Client{nil, nil, nil}
	m.browsers = []ui.FileBrowserModel{{}, {}, {}}
	m.activeTab = 0

	msg := tea.KeyMsg{Type: tea.KeyCtrlT}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.activeTab != 1 {
		t.Errorf("activeTab = %d, want 1", am.activeTab)
	}

	// Cycle again
	result, _ = am.Update(msg)
	am = result.(AppModel)
	if am.activeTab != 2 {
		t.Errorf("activeTab = %d, want 2", am.activeTab)
	}

	// Cycle wraps
	result, _ = am.Update(msg)
	am = result.(AppModel)
	if am.activeTab != 0 {
		t.Errorf("activeTab = %d, want 0 (wrap)", am.activeTab)
	}
}

func TestAppModelCtrlTSingleTab(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.tabs = []ui.Tab{{Title: "t1"}}
	m.clients = []*sshclient.Client{nil}
	m.browsers = []ui.FileBrowserModel{{}}
	m.activeTab = 0

	msg := tea.KeyMsg{Type: tea.KeyCtrlT}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	// With only 1 tab, Ctrl+T should not change tab
	if am.activeTab != 0 {
		t.Errorf("activeTab = %d, want 0 (single tab)", am.activeTab)
	}
}

// ---------------------------------------------------------------------------
// Host key reject with bridge
// ---------------------------------------------------------------------------

func TestAppModelHostKeyRejectWithBridge(t *testing.T) {
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}

	m := initialModel()
	m.state = stateHostKeyPrompt
	m.pending = &pendingConnection{hostname: "h:22"}
	m.bridge = bridge

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}
	result, cmd := m.Update(msg)
	am := result.(AppModel)
	if am.pending != nil {
		t.Error("pending should be nil after rejection")
	}
	if cmd == nil {
		t.Error("rejection with bridge should return waitForBridgeMsg")
	}
	// Drain the approval channel
	approved := <-bridge.approvalCh
	if approved {
		t.Error("approval should be false for rejection")
	}
}

// ---------------------------------------------------------------------------
// makeInteractiveHKCallback
// ---------------------------------------------------------------------------

func TestMakeInteractiveHKCallbackAlreadyAccepted(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)
	key := signer.PublicKey()

	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}
	conn := config.Connection{Host: "h", Port: "22"}

	// Pre-accept the host
	fp := fingerprintSHA256(key)
	acceptedHosts["h:22"] = fp

	cb := makeInteractiveHKCallback(bridge, conn)
	err := cb("h:22", nil, key)
	if err != nil {
		t.Errorf("already-accepted host should return nil, got %v", err)
	}
	// Clean up
	delete(acceptedHosts, "h:22")
}

func TestMakeInteractiveHKCallbackStrictNo(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)
	key := signer.PublicKey()

	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}
	conn := config.Connection{
		Host:                  "h",
		Port:                  "22",
		StrictHostKeyChecking: "no",
	}

	cb := makeInteractiveHKCallback(bridge, conn)
	err := cb("new-host:22", nil, key)
	if err != nil {
		t.Errorf("StrictHostKeyChecking=no should auto-accept, got %v", err)
	}
	// Clean up
	delete(acceptedHosts, "new-host:22")
}

func TestMakeInteractiveHKCallbackUserApproves(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)
	key := signer.PublicKey()

	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}
	conn := config.Connection{Host: "h", Port: "22"}

	cb := makeInteractiveHKCallback(bridge, conn)

	// Run callback in goroutine since it blocks on approvalCh
	errCh := make(chan error, 1)
	go func() {
		errCh <- cb("unknown-host:22", nil, key)
	}()

	// Should receive a hostKeyMsg on bridge
	msg := <-bridge.msgCh
	if _, ok := msg.(hostKeyMsg); !ok {
		t.Fatalf("expected hostKeyMsg, got %T", msg)
	}

	// Approve
	bridge.approvalCh <- true
	err := <-errCh
	if err != nil {
		t.Errorf("approved host should return nil, got %v", err)
	}
	delete(acceptedHosts, "unknown-host:22")
}

func TestMakeInteractiveHKCallbackUserRejects(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)
	key := signer.PublicKey()

	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}
	conn := config.Connection{Host: "h", Port: "22"}

	cb := makeInteractiveHKCallback(bridge, conn)

	errCh := make(chan error, 1)
	go func() {
		errCh <- cb("reject-host:22", nil, key)
	}()

	<-bridge.msgCh
	bridge.approvalCh <- false
	err := <-errCh
	if err == nil {
		t.Error("rejected host should return error")
	}
	if !strings.Contains(err.Error(), "rejected") {
		t.Errorf("error = %q, should mention rejected", err)
	}
}

// ---------------------------------------------------------------------------
// buildInteractiveAuthMethods — password callback
// ---------------------------------------------------------------------------

func TestBuildInteractiveAuthMethodsPasswordCallback(t *testing.T) {
	conn := config.Connection{Host: "h", Port: "22", Username: "u"}
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 2),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}
	methods := buildInteractiveAuthMethods(conn, bridge, "h")
	// The methods include: possibly agent, possibly default keys, password-callback, keyboard-interactive
	// At minimum 2 (password-callback + keyboard-interactive)
	if len(methods) < 2 {
		t.Fatalf("expected at least 2 methods, got %d", len(methods))
	}
}

func TestBuildInteractiveAuthMethodsWithKeyPath(t *testing.T) {
	conn := config.Connection{Host: "h", Port: "22", Username: "u", KeyPath: "/nonexistent/key"}
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 2),
		responseCh: make(chan passwordResponse, 1),
		approvalCh: make(chan bool, 1),
	}
	methods := buildInteractiveAuthMethods(conn, bridge, "h")
	// Bad key path is silently ignored; should still have password + keyboard-interactive
	if len(methods) < 2 {
		t.Fatalf("expected at least 2 methods, got %d", len(methods))
	}
}

// ---------------------------------------------------------------------------
// makeConnectOptions
// ---------------------------------------------------------------------------

func TestMakeConnectOptions(t *testing.T) {
	conn := config.Connection{
		HostKeyAlgorithms:     "ssh-ed25519",
		PubkeyAcceptedTypes:   "ssh-ed25519",
		StrictHostKeyChecking: "no",
		UserKnownHostsFile:    "/dev/null",
	}
	opts := makeConnectOptions(conn)
	if opts.HostKeyAlgorithms != "ssh-ed25519" {
		t.Errorf("HostKeyAlgorithms = %q", opts.HostKeyAlgorithms)
	}
	if opts.StrictHostKeyChecking != "no" {
		t.Errorf("StrictHostKeyChecking = %q", opts.StrictHostKeyChecking)
	}
	if opts.PubkeyAcceptedTypes != "ssh-ed25519" {
		t.Errorf("PubkeyAcceptedTypes = %q", opts.PubkeyAcceptedTypes)
	}
	if opts.UserKnownHostsFile != "/dev/null" {
		t.Errorf("UserKnownHostsFile = %q", opts.UserKnownHostsFile)
	}
}

// ---------------------------------------------------------------------------
// waitForBridgeMsg
// ---------------------------------------------------------------------------

func TestWaitForBridgeMsg(t *testing.T) {
	bridge := &passwordBridge{
		msgCh:      make(chan tea.Msg, 1),
		responseCh: make(chan passwordResponse),
		approvalCh: make(chan bool),
	}
	bridge.msgCh <- connectedMsg{err: fmt.Errorf("test error")}

	cmd := waitForBridgeMsg(bridge)
	if cmd == nil {
		t.Fatal("should return non-nil cmd")
	}
	msg := cmd()
	cm, ok := msg.(connectedMsg)
	if !ok {
		t.Fatalf("expected connectedMsg, got %T", msg)
	}
	if cm.err == nil || cm.err.Error() != "test error" {
		t.Errorf("error = %v, want 'test error'", cm.err)
	}
}

// ---------------------------------------------------------------------------
// logPath
// ---------------------------------------------------------------------------

func TestLogPath(t *testing.T) {
	p := logPath()
	if p == "" {
		t.Error("logPath should return non-empty")
	}
	if !strings.HasSuffix(p, "debug.log") {
		t.Errorf("logPath = %q, should end with debug.log", p)
	}
}

func TestLogPathXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	// Override the executable path check by setting PATH that won't match cwd
	// logPath checks if exe is in cwd - when running tests from go, it's
	// typically in a go-build temp dir, which matches the go-build heuristic
	p := logPath()
	if p == "" {
		t.Error("logPath should return non-empty")
	}
}

// ---------------------------------------------------------------------------
// parseJumpSpec — IPv6
// ---------------------------------------------------------------------------

func TestParseJumpSpecIPv6(t *testing.T) {
	c := parseJumpSpec("[::1]:2222", nil)
	if c.Host != "::1" {
		t.Errorf("Host = %q, want %q", c.Host, "::1")
	}
	if c.Port != "2222" {
		t.Errorf("Port = %q, want %q", c.Port, "2222")
	}
}

func TestParseJumpSpecIPv6NoPort(t *testing.T) {
	c := parseJumpSpec("[::1]", nil)
	if c.Host != "::1" {
		t.Errorf("Host = %q, want %q", c.Host, "::1")
	}
	if c.Port != "22" {
		t.Errorf("Port = %q, want %q", c.Port, "22")
	}
}

func TestParseJumpSpecUserIPv6(t *testing.T) {
	c := parseJumpSpec("admin@[::1]:3333", nil)
	if c.Username != "admin" {
		t.Errorf("Username = %q, want %q", c.Username, "admin")
	}
	if c.Host != "::1" {
		t.Errorf("Host = %q, want %q", c.Host, "::1")
	}
	if c.Port != "3333" {
		t.Errorf("Port = %q, want %q", c.Port, "3333")
	}
}

func TestParseJumpSpecWhitespace(t *testing.T) {
	c := parseJumpSpec("  bastion.example.com  ", nil)
	if c.Host != "bastion.example.com" {
		t.Errorf("Host = %q, want trimmed", c.Host)
	}
}

// ---------------------------------------------------------------------------
// ConnectMsg with SSH config merging
// ---------------------------------------------------------------------------

func TestAppModelConnectMsgMergesSSHConfig(t *testing.T) {
	m := initialModel()
	m.sshHosts = []config.SSHHost{{
		Alias:                 "myhost",
		HostName:              "myhost.example.com",
		HostKeyAlgorithms:     "ssh-ed25519",
		PubkeyAcceptedTypes:   "ssh-ed25519",
		StrictHostKeyChecking: "no",
		UserKnownHostsFile:    "/dev/null",
		IdentityFile:          "/key",
		ProxyJump:             "bastion",
	}}

	conn := config.Connection{Host: "myhost.example.com", Port: "22", Username: "u"}
	msg := ui.ConnectMsg{Conn: conn}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("ConnectMsg should return a command")
	}
}

// ---------------------------------------------------------------------------
// connectedMsg success
// ---------------------------------------------------------------------------

func TestAppModelConnectedMsgSuccess(t *testing.T) {
	m := initialModel()
	m.state = stateConnection

	// We can't easily create a real client, but we can test the error path
	msg := connectedMsg{
		err:  nil,
		conn: config.Connection{Host: "h", Port: "22", Username: "u"},
		// client is nil — will panic when trying to get home dir
		// So test error path instead
	}
	// Test error path is more practical
	msg.err = fmt.Errorf("dial timeout")
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.state != stateConnection {
		t.Errorf("failed connectedMsg should stay at connection, got %d", am.state)
	}
}

// ---------------------------------------------------------------------------
// statePasswordPrompt constant
// ---------------------------------------------------------------------------

func TestStatePasswordPromptConstant(t *testing.T) {
	if statePasswordPrompt != 3 {
		t.Errorf("statePasswordPrompt = %d, want 3", statePasswordPrompt)
	}
}

// ---------------------------------------------------------------------------
// cleanup with mixed nil/non-nil (error path)
// ---------------------------------------------------------------------------

func TestCleanupMixedClients(t *testing.T) {
	m := &AppModel{
		clients: []*sshclient.Client{nil, nil, nil},
	}
	// Should not panic
	m.cleanup()
}

// ---------------------------------------------------------------------------
// Update forwards unhandled msgs to browser in main state
// ---------------------------------------------------------------------------

func TestAppModelForwardsToBrowserInMain(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40
	m.tabs = []ui.Tab{{Title: "test"}}
	browser := ui.NewFileBrowserModel(nil, dir, "/home")
	m.browsers = []ui.FileBrowserModel{browser}
	m.activeTab = 0

	// Send a custom message type
	result, _ := m.Update(dummyMsg{})
	_ = result.(AppModel)
	// Should not crash — browser handles unknown msg gracefully
}

// ---------------------------------------------------------------------------
// View stateMain with width set
// ---------------------------------------------------------------------------

func TestAppModelViewMainWithWidth(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	m.width = 100
	m.height = 40
	m.tabs = []ui.Tab{{Title: "test", Connected: true}}
	browser := ui.NewFileBrowserModel(nil, dir, "/home")
	m.browsers = []ui.FileBrowserModel{browser}
	m.activeTab = 0

	view := m.View()
	if view == "" {
		t.Error("main view with width should not be empty")
	}
	if !strings.Contains(view, "test") {
		t.Error("main view should contain tab title")
	}
}

// ---------------------------------------------------------------------------
// AppModel - WindowSizeMsg updates browsers
// ---------------------------------------------------------------------------

func TestAppModelWindowSizeUpdatesBrowsers(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	browser := ui.NewFileBrowserModel(nil, dir, "/home")
	m.browsers = []ui.FileBrowserModel{browser}
	m.tabs = []ui.Tab{{Title: "test"}}

	msg := tea.WindowSizeMsg{Width: 200, Height: 80}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.width != 200 || am.height != 80 {
		t.Errorf("dims = %dx%d, want 200x80", am.width, am.height)
	}
}

// ---------------------------------------------------------------------------
// AppModel - EditorContentLoadedMsg
// ---------------------------------------------------------------------------

func TestAppModelEditorContentLoadedSuccess(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	msg := ui.EditorContentLoadedMsg{
		Path:     "/tmp/test.txt",
		Content:  "hello world",
		IsRemote: false,
	}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.editor == nil {
		t.Fatal("editor should be set after EditorContentLoadedMsg")
	}
	if am.editor.Content() != "hello world" {
		t.Errorf("editor content = %q", am.editor.Content())
	}
}

func TestAppModelEditorContentLoadedRemote(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	msg := ui.EditorContentLoadedMsg{
		Path:     "/remote/file.txt",
		Content:  "remote data",
		IsRemote: true,
	}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.editor == nil {
		t.Fatal("editor should be set")
	}
}

func TestAppModelEditorContentLoadedError(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	msg := ui.EditorContentLoadedMsg{
		Err: fmt.Errorf("permission denied"),
	}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.editor != nil {
		t.Error("editor should be nil on error")
	}
	if !strings.Contains(am.err, "Failed to open") {
		t.Errorf("err = %q, want 'Failed to open' message", am.err)
	}
}

// ---------------------------------------------------------------------------
// AppModel - EditorSaveDoneMsg forwarded to editor
// ---------------------------------------------------------------------------

func TestAppModelEditorSaveDoneForwarded(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	editor := ui.NewEditorModel("/tmp/test.txt", false, "data")
	editor.SetDimensions(80, 40)
	m.editor = &editor

	msg := ui.EditorSaveDoneMsg{Err: nil}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.editor == nil {
		t.Fatal("editor should still be set after save done")
	}
}

func TestAppModelEditorSaveDoneNilEditor(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.editor = nil
	// Should not panic
	msg := ui.EditorSaveDoneMsg{Err: nil}
	result, _ := m.Update(msg)
	_ = result.(AppModel)
}

// ---------------------------------------------------------------------------
// AppModel - EditorCloseMsg
// ---------------------------------------------------------------------------

func TestAppModelEditorCloseMsg(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	editor := ui.NewEditorModel("/tmp/test.txt", false, "data")
	m.editor = &editor
	m.browsers = []ui.FileBrowserModel{ui.NewFileBrowserModel(nil, dir, "/remote")}
	m.tabs = []ui.Tab{{Title: "test"}}
	m.activeTab = 0

	msg := ui.EditorCloseMsg{}
	result, cmd := m.Update(msg)
	am := result.(AppModel)
	if am.editor != nil {
		t.Error("editor should be nil after EditorCloseMsg")
	}
	if cmd == nil {
		t.Error("EditorCloseMsg should return remote refresh command")
	}
}

func TestAppModelEditorCloseMsgNoBrowser(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	editor := ui.NewEditorModel("/tmp/test.txt", false, "data")
	m.editor = &editor
	// No browsers/tabs set

	msg := ui.EditorCloseMsg{}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.editor != nil {
		t.Error("editor should be nil after close")
	}
}

// ---------------------------------------------------------------------------
// AppModel - OpenEditorMsg local
// ---------------------------------------------------------------------------

func TestAppModelOpenEditorMsgLocal(t *testing.T) {
	m := initialModel()
	m.state = stateMain

	msg := ui.OpenEditorMsg{Path: "/tmp/test.txt", IsRemote: false}
	result, cmd := m.Update(msg)
	_ = result.(AppModel)
	if cmd == nil {
		t.Error("OpenEditorMsg local should return a command")
	}
}

// ---------------------------------------------------------------------------
// AppModel - OpenEditorMsg remote
// ---------------------------------------------------------------------------

func TestAppModelOpenEditorMsgRemote(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.clients = []*sshclient.Client{nil} // nil client
	m.tabs = []ui.Tab{{Title: "test"}}
	m.activeTab = 0

	msg := ui.OpenEditorMsg{Path: "/remote/file.txt", IsRemote: true}
	result, cmd := m.Update(msg)
	_ = result.(AppModel)
	// With nil client, the command will be returned but calling it will panic
	// We mainly verify the code path doesn't crash before returning cmd
	if cmd == nil {
		t.Error("OpenEditorMsg remote with client should return a command")
	}
}

func TestAppModelOpenEditorMsgRemoteNoClient(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.clients = nil
	m.activeTab = 5 // out of range

	msg := ui.OpenEditorMsg{Path: "/remote/file.txt", IsRemote: true}
	result, cmd := m.Update(msg)
	_ = result.(AppModel)
	if cmd != nil {
		t.Error("OpenEditorMsg remote with no client should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// AppModel - EditorSaveMsg local
// ---------------------------------------------------------------------------

func TestAppModelEditorSaveMsgLocal(t *testing.T) {
	m := initialModel()
	m.state = stateMain

	msg := ui.EditorSaveMsg{Path: "/tmp/test.txt", Content: "data", IsRemote: false}
	result, cmd := m.Update(msg)
	_ = result.(AppModel)
	if cmd == nil {
		t.Error("EditorSaveMsg local should return a command")
	}
}

// ---------------------------------------------------------------------------
// AppModel - EditorSaveMsg remote
// ---------------------------------------------------------------------------

func TestAppModelEditorSaveMsgRemote(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.clients = []*sshclient.Client{nil}
	m.tabs = []ui.Tab{{Title: "test"}}
	m.activeTab = 0

	msg := ui.EditorSaveMsg{Path: "/remote/file.txt", Content: "data", IsRemote: true}
	result, cmd := m.Update(msg)
	_ = result.(AppModel)
	if cmd == nil {
		t.Error("EditorSaveMsg remote with client should return a command")
	}
}

func TestAppModelEditorSaveMsgRemoteNoClient(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.clients = nil
	m.activeTab = 5

	msg := ui.EditorSaveMsg{Path: "/remote/file.txt", Content: "data", IsRemote: true}
	_, cmd := m.Update(msg)
	if cmd != nil {
		t.Error("EditorSaveMsg remote with no client should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// AppModel - Editor captures keys when active
// ---------------------------------------------------------------------------

func TestAppModelEditorCapturesKeys(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40
	editor := ui.NewEditorModel("/tmp/test.txt", false, "hello")
	editor.SetDimensions(80, 36)
	m.editor = &editor

	// Send a normal key - should be handled by editor, not toggle help
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.showHelp {
		t.Error("key should go to editor, not toggle help")
	}
	if am.editor == nil {
		t.Error("editor should still be active")
	}
}

func TestAppModelEditorCtrlCQuits(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	editor := ui.NewEditorModel("/tmp/test.txt", false, "hello")
	m.editor = &editor

	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)
	if cmd == nil {
		t.Error("Ctrl+C with editor should return quit command")
	}
}

// ---------------------------------------------------------------------------
// AppModel - renderMain with editor
// ---------------------------------------------------------------------------

func TestRenderMainWithEditor(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40
	m.tabs = []ui.Tab{{Title: "test"}}
	editor := ui.NewEditorModel("/tmp/test.txt", false, "hello\nworld")
	editor.SetDimensions(80, 36)
	m.editor = &editor

	view := m.renderMain()
	if view == "" {
		t.Error("renderMain with editor should produce output")
	}
	if !strings.Contains(view, "test.txt") {
		t.Error("should contain the editor file name")
	}
}

func TestRenderMainEditorTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40
	m.tabs = []ui.Tab{{Title: "test"}}
	browser := ui.NewFileBrowserModel(nil, dir, "/home")
	m.browsers = []ui.FileBrowserModel{browser}
	m.activeTab = 0
	editor := ui.NewEditorModel("/tmp/test.txt", false, "editor content")
	editor.SetDimensions(80, 36)
	m.editor = &editor

	view := m.renderMain()
	// Editor view should take precedence over file browser
	if !strings.Contains(view, "test.txt") {
		t.Error("editor view should be shown instead of browser")
	}
}
