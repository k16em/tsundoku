package store

import (
	"errors"
	"reflect"
	"testing"
)

func boolPtr(b bool) *bool { return &b }

func TestAddCreatesANewUnreadBookmark(t *testing.T) {
	s := newTestStore(t)

	res, err := s.Add("https://example.com/a", []string{"blog", "golang"}, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if !res.Created {
		t.Fatalf("Created = false, want true")
	}
	if res.URL != "https://example.com/a" {
		t.Fatalf("URL = %q", res.URL)
	}
	if !reflect.DeepEqual(res.AddedTags, []string{"blog", "golang"}) {
		t.Fatalf("AddedTags = %v, want [blog golang]", res.AddedTags)
	}

	got, err := s.Get(res.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Read {
		t.Fatalf("Read = true, want false")
	}
}

func TestReAddingAnExistingURLOnlyReportsTheNewlyAddedTags(t *testing.T) {
	s := newTestStore(t)

	first, err := s.Add("https://example.com/a", []string{"blog"}, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("first Add() error = %v", err)
	}

	second, err := s.Add("https://example.com/a", []string{"blog", "golang"}, "2026-09-08T01:00:00Z")
	if err != nil {
		t.Fatalf("second Add() error = %v", err)
	}
	if second.Created {
		t.Fatalf("Created = true, want false")
	}
	if second.ID != first.ID {
		t.Fatalf("ID = %d, want %d", second.ID, first.ID)
	}
	if !reflect.DeepEqual(second.AddedTags, []string{"golang"}) {
		t.Fatalf("AddedTags = %v, want [golang]", second.AddedTags)
	}
}

func TestReAddingAnAlreadyPresentTagDoesNotReportItAsAdded(t *testing.T) {
	s := newTestStore(t)

	res, err := s.Add("https://example.com/a", []string{"blog"}, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	res, err = s.Add("https://example.com/a", []string{"blog"}, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if len(res.AddedTags) != 0 {
		t.Fatalf("AddedTags = %v, want empty", res.AddedTags)
	}
}

func TestAddWithNoTagsSucceeds(t *testing.T) {
	s := newTestStore(t)

	res, err := s.Add("https://example.com/a", nil, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if len(res.AddedTags) != 0 {
		t.Fatalf("AddedTags = %v, want empty", res.AddedTags)
	}

	got, err := s.Get(res.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(got.Tags) != 0 {
		t.Fatalf("Tags = %v, want empty", got.Tags)
	}
}

func addFixtures(t *testing.T, s *Store) map[string]int64 {
	t.Helper()
	ids := map[string]int64{}

	a, err := s.Add("https://example.com/a", []string{"blog", "golang"}, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	ids["a"] = a.ID

	b, err := s.Add("https://example.com/b", []string{"golang"}, "2026-09-07T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	ids["b"] = b.ID

	c, err := s.Add("https://example.com/c", []string{"blog", "news"}, "2026-09-09T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	ids["c"] = c.ID

	if err := s.MarkRead(ids["b"]); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	return ids
}

func TestListWithTwoTagsReturnsOnlyBookmarksThatHaveBoth(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	got, err := s.List(ListFilter{Tags: []string{"blog", "golang"}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != ids["a"] {
		t.Fatalf("List() = %+v, want only %d", got, ids["a"])
	}
}

func TestListReturnsEmptyWhenNoBookmarkMatchesAllTags(t *testing.T) {
	s := newTestStore(t)
	addFixtures(t, s)

	got, err := s.List(ListFilter{Tags: []string{"golang", "news"}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("List() = %+v, want empty", got)
	}
}

func TestListFiltersByReadState(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	unread, err := s.List(ListFilter{Read: boolPtr(false)})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("len(unread) = %d, want 2", len(unread))
	}
	for _, b := range unread {
		if b.ID == ids["b"] {
			t.Fatalf("unread list contains read bookmark %d", ids["b"])
		}
	}

	read, err := s.List(ListFilter{Read: boolPtr(true)})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(read) != 1 || read[0].ID != ids["b"] {
		t.Fatalf("read list = %+v, want only %d", read, ids["b"])
	}

	all, err := s.List(ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len(all) = %d, want 3", len(all))
	}
}

func TestListSortsByCreatedDescendingByDefault(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	got, err := s.List(ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []int64{ids["c"], ids["a"], ids["b"]}
	gotIDs := make([]int64, len(got))
	for i, b := range got {
		gotIDs[i] = b.ID
	}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("order = %v, want %v", gotIDs, want)
	}
}

func TestListSortsByIDAndReverse(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	got, err := s.List(ListFilter{Sort: "id"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want := []int64{ids["c"], ids["b"], ids["a"]}
	gotIDs := make([]int64, len(got))
	for i, b := range got {
		gotIDs[i] = b.ID
	}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("order = %v, want %v", gotIDs, want)
	}

	got, err = s.List(ListFilter{Sort: "id", Reverse: true})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	want = []int64{ids["a"], ids["b"], ids["c"]}
	gotIDs = make([]int64, len(got))
	for i, b := range got {
		gotIDs[i] = b.ID
	}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("reversed order = %v, want %v", gotIDs, want)
	}
}

func TestListReturnsAnErrorForAnUnknownSortKey(t *testing.T) {
	s := newTestStore(t)
	addFixtures(t, s)

	_, err := s.List(ListFilter{Sort: "bogus"})
	if err == nil {
		t.Fatalf("List() error = nil, want error")
	}
}

func TestListLimitsTheNumberOfResults(t *testing.T) {
	s := newTestStore(t)
	addFixtures(t, s)

	got, err := s.List(ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
}

func TestListWithDuplicateTagsStillMatchesCorrectly(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	got, err := s.List(ListFilter{Tags: []string{"blog", "blog", "golang"}})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != ids["a"] {
		t.Fatalf("List() = %+v, want only %d", got, ids["a"])
	}
}

func TestListReturnsTagsSortedByNameAscending(t *testing.T) {
	s := newTestStore(t)

	res, err := s.Add("https://example.com/z", []string{"zebra", "alpha", "middle"}, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	got, err := s.List(ListFilter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	var b Bookmark
	for _, cand := range got {
		if cand.ID == res.ID {
			b = cand
		}
	}
	if !reflect.DeepEqual(b.Tags, []string{"alpha", "middle", "zebra"}) {
		t.Fatalf("Tags = %v, want [alpha middle zebra]", b.Tags)
	}
}

func TestGetReturnsErrNotFoundForAMissingID(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Get(999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestUnreadReturnsOldestFirstUpToLimit(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	got, err := s.Unread(1)
	if err != nil {
		t.Fatalf("Unread() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != ids["a"] {
		t.Fatalf("Unread(1) = %+v, want only %d", got, ids["a"])
	}

	got, err = s.Unread(10)
	if err != nil {
		t.Fatalf("Unread() error = %v", err)
	}
	want := []int64{ids["a"], ids["c"]}
	gotIDs := make([]int64, len(got))
	for i, b := range got {
		gotIDs[i] = b.ID
	}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("Unread order = %v, want %v", gotIDs, want)
	}
}

func TestMarkReadRemovesTheBookmarkFromUnread(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	if err := s.MarkRead(ids["a"]); err != nil {
		t.Fatalf("MarkRead() error = %v", err)
	}

	got, err := s.Unread(10)
	if err != nil {
		t.Fatalf("Unread() error = %v", err)
	}
	for _, b := range got {
		if b.ID == ids["a"] {
			t.Fatalf("Unread() still contains %d after MarkRead", ids["a"])
		}
	}
}

func TestUnreadReturnsAnErrorForANonPositiveLimit(t *testing.T) {
	s := newTestStore(t)
	addFixtures(t, s)

	if _, err := s.Unread(0); err == nil {
		t.Fatalf("Unread(0) error = nil, want error")
	}
	if _, err := s.Unread(-1); err == nil {
		t.Fatalf("Unread(-1) error = nil, want error")
	}
}

func TestMarkReadReturnsErrNotFoundForAMissingID(t *testing.T) {
	s := newTestStore(t)

	err := s.MarkRead(999)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("MarkRead() error = %v, want ErrNotFound", err)
	}
}

func TestAllReturnsEveryBookmarkOrderedByIDWithTags(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	got, err := s.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	want := []int64{ids["a"], ids["b"], ids["c"]}
	gotIDs := make([]int64, len(got))
	for i, b := range got {
		gotIDs[i] = b.ID
	}
	if !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("All() order = %v, want %v", gotIDs, want)
	}
}

func TestRemoveDeletesBookmarkTagRowsViaCascade(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	if _, _, err := s.Remove([]int64{ids["a"]}); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM bookmark_tags WHERE bookmark_id = ?", ids["a"]).Scan(&count); err != nil {
		t.Fatalf("count bookmark_tags: %v", err)
	}
	if count != 0 {
		t.Fatalf("bookmark_tags rows for removed bookmark = %d, want 0", count)
	}
}

func TestRemoveDeletesOrphanedTagsButKeepsTagsStillInUse(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	if _, _, err := s.Remove([]int64{ids["a"], ids["b"]}); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	counts, err := s.TagCounts("name", false)
	if err != nil {
		t.Fatalf("TagCounts() error = %v", err)
	}
	names := make([]string, len(counts))
	for i, tc := range counts {
		names[i] = tc.Name
	}
	if !reflect.DeepEqual(names, []string{"blog", "news"}) {
		t.Fatalf("remaining tags = %v, want [blog news]", names)
	}
}

func TestRemoveReportsMissingIDsAndDeletesTheOnesThatExisted(t *testing.T) {
	s := newTestStore(t)
	ids := addFixtures(t, s)

	removed, missing, err := s.Remove([]int64{ids["a"], 999})
	if err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if !reflect.DeepEqual(missing, []int64{999}) {
		t.Fatalf("missing = %v, want [999]", missing)
	}
	if len(removed) != 1 || removed[0].ID != ids["a"] {
		t.Fatalf("removed = %+v, want only %d", removed, ids["a"])
	}

	_, err = s.Get(ids["a"])
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() after Remove() error = %v, want ErrNotFound", err)
	}
}

func TestBookmarkLimitAllowsExistingURLsAndDeletion(t *testing.T) {
	s := newTestStore(t)
	_, err := s.db.Exec(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x < 9999)
 INSERT INTO bookmarks(url, created_at) SELECT 'https://example.com/' || x, '2026-09-09T00:00:00Z' FROM n`)
	if err != nil {
		t.Fatal(err)
	}
	last, err := s.Add("https://example.com/last", nil, "2026-09-09T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("https://example.com/overflow", []string{"overflow"}, "2026-09-09T00:00:00Z"); !errors.Is(err, ErrBookmarkLimit) {
		t.Fatalf("Add overflow = %v", err)
	}
	if _, err := s.Add(last.URL, []string{"updated"}, "2026-09-09T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	all, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != MaxBookmarks || !reflect.DeepEqual(all[len(all)-1].Tags, []string{"updated"}) {
		t.Fatalf("unexpected bookmarks or tags at limit")
	}
	if err := s.ApplyRefresh([]RefreshChange{{ID: last.ID, Added: []string{"refreshed"}}}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Remove([]int64{last.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("https://example.com/replacement", nil, "2026-09-09T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
}

func TestAllLoadsTagsBeyondSQLiteVariableLimit(t *testing.T) {
	s := newTestStore(t)
	_, err := s.db.Exec(`WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x < 32767)
 INSERT INTO bookmarks(url, created_at) SELECT 'https://example.com/' || x, '2026-09-09T00:00:00Z' FROM n`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add("https://example.com/32767", []string{"last"}, "2026-09-09T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	all, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 32767 || !reflect.DeepEqual(all[len(all)-1].Tags, []string{"last"}) {
		t.Fatal("unexpected bookmarks or tags")
	}
}
