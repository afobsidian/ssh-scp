package ui

import (
	"strings"
	"testing"
)

func TestRenderHelpNonEmpty(t *testing.T) {
	got := RenderHelp(80, 40)
	if got == "" {
		t.Error("RenderHelp returned empty string")
	}
}

func TestRenderHelpContainsBindings(t *testing.T) {
	got := RenderHelp(120, 50)
	bindings := []string{"^T", "^K", "^D", "^R", "^N", "^W", "Tab", "Enter", "Backspace", "^C"}
	for _, b := range bindings {
		if !strings.Contains(got, b) {
			t.Errorf("help should contain %q", b)
		}
	}
}

func TestRenderHelpContainsDescriptions(t *testing.T) {
	got := RenderHelp(100, 40)
	descriptions := []string{"local", "remote", "Transfer", "help", "Quit"}
	for _, d := range descriptions {
		if !strings.Contains(got, d) {
			t.Errorf("help should contain description %q", d)
		}
	}
}

func TestRenderHelpContainsEditorBindings(t *testing.T) {
	got := RenderHelp(120, 60)
	editorBindings := []string{
		"Editor", "Vim", "NORMAL", "INSERT", "VISUAL", "COMMAND", "FIND",
		"h/j/k/l", "dd", "yy", "Undo", "redo", ":w", ":q", ":wq",
		"^S",
	}
	for _, b := range editorBindings {
		if !strings.Contains(got, b) {
			t.Errorf("help should contain editor binding %q", b)
		}
	}
}

func TestRenderHelpSmallDimensions(t *testing.T) {
	got := RenderHelp(20, 10)
	if got == "" {
		t.Error("RenderHelp should still return content at small dimensions")
	}
}

func TestRenderHelpContainsFileOpsBindings(t *testing.T) {
	got := RenderHelp(120, 60)
	fileOps := []string{"Create new directory", "Delete selected", "Rename selected"}
	for _, b := range fileOps {
		if !strings.Contains(got, b) {
			t.Errorf("help should contain file op binding %q", b)
		}
	}
}
