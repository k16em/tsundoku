package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRecognizesEachCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"init", []string{"init"}, "init"},
		{"add", []string{"add", "https://example.com/a"}, "add"},
		{"list", []string{"list"}, "list"},
		{"show id", []string{"show", "12"}, "show"},
		{"show unread", []string{"show", "unread"}, "show"},
		{"rm", []string{"rm", "1"}, "rm"},
		{"tag list", []string{"tag", "list"}, "tag list"},
		{"tag refresh", []string{"tag", "refresh"}, "tag refresh"},
		{"skill install", []string{"skill", "install"}, "skill install"},
		{"skill uninstall", []string{"skill", "uninstall"}, "skill uninstall"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(tt.args)
			if err != nil {
				t.Fatalf("Parse(%v) unexpected error: %v", tt.args, err)
			}
			if p.Command != tt.want {
				t.Errorf("Command = %q, want %q", p.Command, tt.want)
			}
		})
	}
}

func TestParseShowDistinguishesIDFromUnread(t *testing.T) {
	p, err := Parse([]string{"show", "unread", "--limit", "3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.ShowUnread {
		t.Errorf("ShowUnread = false, want true")
	}
	if p.ShowLimit != 3 {
		t.Errorf("ShowLimit = %d, want 3", p.ShowLimit)
	}

	p2, err := Parse([]string{"show", "12"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p2.ShowUnread {
		t.Errorf("ShowUnread = true, want false")
	}
	if p2.ShowID != 12 {
		t.Errorf("ShowID = %d, want 12", p2.ShowID)
	}
}

func TestParseShowWithLimitOnNonUnreadIsAnError(t *testing.T) {
	_, err := Parse([]string{"show", "12", "--limit", "3"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseAddCollectsRepeatedTagFlags(t *testing.T) {
	p, err := Parse([]string{"add", "https://example.com/a", "--tag", "blog", "--tag", "golang"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.AddURL != "https://example.com/a" {
		t.Errorf("AddURL = %q", p.AddURL)
	}
	if !reflect.DeepEqual(p.AddTags, []string{"blog", "golang"}) {
		t.Errorf("AddTags = %v, want [blog golang]", p.AddTags)
	}
}

func TestParseGlobalFlagsWorkRegardlessOfPositionAroundTheSubcommand(t *testing.T) {
	variants := [][]string{
		{"--db", "X", "add", "https://example.com/a", "--tag", "blog"},
		{"add", "--db", "X", "https://example.com/a", "--tag", "blog"},
		{"add", "https://example.com/a", "--tag", "blog", "--db", "X"},
	}

	var first *Parsed
	for i, args := range variants {
		p, err := Parse(args)
		if err != nil {
			t.Fatalf("Parse(%v) unexpected error: %v", args, err)
		}
		if i == 0 {
			first = p
			continue
		}
		if p.Global.DB != first.Global.DB || p.AddURL != first.AddURL || !reflect.DeepEqual(p.AddTags, first.AddTags) {
			t.Errorf("variant %d = %+v, want to match variant 0 = %+v", i, p, first)
		}
	}
}

func TestParseListUnreadAndReadTogetherIsAnError(t *testing.T) {
	_, err := Parse([]string{"list", "--unread", "--read"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseListUnreadSetsReadFalse(t *testing.T) {
	p, err := Parse([]string{"list", "--unread"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ListRead == nil || *p.ListRead != false {
		t.Fatalf("ListRead = %v, want pointer to false", p.ListRead)
	}
}

func TestParseListReadSetsReadTrue(t *testing.T) {
	p, err := Parse([]string{"list", "--read"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ListRead == nil || *p.ListRead != true {
		t.Fatalf("ListRead = %v, want pointer to true", p.ListRead)
	}
}

func TestParseListWithNeitherReadNorUnreadLeavesReadNil(t *testing.T) {
	p, err := Parse([]string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ListRead != nil {
		t.Fatalf("ListRead = %v, want nil", p.ListRead)
	}
}

func TestParseBareTagIsAnError(t *testing.T) {
	_, err := Parse([]string{"tag"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseUnknownTagSubcommandIsAnError(t *testing.T) {
	_, err := Parse([]string{"tag", "bogus"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseUnknownCommandIsAnError(t *testing.T) {
	_, err := Parse([]string{"bogus"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseSetFieldsAreTrueOnlyWhenExplicitlyGiven(t *testing.T) {
	explicit, err := Parse([]string{"list", "--limit", "20"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !explicit.SetListLimit {
		t.Errorf("SetListLimit = false, want true when --limit is given explicitly, even matching the default value")
	}

	implicit, err := Parse([]string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if implicit.SetListLimit {
		t.Errorf("SetListLimit = true, want false when --limit is not given")
	}
}

func TestParseSetListReverseIsTrueWhenExplicitlyGivenEvenAsFalse(t *testing.T) {
	p, err := Parse([]string{"list", "--reverse=false"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.SetListReverse {
		t.Errorf("SetListReverse = false, want true because --reverse was explicitly given")
	}
	if p.ListReverse {
		t.Errorf("ListReverse = true, want false")
	}

	p2, err := Parse([]string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p2.SetListReverse {
		t.Errorf("SetListReverse = true, want false when --reverse is not given")
	}
}

func TestParseSetTagSortAndSetTagReverse(t *testing.T) {
	p, err := Parse([]string{"tag", "list", "--sort", "count", "--reverse"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.SetTagSort || p.TagSort != "count" {
		t.Errorf("SetTagSort/TagSort = %v/%q, want true/count", p.SetTagSort, p.TagSort)
	}
	if !p.SetTagReverse || !p.TagReverse {
		t.Errorf("SetTagReverse/TagReverse = %v/%v, want true/true", p.SetTagReverse, p.TagReverse)
	}
}

func TestParseSetShowLimitIsTrueOnlyWhenExplicitlyGiven(t *testing.T) {
	p, err := Parse([]string{"show", "unread", "--limit", "10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.SetShowLimit {
		t.Errorf("SetShowLimit = false, want true")
	}

	p2, err := Parse([]string{"show", "unread"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p2.SetShowLimit {
		t.Errorf("SetShowLimit = true, want false")
	}
}

func TestParseNonNumericShowIDIsAnError(t *testing.T) {
	_, err := Parse([]string{"show", "abc"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseNonNumericRemoveIDIsAnError(t *testing.T) {
	_, err := Parse([]string{"rm", "abc"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseRemoveCollectsMultipleIDs(t *testing.T) {
	p, err := Parse([]string{"rm", "1", "2", "3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(p.RemoveIDs, []int64{1, 2, 3}) {
		t.Errorf("RemoveIDs = %v, want [1 2 3]", p.RemoveIDs)
	}
}

func TestParseRemoveWithNoIDsIsAnError(t *testing.T) {
	_, err := Parse([]string{"rm"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseWithNoSubcommandIsAnError(t *testing.T) {
	_, err := Parse([]string{"--db", "X"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseWithEmptyArgsIsAnError(t *testing.T) {
	_, err := Parse(nil)
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseLongHelpFlagWorksWithoutAnySubcommand(t *testing.T) {
	p, err := Parse([]string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Help {
		t.Errorf("Help = false, want true")
	}
}

func TestParseShortHelpFlagWorksWithoutAnySubcommand(t *testing.T) {
	p, err := Parse([]string{"-h"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Help {
		t.Errorf("Help = false, want true")
	}
}

func TestParseVersionFlagWorksWithoutAnySubcommand(t *testing.T) {
	p, err := Parse([]string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Version {
		t.Errorf("Version = false, want true")
	}
}

func TestParseHelpWorksWithoutASubcommandEvenAlongsideOtherGlobalFlags(t *testing.T) {
	p, err := Parse([]string{"--db", "X", "--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Help {
		t.Errorf("Help = false, want true")
	}
	if p.Global.DB != "X" {
		t.Errorf("Global.DB = %q, want X", p.Global.DB)
	}
}

func TestParseWithOnlyOrdinaryGlobalFlagsAndNoSubcommandIsStillAnError(t *testing.T) {
	_, err := Parse([]string{"--db", "X", "--quiet"})
	if err == nil {
		t.Fatalf("expected an error because there is no --help or --version to except it")
	}
}

func TestParseDoesNotTreatAFlagValueAsTheSubcommandName(t *testing.T) {
	p, err := Parse([]string{"--config", "add", "list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Command != "list" {
		t.Errorf("Command = %q, want list", p.Command)
	}
	if p.Global.Config != "add" {
		t.Errorf("Global.Config = %q, want add", p.Global.Config)
	}
}

func TestParseStopsLookingForTheSubcommandNameAfterDoubleDash(t *testing.T) {
	_, err := Parse([]string{"--", "add", "https://example.com/a"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseQuietShortAndLongFlagsAreEquivalent(t *testing.T) {
	short, err := Parse([]string{"list", "-q"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	long, err := Parse([]string{"list", "--quiet"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !short.Global.Quiet || !long.Global.Quiet {
		t.Errorf("Quiet = %v/%v, want true/true", short.Global.Quiet, long.Global.Quiet)
	}
}

func TestParseShowJSONShortAndLongFlagsAreEquivalent(t *testing.T) {
	short, err := Parse([]string{"show", "1", "-j"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	long, err := Parse([]string{"show", "1", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !short.ShowJSON || !long.ShowJSON {
		t.Errorf("ShowJSON = %v/%v, want true/true", short.ShowJSON, long.ShowJSON)
	}
}

func TestParseShowFrozenFlag(t *testing.T) {
	p, err := Parse([]string{"show", "1", "--frozen"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.ShowFrozen {
		t.Errorf("ShowFrozen = false, want true")
	}
}

func TestParseTagRefreshFlags(t *testing.T) {
	p, err := Parse([]string{"tag", "refresh", "--dry-run", "--prune"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.RefreshDryRun || !p.RefreshPrune {
		t.Errorf("RefreshDryRun/RefreshPrune = %v/%v, want true/true", p.RefreshDryRun, p.RefreshPrune)
	}
}

func TestParseHelpAndVersionBypassPositionalValidation(t *testing.T) {
	p, err := Parse([]string{"add", "--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Help {
		t.Errorf("Help = false, want true")
	}

	p2, err := Parse([]string{"show", "--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p2.Version {
		t.Errorf("Version = false, want true")
	}
}

func TestUsageIsNonEmptyAndMentionsTheProgramName(t *testing.T) {
	got := Usage()
	if got == "" {
		t.Fatalf("Usage() is empty")
	}
	if !strings.Contains(got, "tsundoku") {
		t.Errorf("Usage() = %q, want it to mention tsundoku", got)
	}
}

func TestSubcommandHelpNamesThatSubcommandAndItsOwnFlags(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantContain []string
	}{
		{"init", []string{"init", "--help"}, []string{"tsundoku init"}},
		{"add", []string{"add", "--help"}, []string{"tsundoku add", "-tag"}},
		{"list", []string{"list", "--help"}, []string{"tsundoku list", "-limit", "-tag", "-unread", "-read", "-sort", "-reverse"}},
		{"show", []string{"show", "--help"}, []string{"tsundoku show", "unread", "-frozen", "-json", "-limit"}},
		{"rm", []string{"rm", "--help"}, []string{"tsundoku rm"}},
		{"tag list", []string{"tag", "list", "--help"}, []string{"tsundoku tag list", "-sort", "-reverse"}},
		{"tag refresh", []string{"tag", "refresh", "--help"}, []string{"tsundoku tag refresh", "-dry-run", "-prune"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(tt.args)
			if err != nil {
				t.Fatalf("Parse(%v) unexpected error: %v", tt.args, err)
			}
			if !p.Help {
				t.Fatalf("Help = false, want true")
			}
			if p.HelpText == "" {
				t.Fatalf("HelpText is empty")
			}
			for _, want := range tt.wantContain {
				if !strings.Contains(p.HelpText, want) {
					t.Errorf("HelpText = %q, want it to contain %q", p.HelpText, want)
				}
			}
			if p.HelpText == Usage() {
				t.Errorf("HelpText equals the top-level Usage(), want subcommand-specific text")
			}
		})
	}
}

func TestSubcommandHelpTextsAreAllDistinctFromEachOther(t *testing.T) {
	commands := []struct {
		name string
		args []string
	}{
		{"init", []string{"init", "--help"}},
		{"add", []string{"add", "--help"}},
		{"list", []string{"list", "--help"}},
		{"show", []string{"show", "--help"}},
		{"rm", []string{"rm", "--help"}},
		{"tag list", []string{"tag", "list", "--help"}},
		{"tag refresh", []string{"tag", "refresh", "--help"}},
	}

	seen := make(map[string]string)
	for _, c := range commands {
		p, err := Parse(c.args)
		if err != nil {
			t.Fatalf("Parse(%v) unexpected error: %v", c.args, err)
		}
		if other, ok := seen[p.HelpText]; ok {
			t.Errorf("%s and %s produced identical HelpText", c.name, other)
		}
		seen[p.HelpText] = c.name
	}
}

func TestBareHelpFlagLeavesHelpTextEmptySoAppFallsBackToUsage(t *testing.T) {
	p, err := Parse([]string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.HelpText != "" {
		t.Errorf("HelpText = %q, want empty for a subcommand-less --help", p.HelpText)
	}
}

func TestParseAddWithNoURLIsAnError(t *testing.T) {
	_, err := Parse([]string{"add"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseListAcceptsSortAndTagFlags(t *testing.T) {
	p, err := Parse([]string{"list", "--tag", "blog", "--sort", "id"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(p.ListTags, []string{"blog"}) {
		t.Errorf("ListTags = %v, want [blog]", p.ListTags)
	}
	if p.ListSort != "id" {
		t.Errorf("ListSort = %q, want id", p.ListSort)
	}
}

func TestParseListRejectsAnEmptyExplicitSortValue(t *testing.T) {
	_, err := Parse([]string{"list", "--sort="})
	if err == nil {
		t.Fatalf("expected an error for an explicit empty --sort")
	}
}

func TestParseListRejectsAnUnknownSortValue(t *testing.T) {
	_, err := Parse([]string{"list", "--sort", "bogus"})
	if err == nil {
		t.Fatalf("expected an error for an unknown --sort value")
	}
}

func TestParseListAcceptsEachKnownSortValue(t *testing.T) {
	for _, key := range []string{"created", "id"} {
		t.Run(key, func(t *testing.T) {
			p, err := Parse([]string{"list", "--sort", key})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.ListSort != key {
				t.Errorf("ListSort = %q, want %q", p.ListSort, key)
			}
		})
	}
}

func TestParseListWithoutSortFlagIsNotValidated(t *testing.T) {
	p, err := Parse([]string{"list"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.SetListSort {
		t.Errorf("SetListSort = true, want false")
	}
}

func TestParseTagListRejectsAnEmptyExplicitSortValue(t *testing.T) {
	_, err := Parse([]string{"tag", "list", "--sort="})
	if err == nil {
		t.Fatalf("expected an error for an explicit empty --sort")
	}
}

func TestParseTagListRejectsAnUnknownSortValue(t *testing.T) {
	_, err := Parse([]string{"tag", "list", "--sort", "bogus"})
	if err == nil {
		t.Fatalf("expected an error for an unknown --sort value")
	}
}

func TestParseTagListAcceptsEachKnownSortValue(t *testing.T) {
	for _, key := range []string{"name", "count"} {
		t.Run(key, func(t *testing.T) {
			p, err := Parse([]string{"tag", "list", "--sort", key})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.TagSort != key {
				t.Errorf("TagSort = %q, want %q", p.TagSort, key)
			}
		})
	}
}

func TestParseErrorFromArgumentParsingIsOneLineAndCarriesHelpTextSeparately(t *testing.T) {
	p, err := Parse([]string{"list", "--bogus"})
	if err == nil {
		t.Fatalf("expected an error")
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("err.Error() = %q, want a single line with no embedded help text", err.Error())
	}
	if p == nil {
		t.Fatalf("Parse returned a nil *Parsed alongside the error, want one carrying HelpText")
	}
	if p.HelpText == "" {
		t.Errorf("HelpText is empty, want the list subcommand's help")
	}
	if !strings.Contains(p.HelpText, "tsundoku list") {
		t.Errorf("HelpText = %q, want it to name the list subcommand", p.HelpText)
	}
}

func TestParseSemanticValidationErrorsAlsoCarryHelpTextSeparately(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantContain string
	}{
		{"add with no URL", []string{"add"}, "tsundoku add"},
		{"show with a bad id", []string{"show", "abc"}, "tsundoku show"},
		{"rm with no ids", []string{"rm"}, "tsundoku rm"},
		{"list --unread and --read together", []string{"list", "--unread", "--read"}, "tsundoku list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := Parse(tt.args)
			if err == nil {
				t.Fatalf("Parse(%v) expected an error", tt.args)
			}
			if strings.Contains(err.Error(), "\n") {
				t.Errorf("err.Error() = %q, want a single line", err.Error())
			}
			if p == nil || p.HelpText == "" {
				t.Fatalf("Parse(%v) did not carry HelpText alongside the error", tt.args)
			}
			if !strings.Contains(p.HelpText, tt.wantContain) {
				t.Errorf("HelpText = %q, want it to contain %q", p.HelpText, tt.wantContain)
			}
		})
	}
}

func TestUnresolvedSubcommandErrorsDoNotCarryHelpText(t *testing.T) {
	tests := [][]string{
		{"bogus"},
		{"tag", "bogus"},
		{"tag"},
	}
	for _, args := range tests {
		p, err := Parse(args)
		if err == nil {
			t.Fatalf("Parse(%v) expected an error", args)
		}
		if p != nil {
			t.Errorf("Parse(%v) = %+v, want a nil *Parsed since no subcommand was resolved", args, p)
		}
	}
}

func TestTagRefreshHelpWarnsThatPruneIsDestructive(t *testing.T) {
	p, err := Parse([]string{"tag", "refresh", "--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(p.HelpText, "--prune") {
		t.Errorf("HelpText = %q, want it to mention --prune", p.HelpText)
	}
	if !strings.Contains(strings.ToLower(p.HelpText), "destructive") && !strings.Contains(strings.ToLower(p.HelpText), "manually") {
		t.Errorf("HelpText = %q, want an operational warning that --prune is destructive and removes manual tags", p.HelpText)
	}
	if !strings.Contains(p.HelpText, "--dry-run") {
		t.Errorf("HelpText = %q, want it to recommend --dry-run first", p.HelpText)
	}
}

func TestParseRejectsUnexpectedPositionalArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"init with an argument", []string{"init", "extra"}},
		{"add with two operands", []string{"add", "https://example.com/a", "https://example.com/b"}},
		{"list with a positional argument", []string{"list", "extra"}},
		{"tag list with a positional argument", []string{"tag", "list", "extra"}},
		{"tag refresh with a positional argument", []string{"tag", "refresh", "extra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.args); err == nil {
				t.Fatalf("Parse(%v) expected an error", tt.args)
			}
		})
	}
}

func TestParseRejectsAnUnknownFlag(t *testing.T) {
	_, err := Parse([]string{"list", "--bogus"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseShowWithNoOperandIsAnError(t *testing.T) {
	_, err := Parse([]string{"show"})
	if err == nil {
		t.Fatalf("expected an error")
	}
}

func TestParseRandomDefaultsToOneAndAcceptsACount(t *testing.T) {
	p, err := Parse([]string{"random"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Command != "random" {
		t.Errorf("Command = %q, want %q", p.Command, "random")
	}
	if p.RandomCount != 1 {
		t.Errorf("RandomCount = %d, want 1", p.RandomCount)
	}

	p2, err := Parse([]string{"random", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p2.RandomCount != 5 {
		t.Errorf("RandomCount = %d, want 5", p2.RandomCount)
	}
}

func TestParseRandomJSONShortAndLongFlagsAreEquivalent(t *testing.T) {
	short, err := Parse([]string{"random", "3", "-j"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	long, err := Parse([]string{"random", "3", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !short.RandomJSON || !long.RandomJSON {
		t.Errorf("RandomJSON = %v/%v, want true/true", short.RandomJSON, long.RandomJSON)
	}
}

func TestParseRandomAcceptsFlagsBeforeTheCount(t *testing.T) {
	p, err := Parse([]string{"random", "--frozen", "--json", "4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.RandomCount != 4 || !p.RandomFrozen || !p.RandomJSON {
		t.Errorf("got count=%d frozen=%v json=%v, want 4/true/true", p.RandomCount, p.RandomFrozen, p.RandomJSON)
	}
}

func TestParseRandomRejectsBadCounts(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"not a number", []string{"random", "abc"}},
		{"zero", []string{"random", "0"}},
		{"two operands", []string{"random", "1", "2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.args); err == nil {
				t.Fatalf("Parse(%v) expected an error", tt.args)
			}
		})
	}
}

func TestParseRejectsUnknownSkillSubcommand(t *testing.T) {
	_, err := Parse([]string{"skill", "reinstall"})

	if err == nil {
		t.Fatalf("Parse() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "unknown skill subcommand") {
		t.Errorf("error = %q, want it to mention the unknown subcommand", err)
	}
}

func TestParseRejectsSkillPositionalArguments(t *testing.T) {
	_, err := Parse([]string{"skill", "install", "somewhere"})

	if err == nil {
		t.Fatalf("Parse() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "takes no positional arguments") {
		t.Errorf("error = %q, want it to reject the positional argument", err)
	}
}

func TestParseSkillHelpCarriesHelpText(t *testing.T) {
	p, err := Parse([]string{"skill", "install", "--help"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Help {
		t.Errorf("Help = false, want true")
	}
	if !strings.Contains(p.HelpText, "tsundoku skill install") {
		t.Errorf("HelpText = %q, want it to describe skill install", p.HelpText)
	}
}

func TestUsageListsSkillCommands(t *testing.T) {
	usage := Usage()

	for _, want := range []string{"skill install", "skill uninstall"} {
		if !strings.Contains(usage, want) {
			t.Errorf("Usage() does not mention %q", want)
		}
	}
}
