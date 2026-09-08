package store

import (
	"fmt"

	"github.com/k16em/tsundoku/internal/filter"
)

// RefreshChange is the tag diff computed for one bookmark.
type RefreshChange struct {
	ID      int64
	URL     string
	Added   []string
	Removed []string
}

// PlanRefresh computes the tag diff for every bookmark. It performs no IO.
func PlanRefresh(bs []Bookmark, cs []filter.Compiled, prune bool) []RefreshChange {
	var changes []RefreshChange

	for _, b := range bs {
		matched := filter.Match(cs, b.URL)
		matchedSet := make(map[string]bool, len(matched))
		for _, name := range matched {
			matchedSet[name] = true
		}

		has := make(map[string]bool, len(b.Tags))
		for _, t := range b.Tags {
			has[t] = true
		}

		var added []string
		for _, name := range matched {
			if !has[name] {
				added = append(added, name)
			}
		}

		var removed []string
		if prune {
			for _, t := range b.Tags {
				if !matchedSet[t] {
					removed = append(removed, t)
				}
			}
		}

		if len(added) == 0 && len(removed) == 0 {
			continue
		}

		changes = append(changes, RefreshChange{ID: b.ID, URL: b.URL, Added: added, Removed: removed})
	}

	return changes
}

// ApplyRefresh writes the planned changes in one transaction.
func (s *Store) ApplyRefresh(changes []RefreshChange) error {
	if len(changes) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin refresh transaction: %w", err)
	}
	defer tx.Rollback()

	for _, c := range changes {
		for _, name := range c.Added {
			if _, err := addTagToBookmark(tx, c.ID, name); err != nil {
				return err
			}
		}
		for _, name := range c.Removed {
			if err := removeTagFromBookmark(tx, c.ID, name); err != nil {
				return err
			}
		}
	}

	if err := pruneOrphanedTags(tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit refresh transaction: %w", err)
	}

	return nil
}
