package paths

import (
	"strings"
	"testing"
)

func TestConfigFileUsesXDGConfigHomeWhenSet(t *testing.T) {
	env := Env{XDGConfigHome: "/x"}

	got, err := ConfigFile(env, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/x/tsundoku/config.toml"
	if got != want {
		t.Errorf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestConfigFileFallsBackToHomeWhenXDGConfigHomeUnset(t *testing.T) {
	env := Env{Home: "/home/u"}

	got, err := ConfigFile(env, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/home/u/.config/tsundoku/config.toml"
	if got != want {
		t.Errorf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestConfigFileTreatsEmptyXDGConfigHomeAsUnset(t *testing.T) {
	env := Env{XDGConfigHome: "", Home: "/home/u"}

	got, err := ConfigFile(env, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/home/u/.config/tsundoku/config.toml"
	if got != want {
		t.Errorf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestConfigFileReturnsOverrideVerbatim(t *testing.T) {
	env := Env{XDGConfigHome: "/x", Home: "/home/u"}

	got, err := ConfigFile(env, "/custom/path/config.toml")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/custom/path/config.toml"
	if got != want {
		t.Errorf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestConfigFileErrorsWhenNeitherXDGConfigHomeNorHomeSet(t *testing.T) {
	env := Env{}

	_, err := ConfigFile(env, "")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConfigFileDoesNotRejectPathContainingQuestionMark(t *testing.T) {
	env := Env{XDGConfigHome: "/x?y"}

	got, err := ConfigFile(env, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/x?y/tsundoku/config.toml"
	if got != want {
		t.Errorf("ConfigFile() = %q, want %q", got, want)
	}
}

func TestDBFileUsesXDGDataHomeWhenSet(t *testing.T) {
	env := Env{XDGDataHome: "/d"}

	got, err := DBFile(env, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/d/tsundoku/tsundoku.db"
	if got != want {
		t.Errorf("DBFile() = %q, want %q", got, want)
	}
}

func TestDBFileFallsBackToHomeWhenXDGDataHomeUnset(t *testing.T) {
	env := Env{Home: "/home/u"}

	got, err := DBFile(env, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/home/u/.local/share/tsundoku/tsundoku.db"
	if got != want {
		t.Errorf("DBFile() = %q, want %q", got, want)
	}
}

func TestDBFileTreatsEmptyXDGDataHomeAsUnset(t *testing.T) {
	env := Env{XDGDataHome: "", Home: "/home/u"}

	got, err := DBFile(env, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/home/u/.local/share/tsundoku/tsundoku.db"
	if got != want {
		t.Errorf("DBFile() = %q, want %q", got, want)
	}
}

func TestDBFileReturnsOverrideVerbatim(t *testing.T) {
	env := Env{XDGDataHome: "/d", Home: "/home/u"}

	got, err := DBFile(env, "/custom/path/tsundoku.db")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/custom/path/tsundoku.db"
	if got != want {
		t.Errorf("DBFile() = %q, want %q", got, want)
	}
}

func TestDBFileErrorsWhenNeitherXDGDataHomeNorHomeSet(t *testing.T) {
	env := Env{}

	_, err := DBFile(env, "")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDBFileRejectsOverrideContainingQuestionMark(t *testing.T) {
	env := Env{XDGDataHome: "/d"}

	_, err := DBFile(env, "/tmp/a?b.db")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "?") {
		t.Errorf("error message should mention '?': %v", err)
	}
}

func TestDBFileRejectsDefaultPathDerivedFromHomeContainingQuestionMark(t *testing.T) {
	env := Env{Home: "/home/u?ser"}

	_, err := DBFile(env, "")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDBFileRejectsDefaultPathDerivedFromXDGDataHomeContainingQuestionMark(t *testing.T) {
	env := Env{XDGDataHome: "/d?ata"}

	_, err := DBFile(env, "")

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDBFileAllowsPathContainingHash(t *testing.T) {
	env := Env{XDGDataHome: "/d#ata"}

	got, err := DBFile(env, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/d#ata/tsundoku/tsundoku.db"
	if got != want {
		t.Errorf("DBFile() = %q, want %q", got, want)
	}
}

func TestDBFileAllowsOverrideContainingHash(t *testing.T) {
	env := Env{}

	got, err := DBFile(env, "/tmp/a#b.db")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/tmp/a#b.db"
	if got != want {
		t.Errorf("DBFile() = %q, want %q", got, want)
	}
}

func TestOSEnvReadsFromProcessEnvironment(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/env-config")
	t.Setenv("XDG_DATA_HOME", "/env-data")
	t.Setenv("HOME", "/env-home")

	got := OSEnv()

	want := Env{XDGConfigHome: "/env-config", XDGDataHome: "/env-data", Home: "/env-home"}
	if got != want {
		t.Errorf("OSEnv() = %+v, want %+v", got, want)
	}
}
