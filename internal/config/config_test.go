package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeTOML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

func TestDefaultMatchesSpec(t *testing.T) {
	got := Default()
	want := Config{
		List: ListConfig{Limit: 20, Sort: "created", Reverse: false},
		Tags: TagsConfig{Sort: "name", Reverse: false},
	}
	if got.List != want.List {
		t.Errorf("List = %+v, want %+v", got.List, want.List)
	}
	if got.Tags != want.Tags {
		t.Errorf("Tags = %+v, want %+v", got.Tags, want.Tags)
	}
	if len(got.Filter) != 0 {
		t.Errorf("Filter = %v, want empty", got.Filter)
	}
	if len(got.Compiled) != 0 {
		t.Errorf("Compiled = %v, want empty", got.Compiled)
	}
}

func TestLoadWithMissingFileAndNotExplicitReturnsDefaultWithoutError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.toml")

	var warn bytes.Buffer
	got, err := Load(path, false, &warn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.List != Default().List || got.Tags != Default().Tags {
		t.Errorf("got %+v, want defaults", got)
	}
	if warn.Len() != 0 {
		t.Errorf("expected no warnings, got %q", warn.String())
	}
}

func TestLoadWithMissingFileAndExplicitReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.toml")

	_, err := Load(path, true, nil)
	if err == nil {
		t.Fatalf("expected an error for an explicitly requested missing config file")
	}
}

func TestLoadWithEmptyFileYieldsAllDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "config.toml", "")

	got, err := Load(path, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, Default()) {
		t.Errorf("Load(empty file) = %#v, want exactly Default() = %#v", got, Default())
	}
}

func TestLoadWithOnlyLimitSetKeepsOtherListFieldsAtDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "config.toml", "[list]\nlimit = 5\n")

	got, err := Load(path, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.List.Limit != 5 {
		t.Errorf("List.Limit = %d, want 5", got.List.Limit)
	}
	if got.List.Sort != "created" {
		t.Errorf("List.Sort = %q, want %q", got.List.Sort, "created")
	}
	if got.List.Reverse != false {
		t.Errorf("List.Reverse = %v, want false", got.List.Reverse)
	}
}

func TestLoadWithExplicitReverseFalseBehavesLikeOmittingIt(t *testing.T) {
	dir := t.TempDir()
	explicitPath := writeTOML(t, dir, "explicit.toml", "[list]\nlimit = 20\nsort = \"created\"\nreverse = false\n")
	omittedPath := writeTOML(t, dir, "omitted.toml", "[list]\nlimit = 20\nsort = \"created\"\n")

	explicit, err := Load(explicitPath, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	omitted, err := Load(omittedPath, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if explicit.List != omitted.List {
		t.Errorf("explicit reverse=false %+v differs from omitted %+v", explicit.List, omitted.List)
	}
	if explicit.List.Reverse != false {
		t.Errorf("explicit.List.Reverse = %v, want false", explicit.List.Reverse)
	}
}

func TestLoadWithOnlyTagsSectionKeepsListAtDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "config.toml", "[tags]\nsort = \"count\"\n")

	got, err := Load(path, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.List != Default().List {
		t.Errorf("List = %+v, want defaults %+v", got.List, Default().List)
	}
	if got.Tags.Sort != "count" {
		t.Errorf("Tags.Sort = %q, want %q", got.Tags.Sort, "count")
	}
	if got.Tags.Reverse != false {
		t.Errorf("Tags.Reverse = %v, want false", got.Tags.Reverse)
	}
}

func TestLoadValidatesFields(t *testing.T) {
	tests := []struct {
		name        string
		toml        string
		wantErr     bool
		errContains []string
	}{
		{
			name:        "list.limit of 0 is rejected",
			toml:        "[list]\nlimit = 0\n",
			wantErr:     true,
			errContains: []string{"list.limit", "0"},
		},
		{
			name:        "list.limit of 1001 is rejected",
			toml:        "[list]\nlimit = 1001\n",
			wantErr:     true,
			errContains: []string{"list.limit", "1001"},
		},
		{
			name:        "list.limit of -1 is rejected",
			toml:        "[list]\nlimit = -1\n",
			wantErr:     true,
			errContains: []string{"list.limit", "-1"},
		},
		{
			name:    "list.limit of 1 is accepted",
			toml:    "[list]\nlimit = 1\n",
			wantErr: false,
		},
		{
			name:    "list.limit of 1000 is accepted",
			toml:    "[list]\nlimit = 1000\n",
			wantErr: false,
		},
		{
			name:        `list.sort of "updated" is rejected because it was removed`,
			toml:        "[list]\nsort = \"updated\"\n",
			wantErr:     true,
			errContains: []string{"list.sort", "updated"},
		},
		{
			name:    `list.sort of "created" is accepted`,
			toml:    "[list]\nsort = \"created\"\n",
			wantErr: false,
		},
		{
			name:    `list.sort of "id" is accepted`,
			toml:    "[list]\nsort = \"id\"\n",
			wantErr: false,
		},
		{
			name:        `tags.sort of "created" is rejected`,
			toml:        "[tags]\nsort = \"created\"\n",
			wantErr:     true,
			errContains: []string{"tags.sort", "created"},
		},
		{
			name:    `tags.sort of "name" is accepted`,
			toml:    "[tags]\nsort = \"name\"\n",
			wantErr: false,
		},
		{
			name:    `tags.sort of "count" is accepted`,
			toml:    "[tags]\nsort = \"count\"\n",
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeTOML(t, dir, "config.toml", tc.toml)

			_, err := Load(path, true, nil)
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
		})
	}
}

func TestLoadCompilesFilters(t *testing.T) {
	dir := t.TempDir()
	toml := "[[filter]]\nname = \"blog\"\nrule = \"*blog*\"\n"
	path := writeTOML(t, dir, "config.toml", toml)

	got, err := Load(path, true, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Compiled) != 1 {
		t.Fatalf("expected 1 compiled filter, got %d", len(got.Compiled))
	}
	if got.Compiled[0].Name != "blog" {
		t.Errorf("Compiled[0].Name = %q, want %q", got.Compiled[0].Name, "blog")
	}
	if !got.Compiled[0].Re.MatchString("https://example.com/blog/a") {
		t.Errorf("expected compiled filter to match blog URL")
	}
}

func TestLoadWarnsWhenFilterHasBothRuleAndRegexAndPrefersRule(t *testing.T) {
	dir := t.TempDir()
	toml := "[[filter]]\nname = \"blog\"\nrule = \"*blog*\"\nregex = \"^https://news\\\\.\"\n"
	path := writeTOML(t, dir, "config.toml", toml)

	var warn bytes.Buffer
	got, err := Load(path, true, &warn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warn.Len() == 0 {
		t.Errorf("expected a warning to be written")
	}
	if !got.Compiled[0].Re.MatchString("https://example.com/blog/a") {
		t.Errorf("expected the rule to be used, not the regex")
	}
}

func TestLoadErrorsWhenFilterHasNeitherRuleNorRegex(t *testing.T) {
	dir := t.TempDir()
	toml := "[[filter]]\nname = \"blog\"\n"
	path := writeTOML(t, dir, "config.toml", toml)

	_, err := Load(path, true, nil)
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestLoadErrorsOnInvalidRegexAndNamesTheIndexAndFilter(t *testing.T) {
	dir := t.TempDir()
	toml := "[[filter]]\nname = \"news\"\nregex = \"^(\"\n"
	path := writeTOML(t, dir, "config.toml", toml)

	_, err := Load(path, true, nil)
	if err == nil {
		t.Fatalf("expected an error")
	}
	for _, sub := range []string{"#1", "news"} {
		if !strings.Contains(err.Error(), sub) {
			t.Errorf("expected error to contain %q, got: %v", sub, err)
		}
	}
}

func TestLoadErrorsOnTOMLSyntaxError(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "config.toml", "[list\nlimit = 5\n")

	_, err := Load(path, true, nil)
	if err == nil {
		t.Fatalf("expected an error for malformed TOML")
	}
}

func TestTemplateRoundTripsThroughLoadToExactlyDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeTOML(t, dir, "config.toml", Template())

	got, err := Load(path, true, nil)
	if err != nil {
		t.Fatalf("unexpected error loading template: %v", err)
	}
	want := Default()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Load(Template()) = %#v, want exactly Default() = %#v", got, want)
	}
}

func TestTemplateFilterExamplesAreCommentedOut(t *testing.T) {
	tmpl := Template()
	for _, line := range strings.Split(tmpl, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "[[filter]]" {
			t.Errorf("template contains an active [[filter]] table; example filters must be commented out")
		}
	}
}

func TestTemplateMatchesTheSpecifiedStarterConfig(t *testing.T) {
	want := "[list]\n" +
		"limit   = 20\n" +
		"sort    = \"created\"      # created | id\n" +
		"reverse = false\n" +
		"\n" +
		"[tags]\n" +
		"sort    = \"name\"         # name | count\n" +
		"reverse = false\n" +
		"\n" +
		"# [[filter]]\n" +
		"# name = \"blog\"\n" +
		"# rule = \"*blog*\"\n" +
		"\n" +
		"# [[filter]]\n" +
		"# name = \"news\"\n" +
		"# regex = \"^https://news\\\\.\"\n"

	got := Template()
	if got != want {
		t.Errorf("Template() =\n%s\nwant:\n%s", got, want)
	}
}
