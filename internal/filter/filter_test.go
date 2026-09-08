package filter

import (
	"strings"
	"testing"
)

func TestGlobToRegexp(t *testing.T) {
	tests := []struct {
		name      string
		glob      string
		matches   []string
		noMatches []string
	}{
		{
			name:    "matches substring wildcards",
			glob:    "*blog*",
			matches: []string{"https://example.com/blog/a"},
		},
		{
			name:      "anchors prefix match",
			glob:      "https://example.com/*",
			matches:   []string{"https://example.com/x"},
			noMatches: []string{"https://example.org/x"},
		},
		{
			name:    "matches suffix",
			glob:    "*.pdf",
			matches: []string{"https://example.com/report.pdf"},
		},
		{
			name:      "exact match does not match longer URL",
			glob:      "https://example.com/",
			matches:   []string{"https://example.com/"},
			noMatches: []string{"https://example.com/a"},
		},
		{
			name:      "question mark matches single character",
			glob:      "https://example.com/?",
			matches:   []string{"https://example.com/a"},
			noMatches: []string{"https://example.com/ab"},
		},
		{
			name:      "treats dot as literal",
			glob:      "https://example.com",
			noMatches: []string{"https://exampleXcom"},
		},
		{
			name:      "treats plus and paren as literal",
			glob:      "a+(b)",
			matches:   []string{"a+(b)"},
			noMatches: []string{"aaab"},
		},
		{
			name:    "wildcard crosses slash",
			glob:    "https://*/a",
			matches: []string{"https://x.example.com/deep/a"},
		},
		{
			name:      "treats brackets as literal",
			glob:      "[a-z]",
			matches:   []string{"[a-z]"},
			noMatches: []string{"b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			re, err := GlobToRegexp(tc.glob)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, url := range tc.matches {
				if !re.MatchString(url) {
					t.Errorf("expected %q to match glob %q", url, tc.glob)
				}
			}
			for _, url := range tc.noMatches {
				if re.MatchString(url) {
					t.Errorf("expected %q not to match glob %q", url, tc.glob)
				}
			}
		})
	}
}

func TestCompile(t *testing.T) {
	tests := []struct {
		name         string
		fs           []Filter
		useWarn      bool
		wantWarnings int
		wantErr      bool
		errContains  []string
		wantName     string
		matches      []string
		noMatches    []string
	}{
		{
			name:         "prefers rule over regex and warns once",
			fs:           []Filter{{Name: "blog", Rule: "*blog*", Regex: "^https://news\\."}},
			useWarn:      true,
			wantWarnings: 1,
			matches:      []string{"https://example.com/blog/a"},
			noMatches:    []string{"https://news.example.com/"},
		},
		{
			name: "does not panic when warn is nil",
			fs:   []Filter{{Name: "blog", Rule: "*blog*", Regex: "^https://news\\."}},
		},
		{
			name:        "errors when rule and regex are both empty",
			fs:          []Filter{{Name: "blog"}},
			wantErr:     true,
			errContains: []string{`#1 ("blog")`},
		},
		{
			name:        "errors on invalid regex",
			fs:          []Filter{{Name: "news", Regex: "^("}},
			wantErr:     true,
			errContains: []string{`#1 ("news")`},
		},
		{
			name:        "errors on empty name",
			fs:          []Filter{{Name: "", Rule: "*blog*"}},
			wantErr:     true,
			errContains: []string{"#1", `""`},
		},
		{
			name:        "errors on whitespace-only name",
			fs:          []Filter{{Name: "   ", Rule: "*blog*"}},
			wantErr:     true,
			errContains: []string{"#1", `"   "`},
		},
		{
			name:        "errors on name containing control characters",
			fs:          []Filter{{Name: "a\x1bb", Rule: "*"}},
			wantErr:     true,
			errContains: []string{"#1"},
		},
		{
			name:     "trims and lowercases name",
			fs:       []Filter{{Name: "  Blog ", Rule: "*blog*"}},
			wantName: "blog",
		},
		{
			name:    "regex is not anchored",
			fs:      []Filter{{Name: "blog", Regex: "blog"}},
			matches: []string{"https://example.com/blog/a"},
		},
		{
			name:      "regex with caret anchors prefix",
			fs:        []Filter{{Name: "news", Regex: "^https://news\\."}},
			matches:   []string{"https://news.example.com/a"},
			noMatches: []string{"https://example.com/https://news.example.com/"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var warnings []string
			var warn func(string)
			if tc.useWarn {
				warn = func(msg string) { warnings = append(warnings, msg) }
			}

			cs, err := Compile(tc.fs, warn)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error")
				}
				for _, sub := range tc.errContains {
					if !strings.Contains(err.Error(), sub) {
						t.Errorf("expected error to contain %q, got: %v", sub, err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(cs) != len(tc.fs) {
				t.Fatalf("expected %d compiled filter(s), got %d", len(tc.fs), len(cs))
			}
			if tc.useWarn && len(warnings) != tc.wantWarnings {
				t.Fatalf("expected %d warning(s), got %d: %v", tc.wantWarnings, len(warnings), warnings)
			}
			if tc.wantName != "" && cs[0].Name != tc.wantName {
				t.Errorf("expected name %q, got %q", tc.wantName, cs[0].Name)
			}
			for _, url := range tc.matches {
				if !cs[0].Re.MatchString(url) {
					t.Errorf("expected %q to match", url)
				}
			}
			for _, url := range tc.noMatches {
				if cs[0].Re.MatchString(url) {
					t.Errorf("expected %q not to match", url)
				}
			}
		})
	}
}

func TestMatch(t *testing.T) {
	mustCompile := func(t *testing.T, fs []Filter) []Compiled {
		t.Helper()
		cs, err := Compile(fs, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return cs
	}

	tests := []struct {
		name string
		cs   func(t *testing.T) []Compiled
		url  string
		want []string
	}{
		{
			name: "deduplicates tags from multiple filters with same name",
			cs: func(t *testing.T) []Compiled {
				return mustCompile(t, []Filter{
					{Name: "blog", Rule: "*blog*"},
					{Name: "blog", Regex: "example"},
				})
			},
			url:  "https://example.com/blog/a",
			want: []string{"blog"},
		},
		{
			name: "preserves config order",
			cs: func(t *testing.T) []Compiled {
				return mustCompile(t, []Filter{
					{Name: "news", Regex: "example"},
					{Name: "blog", Rule: "*blog*"},
				})
			},
			url:  "https://example.com/blog/a",
			want: []string{"news", "blog"},
		},
		{
			name: "returns empty when nothing matches",
			cs: func(t *testing.T) []Compiled {
				return mustCompile(t, []Filter{
					{Name: "news", Regex: "^https://news\\."},
				})
			},
			url:  "https://example.com/a",
			want: []string{},
		},
		{
			name: "returns empty when there are no filters",
			cs: func(t *testing.T) []Compiled {
				return nil
			},
			url:  "https://example.com/a",
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Match(tc.cs(t), tc.url)
			if !equalSlices(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
			if len(tc.want) == 0 && got == nil {
				t.Errorf("got nil, want a non-nil empty slice")
			}
		})
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
