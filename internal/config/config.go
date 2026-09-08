package config

import (
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/toml"

	"github.com/k16em/tsundoku/internal/filter"
)

// ListConfig holds defaults for the list command.
type ListConfig struct {
	Limit   int    `toml:"limit"`
	Sort    string `toml:"sort"`
	Reverse bool   `toml:"reverse"`
}

// TagsConfig holds defaults for the tag list command.
type TagsConfig struct {
	Sort    string `toml:"sort"`
	Reverse bool   `toml:"reverse"`
}

// Config is the parsed config file.
type Config struct {
	List   ListConfig      `toml:"list"`
	Tags   TagsConfig      `toml:"tags"`
	Filter []filter.Filter `toml:"filter"`

	Compiled []filter.Compiled `toml:"-"`
}

var listSorts = map[string]bool{"created": true, "id": true}

var tagsSorts = map[string]bool{"name": true, "count": true}

// Default returns the built-in defaults used when no config file exists.
func Default() Config {
	return Config{
		List: ListConfig{Limit: 20, Sort: "created", Reverse: false},
		Tags: TagsConfig{Sort: "name", Reverse: false},
	}
}

// Load reads and validates the config file. When explicit is true a missing
// file is an error. Warnings are written to warn.
func Load(path string, explicit bool, warn io.Writer) (Config, error) {
	cfg := Default()

	_, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		if os.IsNotExist(err) {
			if explicit {
				return Config{}, fmt.Errorf("config: %w", err)
			}
			return Default(), nil
		}
		return Config{}, fmt.Errorf("config: %w", err)
	}

	var warnFn func(string)
	if warn != nil {
		warnFn = func(msg string) { fmt.Fprintln(warn, msg) }
	}

	compiled, err := filter.Compile(cfg.Filter, warnFn)
	if err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if len(compiled) > 0 {
		cfg.Compiled = compiled
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validate(cfg Config) error {
	if cfg.List.Limit < 1 || cfg.List.Limit > 1000 {
		return fmt.Errorf("config: list.limit must be between 1 and 1000, got %d", cfg.List.Limit)
	}
	if !listSorts[cfg.List.Sort] {
		return fmt.Errorf("config: list.sort must be one of created, id; got %q", cfg.List.Sort)
	}
	if !tagsSorts[cfg.Tags.Sort] {
		return fmt.Errorf("config: tags.sort must be one of name, count; got %q", cfg.Tags.Sort)
	}
	return nil
}

// Template returns the starter config file written by `tsundoku init`.
func Template() string {
	return `[list]
limit   = 20
sort    = "created"      # created | id
reverse = false

[tags]
sort    = "name"         # name | count
reverse = false

# [[filter]]
# name = "blog"
# rule = "*blog*"

# [[filter]]
# name = "news"
# regex = "^https://news\\."
`
}
