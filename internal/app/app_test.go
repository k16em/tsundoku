package app

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/k16em/tsundoku/internal/output"
	"github.com/k16em/tsundoku/internal/store"
)

func newPaths(t *testing.T) (dbPath, cfgPath string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "tsundoku.db"), filepath.Join(dir, "config.toml")
}

func run(dbPath, cfgPath string, args ...string) (stdout, stderr string, code int) {
	full := append([]string{"--db", dbPath, "--config", cfgPath}, args...)
	var out, errBuf bytes.Buffer
	code = Run(full, &out, &errBuf)
	return out.String(), errBuf.String(), code
}

func mustRun(t *testing.T, dbPath, cfgPath string, args ...string) (stdout, stderr string) {
	t.Helper()
	stdout, stderr, code := run(dbPath, cfgPath, args...)
	if code != 0 {
		t.Fatalf("Run(%v) exit = %d, want 0; stdout=%q stderr=%q", args, code, stdout, stderr)
	}
	return stdout, stderr
}

func openStore(t *testing.T, dbPath string) *store.Store {
	t.Helper()
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open(%q) error = %v", dbPath, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

type failAfterNWrites struct {
	buf    bytes.Buffer
	calls  int
	failOn int
}

func (w *failAfterNWrites) Write(p []byte) (int, error) {
	w.calls++
	if w.calls >= w.failOn {
		return 0, errors.New("simulated write failure")
	}
	return w.buf.Write(p)
}

type failAfterNBytes struct {
	buf     bytes.Buffer
	allowed int
	written int
}

func (w *failAfterNBytes) Write(p []byte) (int, error) {
	if w.written >= w.allowed {
		return 0, errors.New("simulated write failure")
	}
	remaining := w.allowed - w.written
	if len(p) <= remaining {
		n, err := w.buf.Write(p)
		w.written += n
		return n, err
	}
	n, err := w.buf.Write(p[:remaining])
	w.written += n
	if err != nil {
		return n, err
	}
	return n, errors.New("simulated write failure")
}

func TestInitThenAddThenListProducesExpectedOutput(t *testing.T) {
	dbPath, cfgPath := newPaths(t)

	mustRun(t, dbPath, cfgPath, "init")

	stdout, _ := mustRun(t, dbPath, cfgPath, "add", "https://example.com/a", "--tag", "blog", "--tag", "golang")
	if !strings.HasPrefix(stdout, "added   [1] https://example.com/a  #blog #golang") {
		t.Errorf("add stdout = %q", stdout)
	}

	stdout, _ = mustRun(t, dbPath, cfgPath, "list")
	if !strings.Contains(stdout, "[1]") || !strings.Contains(stdout, "https://example.com/a") {
		t.Errorf("list stdout = %q, want it to contain the added bookmark", stdout)
	}
	if !strings.HasPrefix(stdout, "*") {
		t.Errorf("list stdout = %q, want the unread marker *", stdout)
	}
}

func TestReAddingTheSameURLReportsUpdated(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a", "--tag", "blog")

	stdout, _ := mustRun(t, dbPath, cfgPath, "add", "https://example.com/a", "--tag", "rust")
	if !strings.HasPrefix(stdout, "updated [1] https://example.com/a  +1 tag") {
		t.Errorf("add stdout = %q", stdout)
	}
}

func TestListWithMultipleTagsANDsTheFilter(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a", "--tag", "blog", "--tag", "golang")
	mustRun(t, dbPath, cfgPath, "add", "https://example.org/b", "--tag", "blog")

	stdout, _ := mustRun(t, dbPath, cfgPath, "list", "--tag", "blog", "--tag", "golang")
	if !strings.Contains(stdout, "example.com/a") {
		t.Errorf("stdout = %q, want it to contain example.com/a", stdout)
	}
	if strings.Contains(stdout, "example.org/b") {
		t.Errorf("stdout = %q, want it to not contain example.org/b", stdout)
	}
}

func TestListWithAnExplicitEmptySortValueIsAnErrorRatherThanSilentlyDefaulting(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, stderr, code := run(dbPath, cfgPath, "list", "--sort=")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", stderr)
	}
}

func TestTagListWithAnUnknownSortValueIsAnError(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, stderr, code := run(dbPath, cfgPath, "tag", "list", "--sort", "bogus")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", stderr)
	}
}

func TestShowMarksTheBookmarkReadAndItThenAppearsInListRead(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	mustRun(t, dbPath, cfgPath, "show", "1")

	stdout, _ := mustRun(t, dbPath, cfgPath, "list", "--read")
	if !strings.Contains(stdout, "example.com/a") {
		t.Errorf("list --read stdout = %q, want the now-read bookmark", stdout)
	}
}

func TestShowFrozenDoesNotMarkRead(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	mustRun(t, dbPath, cfgPath, "show", "1", "--frozen")

	stdout, _ := mustRun(t, dbPath, cfgPath, "list", "--unread")
	if !strings.Contains(stdout, "example.com/a") {
		t.Errorf("list --unread stdout = %q, want the still-unread bookmark", stdout)
	}
}

func TestMarkReadFailureIsAnErrorAndIsNotSuppressedByQuiet(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER block_mark_read BEFORE UPDATE OF read ON bookmarks
		BEGIN
			SELECT RAISE(ABORT, 'blocked by test trigger');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"--db", dbPath, "--config", cfgPath, "show", "1", "--quiet"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}

	if !strings.Contains(stdout.String(), "https://example.com/a") {
		t.Fatalf("stdout = %q, want it to contain the displayed bookmark, proving the read+display succeeded before the mark-read attempt", stdout.String())
	}

	if !strings.HasPrefix(stderr.String(), "error: ") {
		t.Errorf("stderr = %q, want the mark-read failure reported as an error even under --quiet", stderr.String())
	}
	if !strings.Contains(stderr.String(), "mark bookmark 1 read") {
		t.Errorf("stderr = %q, want it to identify the mark-read failure specifically, not just report a generic error", stderr.String())
	}

	s := openStore(t, dbPath)
	got, err := s.Get(1)
	if err != nil {
		t.Fatalf("Get(1) error = %v", err)
	}
	if got.Read {
		t.Errorf("bookmark 1 Read = true, want false: the mark-read attempt failed and must not have taken effect")
	}
}

func TestShowJSONOnAnUnreadBookmarkReportsReadTrue(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	stdout, _ := mustRun(t, dbPath, cfgPath, "show", "1", "--json")
	if !strings.Contains(stdout, `"read": true`) {
		t.Errorf("show --json stdout = %q, want it to report read: true", stdout)
	}
}

func TestShowJSONFrozenOnAnUnreadBookmarkReportsReadFalse(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	stdout, _ := mustRun(t, dbPath, cfgPath, "show", "1", "--json", "--frozen")
	if !strings.Contains(stdout, `"read": false`) {
		t.Errorf("show --json --frozen stdout = %q, want it to report read: false", stdout)
	}
}

func TestShowTextOnAnUnreadBookmarkReportsReadYesAndFrozenReportsNo(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/b")

	stdout, _ := mustRun(t, dbPath, cfgPath, "show", "1")
	if !strings.Contains(stdout, "Read  : yes") {
		t.Errorf("show stdout = %q, want Read  : yes", stdout)
	}

	stdout, _ = mustRun(t, dbPath, cfgPath, "show", "2", "--frozen")
	if !strings.Contains(stdout, "Read  : no") {
		t.Errorf("show --frozen stdout = %q, want Read  : no", stdout)
	}
}

func TestShowUnreadMarksAllElementsReadInJSONMode(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/b")

	stdout, _ := mustRun(t, dbPath, cfgPath, "show", "unread", "--json")
	if strings.Count(stdout, `"read": true`) != 2 {
		t.Errorf("show unread --json stdout = %q, want two read: true entries", stdout)
	}

	s := openStore(t, dbPath)
	for _, id := range []int64{1, 2} {
		b, err := s.Get(id)
		if err != nil {
			t.Fatalf("Get(%d) error = %v", id, err)
		}
		if !b.Read {
			t.Errorf("bookmark %d Read = false, want true", id)
		}
	}
}

func TestShowUnreadStopsMarkingAtTheFirstWriteFailure(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/b")

	s := openStore(t, dbPath)
	unread, err := s.Unread(10)
	if err != nil {
		t.Fatalf("Unread() error = %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("len(Unread()) = %d, want 2", len(unread))
	}
	b1, b2 := unread[0], unread[1]
	b1.Read, b2.Read = true, true

	block1 := output.FormatShow(b1)
	separator := strings.Repeat("=", 64) + "\n"
	block2 := output.FormatShow(b2)
	budget := len(block1) + len(separator) + len(block2)/2
	if budget <= len(block1)+len(separator) || budget >= len(block1)+len(separator)+len(block2) {
		t.Fatalf("test setup error: budget %d does not land inside the second block", budget)
	}

	w := &failAfterNBytes{allowed: budget}
	var errBuf bytes.Buffer
	code := Run([]string{"--db", dbPath, "--config", cfgPath, "show", "unread"}, w, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if w.buf.Len() != budget {
		t.Fatalf("bytes actually written = %d, want exactly %d (partial write into the second block)", w.buf.Len(), budget)
	}

	got1, err := s.Get(b1.ID)
	if err != nil {
		t.Fatalf("Get(%d) error = %v", b1.ID, err)
	}
	if !got1.Read {
		t.Errorf("bookmark %d Read = false, want true (first block was written successfully)", b1.ID)
	}
	got2, err := s.Get(b2.ID)
	if err != nil {
		t.Fatalf("Get(%d) error = %v", b2.ID, err)
	}
	if got2.Read {
		t.Errorf("bookmark %d Read = true, want false (its block only partially wrote)", b2.ID)
	}
}

func TestShowUnreadJSONMarksNothingWhenTheWriteFailsMidArray(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/b")

	s := openStore(t, dbPath)
	unread, err := s.Unread(10)
	if err != nil {
		t.Fatalf("Unread() error = %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("len(Unread()) = %d, want 2", len(unread))
	}
	for i := range unread {
		unread[i].Read = true
	}
	full, err := output.FormatShowJSONList(unread)
	if err != nil {
		t.Fatalf("FormatShowJSONList() error = %v", err)
	}
	budget := len(full) / 2
	if budget == 0 {
		t.Fatalf("test setup error: full JSON array is too short to fail mid-array")
	}

	w := &failAfterNBytes{allowed: budget}
	var errBuf bytes.Buffer
	code := Run([]string{"--db", dbPath, "--config", cfgPath, "show", "unread", "--json"}, w, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if w.buf.Len() != budget {
		t.Fatalf("bytes actually written = %d, want exactly %d (partial write mid-array)", w.buf.Len(), budget)
	}
	if w.buf.Len() == 0 || w.buf.Len() >= len(full) {
		t.Fatalf("bytes actually written = %d, want strictly between 0 and %d (the full array length)", w.buf.Len(), len(full))
	}

	for _, b := range unread {
		got, err := s.Get(b.ID)
		if err != nil {
			t.Fatalf("Get(%d) error = %v", b.ID, err)
		}
		if got.Read {
			t.Errorf("bookmark %d Read = true, want false (the JSON array write failed mid-array)", b.ID)
		}
	}
}

func TestRemoveDeletesOrphanedTags(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a", "--tag", "golang")

	mustRun(t, dbPath, cfgPath, "rm", "1")

	stdout, _ := mustRun(t, dbPath, cfgPath, "tag", "list")
	if strings.Contains(stdout, "golang") {
		t.Errorf("tag list stdout = %q, want golang to be gone after rm", stdout)
	}
}

func TestRemoveWithAMissingIDStillRemovesTheExistingOnesAndExitsNonZero(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	stdout, stderr, code := run(dbPath, cfgPath, "rm", "1", "999")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout, "removed") || !strings.Contains(stdout, "example.com/a") {
		t.Errorf("stdout = %q, want the existing bookmark to be reported removed", stdout)
	}
	if stderr == "" {
		t.Errorf("stderr is empty, want a warning about the missing id")
	}
	if strings.Contains(stderr, "error: ") {
		t.Errorf("stderr = %q, missing-id is a warning, not an error", stderr)
	}

	s := openStore(t, dbPath)
	if _, err := s.Get(1); err == nil {
		t.Errorf("bookmark 1 still exists, want it removed")
	}
}

func TestTagRefreshDryRunDoesNotChangeTheDatabase(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	mustRun(t, dbPath, cfgPath, "add", "https://example.com/blog/x")
	writeFilterConfig(t, cfgPath)

	before, _ := mustRun(t, dbPath, cfgPath, "tag", "list")

	stdout, _ := mustRun(t, dbPath, cfgPath, "tag", "refresh", "--dry-run")
	if !strings.Contains(stdout, "(dry-run)") {
		t.Errorf("tag refresh --dry-run stdout = %q, want it to mention dry-run", stdout)
	}

	after, _ := mustRun(t, dbPath, cfgPath, "tag", "list")
	if before != after {
		t.Errorf("tag list changed across dry-run: before=%q after=%q", before, after)
	}
}

func TestTagRefreshPruneRemovesManuallyAddedTagsThatMatchNoFilter(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a", "--tag", "manual")

	writeFilterConfig(t, cfgPath)

	mustRun(t, dbPath, cfgPath, "tag", "refresh", "--prune")

	stdout, _ := mustRun(t, dbPath, cfgPath, "tag", "list")
	if strings.Contains(stdout, "manual") {
		t.Errorf("tag list stdout = %q, want the manual tag pruned", stdout)
	}
}

func TestAddWithAnInvalidURLExitsNonZero(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, stderr, code := run(dbPath, cfgPath, "add", "not-a-url")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", stderr)
	}
}

func TestShowOfAMissingIDExitsNonZero(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, _, code := run(dbPath, cfgPath, "show", "999")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestNotesAndWarningsGoToStderrNeverStdout(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	stdout, stderr, code := run(dbPath, cfgPath, "show", "unread")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty when there are no unread bookmarks", stdout)
	}
	if !strings.Contains(stderr, "note: no unread bookmarks") {
		t.Errorf("stderr = %q, want the no-unread note", stderr)
	}
}

func TestQuietSuppressesNotesButNotErrors(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, stderr, code := run(dbPath, cfgPath, "show", "unread", "--quiet")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty because --quiet suppresses the note", stderr)
	}

	_, stderr, code = run(dbPath, cfgPath, "show", "999", "--quiet")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want the error to still be printed under --quiet", stderr)
	}
}

func TestListWithANonexistentExplicitConfigPathExitsNonZero(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	_, _, code := run(dbPath, cfgPath, "list")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 because --config points at a nonexistent path", code)
	}
}

func TestInitWithANonexistentExplicitConfigPathSucceeds(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	_, _, code := run(dbPath, cfgPath, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0: init treats a missing --config path as the normal case", code)
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("config file was not created: %v", err)
	}
}

func TestAddPicksUpAutomaticTagsFromConfigFilters(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	writeFilterConfig(t, cfgPath)

	stdout, _ := mustRun(t, dbPath, cfgPath, "add", "https://example.com/blog/x")
	if !strings.Contains(stdout, "#blog") {
		t.Errorf("add stdout = %q, want the blog filter tag applied", stdout)
	}
}

func TestDBPathContainingAQuestionMarkExitsNonZeroAndCreatesNoFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	mustRun(t, filepath.Join(dir, "unused.db"), cfgPath, "init")

	dbPath := filepath.Join(dir, "a?b.db")

	_, stderr, code := run(dbPath, cfgPath, "list")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", stderr)
	}
	if !strings.Contains(stderr, "?") {
		t.Errorf("stderr = %q, want it to name the '?' path problem", stderr)
	}
	if !strings.Contains(stderr, "database path") {
		t.Errorf("stderr = %q, want it to explain the database path is rejected", stderr)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() == "a" {
			t.Errorf("file %q was created, want no file created for a rejected db path", filepath.Join(dir, "a"))
		}
	}
}

func writeFilterConfig(t *testing.T, path string) {
	t.Helper()
	content := "[[filter]]\nname = \"blog\"\nrule = \"*blog*\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}

func TestRmWithNonNumericArgumentIsRejectedByParseBeforeTouchingTheStore(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	_, _, code := run(dbPath, cfgPath, "rm", "abc")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}

	s := openStore(t, dbPath)
	if _, err := s.Get(1); err != nil {
		t.Errorf("bookmark 1 should still exist: %v", err)
	}
}

func TestHelpFlagPrintsUsageAndExitsZero(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"list", "--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.Len() == 0 {
		t.Errorf("stdout is empty, want usage text")
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
}

func TestEachSubcommandHelpPrintsItsOwnTextToStdoutAndExitsZero(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantContain string
	}{
		{"init", []string{"init", "--help"}, "tsundoku init"},
		{"add", []string{"add", "--help"}, "tsundoku add"},
		{"list", []string{"list", "--help"}, "tsundoku list"},
		{"show", []string{"show", "--help"}, "tsundoku show"},
		{"random", []string{"random", "--help"}, "tsundoku random"},
		{"rm", []string{"rm", "--help"}, "tsundoku rm"},
		{"tag list", []string{"tag", "list", "--help"}, "tsundoku tag list"},
		{"tag refresh", []string{"tag", "refresh", "--help"}, "tsundoku tag refresh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errBuf bytes.Buffer
			code := Run(tt.args, &out, &errBuf)
			if code != 0 {
				t.Fatalf("Run(%v) exit code = %d, want 0", tt.args, code)
			}
			if errBuf.Len() != 0 {
				t.Errorf("Run(%v) stderr = %q, want empty", tt.args, errBuf.String())
			}
			if !strings.Contains(out.String(), tt.wantContain) {
				t.Errorf("Run(%v) stdout = %q, want it to contain %q", tt.args, out.String(), tt.wantContain)
			}
		})
	}
}

func TestSubcommandHelpOutputDiffersFromBareHelpOutput(t *testing.T) {
	var bare bytes.Buffer
	var errBuf bytes.Buffer
	if code := Run([]string{"--help"}, &bare, &errBuf); code != 0 {
		t.Fatalf("Run([--help]) exit code = %d, want 0", code)
	}

	var listHelp bytes.Buffer
	if code := Run([]string{"list", "--help"}, &listHelp, &errBuf); code != 0 {
		t.Fatalf("Run([list --help]) exit code = %d, want 0", code)
	}

	if bare.String() == listHelp.String() {
		t.Errorf("list --help produced the same output as bare --help, want subcommand-specific text")
	}
}

func TestBothTagSubcommandsGetDistinctHelpText(t *testing.T) {
	var listOut, refreshOut, errBuf bytes.Buffer
	if code := Run([]string{"tag", "list", "--help"}, &listOut, &errBuf); code != 0 {
		t.Fatalf("Run([tag list --help]) exit code = %d, want 0", code)
	}
	if code := Run([]string{"tag", "refresh", "--help"}, &refreshOut, &errBuf); code != 0 {
		t.Fatalf("Run([tag refresh --help]) exit code = %d, want 0", code)
	}
	if listOut.String() == refreshOut.String() {
		t.Errorf("tag list --help and tag refresh --help produced identical output")
	}
	if !strings.Contains(listOut.String(), "tag list") {
		t.Errorf("tag list --help stdout = %q, want it to mention tag list", listOut.String())
	}
	if !strings.Contains(refreshOut.String(), "tag refresh") {
		t.Errorf("tag refresh --help stdout = %q, want it to mention tag refresh", refreshOut.String())
	}
}

func TestTagRefreshHelpWarnsAboutPruneBeingDestructive(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"tag", "refresh", "--help"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	text := out.String()
	if !strings.Contains(text, "--prune") {
		t.Errorf("stdout = %q, want it to mention --prune", text)
	}
	if !strings.Contains(text, "--dry-run") {
		t.Errorf("stdout = %q, want it to recommend --dry-run", text)
	}
}

func TestVersionFlagPrintsToStdoutAndExitsZero(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"list", "--version"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.Len() == 0 {
		t.Errorf("stdout is empty, want a version string")
	}
}

func TestVersionOutputIsExactlyTsundokuSpaceVersion(t *testing.T) {
	old := Version
	Version = "v0.1.0"
	defer func() { Version = old }()

	var out, errBuf bytes.Buffer
	code := Run([]string{"list", "--version"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.String() != "tsundoku v0.1.0\n" {
		t.Errorf("stdout = %q, want %q", out.String(), "tsundoku v0.1.0\n")
	}
}

func TestVersionDefaultsToDev(t *testing.T) {
	if Version != "dev" {
		t.Errorf("Version = %q, want the default %q (unless overridden at build time)", Version, "dev")
	}
}

func TestBareHelpFlagWithNoSubcommandPrintsUsageToStdoutAndExitsZero(t *testing.T) {
	tests := [][]string{
		{"--help"},
		{"-h"},
	}
	for _, args := range tests {
		var out, errBuf bytes.Buffer
		code := Run(args, &out, &errBuf)
		if code != 0 {
			t.Errorf("Run(%v) exit code = %d, want 0", args, code)
		}
		if out.Len() == 0 {
			t.Errorf("Run(%v) stdout is empty, want usage text", args)
		}
		if errBuf.Len() != 0 {
			t.Errorf("Run(%v) stderr = %q, want empty", args, errBuf.String())
		}
	}
}

func TestBareVersionFlagWithNoSubcommandPrintsToStdoutAndExitsZero(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"--version"}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.Len() == 0 {
		t.Errorf("stdout is empty, want a version string")
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errBuf.String())
	}
}

func TestNoArgumentsAtAllPrintsUsageToStderrAndExitsOne(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run(nil, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if errBuf.Len() == 0 {
		t.Errorf("stderr is empty, want usage text")
	}
}

func TestOrdinaryGlobalFlagsWithNoSubcommandAndNoHelpOrVersionStillError(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"--db", "X", "--quiet"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(errBuf.String(), "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestAddTagsAreDeduplicatedWithExplicitBeforeFilterDerived(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	writeFilterConfig(t, cfgPath)

	stdout, _ := mustRun(t, dbPath, cfgPath, "add", "https://example.com/blog/x", "--tag", "blog")
	if strings.Count(stdout, "#blog") != 1 {
		t.Errorf("add stdout = %q, want #blog listed exactly once", stdout)
	}
}

func TestUnknownStoreIDInShowReportsAnErrorPrefixedMessage(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, stderr, code := run(dbPath, cfgPath, "show", "42")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", stderr)
	}
}

func TestShowOfAMissingIDReportsACleanNotFoundMessageNamingTheID(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, stderr, code := run(dbPath, cfgPath, "show", "42")
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "42") {
		t.Errorf("stderr = %q, want it to name the missing id 42", stderr)
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr = %q, want a clean not-found message", stderr)
	}
}

func TestArgumentParseErrorPrintsAOneLineErrorFollowedBySubcommandHelpAsASeparateBlock(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"list", "--bogus"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}

	lines := strings.SplitN(errBuf.String(), "\n", 2)
	if len(lines) < 1 || !strings.HasPrefix(lines[0], "error: ") {
		t.Fatalf("stderr first line = %q, want it to start with error: ", lines[0])
	}
	if strings.Contains(lines[0], "tsundoku list") {
		t.Errorf("error line = %q, want the help text kept out of the error line itself", lines[0])
	}
	if !strings.Contains(errBuf.String(), "tsundoku list") {
		t.Errorf("stderr = %q, want a separate help block naming the list subcommand", errBuf.String())
	}
}

func TestParseErrorsAreReportedWithErrorPrefixAndExitOne(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := Run([]string{"--db", "X"}, &out, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(errBuf.String(), "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", errBuf.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestShowUnreadTextOrdersOldestFirstAndSeparatesBlocks(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/b")

	stdout, _ := mustRun(t, dbPath, cfgPath, "show", "unread")
	ia := strings.Index(stdout, "example.com/a")
	ib := strings.Index(stdout, "example.com/b")
	if ia == -1 || ib == -1 || ia > ib {
		t.Errorf("stdout = %q, want example.com/a before example.com/b (oldest first)", stdout)
	}
	if !strings.Contains(stdout, strings.Repeat("=", 64)) {
		t.Errorf("stdout = %q, want a 64-character separator between blocks", stdout)
	}
}

func TestConfigFilterWarningsGoToStderrAndAreSuppressedByQuiet(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	content := "[[filter]]\nname = \"blog\"\nrule = \"*blog*\"\nregex = \"^https://news\\\\.\"\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	_, stderr, code := run(dbPath, cfgPath, "list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr == "" {
		t.Errorf("stderr is empty, want a warning about the conflicting rule/regex filter")
	}

	_, stderr, code = run(dbPath, cfgPath, "list", "--quiet")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty because --quiet suppresses config warnings", stderr)
	}
}

func TestOpeningADatabaseWithATooNewSchemaReportsAnActionableError(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 999"); err != nil {
		t.Fatalf("bump user_version: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("db.Close: %v", err)
	}

	_, stderr, code := run(dbPath, cfgPath, "list")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", stderr)
	}
	if !strings.Contains(stderr, "upgrade tsundoku") {
		t.Errorf("stderr = %q, want an actionable upgrade hint", stderr)
	}
}

func TestInitSkipsAnAlreadyExistingConfigFileWithoutOverwritingIt(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	custom := "[list]\nlimit = 5\nsort = \"created\"\nreverse = false\n\n[tags]\nsort = \"name\"\nreverse = false\n"
	if err := os.WriteFile(cfgPath, []byte(custom), 0o644); err != nil {
		t.Fatalf("writing custom config: %v", err)
	}

	stdout, stderr, code := run(dbPath, cfgPath, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, cfgPath) {
		t.Errorf("stdout = %q, want it to report the existing config path", stdout)
	}
	if !strings.Contains(stderr, "note:") {
		t.Errorf("stderr = %q, want a note that the config was not overwritten", stderr)
	}

	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if string(got) != custom {
		t.Errorf("config file was overwritten, want it unchanged")
	}
}

func TestShowUnreadJSONWithNoUnreadBookmarksPrintsAnEmptyArray(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	stdout, stderr := mustRun(t, dbPath, cfgPath, "show", "unread", "--json")
	if strings.TrimSpace(stdout) != "[]" {
		t.Errorf("stdout = %q, want []", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty (no note for --json mode)", stderr)
	}
}

func TestTagRefreshWithNoChangesPrintsNothingToStdoutAndANoteToStderr(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	stdout, stderr, code := run(dbPath, cfgPath, "tag", "refresh")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "note: no tag changes") {
		t.Errorf("stderr = %q, want the no-changes note", stderr)
	}
}

func TestAddWithAControlCharacterInATagIsAnError(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, stderr, code := run(dbPath, cfgPath, "add", "https://example.com/a", "--tag", "bad\x01tag")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", stderr)
	}
}

func TestListWithAControlCharacterInATagIsAnError(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	_, stderr, code := run(dbPath, cfgPath, "list", "--tag", "bad\x01tag")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(stderr, "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", stderr)
	}
}

func TestRemoveWriteFailureStopsBeforeReportingTheMissingIDs(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	w := &failAfterNWrites{failOn: 1}
	var errBuf bytes.Buffer
	code := Run([]string{"--db", dbPath, "--config", cfgPath, "rm", "1"}, w, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(errBuf.String(), "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", errBuf.String())
	}
}

func TestTagRefreshWriteFailureIsReportedAsAnError(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/blog/x")
	writeFilterConfig(t, cfgPath)

	w := &failAfterNWrites{failOn: 1}
	var errBuf bytes.Buffer
	code := Run([]string{"--db", dbPath, "--config", cfgPath, "tag", "refresh"}, w, &errBuf)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.HasPrefix(errBuf.String(), "error: ") {
		t.Errorf("stderr = %q, want it to start with error: ", errBuf.String())
	}
}

func TestTagListReflectsConfigDefaultsWhenNoFlagsAreGiven(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a", "--tag", "later")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/b", "--tag", "later", "--tag", "early")

	stdout, _ := mustRun(t, dbPath, cfgPath, "tag", "list")
	if strings.Index(stdout, "early") > strings.Index(stdout, "later") {
		t.Fatalf("stdout = %q, want built-in default (name asc) to list early before later", stdout)
	}

	content := "[tags]\nsort = \"count\"\nreverse = true\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	stdout, _ = mustRun(t, dbPath, cfgPath, "tag", "list")
	if strings.Index(stdout, "later") > strings.Index(stdout, "early") {
		t.Errorf("stdout = %q, want config's tags.sort=count/reverse=true to list later (count 2) before early (count 1)", stdout)
	}
}

func TestShowUnreadLimitDefaultsToTenAndCanBeOverridden(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	for i := 0; i < 3; i++ {
		mustRun(t, dbPath, cfgPath, "add", "https://example.com/"+strconv.Itoa(i))
	}

	stdout, _ := mustRun(t, dbPath, cfgPath, "show", "unread", "--limit", "2")
	if strings.Count(stdout, "URL   :") != 2 {
		t.Errorf("stdout = %q, want exactly 2 blocks", stdout)
	}
}

func TestRandomReturnsTheRequestedNumberOfUnreadBookmarksAndMarksThemRead(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	for i := 0; i < 4; i++ {
		mustRun(t, dbPath, cfgPath, "add", "https://example.com/"+strconv.Itoa(i))
	}

	stdout, _ := mustRun(t, dbPath, cfgPath, "random", "2")
	if strings.Count(stdout, "URL   :") != 2 {
		t.Fatalf("stdout = %q, want exactly 2 blocks", stdout)
	}

	s := openStore(t, dbPath)
	unread, err := s.Unread(10)
	if err != nil {
		t.Fatalf("Unread() error = %v", err)
	}
	if len(unread) != 2 {
		t.Errorf("%d bookmarks left unread, want 2", len(unread))
	}
}

func TestRandomWithNoCountDrawsASingleBookmark(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/b")

	stdout, _ := mustRun(t, dbPath, cfgPath, "random")
	if strings.Count(stdout, "URL   :") != 1 {
		t.Errorf("stdout = %q, want exactly 1 block", stdout)
	}
}

func TestRandomFrozenDoesNotMarkRead(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")

	stdout, _ := mustRun(t, dbPath, cfgPath, "random", "1", "--frozen")
	if !strings.Contains(stdout, "Read  : no") {
		t.Errorf("stdout = %q, want it to report Read : no", stdout)
	}

	s := openStore(t, dbPath)
	b, err := s.Get(1)
	if err != nil {
		t.Fatalf("Get(1) error = %v", err)
	}
	if b.Read {
		t.Errorf("bookmark 1 Read = true, want false")
	}
}

func TestRandomJSONMarksEveryElementReadUnlessFrozen(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/a")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/b")

	frozen, _ := mustRun(t, dbPath, cfgPath, "random", "2", "-j", "--frozen")
	if strings.Count(frozen, `"read": false`) != 2 {
		t.Errorf("random -j --frozen stdout = %q, want two read: false entries", frozen)
	}

	stdout, _ := mustRun(t, dbPath, cfgPath, "random", "2", "--json")
	if strings.Count(stdout, `"read": true`) != 2 {
		t.Errorf("random --json stdout = %q, want two read: true entries", stdout)
	}

	s := openStore(t, dbPath)
	for _, id := range []int64{1, 2} {
		b, err := s.Get(id)
		if err != nil {
			t.Fatalf("Get(%d) error = %v", id, err)
		}
		if !b.Read {
			t.Errorf("bookmark %d Read = false, want true", id)
		}
	}
}

func TestRandomWithNoUnreadBookmarksNotesToStderrAndPrintsAnEmptyJSONArray(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	stdout, stderr := mustRun(t, dbPath, cfgPath, "random", "3")
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "no unread bookmarks") {
		t.Errorf("stderr = %q, want a no-unread note", stderr)
	}

	jsonOut, _ := mustRun(t, dbPath, cfgPath, "random", "3", "--json")
	if strings.TrimSpace(jsonOut) != "[]" {
		t.Errorf("stdout = %q, want []", jsonOut)
	}
}

func TestRandomDrawsOnlyFromUnreadBookmarks(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/read")
	mustRun(t, dbPath, cfgPath, "add", "https://example.com/unread")
	mustRun(t, dbPath, cfgPath, "show", "1")

	for i := 0; i < 20; i++ {
		stdout, _ := mustRun(t, dbPath, cfgPath, "random", "1", "--frozen")
		if strings.Contains(stdout, "example.com/read") {
			t.Fatalf("stdout = %q, want the already-read bookmark to be excluded", stdout)
		}
	}
}

func TestRandomWithANonNumericCountIsRejectedByParse(t *testing.T) {
	dbPath, cfgPath := newPaths(t)
	mustRun(t, dbPath, cfgPath, "init")

	stdout, stderr, code := run(dbPath, cfgPath, "random", "abc")
	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero")
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "error:") {
		t.Errorf("stderr = %q, want an error: prefix", stderr)
	}
}
