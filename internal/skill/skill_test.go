package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func destIn(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), ".agents", "skills", "tsundoku", "SKILL.md")
}

func TestDocHasFrontmatterAndCommands(t *testing.T) {
	if !strings.Contains(Doc, "name: tsundoku") {
		t.Errorf("Doc is missing the skill frontmatter name")
	}
	if !strings.Contains(Doc, "tsundoku add") {
		t.Errorf("Doc does not document the commands")
	}
}

func TestInstallWritesDocAndCreatesDirectories(t *testing.T) {
	dest := destIn(t)

	if err := Install(dest); err != nil {
		t.Fatalf("Install(%q) error = %v", dest, err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", dest, err)
	}
	if string(got) != Doc {
		t.Errorf("installed content does not match Doc")
	}
}

func TestInstallOverwritesExistingFile(t *testing.T) {
	dest := destIn(t)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}
	if err := os.WriteFile(dest, []byte("stale"), 0o644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	if err := Install(dest); err != nil {
		t.Fatalf("Install(%q) error = %v", dest, err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", dest, err)
	}
	if string(got) == "stale" {
		t.Errorf("Install did not overwrite the existing file")
	}
}

func TestUninstallRemovesFileAndEmptyDirectory(t *testing.T) {
	dest := destIn(t)
	if err := Install(dest); err != nil {
		t.Fatalf("Install(%q) error = %v", dest, err)
	}

	removed, err := Uninstall(dest)

	if err != nil {
		t.Fatalf("Uninstall(%q) error = %v", dest, err)
	}
	if !removed {
		t.Errorf("removed = false, want true")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("skill file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(dest)); !os.IsNotExist(err) {
		t.Errorf("skill directory still exists: %v", err)
	}
}

func TestUninstallIsNoopWhenAbsent(t *testing.T) {
	dest := destIn(t)

	removed, err := Uninstall(dest)

	if err != nil {
		t.Fatalf("Uninstall(%q) error = %v", dest, err)
	}
	if removed {
		t.Errorf("removed = true, want false")
	}
}
