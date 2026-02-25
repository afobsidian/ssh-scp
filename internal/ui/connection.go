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

// connectionPane tracks which section has keyboard focus on the connection screen.
type connectionPane int

const (
	paneForm connectionPane = iota
	paneList
)

// ConnectionModel is the connection screen.
type ConnectionModel struct {
	inputs     []textinput.Model
	focused    connectionField
	connList   list.Model
	hasItems   bool
	activePane connectionPane
	cfg        *config.Config
	sshHosts   []config.SSHHost
	width      int
	height     int
	err        string
}

// connItem is a list item representing either a recent connection or an SSH config host.
type connItem struct {
	conn   config.Connection
	source string // "recent" or "ssh-config"
}

func (c connItem) Title() string {
	title := fmt.Sprintf("%s@%s:%s", c.conn.Username, c.conn.Host, c.conn.Port)
	if c.conn.Username == "" {
		title = fmt.Sprintf("%s:%s", c.conn.Host, c.conn.Port)
	}
	return title
}
func (c connItem) Description() string {
	tag := "recent"
	if c.source == "ssh-config" {
		tag = "~/.ssh/config"
	}
	name := c.conn.Name
	if name == "" {
		name = c.conn.Host
	}
	return fmt.Sprintf("[%s] %s", tag, name)
}
func (c connItem) FilterValue() string {
	return c.conn.Host + " " + c.conn.Name
}

// NewConnectionModel creates a new connection screen model.
func NewConnectionModel(cfg *config.Config) ConnectionModel {
	return NewConnectionModelWithSSH(cfg, config.LoadSSHConfig())
}

// NewConnectionModelWithSSH creates a connection screen with explicit SSH config hosts.
func NewConnectionModelWithSSH(cfg *config.Config, sshHosts []config.SSHHost) ConnectionModel {
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

	// Build combined list: SSH config hosts first, then recent connections.
	var items []list.Item
	for _, h := range sshHosts {
		items = append(items, connItem{conn: h.ToConnection(), source: "ssh-config"})
	}
	for _, c := range cfg.RecentConnections {
		items = append(items, connItem{conn: c, source: "recent"})
	}

	l := list.New(items, list.NewDefaultDelegate(), 44, 14)
	l.Title = "Connections"
	l.SetShowStatusBar(false)

	return ConnectionModel{
		inputs:     inputs,
		focused:    fieldHost,
		connList:   l,
		hasItems:   len(items) > 0,
		activePane: paneForm,
		cfg:        cfg,
		sshHosts:   sshHosts,
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
		m.connList.SetWidth(msg.Width / 3)
		listH := msg.Height - 6
		if listH < 4 {
			listH = 4
		}
		m.connList.SetHeight(listH)
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		case "ctrl+l":
			// Toggle between form and connection list.
			if m.hasItems {
				if m.activePane == paneForm {
					m.inputs[m.focused].Blur()
					m.activePane = paneList
				} else {
					m.activePane = paneForm
					m.inputs[m.focused].Focus()
				}
			}
			return m, nil

		case "tab", "down":
			if m.activePane == paneForm {
				m.inputs[m.focused].Blur()
				m.focused = (m.focused + 1) % fieldCount
				m.inputs[m.focused].Focus()
				return m, nil
			}
		case "shift+tab", "up":
			if m.activePane == paneForm {
				m.inputs[m.focused].Blur()
				m.focused = (m.focused - 1 + fieldCount) % fieldCount
				m.inputs[m.focused].Focus()
				return m, nil
			}

		case "enter":
			if m.activePane == paneList {
				// Populate form from selected list item and connect.
				if item, ok := m.connList.SelectedItem().(connItem); ok {
					m.fillForm(item.conn)
					m.activePane = paneForm
					m.inputs[m.focused].Focus()
					return m, m.submitForm()
				}
				return m, nil
			}
			return m, m.submitForm()
		}
	}

	if m.activePane == paneList {
		var cmd tea.Cmd
		m.connList, cmd = m.connList.Update(msg)
		return m, cmd
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	return m, cmd
}

// fillForm populates the input fields from a connection.
func (m *ConnectionModel) fillForm(c config.Connection) {
	m.inputs[fieldHost].SetValue(c.Host)
	m.inputs[fieldPort].SetValue(c.Port)
	m.inputs[fieldUser].SetValue(c.Username)
	m.inputs[fieldPass].SetValue(c.Password)
	m.inputs[fieldKey].SetValue(c.KeyPath)
}

// submitForm validates and submits the form.
func (m *ConnectionModel) submitForm() tea.Cmd {
	host := m.inputs[fieldHost].Value()
	port := m.inputs[fieldPort].Value()
	user := m.inputs[fieldUser].Value()
	pass := m.inputs[fieldPass].Value()
	key := m.inputs[fieldKey].Value()

	if host == "" || user == "" {
		m.err = "Host and username are required"
		return nil
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

	return func() tea.Msg { return ConnectMsg{Conn: conn} }
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

	dimBoxStyle = inputBoxStyle.
			BorderForeground(lipgloss.Color("#333333"))

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
	var boxStyle lipgloss.Style
	if m.activePane == paneForm {
		boxStyle = focusedInputBoxStyle
	} else {
		boxStyle = dimBoxStyle
	}
	box := boxStyle.Render(form)

	title := titleStyle.Render("SSH TUI - New Connection")
	paneHint := "Ctrl+L: connection list"
	if m.activePane == paneList {
		paneHint = "Ctrl+L: form"
	}
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Render(
		"Tab/↑↓: navigate • Enter: connect • " + paneHint + " • Ctrl+C: quit",
	)

	var errMsg string
	if m.err != "" {
		errMsg = "\n" + errorStyle.Render("⚠  "+m.err)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", box, "", hint+errMsg)

	if m.hasItems {
		listView := m.connList.View()
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, "  ", listView)
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
