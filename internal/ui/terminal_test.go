package ui

import (
	"io"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// splitOutputLines
// ---------------------------------------------------------------------------

func TestSplitOutputLinesBasic(t *testing.T) {
	lines := splitOutputLines("line1\nline2\nline3", 10)
	if len(lines) != 3 {
		t.Fatalf("expected 3, got %d", len(lines))
	}
}

func TestSplitOutputLinesTruncates(t *testing.T) {
	input := "a\nb\nc\nd\ne"
	lines := splitOutputLines(input, 3)
	if len(lines) != 3 {
		t.Fatalf("expected 3 (truncated), got %d", len(lines))
	}
	if lines[0] != "c" {
		t.Errorf("first visible line = %q, want %q", lines[0], "c")
	}
}

func TestSplitOutputLinesZeroN(t *testing.T) {
	lines := splitOutputLines("a\nb\nc", 0)
	if len(lines) != 3 {
		t.Errorf("n=0 should return all, got %d", len(lines))
	}
}

func TestSplitOutputLinesEmpty(t *testing.T) {
	lines := splitOutputLines("", 10)
	if len(lines) != 0 {
		t.Errorf("expected 0, got %d", len(lines))
	}
}

func TestSplitOutputLinesSingle(t *testing.T) {
	lines := splitOutputLines("hello", 1)
	if len(lines) != 1 || lines[0] != "hello" {
		t.Errorf("expected [hello], got %v", lines)
	}
}

func TestSplitOutputLinesNoNewline(t *testing.T) {
	lines := splitOutputLines("hello world", 10)
	if len(lines) != 1 || lines[0] != "hello world" {
		t.Errorf("expected [hello world], got %v", lines)
	}
}

// ---------------------------------------------------------------------------
// joinLines
// ---------------------------------------------------------------------------

func TestJoinLinesMultiple(t *testing.T) {
	got := joinLines([]string{"a", "b", "c"})
	if got != "a\nb\nc" {
		t.Errorf("joinLines = %q, want %q", got, "a\nb\nc")
	}
}

func TestJoinLinesEmpty(t *testing.T) {
	got := joinLines(nil)
	if got != "" {
		t.Errorf("joinLines nil = %q, want empty", got)
	}
}

func TestJoinLinesSingle(t *testing.T) {
	got := joinLines([]string{"hello"})
	if got != "hello" {
		t.Errorf("joinLines single = %q, want %q", got, "hello")
	}
}

// ---------------------------------------------------------------------------
// TerminalModel - AppendOutput & BufferedOutput
// ---------------------------------------------------------------------------

func TestTerminalModelAppendOutput(t *testing.T) {
	m := &TerminalModel{}
	m.AppendOutput([]byte("hello "))
	m.AppendOutput([]byte("world"))
	got := m.BufferedOutput()
	if got != "hello world" {
		t.Errorf("BufferedOutput = %q, want %q", got, "hello world")
	}
}

func TestTerminalModelBufferCap(t *testing.T) {
	m := &TerminalModel{}
	// Write > 100KB to trigger trim
	bigData := make([]byte, 80*1024)
	for i := range bigData {
		bigData[i] = 'A'
	}
	m.AppendOutput(bigData)
	m.AppendOutput(bigData) // ~160KB total, should trigger trim to ~50KB

	output := m.BufferedOutput()
	if len(output) > 100*1024 {
		t.Errorf("buffer should be capped, got %d bytes", len(output))
	}
	if len(output) < 50*1024 {
		t.Errorf("buffer should retain ~50KB, got %d bytes", len(output))
	}
}

func TestTerminalModelSetError(t *testing.T) {
	m := &TerminalModel{}
	m.SetError("something broke")
	if m.err != "something broke" {
		t.Errorf("err = %q, want %q", m.err, "something broke")
	}
}

func TestTerminalModelRenderWithError(t *testing.T) {
	m := &TerminalModel{}
	m.SetError("test error")
	view := m.RenderTerminal(true, 80, 24)
	if !strings.Contains(view, "Error: test error") {
		t.Errorf("view should contain error text, got:\n%s", view)
	}
}

func TestTerminalModelRenderNormal(t *testing.T) {
	m := &TerminalModel{}
	m.AppendOutput([]byte("prompt$ "))
	view := m.RenderTerminal(false, 80, 24)
	if !strings.Contains(view, "prompt$") {
		t.Errorf("view should contain terminal output, got:\n%s", view)
	}
}

func TestTerminalModelWriteNilStdin(t *testing.T) {
	m := &TerminalModel{}
	err := m.Write([]byte("data"))
	if err != nil {
		t.Errorf("Write with nil stdin should return nil, got %v", err)
	}
}

func TestTerminalModelResize(t *testing.T) {
	m := &TerminalModel{}
	m.Resize(120, 40)
	if m.width != 120 || m.height != 40 {
		t.Errorf("dimensions = %dx%d, want 120x40", m.width, m.height)
	}
}

func TestTerminalModelCloseNoSession(t *testing.T) {
	m := &TerminalModel{}
	err := m.Close()
	if err != nil {
		t.Errorf("Close with no session should return nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewTerminalModel
// ---------------------------------------------------------------------------

func TestNewTerminalModel(t *testing.T) {
	m := NewTerminalModel(nil)
	if m == nil {
		t.Fatal("NewTerminalModel should not return nil")
	}
	if !m.active {
		t.Error("active should be true")
	}
	if m.client != nil {
		t.Error("client should be nil when passed nil")
	}
}

// ---------------------------------------------------------------------------
// SetProgram
// ---------------------------------------------------------------------------

func TestSetProgram(t *testing.T) {
	m := NewTerminalModel(nil)
	m.SetProgram(nil)
	if m.program != nil {
		t.Error("program should be nil")
	}
}

// ---------------------------------------------------------------------------
// Write with io.Pipe (simulated stdin)
// ---------------------------------------------------------------------------

func TestTerminalModelWriteWithPipe(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()

	m := &TerminalModel{stdin: w}
	go func() {
		buf := make([]byte, 10)
		r.Read(buf)
	}()

	err := m.Write([]byte("hi"))
	if err != nil {
		t.Errorf("Write with pipe = %v, want nil", err)
	}
}

// ---------------------------------------------------------------------------
// Close with stdin pipe
// ---------------------------------------------------------------------------

func TestTerminalModelCloseWithStdin(t *testing.T) {
	_, w := io.Pipe()
	m := &TerminalModel{stdin: w}

	err := m.Close()
	if err != nil {
		t.Errorf("Close with stdin pipe should succeed, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// RenderTerminal active vs inactive styling
// ---------------------------------------------------------------------------

func TestTerminalModelRenderActive(t *testing.T) {
	m := &TerminalModel{}
	m.AppendOutput([]byte("test"))
	active := m.RenderTerminal(true, 80, 24)
	inactive := m.RenderTerminal(false, 80, 24)
	// Both should render but may differ in border color
	if active == "" || inactive == "" {
		t.Error("both views should be non-empty")
	}
}

// ---------------------------------------------------------------------------
// BufferedOutput empty
// ---------------------------------------------------------------------------

func TestTerminalModelBufferedOutputEmpty(t *testing.T) {
	m := &TerminalModel{}
	got := m.BufferedOutput()
	if got != "" {
		t.Errorf("empty buffer should return empty string, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// AppendOutput multiple calls
// ---------------------------------------------------------------------------

func TestTerminalModelAppendOutputMultiple(t *testing.T) {
	m := &TerminalModel{}
	for i := 0; i < 100; i++ {
		m.AppendOutput([]byte("line\n"))
	}
	output := m.BufferedOutput()
	if len(output) == 0 {
		t.Error("should have output after many appends")
	}
}
