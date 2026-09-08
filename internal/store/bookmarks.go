package store

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Bookmark is one row of the bookmarks table with its tags attached.
type Bookmark struct {
	ID        int64
	URL       string
	Read      bool
	CreatedAt string
	Tags      []string
}

// AddResult reports what Add did.
type AddResult struct {
	ID        int64
	URL       string
	Created   bool
	AddedTags []string
}

// ListFilter selects and orders bookmarks for List.
type ListFilter struct {
	Tags    []string
	Limit   int
	Read    *bool
	Sort    string
	Reverse bool
}

const MaxBookmarks = 10000

var ErrBookmarkLimit = errors.New("bookmark limit of 10000 URLs reached")

var listSortColumns = map[string]string{
	"":        "b.created_at",
	"created": "b.created_at",
	"id":      "b.id",
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// Add inserts a bookmark, or adds tags to the existing row with the same URL.
func (s *Store) Add(url string, tags []string, now string) (AddResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return AddResult{}, fmt.Errorf("begin add transaction: %w", err)
	}
	defer tx.Rollback()

	var id int64
	created := false
	err = tx.QueryRow("SELECT id FROM bookmarks WHERE url = ?", url).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM bookmarks").Scan(&count); err != nil {
			return AddResult{}, fmt.Errorf("count bookmarks: %w", err)
		}
		if count >= MaxBookmarks {
			return AddResult{}, ErrBookmarkLimit
		}
		res, insertErr := tx.Exec("INSERT INTO bookmarks (url, read, created_at) VALUES (?, 0, ?)", url, now)
		if insertErr != nil {
			return AddResult{}, fmt.Errorf("insert bookmark: %w", insertErr)
		}
		id, err = res.LastInsertId()
		if err != nil {
			return AddResult{}, fmt.Errorf("read new bookmark id: %w", err)
		}
		created = true
	case err != nil:
		return AddResult{}, fmt.Errorf("look up bookmark %q: %w", url, err)
	}

	added := make([]string, 0, len(tags))
	seen := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if seen[tag] {
			continue
		}
		seen[tag] = true

		wasNew, err := addTagToBookmark(tx, id, tag)
		if err != nil {
			return AddResult{}, err
		}
		if wasNew {
			added = append(added, tag)
		}
	}

	if err := tx.Commit(); err != nil {
		return AddResult{}, fmt.Errorf("commit add transaction: %w", err)
	}

	return AddResult{ID: id, URL: url, Created: created, AddedTags: added}, nil
}

func addTagToBookmark(tx *sql.Tx, bookmarkID int64, name string) (bool, error) {
	if _, err := tx.Exec("INSERT INTO tags (name) VALUES (?) ON CONFLICT(name) DO NOTHING", name); err != nil {
		return false, fmt.Errorf("insert tag %q: %w", name, err)
	}

	var tagID int64
	if err := tx.QueryRow("SELECT id FROM tags WHERE name = ?", name).Scan(&tagID); err != nil {
		return false, fmt.Errorf("look up tag %q: %w", name, err)
	}

	res, err := tx.Exec("INSERT OR IGNORE INTO bookmark_tags (bookmark_id, tag_id) VALUES (?, ?)", bookmarkID, tagID)
	if err != nil {
		return false, fmt.Errorf("link tag %q to bookmark %d: %w", name, bookmarkID, err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check tag link result for %q: %w", name, err)
	}
	return n > 0, nil
}

func removeTagFromBookmark(tx *sql.Tx, bookmarkID int64, name string) error {
	_, err := tx.Exec(
		"DELETE FROM bookmark_tags WHERE bookmark_id = ? AND tag_id = (SELECT id FROM tags WHERE name = ?)",
		bookmarkID, name,
	)
	if err != nil {
		return fmt.Errorf("remove tag %q from bookmark %d: %w", name, bookmarkID, err)
	}
	return nil
}

func pruneOrphanedTags(tx *sql.Tx) error {
	if _, err := tx.Exec("DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM bookmark_tags)"); err != nil {
		return fmt.Errorf("prune orphaned tags: %w", err)
	}
	return nil
}

// List returns bookmarks matching f.
func (s *Store) List(f ListFilter) ([]Bookmark, error) {
	col, ok := listSortColumns[f.Sort]
	if !ok {
		return nil, fmt.Errorf("unknown sort key %q", f.Sort)
	}
	dir := "DESC"
	if f.Reverse {
		dir = "ASC"
	}

	tags := dedupeStrings(f.Tags)

	var query strings.Builder
	var args []any
	query.WriteString("SELECT b.id, b.url, b.read, b.created_at FROM bookmarks b")

	var where []string
	if len(tags) > 0 {
		where = append(where, fmt.Sprintf(
			`b.id IN (
				SELECT bt.bookmark_id FROM bookmark_tags bt
				JOIN tags t ON t.id = bt.tag_id
				WHERE t.name IN (%s)
				GROUP BY bt.bookmark_id
				HAVING COUNT(DISTINCT t.name) = ?
			)`,
			placeholders(len(tags)),
		))
		args = append(args, stringArgs(tags)...)
		args = append(args, len(tags))
	}
	if f.Read != nil {
		where = append(where, "b.read = ?")
		args = append(args, boolToInt(*f.Read))
	}
	if len(where) > 0 {
		query.WriteString(" WHERE ")
		query.WriteString(strings.Join(where, " AND "))
	}

	fmt.Fprintf(&query, " ORDER BY %s %s, b.id %s", col, dir, dir)

	if f.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, f.Limit)
	}

	rows, err := s.db.Query(query.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list bookmarks: %w", err)
	}
	bookmarks, err := scanBookmarks(rows)
	if err != nil {
		return nil, err
	}

	return attachTags(s.db, bookmarks)
}

// Get returns one bookmark by id, or ErrNotFound.
func (s *Store) Get(id int64) (Bookmark, error) {
	var b Bookmark
	var readInt int
	err := s.db.QueryRow("SELECT id, url, read, created_at FROM bookmarks WHERE id = ?", id).
		Scan(&b.ID, &b.URL, &readInt, &b.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Bookmark{}, ErrNotFound
	}
	if err != nil {
		return Bookmark{}, fmt.Errorf("get bookmark %d: %w", id, err)
	}
	b.Read = readInt != 0

	tagsByID, err := tagsForIDs(s.db, []int64{id})
	if err != nil {
		return Bookmark{}, err
	}
	b.Tags = tagsByID[id]

	return b, nil
}

// Unread returns up to limit unread bookmarks, oldest first. limit must be positive.
func (s *Store) Unread(limit int) ([]Bookmark, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("unread limit must be positive, got %d", limit)
	}
	unread := false
	return s.List(ListFilter{Read: &unread, Sort: "created", Reverse: true, Limit: limit})
}

// MarkRead sets read = 1 on one bookmark.
func (s *Store) MarkRead(id int64) error {
	res, err := s.db.Exec("UPDATE bookmarks SET read = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("mark bookmark %d read: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check mark-read result for %d: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// All returns every bookmark with its tags, ordered by id.
func (s *Store) All() ([]Bookmark, error) {
	rows, err := s.db.Query("SELECT id, url, read, created_at FROM bookmarks ORDER BY id ASC")
	if err != nil {
		return nil, fmt.Errorf("list all bookmarks: %w", err)
	}
	bookmarks, err := scanBookmarks(rows)
	if err != nil {
		return nil, err
	}
	return attachTags(s.db, bookmarks)
}

// Remove deletes bookmarks by id, reporting what was removed and which ids were absent.
func (s *Store) Remove(ids []int64) (removed []Bookmark, missing []int64, err error) {
	if len(ids) == 0 {
		return nil, nil, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("begin remove transaction: %w", err)
	}
	defer tx.Rollback()

	ph := placeholders(len(ids))
	args := int64Args(ids)

	rows, err := tx.Query(
		fmt.Sprintf("SELECT id, url, read, created_at FROM bookmarks WHERE id IN (%s) ORDER BY id", ph),
		args...,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("select bookmarks to remove: %w", err)
	}
	existing, err := scanBookmarks(rows)
	if err != nil {
		return nil, nil, err
	}

	found := make(map[int64]bool, len(existing))
	existingIDs := make([]int64, len(existing))
	for i, b := range existing {
		found[b.ID] = true
		existingIDs[i] = b.ID
	}
	for _, id := range ids {
		if !found[id] {
			missing = append(missing, id)
		}
	}

	tagsByID, err := tagsForIDs(tx, existingIDs)
	if err != nil {
		return nil, nil, err
	}
	for i := range existing {
		existing[i].Tags = tagsByID[existing[i].ID]
	}

	if len(existing) > 0 {
		if _, err := tx.Exec(fmt.Sprintf("DELETE FROM bookmarks WHERE id IN (%s)", ph), args...); err != nil {
			return nil, nil, fmt.Errorf("delete bookmarks: %w", err)
		}
		if err := pruneOrphanedTags(tx); err != nil {
			return nil, nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("commit remove transaction: %w", err)
	}

	return existing, missing, nil
}

func scanBookmarks(rows *sql.Rows) ([]Bookmark, error) {
	defer rows.Close()

	var bookmarks []Bookmark
	for rows.Next() {
		var b Bookmark
		var readInt int
		if err := rows.Scan(&b.ID, &b.URL, &readInt, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan bookmark row: %w", err)
		}
		b.Read = readInt != 0
		bookmarks = append(bookmarks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read bookmark rows: %w", err)
	}

	return bookmarks, nil
}

func attachTags(q querier, bookmarks []Bookmark) ([]Bookmark, error) {
	ids := make([]int64, len(bookmarks))
	for i, b := range bookmarks {
		ids[i] = b.ID
	}

	tagsByID, err := tagsForIDs(q, ids)
	if err != nil {
		return nil, err
	}
	for i := range bookmarks {
		bookmarks[i].Tags = tagsByID[bookmarks[i].ID]
	}

	return bookmarks, nil
}

func tagsForIDs(q querier, ids []int64) (map[int64][]string, error) {
	result := make(map[int64][]string, len(ids))
	for start := 0; start < len(ids); start += 900 {
		batch, err := tagsForIDBatch(q, ids[start:min(start+900, len(ids))])
		if err != nil {
			return nil, err
		}
		for id, tags := range batch {
			result[id] = tags
		}
	}
	return result, nil
}

func tagsForIDBatch(q querier, ids []int64) (map[int64][]string, error) {
	result := make(map[int64][]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	rows, err := q.Query(
		fmt.Sprintf(
			"SELECT bt.bookmark_id, t.name FROM bookmark_tags bt JOIN tags t ON t.id = bt.tag_id WHERE bt.bookmark_id IN (%s)",
			placeholders(len(ids)),
		),
		int64Args(ids)...,
	)
	if err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan tag row: %w", err)
		}
		result[id] = append(result[id], name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tag rows: %w", err)
	}

	for id := range result {
		sort.Strings(result[id])
	}

	return result, nil
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func stringArgs(in []string) []any {
	args := make([]any, len(in))
	for i, s := range in {
		args[i] = s
	}
	return args
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
