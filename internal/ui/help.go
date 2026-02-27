package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var helpContent = `
  File Browser
  ^←/→      Switch between local and remote panels
  Tab       Switch between local and remote panels
  ^T        Transfer selected file (upload or download)
  ^K        Create new directory
  ^D        Delete selected file/directory
  ^R        Rename selected file/directory
  ^]        Switch to next tab
  ^N        New connection tab
  ^W        Close current tab
  Enter     Navigate into directory / edit text file
  Backspace Go up one directory
  ?         Toggle this help overlay
  ^C        Quit

  Editor (Vim-style)
  Modes: NORMAL → i/I/a/A/o/O → INSERT, v → VISUAL, : → COMMAND, / → FIND

  Normal Mode
  h/j/k/l       Move left/down/up/right
  w/b/e         Word forward / backward / end
  0/^/$         Line start / first non-blank / line end
  gg/G          Go to first / last line
  ^D/U          Half-page down / up
  ^F/B          Full page down / up
  dd/yy/cc      Delete / yank / change line
  x/X           Delete char forward / backward
  p/P           Paste after / before cursor
  r<char>       Replace character under cursor
  J             Join current line with next
  u / ^R        Undo / redo
  /term         Search forward     ?term — search backward
  n/N           Next / previous search match
  :w  :q  :wq   Save / quit / save & quit
  :<number>     Go to line
  Esc           Close editor (from normal mode)
  ^S            Save (any mode)
`

var helpStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7D56F4")).
	Padding(1, 3).
	Bold(false)

// RenderHelp returns the help overlay view.
func RenderHelp(width, height int) string {
	box := helpStyle.Render(helpContent)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
