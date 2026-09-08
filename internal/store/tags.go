package store

import "fmt"

// TagCount is one row of the tag list output.
type TagCount struct {
	Name  string
	Count int
}

var tagCountSortColumns = map[string]string{
	"":      "t.name",
	"name":  "t.name",
	"count": "count",
}

// TagCounts returns every tag with the number of bookmarks carrying it.
func (s *Store) TagCounts(sortKey string, reverse bool) ([]TagCount, error) {
	col, ok := tagCountSortColumns[sortKey]
	if !ok {
		return nil, fmt.Errorf("unknown sort key %q", sortKey)
	}
	dir := "ASC"
	if reverse {
		dir = "DESC"
	}

	query := fmt.Sprintf(
		`SELECT t.name, COUNT(bt.bookmark_id) AS count
		FROM tags t
		LEFT JOIN bookmark_tags bt ON bt.tag_id = t.id
		GROUP BY t.id
		ORDER BY %s %s, t.name ASC`,
		col, dir,
	)

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list tag counts: %w", err)
	}
	defer rows.Close()

	var counts []TagCount
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Name, &tc.Count); err != nil {
			return nil, fmt.Errorf("scan tag count row: %w", err)
		}
		counts = append(counts, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read tag count rows: %w", err)
	}

	return counts, nil
}
