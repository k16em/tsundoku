package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Env holds the environment variables that affect path resolution.
type Env struct {
	XDGConfigHome string
	XDGDataHome   string
	Home          string
}

// OSEnv reads Env from the process environment.
func OSEnv() Env {
	return Env{
		XDGConfigHome: os.Getenv("XDG_CONFIG_HOME"),
		XDGDataHome:   os.Getenv("XDG_DATA_HOME"),
		Home:          os.Getenv("HOME"),
	}
}

// ConfigFile resolves the config file path. override wins when non-empty.
func ConfigFile(env Env, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env.XDGConfigHome != "" {
		return filepath.Join(env.XDGConfigHome, "tsundoku", "config.toml"), nil
	}
	if env.Home != "" {
		return filepath.Join(env.Home, ".config", "tsundoku", "config.toml"), nil
	}
	return "", fmt.Errorf("cannot resolve config path: neither XDG_CONFIG_HOME nor HOME is set")
}

// DBFile resolves the database file path. override wins when non-empty.
func DBFile(env Env, override string) (string, error) {
	path, err := dbFilePath(env, override)
	if err != nil {
		return "", err
	}
	if strings.Contains(path, "?") {
		return "", fmt.Errorf("database path must not contain '?': %s", path)
	}
	return path, nil
}

func dbFilePath(env Env, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if env.XDGDataHome != "" {
		return filepath.Join(env.XDGDataHome, "tsundoku", "tsundoku.db"), nil
	}
	if env.Home != "" {
		return filepath.Join(env.Home, ".local", "share", "tsundoku", "tsundoku.db"), nil
	}
	return "", fmt.Errorf("cannot resolve database path: neither XDG_DATA_HOME nor HOME is set")
}

// SkillFile resolves the install path of the agent skill document.
func SkillFile(env Env) (string, error) {
	if env.Home == "" {
		return "", fmt.Errorf("cannot resolve skill path: HOME is not set")
	}
	return filepath.Join(env.Home, ".agents", "skills", "tsundoku", "SKILL.md"), nil
}
