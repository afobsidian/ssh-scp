package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// NewEditorModel
// ---------------------------------------------------------------------------

func TestNewEditorModelBasic(t *testing.T) {
	m := NewEditorModel("/tmp/test.txt", false, "hello\nworld")
	if m.path != "/tmp/test.txt" {
		t.Errorf("path = %q, want /tmp/test.txt", m.path)
	}
	if m.isRemote {
		t.Error("isRemote should be false")
	}
	if len(m.lines) != 2 {
		t.Errorf("lines = %d, want 2", len(m.lines))
	}
	if m.lines[0] != "hello" || m.lines[1] != "world" {
		t.Errorf("lines = %v", m.lines)
	}
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want modeNormal", m.mode)
	}
}

func TestNewEditorModelRemote(t *testing.T) {
	m := NewEditorModel("/remote/file.txt", true, "content")
	if !m.isRemote {
		t.Error("isRemote should be true")
	}
}

func TestNewEditorModelEmptyContent(t *testing.T) {
	m := NewEditorModel("/tmp/empty.txt", false, "")
	if len(m.lines) != 1 {
		t.Errorf("empty content should have 1 line, got %d", len(m.lines))
	}
	if m.lines[0] != "" {
		t.Errorf("empty content line = %q, want empty", m.lines[0])
	}
}

func TestNewEditorModelCRLFNormalization(t *testing.T) {
	m := NewEditorModel("/tmp/crlf.txt", false, "a\r\nb\rc")
	if len(m.lines) != 3 {
		t.Errorf("lines = %d, want 3", len(m.lines))
	}
	if m.lines[0] != "a" || m.lines[1] != "b" || m.lines[2] != "c" {
		t.Errorf("lines = %v", m.lines)
	}
}

func TestNewEditorModelInitialUndo(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	if len(m.undoStack) != 1 {
		t.Errorf("undoStack should have 1 entry (initial), got %d", len(m.undoStack))
	}
}

// ---------------------------------------------------------------------------
// Content
// ---------------------------------------------------------------------------

func TestEditorContent(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "line1\nline2\nline3")
	got := m.Content()
	if got != "line1\nline2\nline3" {
		t.Errorf("Content = %q", got)
	}
}

// ---------------------------------------------------------------------------
// SetDimensions
// ---------------------------------------------------------------------------

func TestEditorSetDimensions(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(100, 50)
	if m.width != 100 || m.height != 50 {
		t.Errorf("dimensions = %dx%d, want 100x50", m.width, m.height)
	}
}

// ---------------------------------------------------------------------------
// visibleRows, gutterWidth, contentWidth
// ---------------------------------------------------------------------------

func TestEditorVisibleRows(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.height = 20
	if got := m.visibleRows(); got != 17 { // 20 - 3
		t.Errorf("visibleRows = %d, want 17", got)
	}
}

func TestEditorVisibleRowsMinimum(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.height = 2
	if got := m.visibleRows(); got != 1 {
		t.Errorf("visibleRows minimum = %d, want 1", got)
	}
}

func TestEditorGutterWidth(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	gw := m.gutterWidth()
	if gw < 4 {
		t.Errorf("gutterWidth = %d, should be >= 4", gw)
	}
}

func TestEditorGutterWidthManyLines(t *testing.T) {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = "x"
	}
	m := NewEditorModel("/tmp/t.txt", false, strings.Join(lines, "\n"))
	gw := m.gutterWidth()
	if gw < 5 { // 1000 = 4 digits + 1 space = 5
		t.Errorf("gutterWidth for 1000 lines = %d, want >= 5", gw)
	}
}

func TestEditorContentWidth(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.width = 80
	cw := m.contentWidth()
	expected := 80 - m.gutterWidth() - 2
	if cw != expected {
		t.Errorf("contentWidth = %d, want %d", cw, expected)
	}
}

func TestEditorContentWidthMinimum(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.width = 5
	cw := m.contentWidth()
	if cw < 10 {
		t.Errorf("contentWidth min = %d, want >= 10", cw)
	}
}

// ---------------------------------------------------------------------------
// editorMode.String
// ---------------------------------------------------------------------------

func TestEditorModeString(t *testing.T) {
	tests := []struct {
		mode editorMode
		want string
	}{
		{modeNormal, "NORMAL"},
		{modeInsert, "INSERT"},
		{modeVisual, "VISUAL"},
		{modeCommand, "COMMAND"},
		{modeFind, "FIND"},
		{editorMode(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Errorf("mode %d -> %q, want %q", tt.mode, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// clampCol
// ---------------------------------------------------------------------------

func TestClampColNormalMode(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.cursorCol = 10
	m.clampCol()
	if m.cursorCol != 4 { // len("hello")-1
		t.Errorf("cursorCol = %d, want 4", m.cursorCol)
	}
}

func TestClampColInsertMode(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.mode = modeInsert
	m.cursorCol = 10
	m.clampCol()
	if m.cursorCol != 5 { // len("hello") allowed in insert
		t.Errorf("cursorCol = %d, want 5", m.cursorCol)
	}
}

func TestClampColNegative(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.cursorCol = -5
	m.clampCol()
	if m.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0", m.cursorCol)
	}
}

func TestClampColEmptyLine(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "")
	m.cursorCol = 5
	m.clampCol()
	if m.cursorCol != 0 {
		t.Errorf("cursorCol = %d, want 0", m.cursorCol)
	}
}

// ---------------------------------------------------------------------------
// copyLines
// ---------------------------------------------------------------------------

func TestCopyLines(t *testing.T) {
	original := []string{"a", "b", "c"}
	copied := copyLines(original)
	if len(copied) != 3 {
		t.Fatalf("len = %d, want 3", len(copied))
	}
	copied[0] = "x"
	if original[0] != "a" {
		t.Error("modifying copy should not affect original")
	}
}

// ---------------------------------------------------------------------------
// Undo / Redo
// ---------------------------------------------------------------------------

func TestUndoRedo(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "original")
	// Make a change
	m.pushUndo()
	m.lines[0] = "modified"
	m.dirty = true

	m.undo()
	if m.lines[0] != "original" {
		t.Errorf("after undo, line = %q, want original", m.lines[0])
	}

	m.redo()
	if m.lines[0] != "modified" {
		t.Errorf("after redo, line = %q, want modified", m.lines[0])
	}
}

func TestUndoAtOldest(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.undo() // only initial state exists
	if !strings.Contains(m.statusMsg, "oldest") {
		t.Errorf("statusMsg = %q, want oldest change message", m.statusMsg)
	}
}

func TestRedoAtNewest(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.redo()
	if !strings.Contains(m.statusMsg, "newest") {
		t.Errorf("statusMsg = %q, want newest change message", m.statusMsg)
	}
}

func TestPushUndoClearsRedo(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.pushUndo()
	m.lines[0] = "change1"
	m.undo()
	// redoStack should have 1 entry now
	if len(m.redoStack) != 1 {
		t.Fatalf("redoStack = %d, want 1", len(m.redoStack))
	}
	m.pushUndo() // new change clears redo
	if len(m.redoStack) != 0 {
		t.Errorf("redoStack = %d, want 0 after new push", len(m.redoStack))
	}
}

func TestUndoHistoryCapLimit(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	for i := 0; i < maxUndoHistory+50; i++ {
		m.pushUndo()
	}
	if len(m.undoStack) > maxUndoHistory {
		t.Errorf("undoStack = %d, should be capped at %d", len(m.undoStack), maxUndoHistory)
	}
}

// ---------------------------------------------------------------------------
// Word motions
// ---------------------------------------------------------------------------

func TestWordForward(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world foo")
	m.cursorCol = 0
	m.wordForward()
	if m.cursorCol != 6 { // "world"
		t.Errorf("wordForward col = %d, want 6", m.cursorCol)
	}
}

func TestWordForwardAtEnd(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld")
	m.cursorCol = 3
	m.wordForward()
	// Should skip to end of word, then wrap
	if m.cursorRow == 1 && m.cursorCol == 0 {
		// wrapped to next line
	} else if m.cursorRow == 0 {
		// stayed on line, OK too
	} else {
		t.Errorf("wordForward wrap: row=%d col=%d", m.cursorRow, m.cursorCol)
	}
}

func TestWordBackward(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.cursorCol = 8
	m.wordBackward()
	if m.cursorCol != 6 {
		t.Errorf("wordBackward col = %d, want 6", m.cursorCol)
	}
}

func TestWordBackwardAtStart(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "first\nsecond")
	m.cursorRow = 1
	m.cursorCol = 0
	m.wordBackward()
	if m.cursorRow != 0 {
		t.Errorf("wordBackward row = %d, want 0", m.cursorRow)
	}
}

func TestWordEnd(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.cursorCol = 0
	m.wordEnd()
	if m.cursorCol != 4 { // end of "hello"
		t.Errorf("wordEnd col = %d, want 4", m.cursorCol)
	}
}

// ---------------------------------------------------------------------------
// firstNonBlank
// ---------------------------------------------------------------------------

func TestFirstNonBlank(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "   hello")
	m.cursorCol = 0
	m.firstNonBlank()
	if m.cursorCol != 3 {
		t.Errorf("firstNonBlank col = %d, want 3", m.cursorCol)
	}
}

func TestFirstNonBlankNoLeading(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.cursorCol = 3
	m.firstNonBlank()
	if m.cursorCol != 0 {
		t.Errorf("firstNonBlank col = %d, want 0", m.cursorCol)
	}
}

// ---------------------------------------------------------------------------
// Line operations
// ---------------------------------------------------------------------------

func TestDeleteLine(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo\nthree")
	deleted := m.deleteLine(1)
	if deleted != "two" {
		t.Errorf("deleted = %q, want two", deleted)
	}
	if len(m.lines) != 2 {
		t.Errorf("lines = %d, want 2", len(m.lines))
	}
}

func TestDeleteLineSingleLine(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "only")
	deleted := m.deleteLine(0)
	if deleted != "only" {
		t.Errorf("deleted = %q, want only", deleted)
	}
	if m.lines[0] != "" {
		t.Errorf("single line delete should leave empty line, got %q", m.lines[0])
	}
}

func TestDeleteLineOutOfBounds(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	deleted := m.deleteLine(5)
	if deleted != "" {
		t.Errorf("out of bounds delete = %q, want empty", deleted)
	}
}

func TestYankLine(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo\nthree")
	yanked := m.yankLine(1)
	if yanked != "two" {
		t.Errorf("yanked = %q, want two", yanked)
	}
	if len(m.lines) != 3 {
		t.Error("yank should not modify lines")
	}
}

func TestYankLineOutOfBounds(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	yanked := m.yankLine(10)
	if yanked != "" {
		t.Errorf("out of bounds yank = %q, want empty", yanked)
	}
}

func TestInsertLineBelow(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo")
	m.insertLineBelow(0, "inserted")
	if len(m.lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(m.lines))
	}
	if m.lines[1] != "inserted" {
		t.Errorf("lines[1] = %q, want inserted", m.lines[1])
	}
}

func TestInsertLineAbove(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo")
	m.insertLineAbove(1, "inserted")
	if len(m.lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(m.lines))
	}
	if m.lines[1] != "inserted" {
		t.Errorf("lines[1] = %q, want inserted", m.lines[1])
	}
	if m.lines[2] != "two" {
		t.Errorf("lines[2] = %q, want two", m.lines[2])
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestExecuteSearch(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world\nhello again\nfoo")
	m.executeSearch("hello", true)
	if len(m.findMatches) != 2 {
		t.Errorf("findMatches = %d, want 2", len(m.findMatches))
	}
	if m.lastSearch != "hello" {
		t.Errorf("lastSearch = %q, want hello", m.lastSearch)
	}
}

func TestExecuteSearchCaseInsensitive(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "Hello HELLO hello")
	m.executeSearch("hello", true)
	if len(m.findMatches) != 3 {
		t.Errorf("findMatches = %d, want 3", len(m.findMatches))
	}
}

func TestExecuteSearchNotFound(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.executeSearch("xyz", true)
	if len(m.findMatches) != 0 {
		t.Errorf("findMatches = %d, want 0", len(m.findMatches))
	}
	if !strings.Contains(m.statusMsg, "not found") {
		t.Errorf("statusMsg = %q, want 'not found'", m.statusMsg)
	}
}

func TestExecuteSearchEmpty(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.executeSearch("", true)
	if len(m.findMatches) != 0 {
		t.Error("empty search should have no matches")
	}
}

func TestJumpToNextMatch(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "aaa\naaa\naaa")
	m.executeSearch("aaa", true)
	if len(m.findMatches) != 3 {
		t.Fatalf("findMatches = %d, want 3", len(m.findMatches))
	}
	// Jump forward
	m.jumpToNextMatch(true)
	if m.cursorRow == 0 && m.cursorCol == 0 {
		// jumped somewhere
	}
}

func TestJumpToNextMatchBackward(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "aaa\nbbb\naaa")
	m.cursorRow = 2
	m.cursorCol = 0
	m.executeSearch("aaa", true)
	// After executeSearch, cursor may have moved forward from row 2.
	// Force cursor to row 2, col 0 so backward skips this exact position
	// and wraps to the match at row 0.
	m.cursorRow = 2
	m.cursorCol = 0
	m.jumpToNextMatch(false)
	if m.cursorRow != 0 {
		t.Errorf("backward jump row = %d, want 0", m.cursorRow)
	}
}

func TestJumpToNextMatchNoMatches(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.jumpToNextMatch(true)
	if !strings.Contains(m.statusMsg, "No matches") {
		t.Errorf("statusMsg = %q, want 'No matches'", m.statusMsg)
	}
}

// ---------------------------------------------------------------------------
// Visual mode helpers
// ---------------------------------------------------------------------------

func TestVisualRange(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.visualRow = 0
	m.visualCol = 2
	m.cursorRow = 0
	m.cursorCol = 7
	sr, sc, er, ec := m.visualRange()
	if sr != 0 || sc != 2 || er != 0 || ec != 7 {
		t.Errorf("visualRange = (%d,%d)-(%d,%d), want (0,2)-(0,7)", sr, sc, er, ec)
	}
}

func TestVisualRangeReversed(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.visualRow = 0
	m.visualCol = 7
	m.cursorRow = 0
	m.cursorCol = 2
	sr, sc, er, ec := m.visualRange()
	if sr != 0 || sc != 2 || er != 0 || ec != 7 {
		t.Errorf("reversed visualRange = (%d,%d)-(%d,%d), want (0,2)-(0,7)", sr, sc, er, ec)
	}
}

func TestVisualText(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.visualRow = 0
	m.visualCol = 0
	m.cursorRow = 0
	m.cursorCol = 4
	got := m.visualText()
	if got != "hello" {
		t.Errorf("visualText = %q, want hello", got)
	}
}

func TestVisualTextMultiLine(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld\nfoo")
	m.visualRow = 0
	m.visualCol = 3
	m.cursorRow = 2
	m.cursorCol = 1
	got := m.visualText()
	// Should include "lo\nworld\nfo"
	if !strings.Contains(got, "world") {
		t.Errorf("multi-line visual text = %q, should contain 'world'", got)
	}
}

func TestDeleteVisualSelection(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.visualRow = 0
	m.visualCol = 0
	m.cursorRow = 0
	m.cursorCol = 4
	deleted := m.deleteVisualSelection()
	if deleted != "hello" {
		t.Errorf("deleted text = %q, want hello", deleted)
	}
	if m.lines[0] != " world" {
		t.Errorf("remaining line = %q, want ' world'", m.lines[0])
	}
}

func TestDeleteVisualSelectionMultiLine(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld\nfoo")
	m.visualRow = 0
	m.visualCol = 3
	m.cursorRow = 1
	m.cursorCol = 2
	m.deleteVisualSelection()
	if len(m.lines) != 2 {
		t.Errorf("lines = %d, want 2", len(m.lines))
	}
}

// ---------------------------------------------------------------------------
// doSave
// ---------------------------------------------------------------------------

func TestDoSave(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "content")
	cmd := m.doSave()
	if cmd == nil {
		t.Error("doSave should return a command")
	}
	if !m.saving {
		t.Error("saving should be true")
	}
	if m.statusMsg != "Saving..." {
		t.Errorf("statusMsg = %q, want 'Saving...'", m.statusMsg)
	}
}

func TestDoSaveWhileSaving(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "content")
	m.saving = true
	cmd := m.doSave()
	if cmd != nil {
		t.Error("doSave should return nil when already saving")
	}
}

func TestDoSaveProducesEditorSaveMsg(t *testing.T) {
	m := NewEditorModel("/tmp/test.txt", false, "data")
	cmd := m.doSave()
	if cmd == nil {
		t.Fatal("expected command")
	}
	msg := cmd()
	saveMsg, ok := msg.(EditorSaveMsg)
	if !ok {
		t.Fatalf("expected EditorSaveMsg, got %T", msg)
	}
	if saveMsg.Path != "/tmp/test.txt" {
		t.Errorf("path = %q", saveMsg.Path)
	}
	if saveMsg.Content != "data" {
		t.Errorf("content = %q", saveMsg.Content)
	}
	if saveMsg.IsRemote {
		t.Error("isRemote should be false")
	}
}

// ---------------------------------------------------------------------------
// Update - EditorSaveDoneMsg
// ---------------------------------------------------------------------------

func TestUpdateSaveDoneSuccess(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.saving = true
	m.dirty = true
	m, _ = m.Update(EditorSaveDoneMsg{Err: nil})
	if m.saving {
		t.Error("saving should be false after success")
	}
	if m.dirty {
		t.Error("dirty should be false after successful save")
	}
	if m.statusMsg != "Saved!" {
		t.Errorf("statusMsg = %q, want Saved!", m.statusMsg)
	}
}

func TestUpdateSaveDoneError(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.saving = true
	m.dirty = true
	m, _ = m.Update(EditorSaveDoneMsg{Err: errTest("disk full")})
	if m.saving {
		t.Error("saving should be false after error")
	}
	if !strings.Contains(m.statusMsg, "Save failed") {
		t.Errorf("statusMsg = %q, want Save failed", m.statusMsg)
	}
}

func TestUpdateSaveDoneWithError(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.saving = true
	m.dirty = true
	m, _ = m.Update(EditorSaveDoneMsg{Err: errTest("save failed")})
	if m.saving {
		t.Error("saving should be false")
	}
	if !strings.Contains(m.statusMsg, "Save failed") {
		t.Errorf("statusMsg = %q, want Save failed", m.statusMsg)
	}
}

func TestUpdateSaveDoneWQ(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.saving = true
	m.dirty = true
	m.closeAfterSave = true
	m, cmd := m.Update(EditorSaveDoneMsg{Err: nil})
	if cmd == nil {
		t.Error("wq save done should return close command")
	}
	// Execute the command and check it produces EditorCloseMsg
	msg := cmd()
	if _, ok := msg.(EditorCloseMsg); !ok {
		t.Errorf("expected EditorCloseMsg, got %T", msg)
	}
}

// errTest is a simple error type for testing.
type errTest string

func (e errTest) Error() string { return string(e) }

// ---------------------------------------------------------------------------
// Normal mode - basic movement
// ---------------------------------------------------------------------------

func TestNormalMoveHJKL(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld\nfoo")
	m.SetDimensions(80, 40)

	// j - down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursorRow != 1 {
		t.Errorf("j: row = %d, want 1", m.cursorRow)
	}

	// l - right
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.cursorCol != 1 {
		t.Errorf("l: col = %d, want 1", m.cursorCol)
	}

	// k - up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if m.cursorRow != 0 {
		t.Errorf("k: row = %d, want 0", m.cursorRow)
	}

	// h - left
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if m.cursorCol != 0 {
		t.Errorf("h: col = %d, want 0", m.cursorCol)
	}
}

func TestNormalMoveArrows(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld")
	m.SetDimensions(80, 40)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursorRow != 1 {
		t.Errorf("down: row = %d, want 1", m.cursorRow)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if m.cursorCol != 1 {
		t.Errorf("right: col = %d, want 1", m.cursorCol)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursorRow != 0 {
		t.Errorf("up: row = %d, want 0", m.cursorRow)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.cursorCol != 0 {
		t.Errorf("left: col = %d, want 0", m.cursorCol)
	}
}

func TestNormalMoveZeroAndDollar(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)
	m.cursorCol = 5

	// $ - end of line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("$")})
	if m.cursorCol != 10 { // len("hello world")-1
		t.Errorf("$ col = %d, want 10", m.cursorCol)
	}

	// 0 - start of line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0")})
	if m.cursorCol != 0 {
		t.Errorf("0 col = %d, want 0", m.cursorCol)
	}
}

func TestNormalMoveCaret(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "   hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("^")})
	if m.cursorCol != 3 {
		t.Errorf("^ col = %d, want 3", m.cursorCol)
	}
}

func TestNormalMoveG(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo\nthree")
	m.SetDimensions(80, 40)

	// G - last line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.cursorRow != 2 {
		t.Errorf("G: row = %d, want 2", m.cursorRow)
	}

	// gg - first line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if m.cursorRow != 0 {
		t.Errorf("gg: row = %d, want 0", m.cursorRow)
	}
}

func TestNormalMoveW(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	if m.cursorCol != 6 {
		t.Errorf("w: col = %d, want 6", m.cursorCol)
	}
}

func TestNormalMoveB(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)
	m.cursorCol = 8

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	if m.cursorCol != 6 {
		t.Errorf("b: col = %d, want 6", m.cursorCol)
	}
}

func TestNormalMoveE(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if m.cursorCol != 4 {
		t.Errorf("e: col = %d, want 4", m.cursorCol)
	}
}

// ---------------------------------------------------------------------------
// Normal mode - mode transitions
// ---------------------------------------------------------------------------

func TestNormalToInsertI(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	if m.mode != modeInsert {
		t.Errorf("mode = %v, want INSERT", m.mode)
	}
}

func TestNormalToInsertA(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.cursorCol = 2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m.mode != modeInsert {
		t.Errorf("mode = %v, want INSERT", m.mode)
	}
	if m.cursorCol != 3 {
		t.Errorf("a: col = %d, want 3", m.cursorCol)
	}
}

func TestNormalToInsertBigA(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("A")})
	if m.mode != modeInsert {
		t.Errorf("mode = %v, want INSERT", m.mode)
	}
	if m.cursorCol != 5 { // end of "hello"
		t.Errorf("A: col = %d, want 5", m.cursorCol)
	}
}

func TestNormalToInsertBigI(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "   hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("I")})
	if m.mode != modeInsert {
		t.Errorf("mode = %v, want INSERT", m.mode)
	}
	if m.cursorCol != 3 {
		t.Errorf("I: col = %d, want 3", m.cursorCol)
	}
}

func TestNormalToInsertO(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	if m.mode != modeInsert {
		t.Errorf("mode = %v, want INSERT", m.mode)
	}
	if m.cursorRow != 1 {
		t.Errorf("o: row = %d, want 1", m.cursorRow)
	}
	if len(m.lines) != 2 {
		t.Errorf("o: lines = %d, want 2", len(m.lines))
	}
	if !m.dirty {
		t.Error("o: should set dirty")
	}
}

func TestNormalToInsertBigO(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("O")})
	if m.mode != modeInsert {
		t.Errorf("mode = %v, want INSERT", m.mode)
	}
	if m.cursorRow != 0 {
		t.Errorf("O: row = %d, want 0", m.cursorRow)
	}
	if len(m.lines) != 2 {
		t.Errorf("O: lines = %d, want 2", len(m.lines))
	}
}

func TestNormalToVisualV(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.cursorCol = 2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if m.mode != modeVisual {
		t.Errorf("mode = %v, want VISUAL", m.mode)
	}
	if m.visualRow != 0 || m.visualCol != 2 {
		t.Errorf("visual anchor = (%d,%d), want (0,2)", m.visualRow, m.visualCol)
	}
}

func TestNormalToCommand(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	if m.mode != modeCommand {
		t.Errorf("mode = %v, want COMMAND", m.mode)
	}
}

func TestNormalToFindSlash(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	if m.mode != modeFind {
		t.Errorf("mode = %v, want FIND", m.mode)
	}
	if !m.searchFwd {
		t.Error("/ should set searchFwd = true")
	}
}

func TestNormalToFindQuestion(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if m.mode != modeFind {
		t.Errorf("mode = %v, want FIND", m.mode)
	}
	if m.searchFwd {
		t.Error("? should set searchFwd = false")
	}
}

// ---------------------------------------------------------------------------
// Normal mode - editing
// ---------------------------------------------------------------------------

func TestNormalDeleteX(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.cursorCol = 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.lines[0] != "hllo" {
		t.Errorf("after x: line = %q, want hllo", m.lines[0])
	}
	if m.yankRegister != "e" {
		t.Errorf("yank register = %q, want e", m.yankRegister)
	}
	if !m.dirty {
		t.Error("x should set dirty")
	}
}

func TestNormalDeleteBigX(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.cursorCol = 2
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if m.lines[0] != "hllo" {
		t.Errorf("after X: line = %q, want hllo", m.lines[0])
	}
	if m.cursorCol != 1 {
		t.Errorf("after X: col = %d, want 1", m.cursorCol)
	}
}

func TestNormalDD(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo\nthree")
	m.SetDimensions(80, 40)
	m.cursorRow = 1
	// d
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.pendingOp != 'd' {
		t.Fatalf("pending op = %c, want d", m.pendingOp)
	}
	// d again = dd
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if len(m.lines) != 2 {
		t.Errorf("after dd: lines = %d, want 2", len(m.lines))
	}
	if m.yankRegister != "two" {
		t.Errorf("yank register = %q, want two", m.yankRegister)
	}
	if !m.yankLinewise {
		t.Error("dd should set yankLinewise")
	}
}

func TestNormalYY(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.yankRegister != "one" {
		t.Errorf("yank register = %q, want one", m.yankRegister)
	}
	if len(m.lines) != 2 {
		t.Error("yy should not delete lines")
	}
}

func TestNormalCC(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if m.mode != modeInsert {
		t.Errorf("mode = %v, want INSERT after cc", m.mode)
	}
	if m.lines[0] != "" {
		t.Errorf("line should be empty after cc, got %q", m.lines[0])
	}
	if m.yankRegister != "one" {
		t.Errorf("yank register = %q, want one", m.yankRegister)
	}
}

func TestNormalPaste(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)
	m.yankRegister = "XYZ"
	m.yankLinewise = false
	m.cursorCol = 4
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if !strings.Contains(m.lines[0], "XYZ") {
		t.Errorf("after p: line = %q, should contain XYZ", m.lines[0])
	}
	if !m.dirty {
		t.Error("paste should set dirty")
	}
}

func TestNormalPasteLinewise(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.yankRegister = "new line"
	m.yankLinewise = true
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	if len(m.lines) != 2 {
		t.Errorf("lines = %d, want 2", len(m.lines))
	}
	if m.lines[1] != "new line" {
		t.Errorf("lines[1] = %q, want 'new line'", m.lines[1])
	}
	if m.cursorRow != 1 {
		t.Errorf("cursor should be on pasted line, row = %d", m.cursorRow)
	}
}

func TestNormalPasteBigP(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.yankRegister = "above"
	m.yankLinewise = true
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("P")})
	if len(m.lines) != 2 {
		t.Errorf("lines = %d, want 2", len(m.lines))
	}
	if m.lines[0] != "above" {
		t.Errorf("lines[0] = %q, want above", m.lines[0])
	}
}

func TestNormalUndoRedo(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	// Make a change: x deletes char
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.lines[0] != "ello" {
		t.Fatalf("after x: line = %q", m.lines[0])
	}
	// u - undo
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	if m.lines[0] != "hello" {
		t.Errorf("after undo: line = %q, want hello", m.lines[0])
	}
	// ctrl+r - redo
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if m.lines[0] != "ello" {
		t.Errorf("after redo: line = %q, want ello", m.lines[0])
	}
}

func TestNormalSearchN(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "aaa\nbbb\naaa")
	m.SetDimensions(80, 40)
	m.lastSearch = "aaa"
	m.executeSearch("aaa", true)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	// Should move to next match
	if m.cursorRow == 1 {
		t.Error("n should skip non-matching lines")
	}
}

func TestNormalSearchNoPreviousSearch(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if !strings.Contains(m.statusMsg, "No previous search") {
		t.Errorf("statusMsg = %q", m.statusMsg)
	}
}

func TestNormalReplace(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.cursorCol = 1
	// r then a character
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	if m.pendingOp != 'r' {
		t.Fatalf("pendingOp = %c, want r", m.pendingOp)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if m.lines[0] != "hXllo" {
		t.Errorf("after r X: line = %q, want hXllo", m.lines[0])
	}
}

func TestNormalJoinLines(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld")
	m.SetDimensions(80, 40)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("J")})
	if len(m.lines) != 1 {
		t.Errorf("lines = %d, want 1", len(m.lines))
	}
	if m.lines[0] != "hello world" {
		t.Errorf("joined line = %q, want 'hello world'", m.lines[0])
	}
}

func TestNormalCtrlSave(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Error("ctrl+s should return save command")
	}
	if !m.saving {
		t.Error("saving should be true")
	}
}

func TestNormalPageMovement(t *testing.T) {
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "line"
	}
	m := NewEditorModel("/tmp/t.txt", false, strings.Join(lines, "\n"))
	m.SetDimensions(80, 40)
	// ctrl+f page down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if m.cursorRow == 0 {
		t.Error("pgdown should move cursor")
	}
	// ctrl+b page up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.cursorRow != 0 {
		t.Errorf("pgup row = %d, want 0", m.cursorRow)
	}
}

func TestNormalEscClearsSearch(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.findMatches = [][2]int{{0, 0}}
	m.statusMsg = "something"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.findMatches != nil {
		t.Error("esc should clear findMatches")
	}
	if m.statusMsg != "" {
		t.Errorf("esc should clear statusMsg, got %q", m.statusMsg)
	}
}

// ---------------------------------------------------------------------------
// Insert mode
// ---------------------------------------------------------------------------

func TestInsertModeTyping(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorCol = 5
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")})
	if m.lines[0] != "helloX" {
		t.Errorf("after typing: line = %q, want helloX", m.lines[0])
	}
	if !m.dirty {
		t.Error("typing should set dirty")
	}
}

func TestInsertModeEnter(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorCol = 5
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(m.lines))
	}
	if m.lines[0] != "hello" {
		t.Errorf("lines[0] = %q, want hello", m.lines[0])
	}
	if m.lines[1] != " world" {
		t.Errorf("lines[1] = %q, want ' world'", m.lines[1])
	}
	if m.cursorRow != 1 || m.cursorCol != 0 {
		t.Errorf("cursor = (%d,%d), want (1,0)", m.cursorRow, m.cursorCol)
	}
}

func TestInsertModeBackspace(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorCol = 3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.lines[0] != "helo" {
		t.Errorf("after backspace: line = %q, want helo", m.lines[0])
	}
	if m.cursorCol != 2 {
		t.Errorf("col = %d, want 2", m.cursorCol)
	}
}

func TestInsertModeBackspaceJoinLines(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorRow = 1
	m.cursorCol = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if len(m.lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(m.lines))
	}
	if m.lines[0] != "helloworld" {
		t.Errorf("joined = %q, want helloworld", m.lines[0])
	}
}

func TestInsertModeDelete(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorCol = 1
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDelete})
	if m.lines[0] != "hllo" {
		t.Errorf("after delete: line = %q, want hllo", m.lines[0])
	}
}

func TestInsertModeTab(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorCol = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !strings.HasPrefix(m.lines[0], "    ") {
		t.Errorf("tab should insert 4 spaces, got %q", m.lines[0])
	}
	if m.cursorCol != 4 {
		t.Errorf("col = %d, want 4", m.cursorCol)
	}
}

func TestInsertModeEscBackToNormal(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorCol = 3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL", m.mode)
	}
	if m.cursorCol != 2 { // backs up 1 in normal mode
		t.Errorf("col = %d, want 2", m.cursorCol)
	}
}

func TestInsertModeCtrlS(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Error("ctrl+s in insert should return save command")
	}
}

func TestInsertModeArrows(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorCol = 3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.cursorRow != 1 {
		t.Errorf("down row = %d, want 1", m.cursorRow)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.cursorRow != 0 {
		t.Errorf("up row = %d, want 0", m.cursorRow)
	}
}

func TestInsertModeHomeEnd(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.mode = modeInsert
	m.cursorCol = 3
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyHome})
	if m.cursorCol != 0 {
		t.Errorf("home col = %d, want 0", m.cursorCol)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if m.cursorCol != 5 {
		t.Errorf("end col = %d, want 5", m.cursorCol)
	}
}

// ---------------------------------------------------------------------------
// Visual mode
// ---------------------------------------------------------------------------

func TestVisualModeDelete(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)
	m.mode = modeVisual
	m.visualRow = 0
	m.visualCol = 0
	m.cursorCol = 4
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL after visual d", m.mode)
	}
	if m.lines[0] != " world" {
		t.Errorf("line = %q, want ' world'", m.lines[0])
	}
}

func TestVisualModeYank(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)
	m.mode = modeVisual
	m.visualRow = 0
	m.visualCol = 0
	m.cursorCol = 4
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL after visual y", m.mode)
	}
	if m.yankRegister != "hello" {
		t.Errorf("yank register = %q, want hello", m.yankRegister)
	}
	// Lines should not be modified
	if m.lines[0] != "hello world" {
		t.Errorf("line modified after yank: %q", m.lines[0])
	}
}

func TestVisualModeChange(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world")
	m.SetDimensions(80, 40)
	m.mode = modeVisual
	m.visualRow = 0
	m.visualCol = 0
	m.cursorCol = 4
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if m.mode != modeInsert {
		t.Errorf("mode = %v, want INSERT after visual c", m.mode)
	}
}

func TestVisualModeEsc(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.mode = modeVisual
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL after esc", m.mode)
	}
}

func TestVisualModeVTogglesOff(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello")
	m.SetDimensions(80, 40)
	m.mode = modeVisual
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	if m.mode != modeNormal {
		t.Errorf("v in visual should go back to NORMAL, got %v", m.mode)
	}
}

func TestVisualModeMovement(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello\nworld")
	m.SetDimensions(80, 40)
	m.mode = modeVisual
	m.visualRow = 0
	m.visualCol = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if m.cursorRow != 1 {
		t.Errorf("visual j: row = %d, want 1", m.cursorRow)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if m.cursorCol != 1 {
		t.Errorf("visual l: col = %d, want 1", m.cursorCol)
	}
}

func TestVisualModeG(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo\nthree")
	m.SetDimensions(80, 40)
	m.mode = modeVisual
	m.visualRow = 0
	m.visualCol = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if m.cursorRow != 2 {
		t.Errorf("visual G: row = %d, want 2", m.cursorRow)
	}
}

// ---------------------------------------------------------------------------
// Command mode
// ---------------------------------------------------------------------------

func TestCommandModeW(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeCommand
	// Type "w"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	// Press enter
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL after :w", m.mode)
	}
	if cmd == nil {
		t.Error(":w should return save command")
	}
}

func TestCommandModeQ(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeCommand
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL after :q", m.mode)
	}
	if cmd == nil {
		t.Error(":q should return close command")
	}
	// Execute and verify it's EditorCloseMsg
	msg := cmd()
	if _, ok := msg.(EditorCloseMsg); !ok {
		t.Errorf("expected EditorCloseMsg, got %T", msg)
	}
}

func TestCommandModeQDirtyBlocked(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.dirty = true
	m.mode = modeCommand
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error(":q with unsaved changes should not close")
	}
	if !strings.Contains(m.statusMsg, "Unsaved") {
		t.Errorf("statusMsg = %q, want Unsaved warning", m.statusMsg)
	}
}

func TestCommandModeQBang(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.dirty = true
	m.mode = modeCommand
	// Type "q!"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error(":q! should force close")
	}
}

func TestCommandModeWQ(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeCommand
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error(":wq should return save command")
	}
	if !m.closeAfterSave {
		t.Error("closeAfterSave should be true after :wq")
	}
}

func TestCommandModeGoToLine(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo\nthree\nfour\nfive")
	m.SetDimensions(80, 40)
	m.mode = modeCommand
	// Type "3"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("3")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.cursorRow != 2 { // 0-indexed
		t.Errorf("goto line: row = %d, want 2 (line 3)", m.cursorRow)
	}
}

func TestCommandModeUnknown(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeCommand
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(m.statusMsg, "Unknown command") {
		t.Errorf("statusMsg = %q, want Unknown command", m.statusMsg)
	}
}

func TestCommandModeEsc(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeCommand
	m.cmdBuffer = "some"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL after esc", m.mode)
	}
	if m.cmdBuffer != "" {
		t.Errorf("cmdBuffer = %q, want empty", m.cmdBuffer)
	}
}

func TestCommandModeBackspace(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeCommand
	m.cmdBuffer = "wq"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.cmdBuffer != "w" {
		t.Errorf("cmdBuffer = %q, want w", m.cmdBuffer)
	}
}

func TestCommandModeBackspaceEmpty(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeCommand
	m.cmdBuffer = ""
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.mode != modeNormal {
		t.Errorf("backspace on empty cmd should return to normal, got %v", m.mode)
	}
}

// ---------------------------------------------------------------------------
// Find mode
// ---------------------------------------------------------------------------

func TestFindModeSearch(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "hello world\nhello again")
	m.SetDimensions(80, 40)
	m.mode = modeFind
	m.searchFwd = true
	// Type "hello"
	for _, ch := range "hello" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
	}
	if m.findBuffer != "hello" {
		t.Errorf("findBuffer = %q, want hello", m.findBuffer)
	}
	// Press enter to execute search
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL after find enter", m.mode)
	}
	if len(m.findMatches) != 2 {
		t.Errorf("findMatches = %d, want 2", len(m.findMatches))
	}
}

func TestFindModeEsc(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeFind
	m.findBuffer = "partial"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want NORMAL after esc", m.mode)
	}
	if m.findBuffer != "" {
		t.Errorf("findBuffer = %q, want empty", m.findBuffer)
	}
}

func TestFindModeBackspace(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeFind
	m.findBuffer = "ab"
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.findBuffer != "a" {
		t.Errorf("findBuffer = %q, want a", m.findBuffer)
	}
}

func TestFindModeBackspaceEmpty(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 40)
	m.mode = modeFind
	m.findBuffer = ""
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if m.mode != modeNormal {
		t.Errorf("backspace on empty find should return to normal, got %v", m.mode)
	}
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func TestEditorViewZeroWidth(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.width = 0
	view := m.View()
	if view != "Loading editor..." {
		t.Errorf("zero width view = %q", view)
	}
}

func TestEditorViewBasic(t *testing.T) {
	m := NewEditorModel("/tmp/test.txt", false, "hello\nworld")
	m.SetDimensions(80, 20)
	view := m.View()
	if view == "" {
		t.Error("view should not be empty")
	}
	if !strings.Contains(view, "test.txt") {
		t.Error("view should contain filename")
	}
	if !strings.Contains(view, "NORMAL") {
		t.Error("view should contain mode indicator")
	}
}

func TestEditorViewRemoteLabel(t *testing.T) {
	m := NewEditorModel("/remote/file.txt", true, "data")
	m.SetDimensions(80, 20)
	view := m.View()
	if !strings.Contains(view, "Remote") {
		t.Error("remote file view should contain 'Remote'")
	}
}

func TestEditorViewLocalLabel(t *testing.T) {
	m := NewEditorModel("/local/file.txt", false, "data")
	m.SetDimensions(80, 20)
	view := m.View()
	if !strings.Contains(view, "Local") {
		t.Error("local file view should contain 'Local'")
	}
}

func TestEditorViewModifiedIndicator(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 20)
	m.dirty = true
	view := m.View()
	if !strings.Contains(view, "[modified]") {
		t.Error("dirty editor should show [modified]")
	}
}

func TestEditorViewTildes(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "short")
	m.SetDimensions(80, 20)
	view := m.View()
	if !strings.Contains(view, "~") {
		t.Error("view should show ~ for empty lines")
	}
}

func TestEditorViewLineNumbers(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo\nthree")
	m.SetDimensions(80, 20)
	view := m.View()
	if !strings.Contains(view, "1") || !strings.Contains(view, "2") || !strings.Contains(view, "3") {
		t.Error("view should show line numbers")
	}
}

func TestEditorViewCommandMode(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 20)
	m.mode = modeCommand
	m.cmdBuffer = "wq"
	view := m.View()
	if !strings.Contains(view, ":wq") {
		t.Error("command mode should show :wq in status bar")
	}
}

func TestEditorViewFindMode(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 20)
	m.mode = modeFind
	m.findBuffer = "search"
	m.searchFwd = true
	view := m.View()
	if !strings.Contains(view, "/search") {
		t.Error("find mode should show /search in status bar")
	}
}

func TestEditorViewFindModeBackward(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 20)
	m.mode = modeFind
	m.findBuffer = "term"
	m.searchFwd = false
	view := m.View()
	if !strings.Contains(view, "?term") {
		t.Error("backward find should show ?term in status bar")
	}
}

// ---------------------------------------------------------------------------
// renderStatusBar
// ---------------------------------------------------------------------------

func TestRenderStatusBarNormal(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 20)
	bar := m.renderStatusBar()
	if !strings.Contains(bar, "NORMAL") {
		t.Errorf("status bar = %q, want NORMAL", bar)
	}
	if !strings.Contains(bar, "Ln 1") {
		t.Errorf("status bar should show line number")
	}
}

func TestRenderStatusBarInsert(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 20)
	m.mode = modeInsert
	bar := m.renderStatusBar()
	if !strings.Contains(bar, "INSERT") {
		t.Errorf("status bar = %q, want INSERT", bar)
	}
}

func TestRenderStatusBarWithMessage(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "test")
	m.SetDimensions(80, 20)
	m.statusMsg = "test message"
	bar := m.renderStatusBar()
	if !strings.Contains(bar, "test message") {
		t.Errorf("status bar should contain status message")
	}
}

// ---------------------------------------------------------------------------
// IsEditableFile
// ---------------------------------------------------------------------------

func TestIsEditableFileCommonExtensions(t *testing.T) {
	editable := []string{
		"file.txt", "file.md", "file.go", "file.py", "file.js",
		"file.ts", "file.json", "file.yaml", "file.yml", "file.toml",
		"file.html", "file.css", "file.sh", "file.rs", "file.java",
		"file.c", "file.cpp", "file.h", "file.sql", "file.rb",
	}
	for _, name := range editable {
		if !IsEditableFile(name) {
			t.Errorf("IsEditableFile(%q) = false, want true", name)
		}
	}
}

func TestIsEditableFileKnownNames(t *testing.T) {
	names := []string{
		"Makefile", "Dockerfile", "README", "LICENSE",
		".gitignore", ".env", ".editorconfig",
	}
	for _, name := range names {
		if !IsEditableFile(name) {
			t.Errorf("IsEditableFile(%q) = false, want true", name)
		}
	}
}

func TestIsEditableFileBinaryExtensions(t *testing.T) {
	nonEditable := []string{
		"file.exe", "file.bin", "file.png", "file.jpg",
		"file.zip", "file.tar.gz", "file.pdf", "file.mp3",
	}
	for _, name := range nonEditable {
		if IsEditableFile(name) {
			t.Errorf("IsEditableFile(%q) = true, want false", name)
		}
	}
}

func TestIsEditableFileCaseInsensitive(t *testing.T) {
	// Known names are case-insensitive via strings.ToLower
	if !IsEditableFile("makefile") {
		t.Error("IsEditableFile(makefile) should be true")
	}
}

func TestIsEditableFileWithPath(t *testing.T) {
	if !IsEditableFile("/home/user/project/main.go") {
		t.Error("IsEditableFile with full path should work")
	}
}

// ---------------------------------------------------------------------------
// MaxEditableSize constant
// ---------------------------------------------------------------------------

func TestMaxEditableSize(t *testing.T) {
	if MaxEditableSize != 1<<20 {
		t.Errorf("MaxEditableSize = %d, want %d", MaxEditableSize, 1<<20)
	}
}

// ---------------------------------------------------------------------------
// Message types (structure tests)
// ---------------------------------------------------------------------------

func TestOpenEditorMsg(t *testing.T) {
	msg := OpenEditorMsg{Path: "/tmp/file.txt", IsRemote: true}
	if msg.Path != "/tmp/file.txt" {
		t.Errorf("Path = %q", msg.Path)
	}
	if !msg.IsRemote {
		t.Error("IsRemote should be true")
	}
}

func TestEditorContentLoadedMsg(t *testing.T) {
	msg := EditorContentLoadedMsg{Path: "/tmp/f.txt", Content: "data", IsRemote: false}
	if msg.Path != "/tmp/f.txt" || msg.Content != "data" || msg.IsRemote {
		t.Error("unexpected fields")
	}
}

func TestEditorSaveMsg(t *testing.T) {
	msg := EditorSaveMsg{Path: "/tmp/f.txt", Content: "data", IsRemote: true}
	if msg.Path != "/tmp/f.txt" || msg.Content != "data" || !msg.IsRemote {
		t.Error("unexpected fields")
	}
}

func TestEditorSaveDoneMsg(t *testing.T) {
	msg := EditorSaveDoneMsg{Err: nil}
	if msg.Err != nil {
		t.Error("Err should be nil")
	}
}

func TestEditorCloseMsg(t *testing.T) {
	_ = EditorCloseMsg{} // just verify it compiles
}

// ---------------------------------------------------------------------------
// ensureCursorVisible
// ---------------------------------------------------------------------------

func TestEnsureCursorVisible(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, "one\ntwo\nthree\nfour\nfive")
	m.SetDimensions(80, 6) // visibleRows = 3
	m.cursorRow = 4
	m.ensureCursorVisible()
	if m.scrollRow > m.cursorRow {
		t.Errorf("scrollRow=%d should not exceed cursorRow=%d", m.scrollRow, m.cursorRow)
	}
	end := m.scrollRow + m.visibleRows()
	if m.cursorRow >= end {
		t.Errorf("cursorRow=%d should be < scrollRow+vis=%d", m.cursorRow, end)
	}
}

func TestEnsureCursorVisibleHorizontal(t *testing.T) {
	m := NewEditorModel("/tmp/t.txt", false, strings.Repeat("x", 200))
	m.SetDimensions(40, 20)
	m.cursorCol = 150
	m.ensureCursorVisible()
	if m.scrollCol == 0 {
		t.Error("scrollCol should have advanced for far-right cursor")
	}
}
