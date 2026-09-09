package skill

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// Doc is the agent skill document, embedded at build time.
//
//go:embed SKILL.md
var Doc string

// Install writes Doc to dest, creating parent directories as needed. An
// existing file is overwritten so re-running installs the current version.
func Install(dest string) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("skill: create %s: %w", dir, err)
	}
	if err := os.WriteFile(dest, []byte(Doc), 0o644); err != nil {
		return fmt.Errorf("skill: write %s: %w", dest, err)
	}
	return nil
}

// Uninstall removes the installed skill file. It reports whether a file was
// removed, and cleans up the now-empty directory when it can.
func Uninstall(dest string) (bool, error) {
	if err := os.Remove(dest); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("skill: remove %s: %w", dest, err)
	}
	os.Remove(filepath.Dir(dest))
	return true, nil
}
