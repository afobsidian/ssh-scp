package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SSHHost helpers
// ---------------------------------------------------------------------------

func TestSSHHostDisplayHostWithHostName(t *testing.T) {
	h := SSHHost{Alias: "myserver", HostName: "192.168.1.10"}
	if got := h.DisplayHost(); got != "192.168.1.10" {
		t.Errorf("DisplayHost() = %q, want %q", got, "192.168.1.10")
	}
}

func TestSSHHostDisplayHostFallback(t *testing.T) {
	h := SSHHost{Alias: "myserver"}
	if got := h.DisplayHost(); got != "myserver" {
		t.Errorf("DisplayHost() = %q, want %q", got, "myserver")
	}
}

func TestSSHHostToConnection(t *testing.T) {
	h := SSHHost{
		Alias:        "prod",
		HostName:     "prod.example.com",
		Port:         "2222",
		User:         "deploy",
		IdentityFile: "/home/user/.ssh/id_rsa",
	}
	c := h.ToConnection()
	if c.Name != "prod" {
		t.Errorf("Name = %q, want %q", c.Name, "prod")
	}
	if c.Host != "prod.example.com" {
		t.Errorf("Host = %q, want %q", c.Host, "prod.example.com")
	}
	if c.Port != "2222" {
		t.Errorf("Port = %q, want %q", c.Port, "2222")
	}
	if c.Username != "deploy" {
		t.Errorf("Username = %q, want %q", c.Username, "deploy")
	}
	if c.KeyPath != "/home/user/.ssh/id_rsa" {
		t.Errorf("KeyPath = %q, want %q", c.KeyPath, "/home/user/.ssh/id_rsa")
	}
}

func TestSSHHostToConnectionDefaultPort(t *testing.T) {
	h := SSHHost{Alias: "test", HostName: "test.com"}
	c := h.ToConnection()
	if c.Port != "22" {
		t.Errorf("Port = %q, want %q", c.Port, "22")
	}
}

// ---------------------------------------------------------------------------
// ParseSSHConfig
// ---------------------------------------------------------------------------

func TestParseSSHConfigBasic(t *testing.T) {
	input := `
Host webserver
    HostName 192.168.1.100
    Port 22
    User admin
    IdentityFile ~/.ssh/id_rsa

Host dbserver
    HostName db.example.com
    Port 5432
    User postgres
`
	r := strings.NewReader(input)
	hosts := ParseSSHConfig(r)

	if len(hosts) != 2 {
		t.Fatalf("got %d hosts, want 2", len(hosts))
	}

	if hosts[0].Alias != "webserver" {
		t.Errorf("hosts[0].Alias = %q, want %q", hosts[0].Alias, "webserver")
	}
	if hosts[0].HostName != "192.168.1.100" {
		t.Errorf("hosts[0].HostName = %q", hosts[0].HostName)
	}
	if hosts[0].Port != "22" {
		t.Errorf("hosts[0].Port = %q", hosts[0].Port)
	}
	if hosts[0].User != "admin" {
		t.Errorf("hosts[0].User = %q", hosts[0].User)
	}
	home, _ := os.UserHomeDir()
	wantKey := filepath.Join(home, ".ssh", "id_rsa")
	if hosts[0].IdentityFile != wantKey {
		t.Errorf("hosts[0].IdentityFile = %q, want %q", hosts[0].IdentityFile, wantKey)
	}

	if hosts[1].Alias != "dbserver" {
		t.Errorf("hosts[1].Alias = %q", hosts[1].Alias)
	}
	if hosts[1].User != "postgres" {
		t.Errorf("hosts[1].User = %q", hosts[1].User)
	}
}

func TestParseSSHConfigSkipsWildcard(t *testing.T) {
	input := `
Host *
    ServerAliveInterval 60

Host myhost
    HostName 10.0.0.1
    User root
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("got %d hosts, want 1 (wildcard should be skipped)", len(hosts))
	}
	if hosts[0].Alias != "myhost" {
		t.Errorf("Alias = %q, want %q", hosts[0].Alias, "myhost")
	}
}

func TestParseSSHConfigComments(t *testing.T) {
	input := `
# This is a comment
Host server1
    # Another comment
    HostName server1.example.com
    User admin
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("got %d hosts, want 1", len(hosts))
	}
	if hosts[0].HostName != "server1.example.com" {
		t.Errorf("HostName = %q", hosts[0].HostName)
	}
}

func TestParseSSHConfigEqualsDelimiter(t *testing.T) {
	input := `
Host equaltest
    HostName=equal.example.com
    Port=2222
    User=equser
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("got %d hosts, want 1", len(hosts))
	}
	if hosts[0].HostName != "equal.example.com" {
		t.Errorf("HostName = %q", hosts[0].HostName)
	}
	if hosts[0].Port != "2222" {
		t.Errorf("Port = %q", hosts[0].Port)
	}
	if hosts[0].User != "equser" {
		t.Errorf("User = %q", hosts[0].User)
	}
}

func TestParseSSHConfigEmpty(t *testing.T) {
	hosts := ParseSSHConfig(strings.NewReader(""))
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts from empty input, got %d", len(hosts))
	}
}

func TestParseSSHConfigNoHostName(t *testing.T) {
	input := `
Host shortname
    User admin
    Port 22
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("got %d hosts, want 1", len(hosts))
	}
	// DisplayHost should fall back to alias.
	if hosts[0].DisplayHost() != "shortname" {
		t.Errorf("DisplayHost() = %q, want %q", hosts[0].DisplayHost(), "shortname")
	}
}

// ---------------------------------------------------------------------------
// LoadSSHConfigFrom
// ---------------------------------------------------------------------------

func TestLoadSSHConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config")
	content := `
Host filetest
    HostName file.example.com
    User fileuser
`
	if err := os.WriteFile(configFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	hosts := LoadSSHConfigFrom(configFile)
	if len(hosts) != 1 {
		t.Fatalf("got %d hosts, want 1", len(hosts))
	}
	if hosts[0].Alias != "filetest" {
		t.Errorf("Alias = %q", hosts[0].Alias)
	}
}

func TestLoadSSHConfigFromMissing(t *testing.T) {
	hosts := LoadSSHConfigFrom("/nonexistent/path/config")
	if hosts != nil {
		t.Errorf("expected nil for missing file, got %v", hosts)
	}
}

// ---------------------------------------------------------------------------
// splitSSHConfigLine
// ---------------------------------------------------------------------------

func TestSplitSSHConfigLineSpace(t *testing.T) {
	key, val := splitSSHConfigLine("HostName example.com")
	if key != "HostName" || val != "example.com" {
		t.Errorf("got (%q, %q)", key, val)
	}
}

func TestSplitSSHConfigLineEquals(t *testing.T) {
	key, val := splitSSHConfigLine("Port=2222")
	if key != "Port" || val != "2222" {
		t.Errorf("got (%q, %q)", key, val)
	}
}

func TestSplitSSHConfigLineTab(t *testing.T) {
	key, val := splitSSHConfigLine("User\tadmin")
	if key != "User" || val != "admin" {
		t.Errorf("got (%q, %q)", key, val)
	}
}

// ---------------------------------------------------------------------------
// isWildcard
// ---------------------------------------------------------------------------

func TestIsWildcard(t *testing.T) {
	tests := []struct {
		alias string
		want  bool
	}{
		{"*", true},
		{"*.example.com", true},
		{"server?", true},
		{"myhost", false},
		{"prod-server", false},
	}
	for _, tt := range tests {
		if got := isWildcard(tt.alias); got != tt.want {
			t.Errorf("isWildcard(%q) = %v, want %v", tt.alias, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// expandTilde
// ---------------------------------------------------------------------------

func TestExpandTilde(t *testing.T) {
	home := "/home/testuser"
	tests := []struct {
		input string
		want  string
	}{
		{"~/foo/bar", filepath.Join(home, "foo", "bar")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}
	for _, tt := range tests {
		got := expandTilde(tt.input, home)
		if got != tt.want {
			t.Errorf("expandTilde(%q, %q) = %q, want %q", tt.input, home, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Multiple Host blocks
// ---------------------------------------------------------------------------

func TestParseSSHConfigMultipleHosts(t *testing.T) {
	input := `
Host alpha
    HostName alpha.example.com
    User alice

Host beta
    HostName beta.example.com
    User bob
    Port 3022
    IdentityFile ~/.ssh/beta_key

Host gamma
    HostName gamma.example.com
    User charlie
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 3 {
		t.Fatalf("got %d hosts, want 3", len(hosts))
	}
	if hosts[1].Port != "3022" {
		t.Errorf("hosts[1].Port = %q, want %q", hosts[1].Port, "3022")
	}
}

// ---------------------------------------------------------------------------
// Case-insensitive directive matching
// ---------------------------------------------------------------------------

func TestParseSSHConfigCaseInsensitive(t *testing.T) {
	input := `
Host casetest
    HOSTNAME uppercase.example.com
    port 9922
    USER mixedUser
    identityfile ~/.ssh/case_key
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("got %d hosts, want 1", len(hosts))
	}
	if hosts[0].HostName != "uppercase.example.com" {
		t.Errorf("HostName = %q", hosts[0].HostName)
	}
	if hosts[0].Port != "9922" {
		t.Errorf("Port = %q", hosts[0].Port)
	}
	if hosts[0].User != "mixedUser" {
		t.Errorf("User = %q", hosts[0].User)
	}
}
