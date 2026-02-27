package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MaxEditableSize is the maximum file size (in bytes) the editor will open.
const MaxEditableSize int64 = 1 << 20 // 1 MB

// OpenEditorMsg requests opening a file in the editor.
type OpenEditorMsg struct {
	Path     string
	IsRemote bool
}

// EditorContentLoadedMsg carries loaded file content for the editor.
type EditorContentLoadedMsg struct {
	Path     string
	Content  string
	IsRemote bool
	Err      error
}

// EditorSaveMsg requests saving the editor content to disk.
type EditorSaveMsg struct {
	Path     string
	Content  string
	IsRemote bool
}

// EditorSaveDoneMsg reports the result of a save operation.
type EditorSaveDoneMsg struct {
	Err error
}

// EditorCloseMsg requests closing the editor and returning to the file browser.
type EditorCloseMsg struct{}

// editorMode represents the current vim editing mode.
type editorMode int

const (
	modeNormal  editorMode = iota
	modeInsert             // text entry mode
	modeVisual             // character-wise visual selection
	modeCommand            // : command-line mode
	modeFind               // / search mode
)

func (m editorMode) String() string {
	switch m {
	case modeNormal:
		return "NORMAL"
	case modeInsert:
		return "INSERT"
	case modeVisual:
		return "VISUAL"
	case modeCommand:
		return "COMMAND"
	case modeFind:
		return "FIND"
	default:
		return "UNKNOWN"
	}
}

// undoEntry stores a snapshot for undo/redo.
type undoEntry struct {
	lines     []string
	cursorRow int
	cursorCol int
}

const maxUndoHistory = 100

// EditorModel is a vim-style text editor.
type EditorModel struct {
	path     string
	isRemote bool
	lines    []string

	cursorRow int
	cursorCol int
	scrollRow int
	scrollCol int
	width     int
	height    int

	mode      editorMode
	dirty     bool
	saving    bool
	statusMsg string

	// Vim state
	pendingOp    rune   // pending operator: 'd', 'y', 'c'
	yankRegister string // internal yank register
	yankLinewise bool   // whether the register holds whole lines

	// Undo/redo
	undoStack []undoEntry
	redoStack []undoEntry

	// Visual mode selection anchor
	visualRow int
	visualCol int

	// Command / find input buffers
	cmdBuffer      string
	findBuffer     string
	lastSearch     string
	searchFwd      bool
	findMatches    [][2]int // row, col pairs of current search matches
	findMatchIdx   int
	closeAfterSave bool // set by :wq to close after save completes
}

// NewEditorModel creates a new editor model with the given file content.
func NewEditorModel(path string, isRemote bool, content string) EditorModel {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}
	m := EditorModel{
		path:      path,
		isRemote:  isRemote,
		lines:     lines,
		mode:      modeNormal,
		searchFwd: true,
	}
	m.pushUndo()
	return m
}

// SetDimensions sets the editor's display dimensions.
func (m *EditorModel) SetDimensions(width, height int) {
	m.width = width
	m.height = height
}

func (m EditorModel) visibleRows() int {
	v := m.height - 3
	if v < 1 {
		v = 1
	}
	return v
}

func (m EditorModel) gutterWidth() int {
	w := len(fmt.Sprintf("%d", len(m.lines))) + 1
	if w < 4 {
		w = 4
	}
	return w
}

func (m EditorModel) contentWidth() int {
	w := m.width - m.gutterWidth() - 2
	if w < 10 {
		w = 10
	}
	return w
}

// Content returns the full editor content as a string.
func (m EditorModel) Content() string {
	return strings.Join(m.lines, "\n")
}

func (m *EditorModel) ensureCursorVisible() {
	vis := m.visibleRows()
	if m.cursorRow < m.scrollRow {
		m.scrollRow = m.cursorRow
	}
	if m.cursorRow >= m.scrollRow+vis {
		m.scrollRow = m.cursorRow - vis + 1
	}
	cw := m.contentWidth()
	if m.cursorCol < m.scrollCol {
		m.scrollCol = m.cursorCol
	}
	if m.cursorCol >= m.scrollCol+cw {
		m.scrollCol = m.cursorCol - cw + 1
	}
}

// clampCol ensures cursor column is within the current line. In normal mode,
// the cursor sits on characters (not past end), whereas insert mode allows
// the position after the last character.
func (m *EditorModel) clampCol() {
	lineLen := len(m.lines[m.cursorRow])
	if m.mode == modeNormal || m.mode == modeVisual {
		if lineLen > 0 && m.cursorCol >= lineLen {
			m.cursorCol = lineLen - 1
		}
	}
	if m.cursorCol < 0 {
		m.cursorCol = 0
	}
	if m.cursorCol > lineLen {
		m.cursorCol = lineLen
	}
}

// --- Undo / Redo --------------------------------------------------------

func copyLines(lines []string) []string {
	c := make([]string, len(lines))
	copy(c, lines)
	return c
}

func (m *EditorModel) pushUndo() {
	m.undoStack = append(m.undoStack, undoEntry{
		lines:     copyLines(m.lines),
		cursorRow: m.cursorRow,
		cursorCol: m.cursorCol,
	})
	if len(m.undoStack) > maxUndoHistory {
		m.undoStack = m.undoStack[len(m.undoStack)-maxUndoHistory:]
	}
	m.redoStack = nil // new change clears redo
}

func (m *EditorModel) undo() {
	if len(m.undoStack) <= 1 {
		m.statusMsg = "Already at oldest change"
		return
	}
	// Push current state into redo.
	m.redoStack = append(m.redoStack, undoEntry{
		lines:     copyLines(m.lines),
		cursorRow: m.cursorRow,
		cursorCol: m.cursorCol,
	})
	// Pop from undo.
	m.undoStack = m.undoStack[:len(m.undoStack)-1]
	top := m.undoStack[len(m.undoStack)-1]
	m.lines = copyLines(top.lines)
	m.cursorRow = top.cursorRow
	m.cursorCol = top.cursorCol
	m.dirty = true
	m.statusMsg = fmt.Sprintf("Undo (%d left)", len(m.undoStack)-1)
}

func (m *EditorModel) redo() {
	if len(m.redoStack) == 0 {
		m.statusMsg = "Already at newest change"
		return
	}
	top := m.redoStack[len(m.redoStack)-1]
	m.redoStack = m.redoStack[:len(m.redoStack)-1]
	m.undoStack = append(m.undoStack, undoEntry{
		lines:     copyLines(m.lines),
		cursorRow: m.cursorRow,
		cursorCol: m.cursorCol,
	})
	m.lines = copyLines(top.lines)
	m.cursorRow = top.cursorRow
	m.cursorCol = top.cursorCol
	m.dirty = true
	m.statusMsg = fmt.Sprintf("Redo (%d left)", len(m.redoStack))
}

// --- Vim motions --------------------------------------------------------

// wordForward moves cursor to the start of the next word.
func (m *EditorModel) wordForward() {
	line := m.lines[m.cursorRow]
	col := m.cursorCol

	// Skip current word characters.
	for col < len(line) && !unicode.IsSpace(rune(line[col])) {
		col++
	}
	// Skip whitespace.
	for col < len(line) && unicode.IsSpace(rune(line[col])) {
		col++
	}
	if col >= len(line) && m.cursorRow < len(m.lines)-1 {
		m.cursorRow++
		m.cursorCol = 0
		// Skip leading whitespace on next line.
		newLine := m.lines[m.cursorRow]
		for m.cursorCol < len(newLine) && unicode.IsSpace(rune(newLine[m.cursorCol])) {
			m.cursorCol++
		}
	} else {
		m.cursorCol = col
	}
}

// wordBackward moves cursor to the start of the previous word.
func (m *EditorModel) wordBackward() {
	col := m.cursorCol
	if col == 0 && m.cursorRow > 0 {
		m.cursorRow--
		m.cursorCol = len(m.lines[m.cursorRow])
		col = m.cursorCol
	}
	line := m.lines[m.cursorRow]
	if col > 0 {
		col--
	}
	// Skip whitespace backwards.
	for col > 0 && unicode.IsSpace(rune(line[col])) {
		col--
	}
	// Skip word characters backwards.
	for col > 0 && !unicode.IsSpace(rune(line[col-1])) {
		col--
	}
	m.cursorCol = col
}

// wordEnd moves cursor to the end of the current/next word.
func (m *EditorModel) wordEnd() {
	line := m.lines[m.cursorRow]
	col := m.cursorCol
	if col < len(line)-1 {
		col++
	}
	// Skip whitespace.
	for col < len(line) && unicode.IsSpace(rune(line[col])) {
		col++
	}
	// Skip to end of word.
	for col < len(line)-1 && !unicode.IsSpace(rune(line[col+1])) {
		col++
	}
	if col >= len(line) && m.cursorRow < len(m.lines)-1 {
		m.cursorRow++
		line = m.lines[m.cursorRow]
		col = 0
		for col < len(line) && unicode.IsSpace(rune(line[col])) {
			col++
		}
		for col < len(line)-1 && !unicode.IsSpace(rune(line[col+1])) {
			col++
		}
	}
	m.cursorCol = col
}

// firstNonBlank moves the cursor to the first non-whitespace character on the line.
func (m *EditorModel) firstNonBlank() {
	line := m.lines[m.cursorRow]
	m.cursorCol = 0
	for m.cursorCol < len(line) && unicode.IsSpace(rune(line[m.cursorCol])) {
		m.cursorCol++
	}
}

// --- Line operations ----------------------------------------------------

func (m *EditorModel) deleteLine(row int) string {
	if row < 0 || row >= len(m.lines) {
		return ""
	}
	deleted := m.lines[row]
	if len(m.lines) == 1 {
		m.lines[0] = ""
	} else {
		m.lines = append(m.lines[:row], m.lines[row+1:]...)
	}
	if m.cursorRow >= len(m.lines) {
		m.cursorRow = len(m.lines) - 1
	}
	return deleted
}

func (m *EditorModel) yankLine(row int) string {
	if row < 0 || row >= len(m.lines) {
		return ""
	}
	return m.lines[row]
}

func (m *EditorModel) insertLineBelow(row int, text string) {
	newLines := make([]string, 0, len(m.lines)+1)
	newLines = append(newLines, m.lines[:row+1]...)
	newLines = append(newLines, text)
	if row+1 < len(m.lines) {
		newLines = append(newLines, m.lines[row+1:]...)
	}
	m.lines = newLines
}

func (m *EditorModel) insertLineAbove(row int, text string) {
	newLines := make([]string, 0, len(m.lines)+1)
	newLines = append(newLines, m.lines[:row]...)
	newLines = append(newLines, text)
	newLines = append(newLines, m.lines[row:]...)
	m.lines = newLines
}

// --- Search -------------------------------------------------------------

func (m *EditorModel) executeSearch(term string, forward bool) {
	m.lastSearch = term
	m.searchFwd = forward
	m.findMatches = nil
	m.findMatchIdx = -1
	if term == "" {
		return
	}
	lower := strings.ToLower(term)
	for i, line := range m.lines {
		lineL := strings.ToLower(line)
		idx := 0
		for {
			pos := strings.Index(lineL[idx:], lower)
			if pos < 0 {
				break
			}
			m.findMatches = append(m.findMatches, [2]int{i, idx + pos})
			idx += pos + 1
		}
	}
	if len(m.findMatches) == 0 {
		m.statusMsg = fmt.Sprintf("Pattern not found: %s", term)
		return
	}
	m.findMatchIdx = 0
	m.jumpToNextMatch(forward)
}

func (m *EditorModel) jumpToNextMatch(forward bool) {
	if len(m.findMatches) == 0 {
		m.statusMsg = "No matches"
		return
	}
	best := -1
	if forward {
		for i, match := range m.findMatches {
			if match[0] > m.cursorRow || (match[0] == m.cursorRow && match[1] > m.cursorCol) {
				best = i
				break
			}
		}
		if best < 0 {
			best = 0 // wrap around
		}
	} else {
		for i := len(m.findMatches) - 1; i >= 0; i-- {
			match := m.findMatches[i]
			if match[0] < m.cursorRow || (match[0] == m.cursorRow && match[1] < m.cursorCol) {
				best = i
				break
			}
		}
		if best < 0 {
			best = len(m.findMatches) - 1 // wrap around
		}
	}
	m.findMatchIdx = best
	m.cursorRow = m.findMatches[best][0]
	m.cursorCol = m.findMatches[best][1]
	m.statusMsg = fmt.Sprintf("[%d/%d] %s", best+1, len(m.findMatches), m.lastSearch)
}

// --- Visual mode helpers ------------------------------------------------

// visualRange returns the ordered start/end positions of the visual selection.
func (m EditorModel) visualRange() (startRow, startCol, endRow, endCol int) {
	startRow, startCol = m.visualRow, m.visualCol
	endRow, endCol = m.cursorRow, m.cursorCol
	if startRow > endRow || (startRow == endRow && startCol > endCol) {
		startRow, startCol, endRow, endCol = endRow, endCol, startRow, startCol
	}
	return
}

// visualText extracts the visually selected text.
func (m EditorModel) visualText() string {
	sr, sc, er, ec := m.visualRange()
	if sr == er {
		line := m.lines[sr]
		end := ec + 1
		if end > len(line) {
			end = len(line)
		}
		if sc > len(line) {
			sc = len(line)
		}
		return line[sc:end]
	}
	var sb strings.Builder
	// First line
	if sc < len(m.lines[sr]) {
		sb.WriteString(m.lines[sr][sc:])
	}
	sb.WriteByte('\n')
	// Middle lines
	for r := sr + 1; r < er; r++ {
		sb.WriteString(m.lines[r])
		sb.WriteByte('\n')
	}
	// Last line
	end := ec + 1
	if end > len(m.lines[er]) {
		end = len(m.lines[er])
	}
	sb.WriteString(m.lines[er][:end])
	return sb.String()
}

// deleteVisualSelection deletes the selected text and returns it.
func (m *EditorModel) deleteVisualSelection() string {
	sr, sc, er, ec := m.visualRange()
	text := m.visualText()

	if sr == er {
		line := m.lines[sr]
		end := ec + 1
		if end > len(line) {
			end = len(line)
		}
		if sc > len(line) {
			sc = len(line)
		}
		m.lines[sr] = line[:sc] + line[end:]
	} else {
		firstPart := ""
		if sc <= len(m.lines[sr]) {
			firstPart = m.lines[sr][:sc]
		}
		lastPart := ""
		end := ec + 1
		if end <= len(m.lines[er]) {
			lastPart = m.lines[er][end:]
		}
		m.lines[sr] = firstPart + lastPart
		m.lines = append(m.lines[:sr+1], m.lines[er+1:]...)
	}
	m.cursorRow = sr
	m.cursorCol = sc
	return text
}

// --- Save helper --------------------------------------------------------

func (m *EditorModel) doSave() tea.Cmd {
	if m.saving {
		return nil
	}
	m.saving = true
	m.statusMsg = "Saving..."
	content := m.Content()
	path := m.path
	isRemote := m.isRemote
	return func() tea.Msg {
		return EditorSaveMsg{Path: path, Content: content, IsRemote: isRemote}
	}
}

// --- Update dispatch per mode -------------------------------------------

// Update handles messages for the editor.
func (m EditorModel) Update(msg tea.Msg) (EditorModel, tea.Cmd) {
	switch msg := msg.(type) {
	case EditorSaveDoneMsg:
		m.saving = false
		if msg.Err != nil {
			m.statusMsg = "Save failed: " + msg.Err.Error()
			m.closeAfterSave = false
		} else {
			m.dirty = false
			if m.closeAfterSave {
				return m, func() tea.Msg { return EditorCloseMsg{} }
			}
			m.statusMsg = "Saved!"
		}
		return m, nil

	case tea.KeyMsg:
		var cmd tea.Cmd
		switch m.mode {
		case modeNormal:
			cmd = m.updateNormal(msg)
		case modeInsert:
			cmd = m.updateInsert(msg)
		case modeVisual:
			cmd = m.updateVisual(msg)
		case modeCommand:
			cmd = m.updateCommand(msg)
		case modeFind:
			cmd = m.updateFind(msg)
		}
		m.clampCol()
		m.ensureCursorVisible()
		return m, cmd
	}
	return m, nil
}

// --- Normal mode --------------------------------------------------------

func (m *EditorModel) updateNormal(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// Handle pending operator (dd, yy, cc, gg).
	if m.pendingOp != 0 {
		op := m.pendingOp
		m.pendingOp = 0
		if len(msg.Runes) == 1 && rune(msg.Runes[0]) == op {
			switch op {
			case 'd': // dd — delete line
				m.pushUndo()
				deleted := m.deleteLine(m.cursorRow)
				m.yankRegister = deleted
				m.yankLinewise = true
				m.dirty = true
				m.statusMsg = "Line deleted"
			case 'y': // yy — yank line
				m.yankRegister = m.yankLine(m.cursorRow)
				m.yankLinewise = true
				m.statusMsg = "Line yanked"
			case 'c': // cc — change line
				m.pushUndo()
				m.yankRegister = m.lines[m.cursorRow]
				m.yankLinewise = true
				m.lines[m.cursorRow] = ""
				m.cursorCol = 0
				m.dirty = true
				m.mode = modeInsert
				m.statusMsg = ""
			case 'g': // gg — go to first line
				m.cursorRow = 0
				m.cursorCol = 0
				m.statusMsg = ""
			}
			return nil
		}
		// Pending op followed by a different key:
		if op == 'r' && len(msg.Runes) == 1 {
			line := m.lines[m.cursorRow]
			if m.cursorCol < len(line) {
				m.pushUndo()
				m.lines[m.cursorRow] = line[:m.cursorCol] + string(msg.Runes[0]) + line[m.cursorCol+1:]
				m.dirty = true
			}
		}
		m.statusMsg = ""
		return nil
	}

	// Regular normal-mode keys.
	switch key {
	// --- Mode transitions ---
	case "i":
		m.mode = modeInsert
		m.statusMsg = ""
		return nil
	case "I":
		m.firstNonBlank()
		m.mode = modeInsert
		m.statusMsg = ""
		return nil
	case "a":
		if len(m.lines[m.cursorRow]) > 0 {
			m.cursorCol++
		}
		m.mode = modeInsert
		m.statusMsg = ""
		return nil
	case "A":
		m.cursorCol = len(m.lines[m.cursorRow])
		m.mode = modeInsert
		m.statusMsg = ""
		return nil
	case "o":
		m.pushUndo()
		m.insertLineBelow(m.cursorRow, "")
		m.cursorRow++
		m.cursorCol = 0
		m.mode = modeInsert
		m.dirty = true
		m.statusMsg = ""
		return nil
	case "O":
		m.pushUndo()
		m.insertLineAbove(m.cursorRow, "")
		m.cursorCol = 0
		m.mode = modeInsert
		m.dirty = true
		m.statusMsg = ""
		return nil
	case "v":
		m.mode = modeVisual
		m.visualRow = m.cursorRow
		m.visualCol = m.cursorCol
		m.statusMsg = ""
		return nil
	case ":":
		m.mode = modeCommand
		m.cmdBuffer = ""
		m.statusMsg = ""
		return nil
	case "/":
		m.mode = modeFind
		m.findBuffer = ""
		m.searchFwd = true
		m.statusMsg = ""
		return nil
	case "?":
		m.mode = modeFind
		m.findBuffer = ""
		m.searchFwd = false
		m.statusMsg = ""
		return nil

	// --- Movement ---
	case "h", "left":
		if m.cursorCol > 0 {
			m.cursorCol--
		}
	case "l", "right":
		lineLen := len(m.lines[m.cursorRow])
		if lineLen > 0 && m.cursorCol < lineLen-1 {
			m.cursorCol++
		}
	case "j", "down":
		if m.cursorRow < len(m.lines)-1 {
			m.cursorRow++
		}
	case "k", "up":
		if m.cursorRow > 0 {
			m.cursorRow--
		}
	case "w":
		m.wordForward()
	case "b":
		m.wordBackward()
	case "e":
		m.wordEnd()
	case "0", "home":
		m.cursorCol = 0
	case "^":
		m.firstNonBlank()
	case "$", "end":
		lineLen := len(m.lines[m.cursorRow])
		if lineLen > 0 {
			m.cursorCol = lineLen - 1
		}

	// G — go to last line
	case "G":
		m.cursorRow = len(m.lines) - 1
	case "g":
		m.pendingOp = 'g'
		m.statusMsg = "g..."
		return nil

	// Page movement
	case "ctrl+f", "pgdown":
		vis := m.visibleRows()
		m.cursorRow += vis
		if m.cursorRow >= len(m.lines) {
			m.cursorRow = len(m.lines) - 1
		}
	case "ctrl+b", "pgup":
		vis := m.visibleRows()
		m.cursorRow -= vis
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
	case "ctrl+d":
		vis := m.visibleRows() / 2
		m.cursorRow += vis
		if m.cursorRow >= len(m.lines) {
			m.cursorRow = len(m.lines) - 1
		}
	case "ctrl+u":
		vis := m.visibleRows() / 2
		m.cursorRow -= vis
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}

	// --- Editing from normal mode ---
	case "x":
		line := m.lines[m.cursorRow]
		if len(line) > 0 && m.cursorCol < len(line) {
			m.pushUndo()
			m.yankRegister = string(line[m.cursorCol])
			m.yankLinewise = false
			m.lines[m.cursorRow] = line[:m.cursorCol] + line[m.cursorCol+1:]
			m.dirty = true
		}
	case "X":
		if m.cursorCol > 0 {
			m.pushUndo()
			line := m.lines[m.cursorRow]
			m.yankRegister = string(line[m.cursorCol-1])
			m.yankLinewise = false
			m.lines[m.cursorRow] = line[:m.cursorCol-1] + line[m.cursorCol:]
			m.cursorCol--
			m.dirty = true
		}
	case "r":
		m.pendingOp = 'r'
		m.statusMsg = "r..."
		return nil
	case "J":
		if m.cursorRow < len(m.lines)-1 {
			m.pushUndo()
			next := strings.TrimLeft(m.lines[m.cursorRow+1], " \t")
			m.lines[m.cursorRow] += " " + next
			m.lines = append(m.lines[:m.cursorRow+1], m.lines[m.cursorRow+2:]...)
			m.dirty = true
			m.statusMsg = "Lines joined"
		}

	// --- Operators (dd, yy, cc) ---
	case "d":
		m.pendingOp = 'd'
		m.statusMsg = "d..."
		return nil
	case "y":
		m.pendingOp = 'y'
		m.statusMsg = "y..."
		return nil
	case "c":
		m.pendingOp = 'c'
		m.statusMsg = "c..."
		return nil

	// --- Paste ---
	case "p":
		if m.yankRegister != "" {
			m.pushUndo()
			if m.yankLinewise {
				m.insertLineBelow(m.cursorRow, m.yankRegister)
				m.cursorRow++
				m.cursorCol = 0
			} else {
				line := m.lines[m.cursorRow]
				insertAt := m.cursorCol + 1
				if insertAt > len(line) {
					insertAt = len(line)
				}
				m.lines[m.cursorRow] = line[:insertAt] + m.yankRegister + line[insertAt:]
				m.cursorCol = insertAt + len(m.yankRegister) - 1
			}
			m.dirty = true
			m.statusMsg = "Pasted"
		}
	case "P":
		if m.yankRegister != "" {
			m.pushUndo()
			if m.yankLinewise {
				m.insertLineAbove(m.cursorRow, m.yankRegister)
				m.cursorCol = 0
			} else {
				line := m.lines[m.cursorRow]
				m.lines[m.cursorRow] = line[:m.cursorCol] + m.yankRegister + line[m.cursorCol:]
			}
			m.dirty = true
			m.statusMsg = "Pasted"
		}

	// --- Undo / Redo ---
	case "u":
		m.undo()
	case "ctrl+r":
		m.redo()

	// --- Search repeat ---
	case "n":
		if m.lastSearch != "" {
			m.jumpToNextMatch(m.searchFwd)
		} else {
			m.statusMsg = "No previous search"
		}
	case "N":
		if m.lastSearch != "" {
			m.jumpToNextMatch(!m.searchFwd)
		} else {
			m.statusMsg = "No previous search"
		}

	// --- Ctrl+S save from normal mode ---
	case "ctrl+s":
		return m.doSave()

	case "esc":
		m.findMatches = nil
		m.statusMsg = ""
		return nil
	}

	return nil
}

// --- Insert mode --------------------------------------------------------

func (m *EditorModel) updateInsert(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		if m.cursorCol > 0 {
			m.cursorCol--
		}
		m.statusMsg = ""
		return nil

	case "ctrl+s":
		return m.doSave()

	case "up":
		if m.cursorRow > 0 {
			m.cursorRow--
			if m.cursorCol > len(m.lines[m.cursorRow]) {
				m.cursorCol = len(m.lines[m.cursorRow])
			}
		}
	case "down":
		if m.cursorRow < len(m.lines)-1 {
			m.cursorRow++
			if m.cursorCol > len(m.lines[m.cursorRow]) {
				m.cursorCol = len(m.lines[m.cursorRow])
			}
		}
	case "left":
		if m.cursorCol > 0 {
			m.cursorCol--
		} else if m.cursorRow > 0 {
			m.cursorRow--
			m.cursorCol = len(m.lines[m.cursorRow])
		}
	case "right":
		if m.cursorCol < len(m.lines[m.cursorRow]) {
			m.cursorCol++
		} else if m.cursorRow < len(m.lines)-1 {
			m.cursorRow++
			m.cursorCol = 0
		}
	case "home", "ctrl+a":
		m.cursorCol = 0
	case "end", "ctrl+e":
		m.cursorCol = len(m.lines[m.cursorRow])

	case "pgup":
		vis := m.visibleRows()
		m.cursorRow -= vis
		if m.cursorRow < 0 {
			m.cursorRow = 0
		}
		if m.cursorCol > len(m.lines[m.cursorRow]) {
			m.cursorCol = len(m.lines[m.cursorRow])
		}
	case "pgdown":
		vis := m.visibleRows()
		m.cursorRow += vis
		if m.cursorRow >= len(m.lines) {
			m.cursorRow = len(m.lines) - 1
		}
		if m.cursorCol > len(m.lines[m.cursorRow]) {
			m.cursorCol = len(m.lines[m.cursorRow])
		}

	case "enter":
		m.pushUndo()
		line := m.lines[m.cursorRow]
		before := line[:m.cursorCol]
		after := line[m.cursorCol:]
		m.lines[m.cursorRow] = before
		newLines := make([]string, 0, len(m.lines)+1)
		newLines = append(newLines, m.lines[:m.cursorRow+1]...)
		newLines = append(newLines, after)
		newLines = append(newLines, m.lines[m.cursorRow+1:]...)
		m.lines = newLines
		m.cursorRow++
		m.cursorCol = 0
		m.dirty = true

	case "backspace":
		if m.cursorCol > 0 {
			m.pushUndo()
			line := m.lines[m.cursorRow]
			m.lines[m.cursorRow] = line[:m.cursorCol-1] + line[m.cursorCol:]
			m.cursorCol--
			m.dirty = true
		} else if m.cursorRow > 0 {
			m.pushUndo()
			prevLen := len(m.lines[m.cursorRow-1])
			m.lines[m.cursorRow-1] += m.lines[m.cursorRow]
			m.lines = append(m.lines[:m.cursorRow], m.lines[m.cursorRow+1:]...)
			m.cursorRow--
			m.cursorCol = prevLen
			m.dirty = true
		}

	case "delete":
		line := m.lines[m.cursorRow]
		if m.cursorCol < len(line) {
			m.pushUndo()
			m.lines[m.cursorRow] = line[:m.cursorCol] + line[m.cursorCol+1:]
			m.dirty = true
		} else if m.cursorRow < len(m.lines)-1 {
			m.pushUndo()
			m.lines[m.cursorRow] += m.lines[m.cursorRow+1]
			m.lines = append(m.lines[:m.cursorRow+1], m.lines[m.cursorRow+2:]...)
			m.dirty = true
		}

	case "tab":
		m.pushUndo()
		line := m.lines[m.cursorRow]
		m.lines[m.cursorRow] = line[:m.cursorCol] + "    " + line[m.cursorCol:]
		m.cursorCol += 4
		m.dirty = true

	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			m.pushUndo()
			ch := string(msg.Runes)
			line := m.lines[m.cursorRow]
			m.lines[m.cursorRow] = line[:m.cursorCol] + ch + line[m.cursorCol:]
			m.cursorCol += len(ch)
			m.dirty = true
		}
	}
	return nil
}

// --- Visual mode --------------------------------------------------------

func (m *EditorModel) updateVisual(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.statusMsg = ""
		return nil
	case "v":
		m.mode = modeNormal
		m.statusMsg = ""
		return nil

	// Movement (extends selection)
	case "h", "left":
		if m.cursorCol > 0 {
			m.cursorCol--
		}
	case "l", "right":
		if m.cursorCol < len(m.lines[m.cursorRow]) {
			m.cursorCol++
		}
	case "j", "down":
		if m.cursorRow < len(m.lines)-1 {
			m.cursorRow++
		}
	case "k", "up":
		if m.cursorRow > 0 {
			m.cursorRow--
		}
	case "w":
		m.wordForward()
	case "b":
		m.wordBackward()
	case "e":
		m.wordEnd()
	case "0", "home":
		m.cursorCol = 0
	case "$", "end":
		m.cursorCol = len(m.lines[m.cursorRow])
	case "G":
		m.cursorRow = len(m.lines) - 1
		m.cursorCol = len(m.lines[m.cursorRow])

	// Operations on selection
	case "d", "x":
		m.pushUndo()
		m.yankRegister = m.deleteVisualSelection()
		m.yankLinewise = false
		m.dirty = true
		m.mode = modeNormal
		m.statusMsg = "Deleted"
	case "y":
		m.yankRegister = m.visualText()
		m.yankLinewise = false
		m.mode = modeNormal
		m.cursorRow = m.visualRow
		m.cursorCol = m.visualCol
		m.statusMsg = "Yanked"
	case "c":
		m.pushUndo()
		m.yankRegister = m.deleteVisualSelection()
		m.yankLinewise = false
		m.dirty = true
		m.mode = modeInsert
		m.statusMsg = ""
	}
	return nil
}

// --- Command mode (:) ---------------------------------------------------

func (m *EditorModel) updateCommand(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.cmdBuffer = ""
		m.statusMsg = ""
		return nil

	case "enter":
		cmd := m.cmdBuffer
		m.mode = modeNormal
		m.cmdBuffer = ""
		return m.executeCommand(cmd)

	case "backspace":
		if len(m.cmdBuffer) > 0 {
			m.cmdBuffer = m.cmdBuffer[:len(m.cmdBuffer)-1]
		} else {
			m.mode = modeNormal
			m.statusMsg = ""
		}

	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			m.cmdBuffer += string(msg.Runes)
		}
	}
	return nil
}

func (m *EditorModel) executeCommand(cmd string) tea.Cmd {
	cmd = strings.TrimSpace(cmd)
	switch cmd {
	case "w":
		return m.doSave()
	case "q":
		if m.dirty {
			m.statusMsg = "Unsaved changes! Use :q! to force or :wq to save & quit"
			return nil
		}
		return func() tea.Msg { return EditorCloseMsg{} }
	case "q!":
		return func() tea.Msg { return EditorCloseMsg{} }
	case "wq", "x":
		m.closeAfterSave = true
		return m.doSave()
	default:
		// Try to parse :<number> as go-to-line.
		var lineNum int
		if _, err := fmt.Sscanf(cmd, "%d", &lineNum); err == nil && lineNum > 0 {
			lineNum-- // 0-indexed
			if lineNum >= len(m.lines) {
				lineNum = len(m.lines) - 1
			}
			m.cursorRow = lineNum
			m.cursorCol = 0
			m.statusMsg = fmt.Sprintf("Line %d", lineNum+1)
			return nil
		}
		m.statusMsg = fmt.Sprintf("Unknown command: %s", cmd)
	}
	return nil
}

// --- Find mode (/) ------------------------------------------------------

func (m *EditorModel) updateFind(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.findBuffer = ""
		m.findMatches = nil
		m.statusMsg = ""
		return nil

	case "enter":
		term := m.findBuffer
		m.mode = modeNormal
		m.executeSearch(term, m.searchFwd)
		return nil

	case "backspace":
		if len(m.findBuffer) > 0 {
			m.findBuffer = m.findBuffer[:len(m.findBuffer)-1]
		} else {
			m.mode = modeNormal
			m.statusMsg = ""
		}

	default:
		if len(msg.Runes) > 0 && !msg.Alt {
			m.findBuffer += string(msg.Runes)
		}
	}
	return nil
}

// --- View ---------------------------------------------------------------

func (m EditorModel) View() string {
	if m.width == 0 {
		return "Loading editor..."
	}

	// Header
	loc := "Local"
	if m.isRemote {
		loc = "Remote"
	}
	modified := ""
	if m.dirty {
		modified = " [modified]"
	}
	header := editorHeaderStyle.Width(m.width).Render(
		fmt.Sprintf(" %s: %s%s", loc, m.path, modified),
	)

	vis := m.visibleRows()
	gw := m.gutterWidth()
	cw := m.contentWidth()

	// Precompute search highlight set for visible rows.
	type hlRange struct{ col, length int }
	matchesByRow := map[int][]hlRange{}
	if len(m.findMatches) > 0 && m.lastSearch != "" {
		termLen := len(m.lastSearch)
		for _, match := range m.findMatches {
			r := match[0]
			if r >= m.scrollRow && r < m.scrollRow+vis {
				matchesByRow[r] = append(matchesByRow[r], hlRange{match[1], termLen})
			}
		}
	}

	// Build visual selection set.
	inVisualSelection := func(row, col int) bool {
		if m.mode != modeVisual {
			return false
		}
		sr, sc, er, ec := m.visualRange()
		if row < sr || row > er {
			return false
		}
		if row == sr && row == er {
			return col >= sc && col <= ec
		}
		if row == sr {
			return col >= sc
		}
		if row == er {
			return col <= ec
		}
		return true
	}

	var rows []string
	for i := m.scrollRow; i < len(m.lines) && i < m.scrollRow+vis; i++ {
		gutter := editorGutterStyle.Render(fmt.Sprintf("%*d ", gw-1, i+1))
		line := m.lines[i]

		var displayLine string
		dispStart := m.scrollCol
		if dispStart > len(line) {
			dispStart = len(line)
		}
		end := m.scrollCol + cw
		if end > len(line) {
			end = len(line)
		}
		displayLine = line[dispStart:end]

		// Render the line character by character with highlights.
		var sb strings.Builder
		sb.WriteString(gutter)
		for j := 0; j < len(displayLine); j++ {
			absCol := j + m.scrollCol
			ch := string(displayLine[j])

			isCursor := i == m.cursorRow && absCol == m.cursorCol
			isVisual := inVisualSelection(i, absCol)
			isSearchHL := false
			for _, hl := range matchesByRow[i] {
				if absCol >= hl.col && absCol < hl.col+hl.length {
					isSearchHL = true
					break
				}
			}

			switch {
			case isCursor:
				sb.WriteString(editorCursorStyle.Render(ch))
			case isVisual:
				sb.WriteString(editorVisualStyle.Render(ch))
			case isSearchHL:
				sb.WriteString(editorSearchHLStyle.Render(ch))
			default:
				sb.WriteString(ch)
			}
		}
		// Show block cursor at end of line if needed.
		if i == m.cursorRow && m.cursorCol-m.scrollCol >= len(displayLine) {
			sb.WriteString(editorCursorStyle.Render(" "))
		}

		rows = append(rows, sb.String())
	}

	for len(rows) < vis {
		gutter := editorGutterStyle.Render(strings.Repeat(" ", gw))
		rows = append(rows, gutter+editorTildeStyle.Render("~"))
	}

	body := strings.Join(rows, "\n")

	// Status bar
	status := m.renderStatusBar()

	return lipgloss.JoinVertical(lipgloss.Left, header, body, status)
}

func (m EditorModel) renderStatusBar() string {
	modeStr := m.mode.String()

	switch m.mode {
	case modeCommand:
		left := fmt.Sprintf(" :%s", m.cmdBuffer)
		right := fmt.Sprintf(" Ln %d, Col %d ", m.cursorRow+1, m.cursorCol+1)
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
		if gap < 0 {
			gap = 1
		}
		return editorCommandBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)

	case modeFind:
		prefix := "/"
		if !m.searchFwd {
			prefix = "?"
		}
		left := fmt.Sprintf(" %s%s", prefix, m.findBuffer)
		right := fmt.Sprintf(" Ln %d, Col %d ", m.cursorRow+1, m.cursorCol+1)
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
		if gap < 0 {
			gap = 1
		}
		return editorCommandBarStyle.Width(m.width).Render(left + strings.Repeat(" ", gap) + right)

	default:
		modeDisplay := editorModeStyle.Render(fmt.Sprintf(" %s ", modeStr))
		posInfo := fmt.Sprintf(" Ln %d, Col %d", m.cursorRow+1, m.cursorCol+1)
		if m.statusMsg != "" {
			posInfo += " │ " + m.statusMsg
		}
		right := fmt.Sprintf(" %d lines ", len(m.lines))
		gap := m.width - lipgloss.Width(modeDisplay) - lipgloss.Width(posInfo) - lipgloss.Width(right)
		if gap < 0 {
			gap = 1
		}
		return editorStatusStyle.Width(m.width).Render(modeDisplay + posInfo + strings.Repeat(" ", gap) + right)
	}
}

// IsEditableFile checks whether a filename has a known text-editable extension
// or is a known text file without an extension (e.g. Makefile, Dockerfile).
func IsEditableFile(name string) bool {
	base := strings.ToLower(filepath.Base(name))

	knownNames := map[string]bool{
		"makefile": true, "dockerfile": true, "readme": true,
		"license": true, "changelog": true, "authors": true,
		".gitignore": true, ".gitattributes": true, ".editorconfig": true,
		".env": true, "vagrantfile": true, "procfile": true,
		".dockerignore": true, ".gitmodules": true,
	}
	if knownNames[base] {
		return true
	}

	ext := strings.ToLower(filepath.Ext(name))
	editableExts := map[string]bool{
		".txt": true, ".md": true, ".markdown": true,
		".json": true, ".yaml": true, ".yml": true, ".toml": true,
		".xml": true, ".csv": true, ".tsv": true,
		".log": true, ".conf": true, ".cfg": true, ".ini": true,
		".sh": true, ".bash": true, ".zsh": true, ".fish": true,
		".py": true, ".go": true, ".rs": true, ".rb": true,
		".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".html": true, ".htm": true, ".css": true, ".scss": true, ".less": true,
		".sql": true, ".lua": true, ".pl": true, ".pm": true,
		".r": true, ".swift": true, ".kt": true, ".kts": true,
		".java": true, ".c": true, ".cpp": true, ".cc": true,
		".h": true, ".hpp": true, ".hh": true,
		".cs": true, ".fs": true, ".fsx": true,
		".php": true, ".ex": true, ".exs": true,
		".erl": true, ".hrl": true, ".hs": true,
		".vim": true, ".el": true, ".lisp": true, ".clj": true,
		".bat": true, ".cmd": true, ".ps1": true,
		".properties": true, ".env": true,
		".tf": true, ".hcl": true,
		".graphql": true, ".gql": true,
		".proto": true, ".gradle": true,
		".cmake": true, ".mk": true,
		".rst": true, ".tex": true, ".bib": true,
		".diff": true, ".patch": true,
		".sum": true, ".mod": true, ".lock": true,
	}
	return editableExts[ext]
}

// Styles for the editor.
var (
	editorHeaderStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#7D56F4")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Bold(true)

	editorGutterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#555555"))

	editorCursorStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#FFFFFF")).
				Foreground(lipgloss.Color("#000000"))

	editorVisualStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#264F78")).
				Foreground(lipgloss.Color("#FFFFFF"))

	editorSearchHLStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#9E6A03")).
				Foreground(lipgloss.Color("#FFFFFF"))

	editorTildeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#444444"))

	editorStatusStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#333333")).
				Foreground(lipgloss.Color("#AAAAAA"))

	editorModeStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	editorCommandBarStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1E1E1E")).
				Foreground(lipgloss.Color("#FFFFFF"))
)
