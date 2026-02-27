package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SSHHost represents a single Host block from ~/.ssh/config.
type SSHHost struct {
	Alias                 string // the Host alias (e.g. "myserver")
	HostName              string // HostName directive (actual hostname / IP)
	Port                  string // Port directive (default "22")
	User                  string // User directive
	IdentityFile          string // IdentityFile path (~ expanded)
	HostKeyAlgorithms     string // HostKeyAlgorithms directive (comma-separated)
	PubkeyAcceptedTypes   string // PubkeyAcceptedKeyTypes / PubkeyAcceptedAlgorithms
	StrictHostKeyChecking string // StrictHostKeyChecking (yes/no/ask)
	UserKnownHostsFile    string // UserKnownHostsFile path
	ProxyJump             string // ProxyJump directive (user@host:port)
}

// DisplayHost returns the effective hostname (HostName if set, otherwise Alias).
func (h SSHHost) DisplayHost() string {
	if h.HostName != "" {
		return h.HostName
	}
	return h.Alias
}

// ToConnection converts an SSHHost into a Connection suitable for connecting.
func (h SSHHost) ToConnection() Connection {
	host := h.DisplayHost()
	port := h.Port
	if port == "" {
		port = "22"
	}
	return Connection{
		Name:                  h.Alias,
		Host:                  host,
		Port:                  port,
		Username:              h.User,
		KeyPath:               h.IdentityFile,
		HostKeyAlgorithms:     h.HostKeyAlgorithms,
		PubkeyAcceptedTypes:   h.PubkeyAcceptedTypes,
		StrictHostKeyChecking: h.StrictHostKeyChecking,
		UserKnownHostsFile:    h.UserKnownHostsFile,
		ProxyJump:             h.ProxyJump,
	}
}

// sshConfigPath returns the default SSH config file path.
func sshConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "config")
}

// LoadSSHConfig reads and parses ~/.ssh/config, returning all non-wildcard
// Host entries. If the file doesn't exist or can't be read, an empty slice
// is returned with no error (this is not a fatal condition).
func LoadSSHConfig() []SSHHost {
	return LoadSSHConfigFrom(sshConfigPath())
}

// LoadSSHConfigFrom reads and parses an SSH config file at the given path.
func LoadSSHConfigFrom(path string) []SSHHost {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()

	return ParseSSHConfig(f)
}

// ParseSSHConfig parses SSH config content from a reader.
func ParseSSHConfig(r io.Reader) []SSHHost {
	var hosts []SSHHost
	var current *SSHHost
	var wildcard SSHHost // holds Host * defaults

	home, _ := os.UserHomeDir()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split into key and value. SSH config uses space or = as delimiter.
		key, value := splitSSHConfigLine(line)
		if key == "" {
			continue
		}

		switch strings.ToLower(key) {
		case "host":
			// Finish previous host block.
			if current != nil && !isWildcard(current.Alias) {
				hosts = append(hosts, *current)
			} else if current != nil && isWildcard(current.Alias) {
				wildcard = *current
			}
			current = &SSHHost{Alias: value}

		case "hostname":
			if current != nil {
				current.HostName = value
			}
		case "port":
			if current != nil {
				current.Port = value
			}
		case "user":
			if current != nil {
				current.User = value
			}
		case "identityfile":
			if current != nil {
				current.IdentityFile = expandTilde(value, home)
			}
		case "hostkeyalgorithms":
			if current != nil {
				current.HostKeyAlgorithms = value
			}
		case "pubkeyacceptedkeytypes", "pubkeyacceptedalgorithms":
			if current != nil {
				current.PubkeyAcceptedTypes = value
			}
		case "stricthostkeychecking":
			if current != nil {
				current.StrictHostKeyChecking = value
			}
		case "userknownhostsfile":
			if current != nil {
				current.UserKnownHostsFile = expandTilde(value, home)
			}
		case "proxyjump":
			if current != nil {
				current.ProxyJump = value
			}
		}
	}

	// Don't forget the last host block.
	if current != nil && !isWildcard(current.Alias) {
		hosts = append(hosts, *current)
	} else if current != nil && isWildcard(current.Alias) {
		wildcard = *current
	}

	// Apply Host * defaults to all hosts for empty fields.
	for i := range hosts {
		applyDefaults(&hosts[i], &wildcard)
	}

	// Filter out hosts without a User — they can't be used for connections.
	filtered := hosts[:0]
	for _, h := range hosts {
		if h.User != "" {
			filtered = append(filtered, h)
		}
	}

	return filtered
}

// applyDefaults fills empty fields in dst from the wildcard defaults.
func applyDefaults(dst, defaults *SSHHost) {
	if dst.Port == "" && defaults.Port != "" {
		dst.Port = defaults.Port
	}
	if dst.User == "" && defaults.User != "" {
		dst.User = defaults.User
	}
	if dst.IdentityFile == "" && defaults.IdentityFile != "" {
		dst.IdentityFile = defaults.IdentityFile
	}
	if dst.HostKeyAlgorithms == "" && defaults.HostKeyAlgorithms != "" {
		dst.HostKeyAlgorithms = defaults.HostKeyAlgorithms
	}
	if dst.PubkeyAcceptedTypes == "" && defaults.PubkeyAcceptedTypes != "" {
		dst.PubkeyAcceptedTypes = defaults.PubkeyAcceptedTypes
	}
	if dst.StrictHostKeyChecking == "" && defaults.StrictHostKeyChecking != "" {
		dst.StrictHostKeyChecking = defaults.StrictHostKeyChecking
	}
	if dst.UserKnownHostsFile == "" && defaults.UserKnownHostsFile != "" {
		dst.UserKnownHostsFile = defaults.UserKnownHostsFile
	}
	if dst.ProxyJump == "" && defaults.ProxyJump != "" {
		dst.ProxyJump = defaults.ProxyJump
	}
}

// MatchSSHHost finds the first SSHHost whose Alias or HostName matches the
// given hostname, and returns a merged copy with wildcard defaults applied.
// Returns nil if no match is found.
func MatchSSHHost(hosts []SSHHost, hostname string) *SSHHost {
	for i := range hosts {
		if hosts[i].Alias == hostname || hosts[i].HostName == hostname {
			h := hosts[i]
			return &h
		}
	}
	return nil
}

// splitSSHConfigLine splits a line like "HostName example.com" or
// "HostName=example.com" into key and value.
func splitSSHConfigLine(line string) (string, string) {
	// Handle "Key=Value" form.
	if idx := strings.IndexByte(line, '='); idx >= 0 {
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		return key, val
	}

	// Handle "Key Value" form — split on first whitespace.
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 {
		parts = strings.SplitN(line, "\t", 2)
	}
	if len(parts) < 2 {
		return parts[0], ""
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

// isWildcard returns true if the host alias contains glob characters.
func isWildcard(alias string) bool {
	return strings.ContainsAny(alias, "*?")
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path, home string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		return home
	}
	return path
}
