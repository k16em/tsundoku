package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/k16em/tsundoku/internal/cli"
	"github.com/k16em/tsundoku/internal/config"
	"github.com/k16em/tsundoku/internal/filter"
	"github.com/k16em/tsundoku/internal/normalize"
	"github.com/k16em/tsundoku/internal/output"
	"github.com/k16em/tsundoku/internal/paths"
	"github.com/k16em/tsundoku/internal/store"
)

var Version = "dev"

// Run executes one command and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		io.WriteString(stderr, cli.Usage())
		return 1
	}

	parsed, err := cli.Parse(args)
	if err != nil {
		code := reportErr(stderr, err)
		if parsed != nil && parsed.HelpText != "" {
			fmt.Fprintln(stderr)
			io.WriteString(stderr, parsed.HelpText)
		}
		return code
	}

	if parsed.Help {
		text := parsed.HelpText
		if text == "" {
			text = cli.Usage()
		}
		if _, err := io.WriteString(stdout, text); err != nil {
			return reportErr(stderr, err)
		}
		return 0
	}
	if parsed.Version {
		if _, err := fmt.Fprintf(stdout, "tsundoku %s\n", Version); err != nil {
			return reportErr(stderr, err)
		}
		return 0
	}

	notify := func(format string, a ...any) {
		if parsed.Global.Quiet {
			return
		}
		fmt.Fprintf(stderr, format+"\n", a...)
	}

	if parsed.Command == "init" {
		return runInit(parsed, stdout, stderr, notify)
	}

	env := paths.OSEnv()

	cfgPath, err := paths.ConfigFile(env, parsed.Global.Config)
	if err != nil {
		return reportErr(stderr, err)
	}
	cfg, err := config.Load(cfgPath, parsed.Global.Config != "", &quietWriter{w: stderr, quiet: parsed.Global.Quiet})
	if err != nil {
		return reportErr(stderr, err)
	}

	dbPath, err := paths.DBFile(env, parsed.Global.DB)
	if err != nil {
		return reportErr(stderr, err)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		return reportOpenErr(stderr, err)
	}
	defer st.Close()

	switch parsed.Command {
	case "add":
		return runAdd(st, cfg, parsed, stdout, stderr)
	case "list":
		return runList(st, cfg, parsed, stdout, stderr)
	case "show":
		return runShow(st, parsed, stdout, stderr, notify)
	case "random":
		return runRandom(st, parsed, stdout, stderr, notify)
	case "rm":
		return runRemove(st, parsed, stdout, stderr, notify)
	case "tag list":
		return runTagList(st, cfg, parsed, stdout, stderr)
	case "tag refresh":
		return runTagRefresh(st, cfg, parsed, stdout, stderr, notify)
	default:
		return reportErr(stderr, fmt.Errorf("app: unknown command %q", parsed.Command))
	}
}

type quietWriter struct {
	w     io.Writer
	quiet bool
}

func (q *quietWriter) Write(p []byte) (int, error) {
	if q.quiet {
		return len(p), nil
	}
	return q.w.Write(p)
}

func reportErr(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "error: %s\n", err)
	return 1
}

func reportOpenErr(stderr io.Writer, err error) int {
	if errors.Is(err, store.ErrSchemaTooNew) {
		return reportErr(stderr, fmt.Errorf("%w; upgrade tsundoku", err))
	}
	return reportErr(stderr, err)
}

func runInit(parsed *cli.Parsed, stdout, stderr io.Writer, notify func(string, ...any)) int {
	env := paths.OSEnv()

	cfgPath, err := paths.ConfigFile(env, parsed.Global.Config)
	if err != nil {
		return reportErr(stderr, err)
	}

	switch _, statErr := os.Stat(cfgPath); {
	case statErr == nil:
		notify("note: config already exists, not overwriting: %s", cfgPath)
	case os.IsNotExist(statErr):
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			return reportErr(stderr, err)
		}
		if err := os.WriteFile(cfgPath, []byte(config.Template()), 0o644); err != nil {
			return reportErr(stderr, err)
		}
	default:
		return reportErr(stderr, statErr)
	}

	dbPath, err := paths.DBFile(env, parsed.Global.DB)
	if err != nil {
		return reportErr(stderr, err)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		return reportOpenErr(stderr, err)
	}
	defer st.Close()

	if _, err := fmt.Fprintf(stdout, "config: %s\n", cfgPath); err != nil {
		return reportErr(stderr, err)
	}
	if _, err := fmt.Fprintf(stdout, "db:     %s\n", dbPath); err != nil {
		return reportErr(stderr, err)
	}
	return 0
}

func runAdd(st *store.Store, cfg config.Config, parsed *cli.Parsed, stdout, stderr io.Writer) int {
	url, err := normalize.URL(parsed.AddURL)
	if err != nil {
		return reportErr(stderr, err)
	}

	explicitTags := make([]string, 0, len(parsed.AddTags))
	for _, t := range parsed.AddTags {
		nt, err := normalize.Tag(t)
		if err != nil {
			return reportErr(stderr, err)
		}
		explicitTags = append(explicitTags, nt)
	}
	explicitTags = normalize.DedupTags(explicitTags)

	filterTags := filter.Match(cfg.Compiled, url)

	combined := append(append([]string{}, explicitTags...), filterTags...)
	allTags := normalize.DedupTags(combined)

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := st.Add(url, allTags, now)
	if err != nil {
		return reportErr(stderr, err)
	}

	if err := output.PrintAdd(stdout, res); err != nil {
		return reportErr(stderr, err)
	}
	return 0
}

func runList(st *store.Store, cfg config.Config, parsed *cli.Parsed, stdout, stderr io.Writer) int {
	limit := parsed.ListLimit
	if !parsed.SetListLimit {
		limit = cfg.List.Limit
	}
	limit = normalize.ClampLimit(limit)

	sortKey := parsed.ListSort
	if !parsed.SetListSort {
		sortKey = cfg.List.Sort
	}

	reverse := parsed.ListReverse
	if !parsed.SetListReverse {
		reverse = cfg.List.Reverse
	}

	tags := make([]string, 0, len(parsed.ListTags))
	for _, t := range parsed.ListTags {
		nt, err := normalize.Tag(t)
		if err != nil {
			return reportErr(stderr, err)
		}
		tags = append(tags, nt)
	}
	tags = normalize.DedupTags(tags)

	bs, err := st.List(store.ListFilter{
		Tags:    tags,
		Limit:   limit,
		Read:    parsed.ListRead,
		Sort:    sortKey,
		Reverse: reverse,
	})
	if err != nil {
		return reportErr(stderr, err)
	}

	if err := output.PrintList(stdout, bs); err != nil {
		return reportErr(stderr, err)
	}
	return 0
}

func runShow(st *store.Store, parsed *cli.Parsed, stdout, stderr io.Writer, notify func(string, ...any)) int {
	if parsed.ShowUnread {
		return runShowUnread(st, parsed, stdout, stderr, notify)
	}

	b, err := st.Get(parsed.ShowID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return reportErr(stderr, fmt.Errorf("bookmark %d not found", parsed.ShowID))
		}
		return reportErr(stderr, err)
	}

	if !parsed.ShowFrozen {
		b.Read = true
	}

	var werr error
	if parsed.ShowJSON {
		werr = output.PrintShowJSON(stdout, b)
	} else {
		werr = output.PrintShow(stdout, b)
	}
	if werr != nil {
		return reportErr(stderr, werr)
	}

	if !parsed.ShowFrozen {
		if err := st.MarkRead(b.ID); err != nil {
			return reportErr(stderr, fmt.Errorf("mark bookmark %d read: %w", b.ID, err))
		}
	}

	return 0
}

func runShowUnread(st *store.Store, parsed *cli.Parsed, stdout, stderr io.Writer, notify func(string, ...any)) int {
	limit := normalize.ClampLimit(parsed.ShowLimit)
	bs, err := st.Unread(limit)
	if err != nil {
		return reportErr(stderr, err)
	}
	return emitUnread(st, bs, parsed.ShowFrozen, parsed.ShowJSON, stdout, stderr, notify)
}

func runRandom(st *store.Store, parsed *cli.Parsed, stdout, stderr io.Writer, notify func(string, ...any)) int {
	limit := normalize.ClampLimit(parsed.RandomCount)
	bs, err := st.Random(limit)
	if err != nil {
		return reportErr(stderr, err)
	}
	return emitUnread(st, bs, parsed.RandomFrozen, parsed.RandomJSON, stdout, stderr, notify)
}

func emitUnread(st *store.Store, bs []store.Bookmark, frozen, jsonOut bool, stdout, stderr io.Writer, notify func(string, ...any)) int {
	if len(bs) == 0 && !jsonOut {
		notify("note: no unread bookmarks")
		return 0
	}

	if !frozen {
		for i := range bs {
			bs[i].Read = true
		}
	}

	if jsonOut {
		if err := output.PrintShowJSONList(stdout, bs); err != nil {
			return reportErr(stderr, err)
		}
		if !frozen {
			for _, b := range bs {
				if err := st.MarkRead(b.ID); err != nil {
					return reportErr(stderr, fmt.Errorf("mark bookmark %d read: %w", b.ID, err))
				}
			}
		}
		return 0
	}

	for i, b := range bs {
		if i > 0 {
			if err := output.PrintSeparator(stdout); err != nil {
				return reportErr(stderr, err)
			}
		}
		if err := output.PrintShow(stdout, b); err != nil {
			return reportErr(stderr, err)
		}
		if !frozen {
			if err := st.MarkRead(b.ID); err != nil {
				return reportErr(stderr, fmt.Errorf("mark bookmark %d read: %w", b.ID, err))
			}
		}
	}

	return 0
}

func runRemove(st *store.Store, parsed *cli.Parsed, stdout, stderr io.Writer, notify func(string, ...any)) int {
	removed, missing, err := st.Remove(parsed.RemoveIDs)
	if err != nil {
		return reportErr(stderr, err)
	}

	for _, b := range removed {
		if err := output.PrintRemoved(stdout, b); err != nil {
			return reportErr(stderr, err)
		}
	}

	if len(missing) > 0 {
		notify("warning: bookmark(s) not found: %s", formatIDs(missing))
		return 1
	}

	return 0
}

func runTagList(st *store.Store, cfg config.Config, parsed *cli.Parsed, stdout, stderr io.Writer) int {
	sortKey := parsed.TagSort
	if !parsed.SetTagSort {
		sortKey = cfg.Tags.Sort
	}
	reverse := parsed.TagReverse
	if !parsed.SetTagReverse {
		reverse = cfg.Tags.Reverse
	}

	tcs, err := st.TagCounts(sortKey, reverse)
	if err != nil {
		return reportErr(stderr, err)
	}

	if err := output.PrintTagCounts(stdout, tcs); err != nil {
		return reportErr(stderr, err)
	}
	return 0
}

func runTagRefresh(st *store.Store, cfg config.Config, parsed *cli.Parsed, stdout, stderr io.Writer, notify func(string, ...any)) int {
	bs, err := st.All()
	if err != nil {
		return reportErr(stderr, err)
	}

	changes := store.PlanRefresh(bs, cfg.Compiled, parsed.RefreshPrune)

	if len(changes) == 0 {
		notify("note: no tag changes")
		return 0
	}

	if err := output.PrintRefresh(stdout, changes, parsed.RefreshDryRun); err != nil {
		return reportErr(stderr, err)
	}

	if parsed.RefreshDryRun {
		return 0
	}

	if err := st.ApplyRefresh(changes); err != nil {
		return reportErr(stderr, err)
	}

	return 0
}

func formatIDs(ids []int64) string {
	strs := make([]string, len(ids))
	for i, id := range ids {
		strs[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(strs, ", ")
}
