package cli

import (
	"flag"
	"reflect"
	"strings"
	"testing"
)

type testStringList []string

func (s *testStringList) String() string { return strings.Join(*s, ",") }

func (s *testStringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func newPermuteTestFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("db", "", "")
	var tags testStringList
	fs.Var(&tags, "tag", "")
	fs.Bool("frozen", false, "")
	fs.Bool("quiet", false, "")
	return fs
}

func newSplitCommandGlobalFlagSet() *flag.FlagSet {
	fs := flag.NewFlagSet("global", flag.ContinueOnError)
	fs.String("db", "", "")
	fs.String("config", "", "")
	fs.Bool("quiet", false, "")
	return fs
}

func TestPermute(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "moves a value flag placed after the operand to the front",
			args: []string{"URL", "--tag", "a"},
			want: []string{"--tag", "a", "URL"},
		},
		{
			name: "leaves the args unchanged when the flag already precedes the operand",
			args: []string{"--tag", "a", "URL"},
			want: []string{"--tag", "a", "URL"},
		},
		{
			name: "moves both occurrences of a repeated flag to the front in order",
			args: []string{"URL", "--tag", "a", "--tag", "b"},
			want: []string{"--tag", "a", "--tag", "b", "URL"},
		},
		{
			name: "treats --tag=a as self-contained and does not consume the next word",
			args: []string{"--tag=a", "URL"},
			want: []string{"--tag=a", "URL"},
		},
		{
			name: "does not let a bool flag consume the following operand",
			args: []string{"URL", "--frozen"},
			want: []string{"--frozen", "URL"},
		},
		{
			name: "does not break when an operand immediately follows a bool flag",
			args: []string{"--frozen", "URL"},
			want: []string{"--frozen", "URL"},
		},
		{
			name: "keeps -- and leaves --help as an operand after it",
			args: []string{"--", "--help"},
			want: []string{"--", "--help"},
		},
		{
			name: "treats everything after -- as an operand",
			args: []string{"URL", "--", "--tag", "a"},
			want: []string{"--", "URL", "--tag", "a"},
		},
		{
			name: "keeps an undefined flag instead of removing it",
			args: []string{"--unknown", "v", "URL"},
			want: []string{"--unknown", "v", "URL"},
		},
		{
			name: "does not add a -- separator when the input has none",
			args: []string{"URL", "--tag", "a", "--frozen"},
			want: []string{"--tag", "a", "--frozen", "URL"},
		},
		{
			name: "leaves a trailing value flag with no value as a trailing operand instead of stealing the preceding operand",
			args: []string{"URL", "--tag"},
			want: []string{"URL", "--tag"},
		},
		{
			name: "keeps a lone trailing value flag unchanged when there is no operand for it to steal",
			args: []string{"--tag"},
			want: []string{"--tag"},
		},
		{
			name: "treats -- as the terminator instead of letting a value flag consume it as its value",
			args: []string{"URL", "--tag", "--", "--frozen"},
			want: []string{"--", "URL", "--tag", "--frozen"},
		},
		{
			name: "does not panic on empty input and returns an empty result",
			args: []string{},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newPermuteTestFlagSet()
			got := Permute(fs, tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Permute(%v) = %v, want %v", tt.args, got, tt.want)
			}
			inputHasSeparator := false
			for _, a := range tt.args {
				if a == "--" {
					inputHasSeparator = true
				}
			}
			if !inputHasSeparator {
				for _, a := range got {
					if a == "--" {
						t.Errorf("Permute(%v) introduced a -- separator that was not in the input: %v", tt.args, got)
					}
				}
			}
		})
	}
}

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
		wantRest []string
		wantErr  bool
	}{
		{
			name:     "finds the subcommand name when it comes before any flags",
			args:     []string{"add", "URL", "--tag", "a"},
			wantName: "add",
			wantRest: []string{"URL", "--tag", "a"},
		},
		{
			name:     "skips a global flag and its value before the subcommand name",
			args:     []string{"--db", "X", "add", "URL"},
			wantName: "add",
			wantRest: []string{"--db", "X", "URL"},
		},
		{
			name:     "skips a global flag and its value after the subcommand name",
			args:     []string{"add", "--db", "X", "URL"},
			wantName: "add",
			wantRest: []string{"--db", "X", "URL"},
		},
		{
			name:     "does not mistake a flag's value for the subcommand name",
			args:     []string{"--config", "add", "list"},
			wantName: "list",
			wantRest: []string{"--config", "add"},
		},
		{
			name:     "skips a bool global flag without consuming the next word as its value",
			args:     []string{"--quiet", "list"},
			wantName: "list",
			wantRest: []string{"--quiet"},
		},
		{
			name:    "errors when no subcommand name is present",
			args:    []string{"--db", "X"},
			wantErr: true,
		},
		{
			name:    "errors when the only non-flag token appears after --",
			args:    []string{"--", "list"},
			wantErr: true,
		},
		{
			name:    "errors when a value flag is immediately followed by -- instead of a value",
			args:    []string{"--config", "--", "list"},
			wantErr: true,
		},
		{
			name:    "errors on empty input",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			global := newSplitCommandGlobalFlagSet()
			gotName, gotRest, err := SplitCommand(global, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SplitCommand(%v) = (%q, %v, nil), want an error", tt.args, gotName, gotRest)
				}
				return
			}
			if err != nil {
				t.Fatalf("SplitCommand(%v) returned unexpected error: %v", tt.args, err)
			}
			if gotName != tt.wantName {
				t.Errorf("SplitCommand(%v) name = %q, want %q", tt.args, gotName, tt.wantName)
			}
			if !reflect.DeepEqual(gotRest, tt.wantRest) {
				t.Errorf("SplitCommand(%v) rest = %v, want %v", tt.args, gotRest, tt.wantRest)
			}
		})
	}
}

func TestPermuteAndSplitCommandAgreeRegardlessOfGlobalFlagPosition(t *testing.T) {
	variants := [][]string{
		{"--db", "X", "add", "URL", "--tag", "a"},
		{"add", "--db", "X", "URL", "--tag", "a"},
		{"add", "URL", "--tag", "a", "--db", "X"},
		{"add", "--tag", "a", "URL", "--db", "X"},
	}

	type parsed struct {
		name     string
		db       string
		tags     []string
		operands []string
	}

	results := make([]parsed, 0, len(variants))
	for _, args := range variants {
		global := newSplitCommandGlobalFlagSet()
		name, rest, err := SplitCommand(global, args)
		if err != nil {
			t.Fatalf("SplitCommand(%v) returned unexpected error: %v", args, err)
		}

		cmdFS := flag.NewFlagSet("add", flag.ContinueOnError)
		db := cmdFS.String("db", "", "")
		var tags testStringList
		cmdFS.Var(&tags, "tag", "")

		permuted := Permute(cmdFS, rest)
		if err := cmdFS.Parse(permuted); err != nil {
			t.Fatalf("cmdFS.Parse(%v) (from %v) returned unexpected error: %v", permuted, args, err)
		}

		results = append(results, parsed{
			name:     name,
			db:       *db,
			tags:     []string(tags),
			operands: cmdFS.Args(),
		})
	}

	for i := 1; i < len(results); i++ {
		if !reflect.DeepEqual(results[0], results[i]) {
			t.Errorf("variant %d (%v) parsed to %+v, want %+v (from variant 0, %v)",
				i, variants[i], results[i], results[0], variants[0])
		}
	}
}
