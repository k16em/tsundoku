package store

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/k16em/tsundoku/internal/filter"
)

func TestPlanRefreshAddsMatchingTagsThatAreNotAlreadyPresent(t *testing.T) {
	cs := []filter.Compiled{
		{Name: "blog", Re: regexp.MustCompile(`^https://example\.com/blog/`)},
	}
	bs := []Bookmark{
		{ID: 1, URL: "https://example.com/blog/a", Tags: nil},
		{ID: 2, URL: "https://example.org/x", Tags: nil},
	}

	got := PlanRefresh(bs, cs, false)

	want := []RefreshChange{
		{ID: 1, URL: "https://example.com/blog/a", Added: []string{"blog"}, Removed: nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PlanRefresh() = %+v, want %+v", got, want)
	}
}

func TestPlanRefreshNeverRemovesTagsWhenPruneIsFalse(t *testing.T) {
	cs := []filter.Compiled{
		{Name: "blog", Re: regexp.MustCompile(`^https://example\.com/blog/`)},
	}
	bs := []Bookmark{
		{ID: 1, URL: "https://example.com/blog/a", Tags: []string{"blog", "manual"}},
	}

	got := PlanRefresh(bs, cs, false)

	for _, c := range got {
		if len(c.Removed) != 0 {
			t.Fatalf("Removed = %v, want empty when prune is false", c.Removed)
		}
	}
}

func TestPlanRefreshWithPruneRemovesTagsNotMatchedByAnyFilter(t *testing.T) {
	cs := []filter.Compiled{
		{Name: "blog", Re: regexp.MustCompile(`^https://example\.com/blog/`)},
	}
	bs := []Bookmark{
		{ID: 1, URL: "https://example.com/blog/a", Tags: []string{"blog", "rust"}},
	}

	got := PlanRefresh(bs, cs, true)

	want := []RefreshChange{
		{ID: 1, URL: "https://example.com/blog/a", Added: nil, Removed: []string{"rust"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PlanRefresh() = %+v, want %+v", got, want)
	}
}

func TestPlanRefreshWithPruneRemovesManuallyAddedTagsTooWithoutDistinguishingOrigin(t *testing.T) {
	cs := []filter.Compiled{}
	bs := []Bookmark{
		{ID: 1, URL: "https://example.com/a", Tags: []string{"manual"}},
	}

	got := PlanRefresh(bs, cs, true)

	want := []RefreshChange{
		{ID: 1, URL: "https://example.com/a", Added: nil, Removed: []string{"manual"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PlanRefresh() = %+v, want %+v", got, want)
	}
}

func TestPlanRefreshOmitsBookmarksWithNoDiff(t *testing.T) {
	cs := []filter.Compiled{
		{Name: "blog", Re: regexp.MustCompile(`^https://example\.com/blog/`)},
	}
	bs := []Bookmark{
		{ID: 1, URL: "https://example.com/blog/a", Tags: []string{"blog"}},
	}

	got := PlanRefresh(bs, cs, true)

	if len(got) != 0 {
		t.Fatalf("PlanRefresh() = %+v, want empty", got)
	}
}

func TestPlanRefreshOmitsABookmarkWithNoTagsWhenNoFilterMatchesEvenWithPrune(t *testing.T) {
	cs := []filter.Compiled{
		{Name: "blog", Re: regexp.MustCompile(`^https://nomatch\.example/`)},
	}
	bs := []Bookmark{
		{ID: 1, URL: "https://example.com/a", Tags: nil},
	}

	got := PlanRefresh(bs, cs, true)

	if len(got) != 0 {
		t.Fatalf("PlanRefresh() = %+v, want empty", got)
	}
}

func TestPlanRefreshAddedTagsFollowFilterConfigOrder(t *testing.T) {
	cs := []filter.Compiled{
		{Name: "z-first", Re: regexp.MustCompile(`example\.com`)},
		{Name: "a-second", Re: regexp.MustCompile(`example\.com`)},
	}
	bs := []Bookmark{
		{ID: 1, URL: "https://example.com/a", Tags: nil},
	}

	got := PlanRefresh(bs, cs, false)

	want := []RefreshChange{
		{ID: 1, URL: "https://example.com/a", Added: []string{"z-first", "a-second"}, Removed: nil},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PlanRefresh() = %+v, want %+v", got, want)
	}
}

func TestApplyRefreshMakesAllReflectThePlannedChanges(t *testing.T) {
	s := newTestStore(t)
	a, err := s.Add("https://example.com/a", []string{"rust"}, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	b, err := s.Add("https://example.com/b", nil, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	changes := []RefreshChange{
		{ID: a.ID, URL: a.URL, Added: []string{"blog"}, Removed: []string{"rust"}},
		{ID: b.ID, URL: b.URL, Added: []string{"golang"}, Removed: nil},
	}

	if err := s.ApplyRefresh(changes); err != nil {
		t.Fatalf("ApplyRefresh() error = %v", err)
	}

	got, err := s.All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	byID := map[int64][]string{}
	for _, bm := range got {
		byID[bm.ID] = bm.Tags
	}
	if !reflect.DeepEqual(byID[a.ID], []string{"blog"}) {
		t.Fatalf("tags of a = %v, want [blog]", byID[a.ID])
	}
	if !reflect.DeepEqual(byID[b.ID], []string{"golang"}) {
		t.Fatalf("tags of b = %v, want [golang]", byID[b.ID])
	}
}

func TestApplyRefreshDeletesTagsThatBecomeOrphaned(t *testing.T) {
	s := newTestStore(t)
	a, err := s.Add("https://example.com/a", []string{"rust"}, "2026-09-08T00:00:00Z")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	changes := []RefreshChange{
		{ID: a.ID, URL: a.URL, Added: nil, Removed: []string{"rust"}},
	}
	if err := s.ApplyRefresh(changes); err != nil {
		t.Fatalf("ApplyRefresh() error = %v", err)
	}

	counts, err := s.TagCounts("name", false)
	if err != nil {
		t.Fatalf("TagCounts() error = %v", err)
	}
	if len(counts) != 0 {
		t.Fatalf("TagCounts() = %+v, want empty", counts)
	}
}
