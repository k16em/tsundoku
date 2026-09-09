package cli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Global holds the options accepted by every subcommand.
type Global struct {
	DB     string
	Config string
	Quiet  bool
}

// Parsed is the result of parsing the command line.
type Parsed struct {
	Global  Global
	Command string
	Help    bool
	Version bool

	// HelpText is the help for the resolved subcommand, populated whenever
	// Help is true and a subcommand was resolved. It is empty when --help
	// was given with no subcommand, in which case callers should fall back
	// to Usage().
	HelpText string

	AddURL  string
	AddTags []string

	ListTags    []string
	ListLimit   int
	ListRead    *bool
	ListSort    string
	ListReverse bool

	ShowID     int64
	ShowUnread bool
	ShowLimit  int
	ShowFrozen bool
	ShowJSON   bool

	RandomCount  int
	RandomFrozen bool
	RandomJSON   bool

	RemoveIDs []int64

	TagSort    string
	TagReverse bool

	RefreshDryRun bool
	RefreshPrune  bool

	SetListLimit   bool
	SetListSort    bool
	SetListReverse bool
	SetTagSort     bool
	SetTagReverse  bool
	SetShowLimit   bool
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

const defaultShowUnreadLimit = 10

const defaultRandomCount = 1

type globalFlags struct {
	db      string
	config  string
	quiet   bool
	help    bool
	version bool
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() { fmt.Fprint(fs.Output(), Usage()) }
	return fs
}

func registerGlobalFlags(fs *flag.FlagSet) *globalFlags {
	g := &globalFlags{}
	fs.StringVar(&g.db, "db", "", "override the database path")
	fs.StringVar(&g.config, "config", "", "override the config file path")
	fs.BoolVar(&g.quiet, "quiet", false, "suppress warnings and notes")
	fs.BoolVar(&g.quiet, "q", false, "suppress warnings and notes")
	fs.BoolVar(&g.help, "help", false, "show help")
	fs.BoolVar(&g.help, "h", false, "show help")
	fs.BoolVar(&g.version, "version", false, "show version")
	return g
}

func applyGlobal(p *Parsed, g *globalFlags) {
	p.Global = Global{DB: g.db, Config: g.config, Quiet: g.quiet}
	p.Help = g.help
	p.Version = g.version
}

func commandHelp(fs *flag.FlagSet, usage string) string {
	var buf bytes.Buffer
	buf.WriteString(usage)
	buf.WriteString("\n\nOptions:\n")
	old := fs.Output()
	fs.SetOutput(&buf)
	fs.PrintDefaults()
	fs.SetOutput(old)
	return buf.String()
}

func visited(fs *flag.FlagSet) map[string]bool {
	seen := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { seen[f.Name] = true })
	return seen
}

func helpErr(help string, err error) (*Parsed, error) {
	return &Parsed{HelpText: help}, err
}

var listSortKeys = map[string]bool{"created": true, "id": true}

var tagSortKeys = map[string]bool{"name": true, "count": true}

const tagRefreshPruneWarning = `
Warning: --prune deletes any tag whose bookmark no longer matches a
configured filter, including tags you added manually with --tag. This is
destructive. Run with --dry-run first to review what would change.
`

// Parse turns raw arguments into a Parsed command.
func Parse(args []string) (*Parsed, error) {
	lookahead := newFlagSet("tsundoku")
	registerGlobalFlags(lookahead)

	name, rest, err := SplitCommand(lookahead, args)
	if err != nil {
		if p, ok := parseGlobalOnly(args); ok {
			return p, nil
		}
		return nil, fmt.Errorf("cli: %w", err)
	}

	switch name {
	case "init":
		return parseInit(rest)
	case "add":
		return parseAdd(rest)
	case "list":
		return parseList(rest)
	case "show":
		return parseShow(rest)
	case "random":
		return parseRandom(rest)
	case "rm":
		return parseRemove(rest)
	case "tag":
		return parseTag(rest)
	case "skill":
		return parseSkill(rest)
	default:
		return nil, fmt.Errorf("cli: unknown command %q", name)
	}
}

func parseGlobalOnly(args []string) (*Parsed, bool) {
	fs := newFlagSet("tsundoku")
	g := registerGlobalFlags(fs)

	if err := fs.Parse(Permute(fs, args)); err != nil {
		return nil, false
	}
	if len(fs.Args()) > 0 {
		return nil, false
	}
	if !g.help && !g.version {
		return nil, false
	}

	p := &Parsed{}
	applyGlobal(p, g)
	return p, true
}

func parseInit(rest []string) (*Parsed, error) {
	fs := newFlagSet("init")
	g := registerGlobalFlags(fs)

	help := commandHelp(fs, "tsundoku init [OPTIONS]")
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: "init"}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	if len(fs.Args()) > 0 {
		return helpErr(help, fmt.Errorf("cli: init takes no arguments"))
	}

	return p, nil
}

func parseAdd(rest []string) (*Parsed, error) {
	fs := newFlagSet("add")
	g := registerGlobalFlags(fs)
	var tags stringList
	fs.Var(&tags, "tag", "attach a tag (repeatable)")

	help := commandHelp(fs, "tsundoku add <URL> [OPTIONS]")
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: "add"}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	operands := fs.Args()
	if len(operands) == 0 {
		return helpErr(help, fmt.Errorf("cli: add requires a URL"))
	}
	if len(operands) > 1 {
		return helpErr(help, fmt.Errorf("cli: add takes exactly one URL, got %d", len(operands)))
	}

	p.AddURL = operands[0]
	p.AddTags = []string(tags)
	return p, nil
}

func parseList(rest []string) (*Parsed, error) {
	fs := newFlagSet("list")
	g := registerGlobalFlags(fs)
	var tags stringList
	fs.Var(&tags, "tag", "filter by tag, AND of all given (repeatable)")
	limit := fs.Int("limit", 0, "maximum number of results")
	unread := fs.Bool("unread", false, "show only unread bookmarks")
	read := fs.Bool("read", false, "show only read bookmarks")
	sortKey := fs.String("sort", "", "sort key: created or id")
	reverse := fs.Bool("reverse", false, "reverse the sort order")

	help := commandHelp(fs, "tsundoku list [OPTIONS]")
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: "list"}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	if len(fs.Args()) > 0 {
		return helpErr(help, fmt.Errorf("cli: list takes no positional arguments"))
	}

	if *unread && *read {
		return helpErr(help, fmt.Errorf("cli: --unread and --read cannot be used together"))
	}

	seen := visited(fs)
	if seen["sort"] && !listSortKeys[*sortKey] {
		return helpErr(help, fmt.Errorf("cli: --sort must be one of created, id; got %q", *sortKey))
	}

	p.ListTags = []string(tags)
	p.ListLimit = *limit
	p.SetListLimit = seen["limit"]
	p.ListSort = *sortKey
	p.SetListSort = seen["sort"]
	p.ListReverse = *reverse
	p.SetListReverse = seen["reverse"]

	switch {
	case *unread:
		v := false
		p.ListRead = &v
	case *read:
		v := true
		p.ListRead = &v
	}

	return p, nil
}

func parseShow(rest []string) (*Parsed, error) {
	fs := newFlagSet("show")
	g := registerGlobalFlags(fs)
	frozen := fs.Bool("frozen", false, "do not mark shown bookmarks as read")
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "output as JSON")
	fs.BoolVar(&jsonOut, "j", false, "output as JSON")
	limit := fs.Int("limit", defaultShowUnreadLimit, "maximum number of results (show unread only)")

	help := commandHelp(fs, "tsundoku show <ID> [OPTIONS]\ntsundoku show unread [OPTIONS]")
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: "show"}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	operands := fs.Args()
	if len(operands) != 1 {
		return helpErr(help, fmt.Errorf("cli: show requires exactly one argument (an id or \"unread\"), got %d", len(operands)))
	}

	seen := visited(fs)
	p.ShowFrozen = *frozen
	p.ShowJSON = jsonOut

	if operands[0] == "unread" {
		p.ShowUnread = true
		p.ShowLimit = *limit
		p.SetShowLimit = seen["limit"]
		return p, nil
	}

	if seen["limit"] {
		return helpErr(help, fmt.Errorf("cli: --limit is only valid with \"show unread\""))
	}

	id, err := strconv.ParseInt(operands[0], 10, 64)
	if err != nil {
		return helpErr(help, fmt.Errorf("cli: invalid bookmark id %q: %w", operands[0], err))
	}
	p.ShowID = id
	return p, nil
}

func parseRandom(rest []string) (*Parsed, error) {
	fs := newFlagSet("random")
	g := registerGlobalFlags(fs)
	frozen := fs.Bool("frozen", false, "do not mark the drawn bookmarks as read")
	var jsonOut bool
	fs.BoolVar(&jsonOut, "json", false, "output as JSON")
	fs.BoolVar(&jsonOut, "j", false, "output as JSON")

	help := commandHelp(fs, "tsundoku random [N] [OPTIONS]")
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: "random"}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	operands := fs.Args()
	if len(operands) > 1 {
		return helpErr(help, fmt.Errorf("cli: random takes at most one count, got %d", len(operands)))
	}

	count := defaultRandomCount
	if len(operands) == 1 {
		n, err := strconv.Atoi(operands[0])
		if err != nil {
			return helpErr(help, fmt.Errorf("cli: invalid count %q: %w", operands[0], err))
		}
		if n < 1 {
			return helpErr(help, fmt.Errorf("cli: random count must be at least 1, got %d", n))
		}
		count = n
	}

	p.RandomCount = count
	p.RandomFrozen = *frozen
	p.RandomJSON = jsonOut
	return p, nil
}

func parseRemove(rest []string) (*Parsed, error) {
	fs := newFlagSet("rm")
	g := registerGlobalFlags(fs)

	help := commandHelp(fs, "tsundoku rm <ID>... [OPTIONS]")
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: "rm"}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	operands := fs.Args()
	if len(operands) == 0 {
		return helpErr(help, fmt.Errorf("cli: rm requires at least one id"))
	}

	ids := make([]int64, len(operands))
	for i, s := range operands {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return helpErr(help, fmt.Errorf("cli: invalid bookmark id %q: %w", s, err))
		}
		ids[i] = id
	}
	p.RemoveIDs = ids
	return p, nil
}

func parseTag(rest []string) (*Parsed, error) {
	lookahead := newFlagSet("tag")
	registerGlobalFlags(lookahead)

	name, rest2, err := SplitCommand(lookahead, rest)
	if err != nil {
		return nil, fmt.Errorf("cli: %w", err)
	}

	switch name {
	case "list":
		return parseTagList(rest2)
	case "refresh":
		return parseTagRefresh(rest2)
	default:
		return nil, fmt.Errorf("cli: unknown tag subcommand %q", name)
	}
}

func parseTagList(rest []string) (*Parsed, error) {
	fs := newFlagSet("tag list")
	g := registerGlobalFlags(fs)
	sortKey := fs.String("sort", "", "sort key: name or count")
	reverse := fs.Bool("reverse", false, "reverse the sort order")

	help := commandHelp(fs, "tsundoku tag list [OPTIONS]")
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: "tag list"}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	if len(fs.Args()) > 0 {
		return helpErr(help, fmt.Errorf("cli: tag list takes no positional arguments"))
	}

	seen := visited(fs)
	if seen["sort"] && !tagSortKeys[*sortKey] {
		return helpErr(help, fmt.Errorf("cli: --sort must be one of name, count; got %q", *sortKey))
	}

	p.TagSort = *sortKey
	p.SetTagSort = seen["sort"]
	p.TagReverse = *reverse
	p.SetTagReverse = seen["reverse"]

	return p, nil
}

func parseTagRefresh(rest []string) (*Parsed, error) {
	fs := newFlagSet("tag refresh")
	g := registerGlobalFlags(fs)
	dryRun := fs.Bool("dry-run", false, "show planned changes without writing them")
	prune := fs.Bool("prune", false, "remove tags that no longer match any filter")

	help := commandHelp(fs, "tsundoku tag refresh [OPTIONS]") + tagRefreshPruneWarning
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: "tag refresh"}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	if len(fs.Args()) > 0 {
		return helpErr(help, fmt.Errorf("cli: tag refresh takes no positional arguments"))
	}

	p.RefreshDryRun = *dryRun
	p.RefreshPrune = *prune

	return p, nil
}

func parseSkill(rest []string) (*Parsed, error) {
	lookahead := newFlagSet("skill")
	registerGlobalFlags(lookahead)

	name, rest2, err := SplitCommand(lookahead, rest)
	if err != nil {
		return nil, fmt.Errorf("cli: %w", err)
	}

	switch name {
	case "install":
		return parseSkillCommand("skill install", "tsundoku skill install [OPTIONS]", rest2)
	case "uninstall":
		return parseSkillCommand("skill uninstall", "tsundoku skill uninstall [OPTIONS]", rest2)
	default:
		return nil, fmt.Errorf("cli: unknown skill subcommand %q", name)
	}
}

func parseSkillCommand(command, usage string, rest []string) (*Parsed, error) {
	fs := newFlagSet(command)
	g := registerGlobalFlags(fs)

	help := commandHelp(fs, usage)
	fs.Usage = func() { fmt.Fprint(fs.Output(), help) }

	if err := fs.Parse(Permute(fs, rest)); err != nil {
		return helpErr(help, fmt.Errorf("cli: %w", err))
	}

	p := &Parsed{Command: command}
	applyGlobal(p, g)
	if p.Help {
		p.HelpText = help
		return p, nil
	}
	if p.Version {
		return p, nil
	}

	if len(fs.Args()) > 0 {
		return helpErr(help, fmt.Errorf("cli: %s takes no positional arguments", command))
	}

	return p, nil
}

// Usage returns the top-level help text.
func Usage() string {
	return `tsundoku <COMMAND> [OPTIONS]

Commands:
  init                  create the config file and database
  add <URL>             add a bookmark
  list                  list bookmarks
  show <ID>             show one bookmark and mark it read
  show unread           show unread bookmarks in order and mark them read
  random [N]            show N random unread bookmarks and mark them read
  rm <ID>...            remove one or more bookmarks
  tag list              list tags with counts
  tag refresh           reapply filters to existing bookmarks
  skill install         install the agent skill document
  skill uninstall       remove the installed agent skill document

Global options:
  --db <PATH>           override the database path
  --config <PATH>       override the config file path
  -q, --quiet           suppress warnings and notes
  -h, --help            show help
  --version             show version
`
}
