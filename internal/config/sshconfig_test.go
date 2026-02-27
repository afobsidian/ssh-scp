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

func TestParseSSHConfigProxyJump(t *testing.T) {
	input := `
Host target
  HostName 10.0.1.5
  User deploy
  ProxyJump bastion.example.com

Host bastion
  HostName bastion.example.com
  Port 2222
  User admin
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].ProxyJump != "bastion.example.com" {
		t.Errorf("ProxyJump = %q, want %q", hosts[0].ProxyJump, "bastion.example.com")
	}
	c := hosts[0].ToConnection()
	if c.ProxyJump != "bastion.example.com" {
		t.Errorf("Connection.ProxyJump = %q, want %q", c.ProxyJump, "bastion.example.com")
	}
}

func TestProxyJumpDefaults(t *testing.T) {
	input := `
Host *
  ProxyJump default-bastion.example.com

Host myserver
  HostName myserver.example.com
  User admin
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].ProxyJump != "default-bastion.example.com" {
		t.Errorf("ProxyJump default not applied: got %q", hosts[0].ProxyJump)
	}
}

// ---------------------------------------------------------------------------
// MatchSSHHost
// ---------------------------------------------------------------------------

func TestMatchSSHHostByAlias(t *testing.T) {
	hosts := []SSHHost{
		{Alias: "prod", HostName: "prod.example.com", User: "deploy"},
		{Alias: "staging", HostName: "staging.example.com", User: "dev"},
	}
	match := MatchSSHHost(hosts, "prod")
	if match == nil {
		t.Fatal("expected match for alias 'prod'")
	}
	if match.HostName != "prod.example.com" {
		t.Errorf("HostName = %q, want %q", match.HostName, "prod.example.com")
	}
}

func TestMatchSSHHostByHostName(t *testing.T) {
	hosts := []SSHHost{
		{Alias: "prod", HostName: "prod.example.com", User: "deploy"},
	}
	match := MatchSSHHost(hosts, "prod.example.com")
	if match == nil {
		t.Fatal("expected match for hostname")
	}
	if match.Alias != "prod" {
		t.Errorf("Alias = %q, want %q", match.Alias, "prod")
	}
}

func TestMatchSSHHostNoMatch(t *testing.T) {
	hosts := []SSHHost{
		{Alias: "prod", HostName: "prod.example.com"},
	}
	match := MatchSSHHost(hosts, "nope")
	if match != nil {
		t.Errorf("expected nil, got %+v", match)
	}
}

func TestMatchSSHHostEmptyList(t *testing.T) {
	match := MatchSSHHost(nil, "anything")
	if match != nil {
		t.Errorf("expected nil for empty list, got %+v", match)
	}
}

// ---------------------------------------------------------------------------
// applyDefaults — full wildcard coverage
// ---------------------------------------------------------------------------

func TestApplyDefaultsAllFields(t *testing.T) {
	defaults := &SSHHost{
		Port:                  "2222",
		User:                  "defaultuser",
		IdentityFile:          "/default/key",
		HostKeyAlgorithms:     "ssh-ed25519",
		PubkeyAcceptedTypes:   "ssh-ed25519",
		StrictHostKeyChecking: "no",
		UserKnownHostsFile:    "/dev/null",
		ProxyJump:             "bastion.example.com",
	}
	dst := &SSHHost{Alias: "test"}
	applyDefaults(dst, defaults)

	if dst.Port != "2222" {
		t.Errorf("Port = %q, want %q", dst.Port, "2222")
	}
	if dst.User != "defaultuser" {
		t.Errorf("User = %q, want %q", dst.User, "defaultuser")
	}
	if dst.IdentityFile != "/default/key" {
		t.Errorf("IdentityFile = %q", dst.IdentityFile)
	}
	if dst.HostKeyAlgorithms != "ssh-ed25519" {
		t.Errorf("HostKeyAlgorithms = %q", dst.HostKeyAlgorithms)
	}
	if dst.PubkeyAcceptedTypes != "ssh-ed25519" {
		t.Errorf("PubkeyAcceptedTypes = %q", dst.PubkeyAcceptedTypes)
	}
	if dst.StrictHostKeyChecking != "no" {
		t.Errorf("StrictHostKeyChecking = %q", dst.StrictHostKeyChecking)
	}
	if dst.UserKnownHostsFile != "/dev/null" {
		t.Errorf("UserKnownHostsFile = %q", dst.UserKnownHostsFile)
	}
	if dst.ProxyJump != "bastion.example.com" {
		t.Errorf("ProxyJump = %q", dst.ProxyJump)
	}
}

func TestApplyDefaultsDoesNotOverwrite(t *testing.T) {
	defaults := &SSHHost{
		Port:                  "2222",
		User:                  "defaultuser",
		IdentityFile:          "/default/key",
		HostKeyAlgorithms:     "ssh-ed25519",
		PubkeyAcceptedTypes:   "ssh-ed25519",
		StrictHostKeyChecking: "no",
		UserKnownHostsFile:    "/dev/null",
		ProxyJump:             "default-bastion",
	}
	dst := &SSHHost{
		Alias:                 "test",
		Port:                  "3333",
		User:                  "myuser",
		IdentityFile:          "/my/key",
		HostKeyAlgorithms:     "rsa-sha2-256",
		PubkeyAcceptedTypes:   "rsa-sha2-256",
		StrictHostKeyChecking: "yes",
		UserKnownHostsFile:    "/my/known_hosts",
		ProxyJump:             "my-bastion",
	}
	applyDefaults(dst, defaults)

	if dst.Port != "3333" {
		t.Errorf("Port overwritten: %q", dst.Port)
	}
	if dst.User != "myuser" {
		t.Errorf("User overwritten: %q", dst.User)
	}
	if dst.IdentityFile != "/my/key" {
		t.Errorf("IdentityFile overwritten: %q", dst.IdentityFile)
	}
	if dst.HostKeyAlgorithms != "rsa-sha2-256" {
		t.Errorf("HostKeyAlgorithms overwritten: %q", dst.HostKeyAlgorithms)
	}
	if dst.PubkeyAcceptedTypes != "rsa-sha2-256" {
		t.Errorf("PubkeyAcceptedTypes overwritten: %q", dst.PubkeyAcceptedTypes)
	}
	if dst.StrictHostKeyChecking != "yes" {
		t.Errorf("StrictHostKeyChecking overwritten: %q", dst.StrictHostKeyChecking)
	}
	if dst.UserKnownHostsFile != "/my/known_hosts" {
		t.Errorf("UserKnownHostsFile overwritten: %q", dst.UserKnownHostsFile)
	}
	if dst.ProxyJump != "my-bastion" {
		t.Errorf("ProxyJump overwritten: %q", dst.ProxyJump)
	}
}

// ---------------------------------------------------------------------------
// ParseSSHConfig — HostKeyAlgorithms, PubkeyAcceptedAlgorithms, StrictHostKeyChecking, UserKnownHostsFile
// ---------------------------------------------------------------------------

func TestParseSSHConfigAllDirectives(t *testing.T) {
	input := `
Host fulltest
    HostName full.example.com
    Port 2222
    User admin
    IdentityFile ~/.ssh/id_ed25519
    HostKeyAlgorithms ssh-ed25519,rsa-sha2-256
    PubkeyAcceptedAlgorithms ssh-ed25519
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    ProxyJump bastion.example.com
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	h := hosts[0]
	if h.HostKeyAlgorithms != "ssh-ed25519,rsa-sha2-256" {
		t.Errorf("HostKeyAlgorithms = %q", h.HostKeyAlgorithms)
	}
	if h.PubkeyAcceptedTypes != "ssh-ed25519" {
		t.Errorf("PubkeyAcceptedTypes = %q", h.PubkeyAcceptedTypes)
	}
	if h.StrictHostKeyChecking != "no" {
		t.Errorf("StrictHostKeyChecking = %q", h.StrictHostKeyChecking)
	}
	if h.UserKnownHostsFile != "/dev/null" {
		t.Errorf("UserKnownHostsFile = %q", h.UserKnownHostsFile)
	}
}

func TestParseSSHConfigPubkeyAcceptedKeyTypes(t *testing.T) {
	input := `
Host keytype
    HostName keytype.example.com
    User deploy
    PubkeyAcceptedKeyTypes ssh-rsa,ssh-ed25519
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].PubkeyAcceptedTypes != "ssh-rsa,ssh-ed25519" {
		t.Errorf("PubkeyAcceptedTypes = %q", hosts[0].PubkeyAcceptedTypes)
	}
}

// ---------------------------------------------------------------------------
// splitSSHConfigLine — single word (no value)
// ---------------------------------------------------------------------------

func TestSplitSSHConfigLineSingleWord(t *testing.T) {
	key, val := splitSSHConfigLine("JustAKey")
	if key != "JustAKey" {
		t.Errorf("key = %q, want %q", key, "JustAKey")
	}
	if val != "" {
		t.Errorf("val = %q, want empty", val)
	}
}

// ---------------------------------------------------------------------------
// ParseSSHConfig — wildcard at end of file
// ---------------------------------------------------------------------------

func TestParseSSHConfigWildcardAtEnd(t *testing.T) {
	input := `
Host myhost
    HostName myhost.example.com
    User admin

Host *
    Port 2222
    User wildcard
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	// Wildcard defaults should be applied
	if hosts[0].Port != "2222" {
		t.Errorf("Port = %q, want %q (from wildcard)", hosts[0].Port, "2222")
	}
}

// ---------------------------------------------------------------------------
// ToConnection with all fields
// ---------------------------------------------------------------------------

func TestToConnectionAllFields(t *testing.T) {
	h := SSHHost{
		Alias:                 "full",
		HostName:              "full.example.com",
		Port:                  "3022",
		User:                  "admin",
		IdentityFile:          "/key",
		HostKeyAlgorithms:     "ssh-ed25519",
		PubkeyAcceptedTypes:   "ssh-ed25519",
		StrictHostKeyChecking: "yes",
		UserKnownHostsFile:    "/known",
		ProxyJump:             "bastion",
	}
	c := h.ToConnection()
	if c.HostKeyAlgorithms != "ssh-ed25519" {
		t.Errorf("HostKeyAlgorithms = %q", c.HostKeyAlgorithms)
	}
	if c.PubkeyAcceptedTypes != "ssh-ed25519" {
		t.Errorf("PubkeyAcceptedTypes = %q", c.PubkeyAcceptedTypes)
	}
	if c.StrictHostKeyChecking != "yes" {
		t.Errorf("StrictHostKeyChecking = %q", c.StrictHostKeyChecking)
	}
	if c.UserKnownHostsFile != "/known" {
		t.Errorf("UserKnownHostsFile = %q", c.UserKnownHostsFile)
	}
	if c.ProxyJump != "bastion" {
		t.Errorf("ProxyJump = %q", c.ProxyJump)
	}
}

// ---------------------------------------------------------------------------
// ParseSSHConfig — hosts without User are filtered out
// ---------------------------------------------------------------------------

func TestParseSSHConfigFiltersNoUser(t *testing.T) {
	input := `
Host withuser
    HostName 10.0.0.1
    User admin

Host nouser
    HostName 10.0.0.2
`
	hosts := ParseSSHConfig(strings.NewReader(input))
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host (nouser filtered), got %d", len(hosts))
	}
	if hosts[0].Alias != "withuser" {
		t.Errorf("remaining host = %q, want %q", hosts[0].Alias, "withuser")
	}
}
