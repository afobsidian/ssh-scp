package ui

import (
	"fmt"
	"strings"

	"ssh-scp/internal/config"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConnectMsg is sent when the user initiates a connection.
type ConnectMsg struct {
	Conn config.Connection
}

type connectionField int

const (
	fieldHost connectionField = iota
	fieldPort
	fieldUser
	fieldPass
	fieldKey
	fieldCount
)

// ConnectionModel is the connection screen.
type ConnectionModel struct {
	inputs     []textinput.Model
	focused    connectionField
	recentList list.Model
	showRecent bool
	cfg        *config.Config
	width      int
	height     int
	err        string
}

type recentItem struct {
	conn config.Connection
}

func (r recentItem) Title() string {
	return fmt.Sprintf("%s@%s:%s", r.conn.Username, r.conn.Host, r.conn.Port)
}
func (r recentItem) Description() string { return r.conn.Name }
func (r recentItem) FilterValue() string { return r.conn.Host }

// NewConnectionModel creates a new connection screen model.
func NewConnectionModel(cfg *config.Config) ConnectionModel {
	inputs := make([]textinput.Model, fieldCount)
	labels := []string{"Host", "Port", "Username", "Password", "SSH Key Path"}
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = labels[i]
		t.CharLimit = 256
		if i == int(fieldPass) {
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}
		inputs[i] = t
	}
	inputs[fieldPort].SetValue("22")
	inputs[fieldHost].Focus()

	items := make([]list.Item, len(cfg.RecentConnections))
	for i, c := range cfg.RecentConnections {
		items[i] = recentItem{conn: c}
	}
	l := list.New(items, list.NewDefaultDelegate(), 40, 10)
	l.Title = "Recent Connections"

	return ConnectionModel{
		inputs:     inputs,
		focused:    fieldHost,
		recentList: l,
		showRecent: len(cfg.RecentConnections) > 0,
		cfg:        cfg,
	}
}

func (m ConnectionModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m ConnectionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recentList.SetWidth(msg.Width / 2)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "down":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused + 1) % fieldCount
			m.inputs[m.focused].Focus()
		case "shift+tab", "up":
			m.inputs[m.focused].Blur()
			m.focused = (m.focused - 1 + fieldCount) % fieldCount
			m.inputs[m.focused].Focus()
		case "enter":
			host := m.inputs[fieldHost].Value()
			port := m.inputs[fieldPort].Value()
			user := m.inputs[fieldUser].Value()
			pass := m.inputs[fieldPass].Value()
			key := m.inputs[fieldKey].Value()

			if host == "" || user == "" {
				m.err = "Host and username are required"
				return m, nil
			}
			if port == "" {
				port = "22"
			}

			conn := config.Connection{
				Name:     fmt.Sprintf("%s@%s", user, host),
				Host:     host,
				Port:     port,
				Username: user,
				Password: pass,
				KeyPath:  key,
			}
			m.cfg.AddRecent(conn)
			if err := config.Save(m.cfg); err != nil {
				m.err = "Failed to save config: " + err.Error()
			}

			return m, func() tea.Msg { return ConnectMsg{Conn: conn} }
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Width(12)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(1, 2).
			Width(50)

	focusedInputBoxStyle = inputBoxStyle.
				BorderForeground(lipgloss.Color("#7D56F4"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)
)

func (m ConnectionModel) View() string {
	labels := []string{"Host:", "Port:", "Username:", "Password:", "SSH Key:"}
	var rows []string
	for i, inp := range m.inputs {
		label := labelStyle.Render(labels[i])
		row := lipgloss.JoinHorizontal(lipgloss.Center, label, inp.View())
		rows = append(rows, row)
	}

	form := strings.Join(rows, "\n")
	boxStyle := inputBoxStyle
	if m.err == "" {
		boxStyle = focusedInputBoxStyle
	}
	box := boxStyle.Render(form)

	title := titleStyle.Render("SSH TUI - New Connection")
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render("Tab/↑↓: navigate • Enter: connect • Ctrl+C: quit")

	var errMsg string
	if m.err != "" {
		errMsg = "\n" + errorStyle.Render("⚠  "+m.err)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", box, "", hint+errMsg)

	if m.showRecent && len(m.cfg.RecentConnections) > 0 {
		recentView := m.recentList.View()
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, "  ", recentView)
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
