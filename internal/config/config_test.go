package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestConfig(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, ".config", "ssh-scp")
	if err := os.MkdirAll(cfgDir, 0700); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(cfgDir, "connections.json")

	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	return cfgFile, func() { os.Setenv("HOME", origHome) }
}

func TestConfigPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "ssh-scp", "connections.json")
	got := configPath()
	if got != want {
		t.Errorf("configPath() = %q, want %q", got, want)
	}
}

func TestLoadNonExistent(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if len(cfg.RecentConnections) != 0 {
		t.Errorf("expected 0 recent connections, got %d", len(cfg.RecentConnections))
	}
}

func TestSaveAndLoad(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg := &Config{
		RecentConnections: []Connection{
			{Name: "test", Host: "host1", Port: "22", Username: "user1"},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.RecentConnections) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(loaded.RecentConnections))
	}
	if loaded.RecentConnections[0].Host != "host1" {
		t.Errorf("Host = %q, want %q", loaded.RecentConnections[0].Host, "host1")
	}
}

func TestLoadInvalidJSON(t *testing.T) {
	cfgFile, cleanup := setupTestConfig(t)
	defer cleanup()

	if err := os.WriteFile(cfgFile, []byte("{invalid json"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() should not return error for invalid JSON, got %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() should return empty config for invalid JSON")
	}
	if len(cfg.RecentConnections) != 0 {
		t.Errorf("expected 0 connections for invalid JSON, got %d", len(cfg.RecentConnections))
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	cfg := &Config{}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	p := filepath.Join(dir, ".config", "ssh-scp", "connections.json")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Errorf("expected config file to exist at %s", p)
	}
}

func TestSaveFilePermissions(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg := &Config{RecentConnections: []Connection{{Host: "h"}}}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	p := configPath()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("config file perm = %o, want 0600", perm)
	}
}

func TestSaveProducesValidJSON(t *testing.T) {
	_, cleanup := setupTestConfig(t)
	defer cleanup()

	cfg := &Config{
		RecentConnections: []Connection{
			{Name: "n", Host: "h", Port: "22", Username: "u", Password: "p", KeyPath: "/k"},
		},
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(configPath())
	if err != nil {
		t.Fatal(err)
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if loaded.RecentConnections[0].Password != "p" {
		t.Errorf("Password = %q, want %q", loaded.RecentConnections[0].Password, "p")
	}
}

func TestAddRecentNew(t *testing.T) {
	cfg := &Config{}
	conn := Connection{Host: "h1", Port: "22", Username: "u1"}
	cfg.AddRecent(conn)
	if len(cfg.RecentConnections) != 1 {
		t.Fatalf("expected 1, got %d", len(cfg.RecentConnections))
	}
	if cfg.RecentConnections[0].Host != "h1" {
		t.Errorf("Host = %q, want %q", cfg.RecentConnections[0].Host, "h1")
	}
}

func TestAddRecentUpdate(t *testing.T) {
	cfg := &Config{
		RecentConnections: []Connection{
			{Name: "old", Host: "h1", Port: "22", Username: "u1"},
		},
	}
	conn := Connection{Name: "new", Host: "h1", Port: "22", Username: "u1"}
	cfg.AddRecent(conn)
	if len(cfg.RecentConnections) != 1 {
		t.Fatalf("expected 1 (updated), got %d", len(cfg.RecentConnections))
	}
	if cfg.RecentConnections[0].Name != "new" {
		t.Errorf("Name = %q, want %q", cfg.RecentConnections[0].Name, "new")
	}
}

func TestAddRecentPrependsNew(t *testing.T) {
	cfg := &Config{
		RecentConnections: []Connection{
			{Host: "h1", Port: "22", Username: "u1"},
		},
	}
	conn := Connection{Host: "h2", Port: "22", Username: "u2"}
	cfg.AddRecent(conn)
	if len(cfg.RecentConnections) != 2 {
		t.Fatalf("expected 2, got %d", len(cfg.RecentConnections))
	}
	if cfg.RecentConnections[0].Host != "h2" {
		t.Errorf("first entry Host = %q, want %q", cfg.RecentConnections[0].Host, "h2")
	}
}

func TestAddRecentMaxTen(t *testing.T) {
	cfg := &Config{}
	for i := 0; i < 12; i++ {
		cfg.AddRecent(Connection{
			Host:     fmt.Sprintf("h%d", i),
			Port:     "22",
			Username: "u",
		})
	}
	if len(cfg.RecentConnections) != 10 {
		t.Errorf("expected max 10 recent, got %d", len(cfg.RecentConnections))
	}
	if cfg.RecentConnections[0].Host != "h11" {
		t.Errorf("first = %q, want h11", cfg.RecentConnections[0].Host)
	}
}

func TestAddRecentDifferentPort(t *testing.T) {
	cfg := &Config{
		RecentConnections: []Connection{
			{Host: "h1", Port: "22", Username: "u1"},
		},
	}
	conn := Connection{Host: "h1", Port: "2222", Username: "u1"}
	cfg.AddRecent(conn)
	if len(cfg.RecentConnections) != 2 {
		t.Errorf("expected 2 (different port), got %d", len(cfg.RecentConnections))
	}
}

func TestConnectionJSONOmitsEmpty(t *testing.T) {
	conn := Connection{
		Name:     "test",
		Host:     "host",
		Port:     "22",
		Username: "user",
	}
	data, err := json.Marshal(conn)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "password") {
		t.Errorf("empty password should be omitted, got %s", s)
	}
	if strings.Contains(s, "key_path") {
		t.Errorf("empty key_path should be omitted, got %s", s)
	}
}

func TestConnectionJSONIncludesNonEmpty(t *testing.T) {
	conn := Connection{
		Name:     "test",
		Host:     "host",
		Port:     "22",
		Username: "user",
		Password: "secret",
		KeyPath:  "/path/to/key",
	}
	data, err := json.Marshal(conn)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"password":"secret"`) {
		t.Errorf("expected password in JSON, got %s", s)
	}
	if !strings.Contains(s, `"key_path":"/path/to/key"`) {
		t.Errorf("expected key_path in JSON, got %s", s)
	}
}
