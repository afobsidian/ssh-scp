package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tab represents a single connection tab.
type Tab struct {
	Title     string
	Connected bool
}

var (
	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Background(lipgloss.Color("#1A1A1A")).
				Padding(0, 2)

	tabBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#0F0F0F"))
)

// RenderTabBar renders the tab bar for the given tabs and active tab index.
func RenderTabBar(tabs []Tab, active int, width int) string {
	var parts []string
	for i, tab := range tabs {
		label := tab.Title
		if tab.Connected {
			label = "● " + label
		} else {
			label = "○ " + label
		}
		if i == active {
			parts = append(parts, tabActiveStyle.Render(label))
		} else {
			parts = append(parts, tabInactiveStyle.Render(label))
		}
	}

	parts = append(parts, tabInactiveStyle.Render("+"))

	bar := strings.Join(parts, " ")
	padding := width - lipgloss.Width(bar)
	if padding > 0 {
		bar += strings.Repeat(" ", padding)
	}

	return tabBarStyle.Width(width).Render(bar)
}

// TabTitle generates a tab title from connection info.
func TabTitle(user, host string, index int) string {
	if user != "" && host != "" {
		return fmt.Sprintf("%s@%s", user, host)
	}
	return fmt.Sprintf("Connection %d", index+1)
}
