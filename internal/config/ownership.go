package config

import (
	"os"
	"path/filepath"
	"syscall"
)

// FixOwnership adjusts the ownership of path (and its parent directories up to
// the user's home directory) so they are owned by the home directory's owner
// rather than root.  This handles the common dev-container scenario where the
// process runs as uid 0 but the home directory belongs to a regular user.
//
// It is a no-op when the process is not running as root or when the home
// directory is already owned by root.
func FixOwnership(path string) {
	if os.Getuid() != 0 {
		return
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return
	}

	info, err := os.Stat(home)
	if err != nil {
		return
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	uid := int(stat.Uid)
	gid := int(stat.Gid)
	if uid == 0 {
		// Home dir is legitimately owned by root; nothing to fix.
		return
	}

	// Chown the target path itself.
	_ = os.Lchown(path, uid, gid)

	// Walk upward and chown intermediate directories that we may have created,
	// stopping at (and including) the first directory that already has the
	// correct owner or once we leave the home directory.
	dir := filepath.Dir(path)
	for {
		rel, err := filepath.Rel(home, dir)
		if err != nil || rel == "." || rel == ".." || len(rel) > 0 && rel[0] == '.' && rel[1] == '.' {
			break
		}
		fi, err := os.Stat(dir)
		if err != nil {
			break
		}
		ds, ok := fi.Sys().(*syscall.Stat_t)
		if !ok {
			break
		}
		if int(ds.Uid) == uid {
			break // already correct from here upward
		}
		_ = os.Lchown(dir, uid, gid)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}
