package ui

import (
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
	if fieldCount != 5 {
		t.Errorf("fieldCount = %d, want 5", fieldCount)
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
	if m.focused != fieldPort {
		t.Errorf("after tab: focused = %d, want fieldPort(%d)", m.focused, fieldPort)
	}
}

func TestConnectionModelShiftTabNavigation(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)

	msg := tea.KeyMsg{Type: tea.KeyShiftTab}
	model, _ := m.Update(msg)
	m = model.(ConnectionModel)
	// Should wrap to last field
	if m.focused != fieldKey {
		t.Errorf("after shift+tab: focused = %d, want fieldKey(%d)", m.focused, fieldKey)
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
	m.inputs[fieldPass].SetValue("secret")

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
	if m.focused != fieldPort {
		t.Errorf("after down: focused = %d, want fieldPort(%d)", m.focused, fieldPort)
	}
}

func TestConnectionModelUpNavigation(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	model, _ := m.Update(upMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldKey {
		t.Errorf("after up from host: focused = %d, want fieldKey(%d)", m.focused, fieldKey)
	}
}

// ---------------------------------------------------------------------------
// ConnectionModel - Cycle through all fields
// ---------------------------------------------------------------------------

func TestConnectionModelCycleFields(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}

	for i := 1; i < int(fieldCount); i++ {
		model, _ := m.Update(tabMsg)
		m = model.(ConnectionModel)
		if m.focused != connectionField(i) {
			t.Errorf("after %d tabs: focused = %d, want %d", i, m.focused, i)
		}
	}
	// Cycle back to host
	model, _ := m.Update(tabMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldHost {
		t.Errorf("after full cycle: focused = %d, want fieldHost(%d)", m.focused, fieldHost)
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
// ConnectionModel - Ctrl+L pane toggle
// ---------------------------------------------------------------------------

func TestConnectionModelCtrlLToggleWithItems(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "srv", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModelWithSSH(cfg, nil)
	if m.activePane != paneForm {
		t.Fatal("should start in form pane")
	}

	ctrlL := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l"), Alt: true}
	// Simulate ctrl+l — the key string is "ctrl+l"
	ctrlLMsg := tea.KeyMsg{Type: tea.KeyCtrlL}
	model, _ := m.Update(ctrlLMsg)
	_ = ctrlL // prevent unused
	m = model.(ConnectionModel)
	if m.activePane != paneList {
		t.Error("Ctrl+L should switch to list pane")
	}

	model, _ = m.Update(ctrlLMsg)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Error("Ctrl+L again should switch back to form pane")
	}
}

func TestConnectionModelCtrlLNoItems(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModelWithSSH(cfg, nil)
	ctrlLMsg := tea.KeyMsg{Type: tea.KeyCtrlL}
	model, _ := m.Update(ctrlLMsg)
	m = model.(ConnectionModel)
	if m.activePane != paneForm {
		t.Error("Ctrl+L with no items should stay in form pane")
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
		Host:     "filled.example.com",
		Port:     "2222",
		Username: "filleduser",
		Password: "filledpass",
		KeyPath:  "/tmp/key",
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
}
