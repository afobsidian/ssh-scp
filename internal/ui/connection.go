package ui

import (
	"fmt"
	"log"
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
	fieldUser
	fieldPort
	fieldKey
	fieldJump
	fieldHostKeyCheck
	fieldKnownHostsFile
	fieldCount
)

// firstAdvancedField is the first field in the collapsible advanced section.
const firstAdvancedField = fieldPort

// connectionPane tracks which section has keyboard focus on the connection screen.
type connectionPane int

const (
	paneForm connectionPane = iota
	paneList
	paneRecent
)

// ConnectionModel is the connection screen.
type ConnectionModel struct {
	inputs        []textinput.Model
	focused       connectionField
	showAdvanced  bool
	focusOnToggle bool
	connList      list.Model
	hasItems      bool
	activePane    connectionPane
	recentIdx     int
	selectedName  string // SSH config alias; empty for manual entries
	cfg           *config.Config
	sshHosts      []config.SSHHost
	width         int
	height        int
	err           string
	connecting    bool
	connectTarget string
}

// connItem is a list item representing either a recent connection or an SSH config host.
type connItem struct {
	conn   config.Connection
	source string // "recent" or "ssh-config"
}

func (c connItem) Title() string {
	if c.conn.Name != "" {
		return c.conn.Name
	}
	return c.conn.Host
}
func (c connItem) Description() string {
	if c.conn.Username == "" {
		return fmt.Sprintf("%s:%s", c.conn.Host, c.conn.Port)
	}
	return fmt.Sprintf("%s@%s:%s", c.conn.Username, c.conn.Host, c.conn.Port)
}
func (c connItem) FilterValue() string {
	return c.conn.Host + " " + c.conn.Name
}

// NewConnectionModel creates a new connection screen model.
func NewConnectionModel(cfg *config.Config) ConnectionModel {
	return NewConnectionModelWithSSH(cfg, config.LoadSSHConfig())
}

// SetError sets an error message to display on the connection screen.
func (m *ConnectionModel) SetError(msg string) {
	m.err = msg
	if msg != "" {
		m.connecting = false
	}
}

// SetConnecting sets the connecting state with a target description.
func (m *ConnectionModel) SetConnecting(target string) {
	m.connecting = true
	m.connectTarget = target
	m.err = ""
}

// ClearConnecting clears the connecting state.
func (m *ConnectionModel) ClearConnecting() {
	m.connecting = false
	m.connectTarget = ""
}

// NewConnectionModelWithSSH creates a connection screen with explicit SSH config hosts.
func NewConnectionModelWithSSH(cfg *config.Config, sshHosts []config.SSHHost) ConnectionModel {
	inputs := make([]textinput.Model, fieldCount)
	placeholders := []string{"Host", "Username", "Port", "SSH Key Path", "Jump Host", "Host Key Checking", "Known Hosts File"}
	for i := range inputs {
		t := textinput.New()
		t.Placeholder = placeholders[i]
		t.CharLimit = 256
		inputs[i] = t
	}
	inputs[fieldPort].SetValue("22")
	inputs[fieldJump].Placeholder = "user@host:port"
	inputs[fieldHostKeyCheck].Placeholder = "yes / no / ask (default: ask)"
	inputs[fieldKnownHostsFile].Placeholder = "/dev/null"
	inputs[fieldHost].Focus()

	// Build list from SSH config hosts.
	var items []list.Item
	for _, h := range sshHosts {
		items = append(items, connItem{conn: h.ToConnection(), source: "ssh-config"})
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FFFFFF")).
		BorderForeground(lipgloss.Color("#7D56F4"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#AAAAAA")).
		BorderForeground(lipgloss.Color("#7D56F4"))

	l := list.New(items, delegate, 44, 14)
	l.Title = "Connections"
	l.SetShowStatusBar(false)
	l.SetShowTitle(true)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

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
		log.Printf("[ConnectionModel] key: type=%d string=%q runes=%v alt=%v pane=%d focused=%d",
			msg.Type, msg.String(), msg.Runes, msg.Alt, m.activePane, m.focused)

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			log.Printf("[ConnectionModel] Enter pressed, pane=%d host=%q user=%q toggle=%v",
				m.activePane, m.inputs[fieldHost].Value(), m.inputs[fieldUser].Value(), m.focusOnToggle)
			if m.activePane == paneForm && m.focusOnToggle {
				m.showAdvanced = !m.showAdvanced
				return m, nil
			}
			if m.activePane == paneRecent {
				// Select from recent connections.
				max := len(m.cfg.RecentConnections)
				if max > 8 {
					max = 8
				}
				if m.recentIdx < max {
					m.fillForm(m.cfg.RecentConnections[m.recentIdx])
					m.activePane = paneForm
					m.focusOnToggle = false
					m.inputs[m.focused].Focus()
					cmd := m.submitForm()
					log.Printf("[ConnectionModel] recent selected idx=%d cmd=%v", m.recentIdx, cmd != nil)
					return m, cmd
				}
				return m, nil
			}
			if m.activePane == paneList {
				// Populate form from selected list item and connect.
				if item, ok := m.connList.SelectedItem().(connItem); ok {
					log.Printf("[ConnectionModel] list item selected: %s@%s", item.conn.Username, item.conn.Host)
					m.fillForm(item.conn)
					m.activePane = paneForm
					m.focusOnToggle = false
					m.inputs[m.focused].Focus()
					cmd := m.submitForm()
					log.Printf("[ConnectionModel] submitForm returned cmd=%v err=%q", cmd != nil, m.err)
					return m, cmd
				}
				return m, nil
			}
			cmd := m.submitForm()
			log.Printf("[ConnectionModel] submitForm returned cmd=%v err=%q", cmd != nil, m.err)
			return m, cmd

		case tea.KeyTab, tea.KeyDown:
			if m.activePane == paneRecent {
				max := m.recentMax()
				if m.recentIdx < max-1 {
					m.recentIdx++
				} else {
					// Wrap from last recent entry back to Host.
					m.activePane = paneForm
					m.focused = fieldHost
					m.inputs[m.focused].Focus()
				}
				return m, nil
			}
			if m.activePane == paneForm {
				m.advanceField()
				return m, nil
			}
		case tea.KeyShiftTab, tea.KeyUp:
			if m.activePane == paneRecent {
				if m.recentIdx > 0 {
					m.recentIdx--
				} else {
					// Wrap from first recent entry back to toggle.
					m.activePane = paneForm
					m.focusOnToggle = true
				}
				return m, nil
			}
			if m.activePane == paneForm {
				m.retreatField()
				return m, nil
			}

		case tea.KeyDelete, tea.KeyBackspace:
			if m.activePane == paneRecent {
				max := m.recentMax()
				if m.recentIdx < max {
					m.cfg.RemoveRecent(m.recentIdx)
					_ = config.Save(m.cfg)
					// Adjust cursor if it's now past the end.
					newMax := m.recentMax()
					if newMax == 0 {
						m.activePane = paneForm
						m.focused = fieldHost
						m.inputs[m.focused].Focus()
					} else if m.recentIdx >= newMax {
						m.recentIdx = newMax - 1
					}
				}
				return m, nil
			}

		case tea.KeyCtrlRight:
			if m.hasItems && m.activePane != paneList {
				if m.activePane == paneForm && !m.focusOnToggle {
					m.inputs[m.focused].Blur()
				}
				m.activePane = paneList
			}
			return m, nil

		case tea.KeyCtrlLeft:
			if m.activePane != paneForm {
				m.activePane = paneForm
				if !m.focusOnToggle {
					m.inputs[m.focused].Focus()
				}
			}
			return m, nil
		}
	}

	if m.activePane == paneList {
		var cmd tea.Cmd
		m.connList, cmd = m.connList.Update(msg)
		return m, cmd
	}

	if m.activePane == paneRecent || m.focusOnToggle {
		return m, nil
	}

	var cmd tea.Cmd
	m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
	// Clear the selected name when the user manually edits form fields
	// so that only connections chosen from the list/recent carry a name.
	if _, ok := msg.(tea.KeyMsg); ok {
		m.selectedName = ""
	}
	return m, cmd
}

// recentMax returns the number of visible recent entries (capped at 8).
func (m *ConnectionModel) recentMax() int {
	n := len(m.cfg.RecentConnections)
	if n > 8 {
		n = 8
	}
	return n
}

// advanceField moves focus to the next visible field in the form.
func (m *ConnectionModel) advanceField() {
	if m.focusOnToggle {
		m.focusOnToggle = false
		if m.showAdvanced {
			m.focused = firstAdvancedField
			m.inputs[m.focused].Focus()
		} else if m.recentMax() > 0 {
			// Move into the recent panel.
			m.activePane = paneRecent
			m.recentIdx = 0
		} else {
			m.focused = fieldHost
			m.inputs[m.focused].Focus()
		}
		return
	}

	m.inputs[m.focused].Blur()
	switch m.focused {
	case fieldUser:
		m.focusOnToggle = true
	case fieldKnownHostsFile:
		if m.recentMax() > 0 {
			m.activePane = paneRecent
			m.recentIdx = 0
		} else {
			m.focused = fieldHost
			m.inputs[m.focused].Focus()
		}
	default:
		m.focused++
		m.inputs[m.focused].Focus()
	}
}

// retreatField moves focus to the previous visible field in the form.
func (m *ConnectionModel) retreatField() {
	if m.focusOnToggle {
		m.focusOnToggle = false
		m.focused = fieldUser
		m.inputs[m.focused].Focus()
		return
	}

	m.inputs[m.focused].Blur()
	switch m.focused {
	case fieldHost:
		if m.recentMax() > 0 {
			// Go to last recent entry.
			m.activePane = paneRecent
			m.recentIdx = m.recentMax() - 1
		} else if m.showAdvanced {
			m.focused = fieldKnownHostsFile
			m.inputs[m.focused].Focus()
		} else {
			m.focusOnToggle = true
		}
	case firstAdvancedField:
		m.focusOnToggle = true
	case fieldUser:
		m.focused = fieldHost
		m.inputs[m.focused].Focus()
	default:
		m.focused--
		m.inputs[m.focused].Focus()
	}
}

// fillForm populates the input fields from a connection.
func (m *ConnectionModel) fillForm(c config.Connection) {
	m.inputs[fieldHost].SetValue(c.Host)
	m.inputs[fieldUser].SetValue(c.Username)
	m.inputs[fieldPort].SetValue(c.Port)
	m.inputs[fieldKey].SetValue(c.KeyPath)
	m.inputs[fieldJump].SetValue(c.ProxyJump)
	m.inputs[fieldHostKeyCheck].SetValue(c.StrictHostKeyChecking)
	m.inputs[fieldKnownHostsFile].SetValue(c.UserKnownHostsFile)
	m.selectedName = c.Name

	// Auto-expand advanced section if any advanced field has a non-default value.
	m.showAdvanced = (c.Port != "" && c.Port != "22") ||
		c.KeyPath != "" ||
		c.ProxyJump != "" ||
		c.StrictHostKeyChecking != "" ||
		c.UserKnownHostsFile != ""
}

// submitForm validates and submits the form.
func (m *ConnectionModel) submitForm() tea.Cmd {
	host := m.inputs[fieldHost].Value()
	user := m.inputs[fieldUser].Value()
	port := m.inputs[fieldPort].Value()
	key := m.inputs[fieldKey].Value()
	jump := m.inputs[fieldJump].Value()
	hostKeyCheck := m.inputs[fieldHostKeyCheck].Value()
	knownHostsFile := m.inputs[fieldKnownHostsFile].Value()

	if host == "" || user == "" {
		m.err = "Host and username are required"
		return nil
	}
	if port == "" {
		port = "22"
	}

	conn := config.Connection{
		Name:                  m.selectedName,
		Host:                  host,
		Port:                  port,
		Username:              user,
		KeyPath:               key,
		StrictHostKeyChecking: hostKeyCheck,
		UserKnownHostsFile:    knownHostsFile,
		ProxyJump:             jump,
	}
	return func() tea.Msg { return ConnectMsg{Conn: conn} }
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Align(lipgloss.Center).
			Width(50)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Width(12)

	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#555555")).
			Padding(1, 2).
			Width(50)

	focusedInputBoxStyle = inputBoxStyle.
				BorderForeground(lipgloss.Color("#7D56F4"))

	dimBoxStyle = inputBoxStyle.
			BorderForeground(lipgloss.Color("#555555"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true)
)

func (m ConnectionModel) View() string {
	basicFields := []connectionField{fieldHost, fieldUser}
	basicLabels := []string{"Host:", "Username:"}

	formTitleStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)
	var rows []string
	rows = append(rows, formTitleStyle.Render("New Connection"), "")
	for i, f := range basicFields {
		label := labelStyle.Render(basicLabels[i])
		row := lipgloss.JoinHorizontal(lipgloss.Center, label, m.inputs[f].View())
		rows = append(rows, row)
	}

	// Advanced options toggle.
	toggleText := "▸ Advanced Options"
	if m.showAdvanced {
		toggleText = "▾ Advanced Options"
	}
	toggleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).PaddingLeft(1)
	if m.focusOnToggle && m.activePane == paneForm {
		toggleStyle = toggleStyle.Foreground(lipgloss.Color("#7D56F4")).Bold(true)
	}
	rows = append(rows, "", toggleStyle.Render(toggleText))

	if m.showAdvanced {
		advFields := []connectionField{fieldPort, fieldKey, fieldJump, fieldHostKeyCheck, fieldKnownHostsFile}
		advLabels := []string{"Port:", "SSH Key:", "Jump Host:", "Host Check:", "Known Hosts:"}
		rows = append(rows, "")
		for i, f := range advFields {
			label := labelStyle.Render(advLabels[i])
			row := lipgloss.JoinHorizontal(lipgloss.Center, label, m.inputs[f].View())
			rows = append(rows, row)
		}
	}

	form := strings.Join(rows, "\n")
	var boxStyle lipgloss.Style
	if m.activePane == paneForm {
		boxStyle = focusedInputBoxStyle
	} else {
		boxStyle = dimBoxStyle
	}
	box := boxStyle.Render(form)

	title := titleStyle.Render("╔═╗╔═╗╦ ╦  ╔═╗╔═╗╔═╗\n╚═╗╚═╗╠═╣  ╚═╗║  ╠═╝\n╚═╝╚═╝╩ ╩  ╚═╝╚═╝╩  ")

	// Status bar — always present with a max width to prevent layout disruption.
	maxWidth := 44 // inputBoxStyle width (50) minus border/padding (6)
	if m.width > 60 {
		maxWidth = m.width/2 - 6
	}

	var statusText string
	statusMsg := ""
	if m.connecting {
		statusText = "⟳  Connecting to " + m.connectTarget + "…"
		if len(statusText) > maxWidth && maxWidth > 3 {
			statusText = statusText[:maxWidth-1] + "…"
		}
		statusText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).Bold(true).
			Render(statusText)
		statusMsg = statusText
	} else if m.err != "" {
		errText := "⚠  " + m.err
		if len(errText) > maxWidth && maxWidth > 3 {
			errText = errText[:maxWidth-1] + "…"
		}
		statusText = errorStyle.Render(errText)
		statusMsg = statusText
	}

	// recent connections section.
	var recentSection string
	if len(m.cfg.RecentConnections) > 0 {
		recentTitleBarStyle := lipgloss.NewStyle().Padding(0, 0, 0, 2)
		recentTitleStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("62")).
			Foreground(lipgloss.Color("230")).
			Padding(0, 1)
		recentHeader := recentTitleBarStyle.Render(recentTitleStyle.Render("Recent Connections"))

		normalStyle := lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"}).
			Padding(0, 0, 0, 2)
		selectedStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 0, 0, 1)

		var recentRows []string
		max := len(m.cfg.RecentConnections)
		if max > 8 {
			max = 8
		}
		for i := 0; i < max; i++ {
			c := m.cfg.RecentConnections[i]
			var label string
			if c.Name != "" {
				label = c.Name
			} else if c.Username != "" {
				label = fmt.Sprintf("%s@%s:%s", c.Username, c.Host, c.Port)
			} else {
				label = fmt.Sprintf("%s:%s", c.Host, c.Port)
			}
			if m.activePane == paneRecent && i == m.recentIdx {
				recentRows = append(recentRows, selectedStyle.Render(label))
			} else {
				recentRows = append(recentRows, normalStyle.Render(label))
			}
		}
		recentContent := recentHeader + "\n\n" + strings.Join(recentRows, "\n")
		var recentBoxStyle lipgloss.Style
		if m.activePane == paneRecent {
			recentBoxStyle = focusedInputBoxStyle
		} else {
			recentBoxStyle = dimBoxStyle
		}
		recentSection = recentBoxStyle.Render(recentContent)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, title, "", box, statusMsg, recentSection)

	if m.hasItems {
		listView := m.connList.View()
		var listBoxStyle lipgloss.Style
		if m.activePane == paneList {
			listBoxStyle = focusedInputBoxStyle
		} else {
			listBoxStyle = dimBoxStyle
		}
		listBox := listBoxStyle.Render(listView)
		content = lipgloss.JoinHorizontal(lipgloss.Top, content, "  ", listBox)
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}
