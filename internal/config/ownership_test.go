package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixOwnership_NoopWhenNotRoot(t *testing.T) {
	// When running tests as a non-root user FixOwnership should silently
	// do nothing (no panic, no error).
	dir := t.TempDir()
	p := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(p, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	// Should not panic or fail regardless of uid.
	FixOwnership(p)

	// File should still be readable.
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("unexpected content: %q", data)
	}
}

func TestFixOwnership_NonexistentPath(t *testing.T) {
	// Should not panic on a path that doesn't exist.
	FixOwnership("/tmp/nonexistent-ssh-scp-test-path-12345")
}
