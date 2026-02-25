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

func TestRecentItemTitle(t *testing.T) {
	r := recentItem{conn: config.Connection{
		Username: "admin",
		Host:     "example.com",
		Port:     "22",
	}}
	got := r.Title()
	if got != "admin@example.com:22" {
		t.Errorf("Title = %q, want %q", got, "admin@example.com:22")
	}
}

func TestRecentItemDescription(t *testing.T) {
	r := recentItem{conn: config.Connection{Name: "my server"}}
	if r.Description() != "my server" {
		t.Errorf("Description = %q", r.Description())
	}
}

func TestRecentItemFilterValue(t *testing.T) {
	r := recentItem{conn: config.Connection{Host: "host1"}}
	if r.FilterValue() != "host1" {
		t.Errorf("FilterValue = %q", r.FilterValue())
	}
}

// ---------------------------------------------------------------------------
// NewConnectionModel
// ---------------------------------------------------------------------------

func TestNewConnectionModelDefaults(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModel(cfg)
	// Port should default to 22
	portVal := m.inputs[fieldPort].Value()
	if portVal != "22" {
		t.Errorf("default port = %q, want %q", portVal, "22")
	}
	// Host field should be focused
	if m.focused != fieldHost {
		t.Errorf("focused = %d, want fieldHost(%d)", m.focused, fieldHost)
	}
}

func TestNewConnectionModelWithRecent(t *testing.T) {
	cfg := &config.Config{
		RecentConnections: []config.Connection{
			{Name: "test", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	m := NewConnectionModel(cfg)
	if !m.showRecent {
		t.Error("showRecent should be true when recent connections exist")
	}
}

func TestNewConnectionModelNoRecent(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModel(cfg)
	if m.showRecent {
		t.Error("showRecent should be false when no recent connections")
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
	m := NewConnectionModel(cfg)

	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	model, _ := m.Update(tabMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldPort {
		t.Errorf("after tab: focused = %d, want fieldPort(%d)", m.focused, fieldPort)
	}
}

func TestConnectionModelShiftTabNavigation(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModel(cfg)

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
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
	m.err = "test error"
	view := m.View()
	if !strings.Contains(view, "test error") {
		t.Error("view should show error")
	}
}

func TestConnectionModelInit(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	model, _ := m.Update(downMsg)
	m = model.(ConnectionModel)
	if m.focused != fieldPort {
		t.Errorf("after down: focused = %d, want fieldPort(%d)", m.focused, fieldPort)
	}
}

func TestConnectionModelUpNavigation(t *testing.T) {
	cfg := &config.Config{}
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
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
	m := NewConnectionModel(cfg)
	// Type some chars
	charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("test")}
	model, _ := m.Update(charMsg)
	m = model.(ConnectionModel)
	val := m.inputs[fieldHost].Value()
	if val != "test" {
		t.Errorf("host input = %q, want %q", val, "test")
	}
}
