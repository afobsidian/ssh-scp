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
	Alias        string // the Host alias (e.g. "myserver")
	HostName     string // HostName directive (actual hostname / IP)
	Port         string // Port directive (default "22")
	User         string // User directive
	IdentityFile string // IdentityFile path (~ expanded)
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
		Name:     h.Alias,
		Host:     host,
		Port:     port,
		Username: h.User,
		KeyPath:  h.IdentityFile,
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
		}
	}

	// Don't forget the last host block.
	if current != nil && !isWildcard(current.Alias) {
		hosts = append(hosts, *current)
	}

	return hosts
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
