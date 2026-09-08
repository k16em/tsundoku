package store

import (
	"reflect"
	"testing"
)

func tagFixtures(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.Add("https://example.com/a", []string{"blog", "golang"}, "2026-09-08T00:00:00Z"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := s.Add("https://example.com/b", []string{"golang"}, "2026-09-07T00:00:00Z"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := s.Add("https://example.com/c", []string{"blog", "news"}, "2026-09-09T00:00:00Z"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
}

func TestTagCountsReturnsTheNumberOfBookmarksPerTag(t *testing.T) {
	s := newTestStore(t)
	tagFixtures(t, s)

	got, err := s.TagCounts("name", false)
	if err != nil {
		t.Fatalf("TagCounts() error = %v", err)
	}
	want := []TagCount{
		{Name: "blog", Count: 2},
		{Name: "golang", Count: 2},
		{Name: "news", Count: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TagCounts() = %+v, want %+v", got, want)
	}
}

func TestTagCountsSortsByNameAscendingByDefault(t *testing.T) {
	s := newTestStore(t)
	tagFixtures(t, s)

	got, err := s.TagCounts("name", false)
	if err != nil {
		t.Fatalf("TagCounts() error = %v", err)
	}
	names := make([]string, len(got))
	for i, tc := range got {
		names[i] = tc.Name
	}
	if !reflect.DeepEqual(names, []string{"blog", "golang", "news"}) {
		t.Fatalf("names = %v, want [blog golang news]", names)
	}
}

func TestTagCountsSortsByNameDescendingWhenReversed(t *testing.T) {
	s := newTestStore(t)
	tagFixtures(t, s)

	got, err := s.TagCounts("name", true)
	if err != nil {
		t.Fatalf("TagCounts() error = %v", err)
	}
	names := make([]string, len(got))
	for i, tc := range got {
		names[i] = tc.Name
	}
	if !reflect.DeepEqual(names, []string{"news", "golang", "blog"}) {
		t.Fatalf("names = %v, want [news golang blog]", names)
	}
}

func TestTagCountsSortsByCountAndReverses(t *testing.T) {
	s := newTestStore(t)
	tagFixtures(t, s)

	got, err := s.TagCounts("count", false)
	if err != nil {
		t.Fatalf("TagCounts() error = %v", err)
	}
	names := make([]string, len(got))
	for i, tc := range got {
		names[i] = tc.Name
	}
	if !reflect.DeepEqual(names, []string{"news", "blog", "golang"}) {
		t.Fatalf("ascending by count = %v, want [news blog golang]", names)
	}

	got, err = s.TagCounts("count", true)
	if err != nil {
		t.Fatalf("TagCounts() error = %v", err)
	}
	names = make([]string, len(got))
	for i, tc := range got {
		names[i] = tc.Name
	}
	if !reflect.DeepEqual(names, []string{"blog", "golang", "news"}) {
		t.Fatalf("descending by count = %v, want [blog golang news]", names)
	}
}

func TestTagCountsReturnsAnErrorForAnUnknownSortKey(t *testing.T) {
	s := newTestStore(t)
	tagFixtures(t, s)

	_, err := s.TagCounts("bogus", false)
	if err == nil {
		t.Fatalf("TagCounts() error = nil, want error")
	}
}

func TestTagCountsBreaksTiesByNameAscendingRegardlessOfDirection(t *testing.T) {
	s := newTestStore(t)
	tagFixtures(t, s)

	got, err := s.TagCounts("count", true)
	if err != nil {
		t.Fatalf("TagCounts() error = %v", err)
	}
	if len(got) < 2 {
		t.Fatalf("not enough tags in fixture")
	}
	if got[0].Count != got[1].Count {
		t.Fatalf("expected fixture's top two counts to tie, got %+v", got[:2])
	}
	if got[0].Name >= got[1].Name {
		t.Fatalf("tie order = [%s %s], want name ascending", got[0].Name, got[1].Name)
	}
}
