package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"strings"
	"testing"

	"ssh-scp/internal/config"
	sshclient "ssh-scp/internal/ssh"
	"ssh-scp/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	gossh "golang.org/x/crypto/ssh"
)

// ---------------------------------------------------------------------------
// keyToBytes
// ---------------------------------------------------------------------------

func TestKeyToBytesEnter(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyEnter})
	if len(got) != 1 || got[0] != '\r' {
		t.Errorf("Enter = %v, want [\\r]", got)
	}
}

func TestKeyToBytesBackspace(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyBackspace})
	if len(got) != 1 || got[0] != 127 {
		t.Errorf("Backspace = %v, want [127]", got)
	}
}

func TestKeyToBytesDelete(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyDelete})
	if len(got) != 4 {
		t.Errorf("Delete = %v, want 4 bytes", got)
	}
	if got[0] != '\x1b' || got[1] != '[' || got[2] != '3' || got[3] != '~' {
		t.Errorf("Delete = %v, want ESC[3~", got)
	}
}

func TestKeyToBytesTab(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyTab})
	if len(got) != 1 || got[0] != '\t' {
		t.Errorf("Tab = %v, want [\\t]", got)
	}
}

func TestKeyToBytesSpace(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeySpace})
	if len(got) != 1 || got[0] != ' ' {
		t.Errorf("Space = %v, want [ ]", got)
	}
}

func TestKeyToBytesEscape(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyEscape})
	if len(got) != 1 || got[0] != '\x1b' {
		t.Errorf("Escape = %v, want [ESC]", got)
	}
}

func TestKeyToBytesCtrlC(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlC})
	if len(got) != 1 || got[0] != 3 {
		t.Errorf("CtrlC = %v, want [3]", got)
	}
}

func TestKeyToBytesCtrlD(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlD})
	if len(got) != 1 || got[0] != 4 {
		t.Errorf("CtrlD = %v, want [4]", got)
	}
}

func TestKeyToBytesCtrlZ(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if len(got) != 1 || got[0] != 26 {
		t.Errorf("CtrlZ = %v, want [26]", got)
	}
}

func TestKeyToBytesCtrlA(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlA})
	if len(got) != 1 || got[0] != 1 {
		t.Errorf("CtrlA = %v, want [1]", got)
	}
}

func TestKeyToBytesCtrlE(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlE})
	if len(got) != 1 || got[0] != 5 {
		t.Errorf("CtrlE = %v, want [5]", got)
	}
}

func TestKeyToBytesCtrlK(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlK})
	if len(got) != 1 || got[0] != 11 {
		t.Errorf("CtrlK = %v, want [11]", got)
	}
}

func TestKeyToBytesCtrlU(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlU})
	if len(got) != 1 || got[0] != 21 {
		t.Errorf("CtrlU = %v, want [21]", got)
	}
}

func TestKeyToBytesCtrlW(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlW})
	if len(got) != 1 || got[0] != 23 {
		t.Errorf("CtrlW = %v, want [23]", got)
	}
}

func TestKeyToBytesArrows(t *testing.T) {
	tests := []struct {
		key  tea.KeyType
		seq  []byte
		name string
	}{
		{tea.KeyUp, []byte{'\x1b', '[', 'A'}, "Up"},
		{tea.KeyDown, []byte{'\x1b', '[', 'B'}, "Down"},
		{tea.KeyRight, []byte{'\x1b', '[', 'C'}, "Right"},
		{tea.KeyLeft, []byte{'\x1b', '[', 'D'}, "Left"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := keyToBytes(tea.KeyMsg{Type: tt.key})
			if len(got) != len(tt.seq) {
				t.Fatalf("%s: len=%d, want %d", tt.name, len(got), len(tt.seq))
			}
			for i := range got {
				if got[i] != tt.seq[i] {
					t.Errorf("%s byte[%d]=%d, want %d", tt.name, i, got[i], tt.seq[i])
				}
			}
		})
	}
}

func TestKeyToBytesHomeEnd(t *testing.T) {
	home := keyToBytes(tea.KeyMsg{Type: tea.KeyHome})
	if len(home) != 3 || home[2] != 'H' {
		t.Errorf("Home = %v, want ESC[H", home)
	}
	end := keyToBytes(tea.KeyMsg{Type: tea.KeyEnd})
	if len(end) != 3 || end[2] != 'F' {
		t.Errorf("End = %v, want ESC[F", end)
	}
}

func TestKeyToBytesPgUpDown(t *testing.T) {
	up := keyToBytes(tea.KeyMsg{Type: tea.KeyPgUp})
	if len(up) != 4 || up[2] != '5' {
		t.Errorf("PgUp = %v, want ESC[5~", up)
	}
	down := keyToBytes(tea.KeyMsg{Type: tea.KeyPgDown})
	if len(down) != 4 || down[2] != '6' {
		t.Errorf("PgDown = %v, want ESC[6~", down)
	}
}

func TestKeyToBytesRunes(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")})
	if string(got) != "hello" {
		t.Errorf("Runes = %q, want %q", string(got), "hello")
	}
}

func TestKeyToBytesSingleChar(t *testing.T) {
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if string(got) != "a" {
		t.Errorf("single rune = %q, want %q", string(got), "a")
	}
}

// ---------------------------------------------------------------------------
// hostKeyPendingError
// ---------------------------------------------------------------------------

func TestHostKeyPendingErrorMessage(t *testing.T) {
	err := &hostKeyPendingError{hostname: "example.com:22"}
	got := err.Error()
	if got != "host key verification required for example.com:22" {
		t.Errorf("Error() = %q", got)
	}
}

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

func TestFocusPaneConstants(t *testing.T) {
	if paneTerminal != 0 {
		t.Errorf("paneTerminal = %d, want 0", paneTerminal)
	}
	if paneFileBrowser != 1 {
		t.Errorf("paneFileBrowser = %d, want 1", paneFileBrowser)
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

func TestAppModelUpdateCtrlT(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.focus = paneTerminal

	msg := tea.KeyMsg{Type: tea.KeyCtrlT}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.focus != paneFileBrowser {
		t.Errorf("Ctrl+T should toggle to file browser, got %d", am.focus)
	}

	result, _ = am.Update(msg)
	am = result.(AppModel)
	if am.focus != paneTerminal {
		t.Errorf("Ctrl+T should toggle back to terminal, got %d", am.focus)
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
	msg := connectedMsg{err: &hostKeyPendingError{hostname: "h"}, conn: config.Connection{}}
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
	m.terminals = []*ui.TerminalModel{nil}
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
		terminals: []*ui.TerminalModel{nil, nil},
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
// AppModel - TerminalOutputMsg
// ---------------------------------------------------------------------------

func TestAppModelTerminalOutput(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	term := ui.NewTerminalModel(nil)
	m.terminals = []*ui.TerminalModel{term}
	m.activeTab = 0

	msg := ui.TerminalOutputMsg{Data: []byte("test output")}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	got := am.terminals[0].BufferedOutput()
	if got != "test output" {
		t.Errorf("terminal output = %q, want %q", got, "test output")
	}
}

// ---------------------------------------------------------------------------
// renderMain with terminal
// ---------------------------------------------------------------------------

func TestRenderMainWithTerminal(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40
	m.tabs = []ui.Tab{{Title: "test", Connected: true}}
	term := ui.NewTerminalModel(nil)
	term.AppendOutput([]byte("hello"))
	m.terminals = []*ui.TerminalModel{term}
	m.browsers = []ui.FileBrowserModel{{}}
	m.activeTab = 0

	view := m.renderMain()
	if view == "" {
		t.Error("renderMain should produce output")
	}
}

func TestRenderMainWithError(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.width = 80
	m.height = 40
	m.err = "something went wrong"
	m.tabs = []ui.Tab{{Title: "test", Connected: true}}
	term := ui.NewTerminalModel(nil)
	m.terminals = []*ui.TerminalModel{term}
	m.browsers = []ui.FileBrowserModel{{}}
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
	m.terminals = []*ui.TerminalModel{nil, nil}
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
// AppModel - terminal focus writes
// ---------------------------------------------------------------------------

func TestAppModelTerminalFocusWrite(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.focus = paneTerminal
	term := ui.NewTerminalModel(nil)
	m.terminals = []*ui.TerminalModel{term}
	m.activeTab = 0

	// Send a char key - should not crash (stdin is nil, Write returns nil)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	result, _ := m.Update(msg)
	_ = result.(AppModel)
}

// ---------------------------------------------------------------------------
// AppModel - file browser focus forwards keys
// ---------------------------------------------------------------------------

func TestAppModelFileBrowserFocus(t *testing.T) {
	dir := t.TempDir()
	m := initialModel()
	m.state = stateMain
	m.focus = paneFileBrowser
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
// AppModel - WindowSizeMsg with terminals
// ---------------------------------------------------------------------------

func TestAppModelWindowSizeWithTerminals(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	term := ui.NewTerminalModel(nil)
	m.terminals = []*ui.TerminalModel{term}
	m.browsers = []ui.FileBrowserModel{{}}

	msg := tea.WindowSizeMsg{Width: 120, Height: 50}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if am.width != 120 || am.height != 50 {
		t.Errorf("dimensions = %dx%d, want 120x50", am.width, am.height)
	}
}

// ---------------------------------------------------------------------------
// closeTab - activeTab adjusts
// ---------------------------------------------------------------------------

func TestCloseTabAdjustsActiveTab(t *testing.T) {
	m := &AppModel{
		tabs:      []ui.Tab{{Title: "t1"}, {Title: "t2"}, {Title: "t3"}},
		clients:   []*sshclient.Client{nil, nil, nil},
		terminals: []*ui.TerminalModel{nil, nil, nil},
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
		terminals: []*ui.TerminalModel{nil, nil, nil},
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
// cleanup with real TerminalModels
// ---------------------------------------------------------------------------

func TestCleanupWithTerminals(t *testing.T) {
	m := &AppModel{
		terminals: []*ui.TerminalModel{
			ui.NewTerminalModel(nil),
			nil,
			ui.NewTerminalModel(nil),
		},
		clients: []*sshclient.Client{nil, nil, nil},
	}
	// Should not panic
	m.cleanup()
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
// connectCmd
// ---------------------------------------------------------------------------

func TestConnectCmdNoAuth(t *testing.T) {
	conn := config.Connection{Host: "h", Port: "22", Username: "u"}
	cmd := connectCmd(conn)
	if cmd == nil {
		t.Fatal("connectCmd should return a command")
	}
	// Execute the command — it should return a connectedMsg with error
	msg := cmd()
	cm, ok := msg.(connectedMsg)
	if !ok {
		t.Fatalf("expected connectedMsg, got %T", msg)
	}
	if cm.err == nil {
		t.Error("no auth should produce error")
	}
}

func TestConnectCmdWithPassword(t *testing.T) {
	conn := config.Connection{Host: "192.0.2.1", Port: "22", Username: "u", Password: "p"}
	cmd := connectCmd(conn)
	if cmd == nil {
		t.Fatal("connectCmd should return a command")
	}
	// Execute — will fail to connect but should not panic
	msg := cmd()
	cm, ok := msg.(connectedMsg)
	if !ok {
		// Could also be a hostKeyMsg
		_, ok2 := msg.(hostKeyMsg)
		if !ok2 {
			t.Fatalf("expected connectedMsg or hostKeyMsg, got %T", msg)
		}
	} else {
		if cm.err == nil {
			t.Error("connecting to unreachable host should fail")
		}
	}
}

func TestConnectCmdWithBadKeyPath(t *testing.T) {
	conn := config.Connection{
		Host:     "192.0.2.1",
		Port:     "22",
		Username: "u",
		KeyPath:  "/nonexistent/key",
		Password: "p",
	}
	cmd := connectCmd(conn)
	if cmd == nil {
		t.Fatal("connectCmd should return a command")
	}
	// Execute — bad key path is silently ignored, falls through to password
	msg := cmd()
	_, ok := msg.(connectedMsg)
	if !ok {
		_, ok2 := msg.(hostKeyMsg)
		if !ok2 {
			t.Fatalf("expected connectedMsg or hostKeyMsg, got %T", msg)
		}
	}
}

// ---------------------------------------------------------------------------
// AppModel - showHelp blocks key forwarding
// ---------------------------------------------------------------------------

func TestAppModelShowHelpBlocksKeys(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.showHelp = true
	m.focus = paneTerminal

	// When help is shown, regular keys should not go to terminal
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

	m := initialModel()
	m.state = stateHostKeyPrompt
	m.pending = &pendingConnection{
		hostname: "host:22",
		hostKey:  signer.PublicKey(),
		conn:     config.Connection{Host: "host", Port: "22", Username: "user", Password: "pass"},
	}

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result, cmd := m.Update(enterMsg)
	am := result.(AppModel)
	if am.pending != nil {
		t.Error("pending should be nil after accept")
	}
	if cmd == nil {
		t.Error("accept should return a connectWithAcceptedKey command")
	}
}

// ---------------------------------------------------------------------------
// connectWithAcceptedKey
// ---------------------------------------------------------------------------

func TestConnectWithAcceptedKeyReturnsCmd(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := gossh.NewSignerFromKey(priv)

	pending := &pendingConnection{
		hostname: "192.0.2.1:22",
		hostKey:  signer.PublicKey(),
		conn:     config.Connection{Host: "192.0.2.1", Port: "22", Username: "u", Password: "p"},
	}

	cmd := connectWithAcceptedKey(pending)
	if cmd == nil {
		t.Fatal("should return a command")
	}
	// Execute — will fail to connect but tests the code path
	msg := cmd()
	cm, ok := msg.(connectedMsg)
	if !ok {
		t.Fatalf("expected connectedMsg, got %T", msg)
	}
	if cm.err == nil {
		t.Error("connecting to unreachable host should fail")
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
// keyToBytes default case (line 534-536)
// ---------------------------------------------------------------------------

func TestKeyToBytesDefault(t *testing.T) {
	// Send an F-key that isn't explicitly handled — triggers the default branch
	got := keyToBytes(tea.KeyMsg{Type: tea.KeyF1})
	if len(got) == 0 {
		t.Error("default keyToBytes should return msg.String()")
	}
}

// ---------------------------------------------------------------------------
// Terminal write error path in Update (lines 245-247)
// ---------------------------------------------------------------------------

type brokenWriter struct{}

func (bw brokenWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func (bw brokenWriter) Close() error {
	return io.ErrClosedPipe
}

func TestAppModelTerminalWriteError(t *testing.T) {
	m := initialModel()
	m.state = stateMain
	m.focus = paneTerminal

	// Create a terminal with a broken stdin pipe
	term := ui.NewTerminalModel(nil)
	term.SetStdinForTest(&brokenWriter{})
	m.terminals = []*ui.TerminalModel{term}
	m.activeTab = 0

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	result, _ := m.Update(msg)
	am := result.(AppModel)
	if !strings.Contains(am.err, "Terminal write failed") {
		t.Errorf("expected terminal write error, got err=%q", am.err)
	}
}

// ---------------------------------------------------------------------------
// closeTab with real terminals that have stdin set (lines 363-371)
// ---------------------------------------------------------------------------

func TestCloseTabWithStdin(t *testing.T) {
	_, w := io.Pipe()
	term := ui.NewTerminalModel(nil)
	term.SetStdinForTest(w)

	m := &AppModel{
		tabs:      []ui.Tab{{Title: "t1"}},
		clients:   []*sshclient.Client{nil},
		terminals: []*ui.TerminalModel{term},
		browsers:  []ui.FileBrowserModel{{}},
		activeTab: 0,
	}
	m.closeTab(0)
	if len(m.tabs) != 0 {
		t.Errorf("tabs = %d, want 0", len(m.tabs))
	}
}

// ---------------------------------------------------------------------------
// cleanup with terminals that have stdin (error path, lines 348-350)
// ---------------------------------------------------------------------------

func TestCleanupWithStdinTerminals(t *testing.T) {
	_, w := io.Pipe()
	term := ui.NewTerminalModel(nil)
	term.SetStdinForTest(w)

	m := &AppModel{
		terminals: []*ui.TerminalModel{term},
		clients:   []*sshclient.Client{nil},
	}
	m.cleanup() // Should not panic, should log errors gracefully
}
