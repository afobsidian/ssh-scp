package ui

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// TabTitle
// ---------------------------------------------------------------------------

func TestTabTitleWithUserHost(t *testing.T) {
	got := TabTitle("admin", "server", 0)
	want := "admin@server"
	if got != want {
		t.Errorf("TabTitle = %q, want %q", got, want)
	}
}

func TestTabTitleFallback(t *testing.T) {
	got := TabTitle("", "", 3)
	want := "Connection 4"
	if got != want {
		t.Errorf("TabTitle fallback = %q, want %q", got, want)
	}
}

func TestTabTitleNoHost(t *testing.T) {
	got := TabTitle("user", "", 0)
	want := "Connection 1"
	if got != want {
		t.Errorf("TabTitle no host = %q, want %q", got, want)
	}
}

func TestTabTitleNoUser(t *testing.T) {
	got := TabTitle("", "host", 2)
	want := "Connection 3"
	if got != want {
		t.Errorf("TabTitle no user = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// RenderTabBar
// ---------------------------------------------------------------------------

func TestRenderTabBarSingleActive(t *testing.T) {
	tabs := []Tab{{Title: "test", Connected: true}}
	bar := RenderTabBar(tabs, 0, 80)
	if !strings.Contains(bar, "test") {
		t.Error("tab bar should contain tab title")
	}
	if !strings.Contains(bar, "●") {
		t.Error("connected tab should show ●")
	}
}

func TestRenderTabBarMultiple(t *testing.T) {
	tabs := []Tab{
		{Title: "tab1", Connected: true},
		{Title: "tab2", Connected: false},
		{Title: "tab3", Connected: true},
	}
	bar := RenderTabBar(tabs, 1, 120)
	if !strings.Contains(bar, "tab1") || !strings.Contains(bar, "tab2") || !strings.Contains(bar, "tab3") {
		t.Error("tab bar should contain all tab titles")
	}
}

func TestRenderTabBarContainsPlus(t *testing.T) {
	bar := RenderTabBar(nil, 0, 80)
	if !strings.Contains(bar, "+") {
		t.Error("tab bar should contain + for new tab")
	}
}

func TestRenderTabBarDisconnectedIcon(t *testing.T) {
	tabs := []Tab{{Title: "x", Connected: false}}
	bar := RenderTabBar(tabs, 0, 80)
	if !strings.Contains(bar, "○") {
		t.Error("disconnected tab should show ○")
	}
}

// ---------------------------------------------------------------------------
// Tab struct
// ---------------------------------------------------------------------------

func TestTabStruct(t *testing.T) {
	tab := Tab{Title: "my-tab", Connected: true}
	if tab.Title != "my-tab" || !tab.Connected {
		t.Errorf("Tab fields = %+v", tab)
	}
}
