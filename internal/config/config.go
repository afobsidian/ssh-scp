package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Connection represents a saved SSH connection.
type Connection struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	KeyPath  string `json:"key_path,omitempty"`
}

// Config holds application configuration.
type Config struct {
	RecentConnections []Connection `json:"recent_connections"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ssh-scp", "connections.json")
}

// Load reads the config from disk.
func Load() (*Config, error) {
	p := configPath()
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{}, nil
	}
	return &cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// AddRecent adds or updates a connection in the recent list.
func (c *Config) AddRecent(conn Connection) {
	for i, rc := range c.RecentConnections {
		if rc.Host == conn.Host && rc.Port == conn.Port && rc.Username == conn.Username {
			c.RecentConnections[i] = conn
			return
		}
	}
	c.RecentConnections = append([]Connection{conn}, c.RecentConnections...)
	if len(c.RecentConnections) > 10 {
		c.RecentConnections = c.RecentConnections[:10]
	}
}
