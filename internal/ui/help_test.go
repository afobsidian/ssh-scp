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
	bindings := []string{"Ctrl+T", "Ctrl+U", "Ctrl+D", "Ctrl+N", "Ctrl+W", "Tab", "Enter", "Backspace", "Ctrl+C"}
	for _, b := range bindings {
		if !strings.Contains(got, b) {
			t.Errorf("help should contain %q", b)
		}
	}
}

func TestRenderHelpContainsDescriptions(t *testing.T) {
	got := RenderHelp(100, 40)
	descriptions := []string{"terminal", "file browser", "Upload", "Download", "help", "Quit"}
	for _, d := range descriptions {
		if !strings.Contains(got, d) {
			t.Errorf("help should contain description %q", d)
		}
	}
}

func TestRenderHelpSmallDimensions(t *testing.T) {
	got := RenderHelp(20, 10)
	if got == "" {
		t.Error("RenderHelp should still return content at small dimensions")
	}
}
