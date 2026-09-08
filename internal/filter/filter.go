package filter

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Filter is one [[filter]] entry from the config file.
type Filter struct {
	Name  string `toml:"name"`
	Rule  string `toml:"rule"`
	Regex string `toml:"regex"`
}

// Compiled is a Filter with its pattern compiled.
type Compiled struct {
	Name string
	Re   *regexp.Regexp
}

// Compile turns config entries into matchable filters.
// warn receives a line for each entry where both rule and regex are set.
func Compile(fs []Filter, warn func(string)) ([]Compiled, error) {
	cs := make([]Compiled, 0, len(fs))
	for i, f := range fs {
		idx := i + 1
		trimmed := strings.TrimSpace(f.Name)
		if trimmed == "" {
			return nil, fmt.Errorf("filter #%d (%q): name is required", idx, f.Name)
		}
		for _, r := range trimmed {
			if unicode.IsControl(r) {
				return nil, fmt.Errorf("filter #%d (%q): name contains control characters", idx, f.Name)
			}
		}
		name := strings.ToLower(trimmed)

		hasRule := f.Rule != ""
		hasRegex := f.Regex != ""
		if !hasRule && !hasRegex {
			return nil, fmt.Errorf("filter #%d (%q): rule and regex are both empty", idx, name)
		}
		if hasRule && hasRegex && warn != nil {
			warn(fmt.Sprintf("filter #%d (%q): both rule and regex set, using rule", idx, name))
		}

		var re *regexp.Regexp
		var err error
		if hasRule {
			re, err = GlobToRegexp(f.Rule)
			if err != nil {
				return nil, fmt.Errorf("filter #%d (%q): invalid rule: %w", idx, name, err)
			}
		} else {
			re, err = regexp.Compile(f.Regex)
			if err != nil {
				return nil, fmt.Errorf("filter #%d (%q): invalid regex: %w", idx, name, err)
			}
		}

		cs = append(cs, Compiled{Name: name, Re: re})
	}
	return cs, nil
}

// Match returns the tag names whose filter matches url, in config order, deduplicated.
func Match(cs []Compiled, url string) []string {
	tags := []string{}
	seen := make(map[string]bool, len(cs))
	for _, c := range cs {
		if seen[c.Name] {
			continue
		}
		if c.Re.MatchString(url) {
			seen[c.Name] = true
			tags = append(tags, c.Name)
		}
	}
	return tags
}

// GlobToRegexp converts a glob-style rule into a fully anchored regexp.
func GlobToRegexp(glob string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range glob {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}
