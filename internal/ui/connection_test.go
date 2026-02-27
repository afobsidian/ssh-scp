package ui

import (
	"fmt"
	"ssh-scp/internal/config"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// recentItem
// ---------------------------------------------------------------------------

func TestConnItemTitle(t *testing.T) {
	r := connItem{conn: config.Connection{
		Username: "admin",
		Host:     "example.com",
		Port:     "22",
	}, source: "recent"}
	got := r.Title()
	if got != "admin@example.com:22" {
		t.Errorf("Title = %q, want %q", got, "admin@example.com:22")
	}
}

func TestConnItemTitleNoUser(t *testing.T) {
	r := connItem{conn: config.Connection{
		Host: "example.com",
		Port: "22",
	}, source: "ssh-config"}
	got := r.Title()
	if got != "example.com:22" {
		t.Errorf("Title = %q, want %q", got, "example.com:22")
	}
}

func TestConnItemDescriptionRecent(t *testing.T) {
	r := connItem{conn: config.Connection{Name: "my server"}, source: "recent"}
	want := "[recent] my server"
	if r.Description() != want {
		t.Errorf("Description = %q, want %q", r.Description(), want)
	}
}

func TestConnItemDescriptionSSHConfig(t *testing.T) {
	r := connItem{conn: config.Connection{Name: "prod", Host: "prod.example.com"}, source: "ssh-config"}
	want := "[~/.ssh/config] prod"
	if r.Description() != want {
		t.Errorf("Description = %q, want %q", r.Description(), want)
	}
}

func TestConnItemFilterValue(t *testing.T) {
	r := connItem{conn: config.Connection{Host: "host1", Name: "myhost"}}
	want := "host1 myhost"
	if r.FilterValue() != want {
		t.Errorf("FilterValue = %q, want %q", r.FilterValue(), want)
	}
}

// ---------------------------------------------------------------------------
// NewConnectionModel
// ---------------------------------------------------------------------------

func TestNewConnectionModelDefaults(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	// Port should default to 22
	portVal := m.inputs[fieldPort].Value()
	if portVal != "22" {
		t.Errorf("default port = %q, want %q", portVal, "22")
	}
	// Host field should be focused
	if m.focused != fieldHost {
		t.Errorf("focused = %d, want fieldHost(%d)", m.focused, fieldHost)
	}
	// Form pane should be active by default
	if m.activePane != paneForm {
		t.Errorf("activePane = %d, want paneForm(%d)", m.activePane, paneForm)
	}
}

func TestNewConnectionModelWithRecent(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "test", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	if !m.hasItems {
		t.Error("hasItems should be true when recent connections exist")
	}
}

func TestNewConnectionModelNoRecent(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	if m.hasItems {
		t.Error("hasItems should be false when no connections or SSH hosts")
	}
}

func TestNewConnectionModelWithSSHHosts(t *testing.T) {
	cfg := &config.Config{}
	hosts := []config.SSHHost{
		{Alias: "prod", HostName: "prod.example.com", User: "deploy", Port: "22"},
	}
	m := NewConnectionModelWithSSH(cfg, hosts)
	if !m.hasItems {
		t.Error("hasItems should be true when SSH hosts exist")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel field count
// ---------------------------------------------------------------------------

func TestFieldCount(t *testing.T) {
	if fieldCount != 7 {
		t.Errorf("fieldCount = %d, want 7", fieldCount)
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - Update tab navigation
// ---------------------------------------------------------------------------

func TestConnectionModelTabNavigation(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)

	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	model, _ := m.Update(tabMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldUser {
		t.Errorf("after tab: focused = %d, want fieldUser(%d)", m.focused, fieldUser)
	}
}

func TestConnectionModelShiftTabNavigation(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)

	msg := tea.KeyMsg{Type: tea.KeyShiftTab}
	model, _ := m.Update(msg)
	m = model.(ConnectionModel)
	// Should go to the advanced toggle (backward from Host wraps to toggle when collapsed)
	if !m.focusOnToggle {
		t.Errorf("after shift+tab from Host: expected focusOnToggle=true, got false (focused=%d)", m.focused)
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - validation
// ---------------------------------------------------------------------------

func TestConnectionModelEnterEmptyFields(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	// Don't set host or username — enter should show error
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	model, _ := m.Update(enterMsg)
	m = model.(ConnectionModel)
	if m.err == "" {
		t.Error("expected validation error for empty host/username")
	}
	if !strings.Contains(m.err, "required") {
		t.Errorf("err = %q, should mention required", m.err)
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - View
// ---------------------------------------------------------------------------

func TestConnectionModelView(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	view := m.View()
	if !strings.Contains(view, "SSH TUI") {
		t.Error("view should contain title")
	}
	if !strings.Contains(view, "Host") {
		t.Error("view should contain Host label")
	}
}

func TestConnectionModelViewWithError(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.err = "test error"
	view := m.View()
	if !strings.Contains(view, "test error") {
		t.Error("view should show error")
	}
}

func TestConnectionModelInit(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a command (textinput.Blink)")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - WindowSizeMsg
// ---------------------------------------------------------------------------

func TestConnectionModelWindowSize(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = model.(ConnectionModel)
	if m.width != 100 || m.height != 50 {
		t.Errorf("dimensions = %dx%d, want 100x50", m.width, m.height)
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - Enter with valid data
// ---------------------------------------------------------------------------

func TestConnectionModelEnterValid(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.inputs[fieldHost].SetValue("example.com")
	m.inputs[fieldUser].SetValue("admin")

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enterMsg)
	if cmd == nil {
		t.Error("valid enter should return a ConnectMsg command")
	}
}

func TestConnectionModelEnterDefaultPort(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.inputs[fieldHost].SetValue("example.com")
	m.inputs[fieldUser].SetValue("admin")
	m.inputs[fieldPort].SetValue("") // empty port

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.Update(enterMsg)
	if cmd == nil {
		t.Error("should produce command with default port")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - Down key for navigation
// ---------------------------------------------------------------------------

func TestConnectionModelDownNavigation(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	model, _ := m.Update(downMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldUser {
		t.Errorf("after down: focused = %d, want fieldUser(%d)", m.focused, fieldUser)
	}
}

func TestConnectionModelUpNavigation(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	model, _ := m.Update(upMsg)
	m = model.(ConnectionModel)
	// From Host, up wraps to toggle when advanced is collapsed
	if !m.focusOnToggle {
		t.Errorf("after up from Host: expected focusOnToggle=true, got false (focused=%d)", m.focused)
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - Cycle through all fields
// ---------------------------------------------------------------------------

func TestConnectionModelCycleFields(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	// Expand advanced so all fields are visible.
	m.showAdvanced = true
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}

	// Host → User
	model, _ := m.Update(tabMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldUser {
		t.Fatalf("step 1: focused = %d, want fieldUser(%d)", m.focused, fieldUser)
	}

	// User → Toggle
	model, _ = m.Update(tabMsg)
	m = model.(ConnectionModel)
	if !m.focusOnToggle {
		t.Fatal("step 2: expected focusOnToggle")
	}

	// Toggle → Port (advanced expanded)
	model, _ = m.Update(tabMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldPort {
		t.Fatalf("step 3: focused = %d, want fieldPort(%d)", m.focused, fieldPort)
	}

	// Port → Key → Jump → HostKeyCheck → KnownHostsFile
	for _, want := range []connectionField{fieldKey, fieldJump, fieldHostKeyCheck, fieldKnownHostsFile} {
		model, _ = m.Update(tabMsg)
		m = model.(ConnectionModel)
		if m.focused != want {
			t.Fatalf("focused = %d, want %d", m.focused, want)
		}
	}

	// KnownHostsFile → Host (wrap)
	model, _ = m.Update(tabMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldHost {
		t.Fatalf("after full cycle: focused = %d, want fieldHost(%d)", m.focused, fieldHost)
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - View with recent connections
// ---------------------------------------------------------------------------

func TestConnectionModelViewWithRecent(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "server1", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	view := m.View()
	// View should not be empty
	if view == "" {
		t.Error("view with recent connections should not be empty")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - Esc/CtrlC key handling
// ---------------------------------------------------------------------------

func TestConnectionModelEscReturnsQuit(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := m.Update(escMsg)
	if cmd == nil {
		t.Error("Esc should return a quit command")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - text input forwarding
// ---------------------------------------------------------------------------

func TestConnectionModelTextInput(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	// Type some chars
	charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("test")}
	model, _ := m.Update(charMsg)
	m = model.(ConnectionModel)
	val := m.inputs[fieldHost].Value()
	if val != "test" {
		t.Errorf("host input = %q, want %q", val, "test")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - Ctrl+Arrow pane toggle
// ---------------------------------------------------------------------------

func TestConnectionModelCtrlRightSwitchesToList(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "srv", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	if m.activePane != paneForm {
		t.Fatal("should start in form pane")
	}

	ctrlRight := tea.KeyMsg{Type: tea.KeyCtrlRight}
	model, _ := m.Update(ctrlRight)
	m = model.(ConnectionModel)
	if m.activePane != paneList {
		t.Error("Ctrl+Right should switch to list pane")
	}
}

func TestConnectionModelCtrlLeftSwitchesToForm(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "srv", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	// Move to list first
	ctrlRight := tea.KeyMsg{Type: tea.KeyCtrlRight}
	model, _ := m.Update(ctrlRight)
	m = model.(ConnectionModel)
	if m.activePane != paneList {
		t.Fatal("should be in list pane after Ctrl+Right")
	}

	ctrlLeft := tea.KeyMsg{Type: tea.KeyCtrlLeft}
	model, _ = m.Update(ctrlLeft)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Error("Ctrl+Left should switch back to form pane")
	}
}

func TestConnectionModelCtrlRightNoItems(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	ctrlRight := tea.KeyMsg{Type: tea.KeyCtrlRight}
	model, _ := m.Update(ctrlRight)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Error("Ctrl+Right with no items should stay in form pane")
	}
}

func TestConnectionModelCtrlLeftAlreadyInForm(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	ctrlLeft := tea.KeyMsg{Type: tea.KeyCtrlLeft}
	model, _ := m.Update(ctrlLeft)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Error("Ctrl+Left when already in form should stay in form pane")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - SSH config hosts in list
// ---------------------------------------------------------------------------

func TestConnectionModelSSHHostsAppearInList(t *testing.T) {
	cfg := &config.Config{}
	hosts := []config.SSHHost{
		{Alias: "prod", HostName: "prod.example.com", User: "deploy", Port: "22"},
		{Alias: "staging", HostName: "staging.example.com", User: "deploy"},
	}
	m := NewConnectionModelWithSSH(cfg, hosts)
	if !m.hasItems {
		t.Error("hasItems should be true")
	}
	m.width = 120
	m.height = 40
	view := m.View()
	if view == "" {
		t.Error("view should not be empty when SSH hosts exist")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - fillForm
// ---------------------------------------------------------------------------

func TestConnectionModelFillForm(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	conn := config.Connection{
		Host:                  "filled.example.com",
		Port:                  "2222",
		Username:              "filleduser",
		KeyPath:               "/tmp/key",
		ProxyJump:             "bastion:2222",
		StrictHostKeyChecking: "no",
		UserKnownHostsFile:    "/dev/null",
	}
	m.fillForm(conn)
	if m.inputs[fieldHost].Value() != "filled.example.com" {
		t.Errorf("host = %q", m.inputs[fieldHost].Value())
	}
	if m.inputs[fieldPort].Value() != "2222" {
		t.Errorf("port = %q", m.inputs[fieldPort].Value())
	}
	if m.inputs[fieldUser].Value() != "filleduser" {
		t.Errorf("user = %q", m.inputs[fieldUser].Value())
	}
	if m.inputs[fieldKey].Value() != "/tmp/key" {
		t.Errorf("key = %q", m.inputs[fieldKey].Value())
	}
	if m.inputs[fieldJump].Value() != "bastion:2222" {
		t.Errorf("jump = %q", m.inputs[fieldJump].Value())
	}
	if m.inputs[fieldHostKeyCheck].Value() != "no" {
		t.Errorf("hostKeyCheck = %q", m.inputs[fieldHostKeyCheck].Value())
	}
	if m.inputs[fieldKnownHostsFile].Value() != "/dev/null" {
		t.Errorf("knownHostsFile = %q", m.inputs[fieldKnownHostsFile].Value())
	}
	// Advanced section should auto-expand because non-default values are set.
	if !m.showAdvanced {
		t.Error("showAdvanced should be true when advanced fields have non-default values")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - SetError
// ---------------------------------------------------------------------------

func TestConnectionModelSetError(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.SetError("bad host")
	if m.err != "bad host" {
		t.Errorf("err = %q, want %q", m.err, "bad host")
	}
	// Clear error
	m.SetError("")
	if m.err != "" {
		t.Errorf("err should be empty after clear, got %q", m.err)
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - list pane Enter with invalid selection
// ---------------------------------------------------------------------------

func TestConnectionModelListPaneEnterNoSelection(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.activePane = paneList

	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	model, cmd := m.Update(enterMsg)
	_ = model.(ConnectionModel)
	if cmd != nil {
		t.Error("Enter in list pane with no items should return nil cmd")
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - list pane key forwarding
// ---------------------------------------------------------------------------

func TestConnectionModelListPaneKeyForwarding(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "srv", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	// Switch to list pane
	ctrlRight := tea.KeyMsg{Type: tea.KeyCtrlRight}
	model, _ := m.Update(ctrlRight)
	m = model.(ConnectionModel)

	// Send a regular char, should forward to list
	charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	model, _ = m.Update(charMsg)
	_ = model.(ConnectionModel)
	// Should not crash
}

// ---------------------------------------------------------------------------
// ConnectionModel - View with dimmed form (list focused)
// ---------------------------------------------------------------------------

func TestConnectionModelViewListFocused(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "srv", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	m.activePane = paneList
	view := m.View()
	if view == "" {
		t.Error("view should not be empty when list is focused")
	}
}

// ---------------------------------------------------------------------------
// connItem Description fallback to host
// ---------------------------------------------------------------------------

func TestConnItemDescriptionNoName(t *testing.T) {
	r := connItem{conn: config.Connection{Host: "myhost.com"}, source: "recent"}
	want := "[recent] myhost.com"
	if r.Description() != want {
		t.Errorf("Description = %q, want %q", r.Description(), want)
	}
}

// ---------------------------------------------------------------------------
// Advanced toggle
// ---------------------------------------------------------------------------

func TestAdvancedToggleEnterExpands(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	tab := tea.KeyMsg{Type: tea.KeyTab}
	enter := tea.KeyMsg{Type: tea.KeyEnter}

	// Host → User
	model, _ := m.Update(tab)
	m = model.(ConnectionModel)
	// User → Toggle
	model, _ = m.Update(tab)
	m = model.(ConnectionModel)
	if !m.focusOnToggle {
		t.Fatal("expected focusOnToggle after 2 tabs")
	}
	if m.showAdvanced {
		t.Fatal("advanced should be hidden initially")
	}
	// Enter toggles advanced open
	model, _ = m.Update(enter)
	m = model.(ConnectionModel)
	if !m.showAdvanced {
		t.Error("Enter on toggle should expand advanced section")
	}
	// Enter toggles advanced closed
	model, _ = m.Update(enter)
	m = model.(ConnectionModel)
	if m.showAdvanced {
		t.Error("Enter on toggle again should collapse advanced section")
	}
}

func TestAdvancedToggleTabSkipsCollapsed(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	tab := tea.KeyMsg{Type: tea.KeyTab}

	// Host → User → Toggle → Host (skip advanced since collapsed)
	model, _ := m.Update(tab) // → User
	m = model.(ConnectionModel)
	model, _ = m.Update(tab) // → Toggle
	m = model.(ConnectionModel)
	if !m.focusOnToggle {
		t.Fatal("expected focusOnToggle")
	}
	model, _ = m.Update(tab) // → Host (collapsed, so wraps)
	m = model.(ConnectionModel)
	if m.focusOnToggle {
		t.Error("should not be on toggle")
	}
	if m.focused != fieldHost {
		t.Errorf("focused = %d, want fieldHost(%d)", m.focused, fieldHost)
	}
}

func TestAdvancedToggleTabEntersAdvanced(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.showAdvanced = true
	tab := tea.KeyMsg{Type: tea.KeyTab}

	// Host → User → Toggle → Port (expanded)
	model, _ := m.Update(tab) // → User
	m = model.(ConnectionModel)
	model, _ = m.Update(tab) // → Toggle
	m = model.(ConnectionModel)
	model, _ = m.Update(tab) // → Port
	m = model.(ConnectionModel)
	if m.focused != fieldPort {
		t.Errorf("focused = %d, want fieldPort(%d)", m.focused, fieldPort)
	}
}

func TestAdvancedFieldsInputBlocked(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.focusOnToggle = true

	// Typing while on toggle should do nothing (no crash, no input forwarding)
	charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	model, _ := m.Update(charMsg)
	_ = model.(ConnectionModel)
}

func TestFillFormDefaultPortNoExpand(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	conn := config.Connection{
		Host:     "example.com",
		Port:     "22",
		Username: "user",
	}
	m.fillForm(conn)
	if m.showAdvanced {
		t.Error("showAdvanced should be false when only default port is set")
	}
}

func TestViewContainsAdvancedToggle(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "Advanced Options") {
		t.Error("view should contain the Advanced Options toggle")
	}
	if strings.Contains(view, "Host Check") {
		t.Error("host check field should be hidden when advanced is collapsed")
	}
}

func TestViewShowsAdvancedFields(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.showAdvanced = true
	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "Host Check") {
		t.Error("view should show Host Check label when advanced is expanded")
	}
	if !strings.Contains(view, "Known Hosts") {
		t.Error("view should show Known Hosts label when advanced is expanded")
	}
	if !strings.Contains(view, "Jump Host") {
		t.Error("view should show Jump Host label when advanced is expanded")
	}
}

func TestSubmitFormIncludesAdvancedFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.inputs[fieldHost].SetValue("example.com")
	m.inputs[fieldUser].SetValue("admin")
	m.inputs[fieldHostKeyCheck].SetValue("no")
	m.inputs[fieldKnownHostsFile].SetValue("/dev/null")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from submit")
	}
	msg := cmd()
	connMsg, ok := msg.(ConnectMsg)
	if !ok {
		t.Fatalf("expected ConnectMsg, got %T", msg)
	}
	if connMsg.Conn.StrictHostKeyChecking != "no" {
		t.Errorf("StrictHostKeyChecking = %q, want \"no\"", connMsg.Conn.StrictHostKeyChecking)
	}
	if connMsg.Conn.UserKnownHostsFile != "/dev/null" {
		t.Errorf("UserKnownHostsFile = %q, want \"/dev/null\"", connMsg.Conn.UserKnownHostsFile)
	}
}

func TestCtrlRightFromToggle(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "srv", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.focusOnToggle = true

	ctrlRight := tea.KeyMsg{Type: tea.KeyCtrlRight}
	model, _ := m.Update(ctrlRight)
	m = model.(ConnectionModel)
	if m.activePane != paneList {
		t.Error("Ctrl+Right from toggle should switch to list pane")
	}
}

func TestCtrlLeftBackToToggle(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "srv", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.focusOnToggle = true
	m.activePane = paneList

	ctrlLeft := tea.KeyMsg{Type: tea.KeyCtrlLeft}
	model, _ := m.Update(ctrlLeft)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Error("Ctrl+Left should switch back to form")
	}
	if !m.focusOnToggle {
		t.Error("focusOnToggle should be preserved when returning from list")
	}
}

func TestRetreatFieldFromAdvanced(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.showAdvanced = true
	m.focused = fieldKnownHostsFile
	m.inputs[fieldKnownHostsFile].Focus()

	up := tea.KeyMsg{Type: tea.KeyUp}
	// KnownHostsFile → HostKeyCheck → Jump → Key → Port → Toggle → User → Host
	expected := []connectionField{fieldHostKeyCheck, fieldJump, fieldKey, fieldPort}
	for _, want := range expected {
		model, _ := m.Update(up)
		m = model.(ConnectionModel)
		if m.focused != want {
			t.Fatalf("focused = %d, want %d", m.focused, want)
		}
	}
	// Port → Toggle
	model, _ := m.Update(up)
	m = model.(ConnectionModel)
	if !m.focusOnToggle {
		t.Fatal("expected focusOnToggle after retreat from Port")
	}
	// Toggle → User
	model, _ = m.Update(up)
	m = model.(ConnectionModel)
	if m.focused != fieldUser {
		t.Fatalf("focused = %d, want fieldUser", m.focused)
	}
}

// ---------------------------------------------------------------------------
// SetConnecting / ClearConnecting
// ---------------------------------------------------------------------------

func TestSetConnecting(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.err = "old error"
	m.SetConnecting("admin@host:22")
	if !m.connecting {
		t.Error("connecting should be true")
	}
	if m.connectTarget != "admin@host:22" {
		t.Errorf("connectTarget = %q, want %q", m.connectTarget, "admin@host:22")
	}
	if m.err != "" {
		t.Errorf("err should be cleared, got %q", m.err)
	}
}

func TestClearConnecting(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.SetConnecting("admin@host:22")
	m.ClearConnecting()
	if m.connecting {
		t.Error("connecting should be false after ClearConnecting")
	}
	if m.connectTarget != "" {
		t.Errorf("connectTarget should be cleared, got %q", m.connectTarget)
	}
}

func TestSetErrorClearsConnecting(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.SetConnecting("admin@host:22")
	m.SetError("connection refused")
	if m.connecting {
		t.Error("connecting should be false after SetError")
	}
	if m.err != "connection refused" {
		t.Errorf("err = %q, want %q", m.err, "connection refused")
	}
}

func TestViewShowsConnecting(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	m.SetConnecting("admin@example.com:22")
	view := m.View()
	if !strings.Contains(view, "Connecting") {
		t.Error("view should show connecting indicator")
	}
	if !strings.Contains(view, "admin@example.com:22") {
		t.Error("view should show the connection target")
	}
}

func TestViewTruncatesLongError(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 80
	m.height = 40
	m.err = strings.Repeat("x", 200)
	view := m.View()
	// The error should not contain the full 200-char string.
	if strings.Contains(view, strings.Repeat("x", 200)) {
		t.Error("long error should be truncated in the view")
	}
	if !strings.Contains(view, "…") {
		t.Error("truncated error should end with ellipsis")
	}
}

// ---------------------------------------------------------------------------
// Recent logins display and number-key selection
// ---------------------------------------------------------------------------

func TestViewShowsRecentLogins(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "admin@host1", Host: "host1.example.com", Port: "22", Username: "admin"},
			{Name: "deploy@prod", Host: "prod.example.com", Port: "22", Username: "deploy"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "Recent Logins") {
		t.Error("view should show Recent Logins header")
	}
	if !strings.Contains(view, "admin@host1.example.com:22") {
		t.Error("view should show first recent login")
	}
	if !strings.Contains(view, "deploy@prod.example.com:22") {
		t.Error("view should show second recent login")
	}
}

func TestViewNoRecentLoginsWhenEmpty(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	view := m.View()
	if strings.Contains(view, "Recent Logins") {
		t.Error("view should not show Recent Logins when there are none")
	}
}

func TestViewRecentLoginsMaxFive(t *testing.T) {
	var conns []config.Connection
	for i := 1; i <= 8; i++ {
		conns = append(conns, config.Connection{
			Host: fmt.Sprintf("h%d", i), Port: "22", Username: "u",
		})
	}
	cfg := &config.Config{RecentConnections: conns}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "u@h5:22") {
		t.Error("should show up to 5 recent logins")
	}
	if strings.Contains(view, "u@h6:22") {
		t.Error("should not show 6th recent login")
	}
}

func TestRecentLoginNoUsername(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "nouser.example.com", Port: "22"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	view := m.View()
	if !strings.Contains(view, "nouser.example.com:22") {
		t.Error("should display host:port when username is empty")
	}
}

// ---------------------------------------------------------------------------
// Recent panel navigation (Ctrl+Down/Up, arrows, Enter)
// ---------------------------------------------------------------------------

func TestTabEntersRecentPaneFromToggle(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	// Navigate to toggle (Host → User → toggle)
	tab := tea.KeyMsg{Type: tea.KeyTab}
	model, _ := m.Update(tab) // Host → User
	m = model.(ConnectionModel)
	model, _ = m.Update(tab) // User → toggle
	m = model.(ConnectionModel)
	if !m.focusOnToggle {
		t.Fatal("expected focusOnToggle after two Tabs")
	}
	model, _ = m.Update(tab) // toggle → recent
	m = model.(ConnectionModel)
	if m.activePane != paneRecent {
		t.Errorf("activePane = %d, want paneRecent(%d)", m.activePane, paneRecent)
	}
	if m.recentIdx != 0 {
		t.Errorf("recentIdx = %d, want 0", m.recentIdx)
	}
}

func TestTabSkipsRecentWhenNone(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	tab := tea.KeyMsg{Type: tea.KeyTab}
	model, _ := m.Update(tab) // Host → User
	m = model.(ConnectionModel)
	model, _ = m.Update(tab) // User → toggle
	m = model.(ConnectionModel)
	model, _ = m.Update(tab) // toggle → Host (no recent)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Error("Tab from toggle with no recent connections should stay in form pane")
	}
	if m.focused != fieldHost {
		t.Errorf("focused = %d, want fieldHost(%d)", m.focused, fieldHost)
	}
}

func TestShiftTabFromRecentReturnsToToggle(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.activePane = paneRecent
	m.recentIdx = 0

	shiftTab := tea.KeyMsg{Type: tea.KeyShiftTab}
	model, _ := m.Update(shiftTab)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Errorf("Shift+Tab from first recent should return to form, got pane=%d", m.activePane)
	}
	if !m.focusOnToggle {
		t.Error("Shift+Tab from first recent should focus on toggle")
	}
}

func TestRecentPaneArrowNavigation(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "h1", Port: "22", Username: "u1"},
			{Host: "h2", Port: "22", Username: "u2"},
			{Host: "h3", Port: "22", Username: "u3"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.activePane = paneRecent
	m.recentIdx = 0

	down := tea.KeyMsg{Type: tea.KeyDown}
	up := tea.KeyMsg{Type: tea.KeyUp}

	// Move down
	model, _ := m.Update(down)
	m = model.(ConnectionModel)
	if m.recentIdx != 1 {
		t.Errorf("recentIdx = %d, want 1", m.recentIdx)
	}

	model, _ = m.Update(down)
	m = model.(ConnectionModel)
	if m.recentIdx != 2 {
		t.Errorf("recentIdx = %d, want 2", m.recentIdx)
	}

	// Going past the end should wrap to Host field
	model, _ = m.Update(down)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Errorf("activePane = %d, want paneForm after down past end", m.activePane)
	}
	if m.focused != fieldHost {
		t.Errorf("focused = %d, want fieldHost after wrapping", m.focused)
	}

	// Reset to recent pane for up tests
	m.activePane = paneRecent
	m.recentIdx = 1

	// Move back up
	model, _ = m.Update(up)
	m = model.(ConnectionModel)
	if m.recentIdx != 0 {
		t.Errorf("recentIdx = %d, want 0", m.recentIdx)
	}

	// Going past the start should return to toggle
	model, _ = m.Update(up)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Errorf("activePane = %d, want paneForm after up past start", m.activePane)
	}
	if !m.focusOnToggle {
		t.Error("expected focusOnToggle after up past first recent entry")
	}
}

func TestRecentPaneEnterSelectsAndConnects(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "host1.example.com", Port: "22", Username: "admin"},
			{Host: "host2.example.com", Port: "2222", Username: "deploy"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.activePane = paneRecent
	m.recentIdx = 1

	enter := tea.KeyMsg{Type: tea.KeyEnter}
	model, cmd := m.Update(enter)
	m = model.(ConnectionModel)
	if cmd == nil {
		t.Fatal("expected a connect command from Enter in recent pane")
	}
	if m.inputs[fieldHost].Value() != "host2.example.com" {
		t.Errorf("host = %q, want %q", m.inputs[fieldHost].Value(), "host2.example.com")
	}
	if m.inputs[fieldUser].Value() != "deploy" {
		t.Errorf("user = %q, want %q", m.inputs[fieldUser].Value(), "deploy")
	}
	if m.inputs[fieldPort].Value() != "2222" {
		t.Errorf("port = %q, want %q", m.inputs[fieldPort].Value(), "2222")
	}
	if m.activePane != paneForm {
		t.Errorf("after Enter, should return to form pane, got %d", m.activePane)
	}
}

func TestRecentPaneViewHighlightsSelected(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "h1", Port: "22", Username: "u1"},
			{Host: "h2", Port: "22", Username: "u2"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.width = 120
	m.height = 40
	m.activePane = paneRecent
	m.recentIdx = 0
	view := m.View()
	if !strings.Contains(view, "▸") {
		t.Error("view should show selection cursor ▸ when recent pane is active")
	}
}

func TestCtrlRightFromRecentPane(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.activePane = paneRecent

	ctrlRight := tea.KeyMsg{Type: tea.KeyCtrlRight}
	model, _ := m.Update(ctrlRight)
	m = model.(ConnectionModel)
	if m.activePane != paneList {
		t.Errorf("Ctrl+Right from recent should go to list pane, got %d", m.activePane)
	}
}

func TestCtrlLeftFromRecentPane(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	m.activePane = paneRecent

	ctrlLeft := tea.KeyMsg{Type: tea.KeyCtrlLeft}
	model, _ := m.Update(ctrlLeft)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Errorf("Ctrl+Left from recent should go to form pane, got %d", m.activePane)
	}
}

func TestRecentPaneKeysDoNotLeakToInputs(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	hostBefore := m.inputs[fieldHost].Value()
	m.activePane = paneRecent

	// Type a character — should be absorbed, not forwarded to any input.
	charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")}
	model, _ := m.Update(charMsg)
	m = model.(ConnectionModel)
	if m.inputs[fieldHost].Value() != hostBefore {
		t.Errorf("host input changed to %q while in recent pane", m.inputs[fieldHost].Value())
	}
}
