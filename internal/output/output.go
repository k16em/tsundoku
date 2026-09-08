package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/k16em/tsundoku/internal/store"
)

// SanitizeLine strips control characters and flattens the result to one line.
func SanitizeLine(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(' ')
		case r < 0x20 || r == 0x7f:
		case r >= 0x80 && r <= 0x9f:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// FormatList renders the list command output.
func FormatList(bs []store.Bookmark) string {
	if len(bs) == 0 {
		return ""
	}
	items := make([]string, len(bs))
	for i, b := range bs {
		items[i] = FormatListItem(b)
	}
	return strings.Join(items, "\n\n") + "\n"
}

// FormatListItem renders one bookmark block for FormatList.
func FormatListItem(b store.Bookmark) string {
	marker := "* "
	if b.Read {
		marker = "  "
	}
	date := b.CreatedAt
	if len(date) > 10 {
		date = date[:10]
	}
	idField := fmt.Sprintf("[%d] ", b.ID)
	first := marker + idField + date
	if len(b.Tags) > 0 {
		tags := make([]string, len(b.Tags))
		for i, t := range b.Tags {
			tags[i] = "#" + SanitizeLine(t)
		}
		first += "  " + strings.Join(tags, " ")
	}
	indent := strings.Repeat(" ", len(marker)+len(idField))
	return first + "\n" + indent + SanitizeLine(b.URL)
}

// FormatShow renders the show command text output for one bookmark.
func FormatShow(b store.Bookmark) string {
	tags := make([]string, len(b.Tags))
	for i, t := range b.Tags {
		tags[i] = SanitizeLine(t)
	}
	read := "no"
	if b.Read {
		read = "yes"
	}
	lines := []string{
		fmt.Sprintf("%-5s : %s", "URL", SanitizeLine(b.URL)),
		fmt.Sprintf("%-5s : %s", "Tags", strings.Join(tags, ", ")),
		fmt.Sprintf("%-5s : %s", "Added", b.CreatedAt),
		fmt.Sprintf("%-5s : %s", "Read", read),
	}
	return strings.Join(lines, "\n") + "\n"
}

type bookmarkJSON struct {
	ID        int64    `json:"id"`
	URL       string   `json:"url"`
	Tags      []string `json:"tags"`
	Read      bool     `json:"read"`
	CreatedAt string   `json:"created_at"`
}

func toBookmarkJSON(b store.Bookmark) bookmarkJSON {
	tags := b.Tags
	if tags == nil {
		tags = []string{}
	}
	return bookmarkJSON{
		ID:        b.ID,
		URL:       b.URL,
		Tags:      tags,
		Read:      b.Read,
		CreatedAt: b.CreatedAt,
	}
}

// FormatShowJSON renders one bookmark as JSON.
func FormatShowJSON(b store.Bookmark) (string, error) {
	data, err := json.MarshalIndent(toBookmarkJSON(b), "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

// FormatShowJSONList renders a slice of bookmarks as a JSON array.
func FormatShowJSONList(bs []store.Bookmark) (string, error) {
	list := make([]bookmarkJSON, len(bs))
	for i, b := range bs {
		list[i] = toBookmarkJSON(b)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// FormatAdd renders the add command summary line.
func FormatAdd(r store.AddResult) string {
	label := "added"
	if !r.Created {
		label = "updated"
	}
	line := fmt.Sprintf("%-7s [%d] %s", label, r.ID, SanitizeLine(r.URL))
	if r.Created {
		if len(r.AddedTags) > 0 {
			tags := make([]string, len(r.AddedTags))
			for i, t := range r.AddedTags {
				tags[i] = "#" + SanitizeLine(t)
			}
			line += "  " + strings.Join(tags, " ")
		}
	} else {
		line += fmt.Sprintf("  +%d %s", len(r.AddedTags), pluralize(len(r.AddedTags), "tag", "tags"))
	}
	return line + "\n"
}

// FormatRemoved renders one rm command summary line.
func FormatRemoved(b store.Bookmark) string {
	return fmt.Sprintf("%-7s [%d] %s\n", "removed", b.ID, SanitizeLine(b.URL))
}

// FormatTagCounts renders the tag list table.
func FormatTagCounts(tcs []store.TagCount) string {
	width := len("COUNT")
	for _, tc := range tcs {
		if w := len(strconv.Itoa(tc.Count)); w > width {
			width = w
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%*s  %s\n", width, "COUNT", "NAME")
	for _, tc := range tcs {
		fmt.Fprintf(&b, "%*d  %s\n", width, tc.Count, SanitizeLine(tc.Name))
	}
	return b.String()
}

// FormatRefresh renders the tag refresh output including its summary line.
func FormatRefresh(cs []store.RefreshChange, dryRun bool) string {
	if len(cs) == 0 {
		return ""
	}
	tagStrs := make([]string, len(cs))
	idWidth, tagWidth := 0, 0
	addedTotal, removedTotal := 0, 0
	for i, c := range cs {
		parts := make([]string, 0, len(c.Added)+len(c.Removed))
		for _, t := range c.Added {
			parts = append(parts, "+"+SanitizeLine(t))
		}
		for _, t := range c.Removed {
			parts = append(parts, "-"+SanitizeLine(t))
		}
		tagStrs[i] = strings.Join(parts, " ")
		if w := utf8.RuneCountInString(tagStrs[i]); w > tagWidth {
			tagWidth = w
		}
		if w := len(strconv.FormatInt(c.ID, 10)); w > idWidth {
			idWidth = w
		}
		addedTotal += len(c.Added)
		removedTotal += len(c.Removed)
	}
	lines := make([]string, len(cs))
	for i, c := range cs {
		lines[i] = fmt.Sprintf("[%*d] %-*s  %s", idWidth, c.ID, tagWidth, tagStrs[i], SanitizeLine(c.URL))
	}

	bookmarkWord := pluralize(len(cs), "bookmark", "bookmarks")
	addedWord := pluralize(addedTotal, "tag", "tags")
	removedWord := pluralize(removedTotal, "tag", "tags")

	var summary string
	if dryRun {
		summary = fmt.Sprintf(
			"(dry-run) %d %s would be updated, %d %s would be added, %d %s would be removed",
			len(cs), bookmarkWord, addedTotal, addedWord, removedTotal, removedWord,
		)
	} else {
		summary = fmt.Sprintf(
			"%d %s updated, %d %s added, %d %s removed",
			len(cs), bookmarkWord, addedTotal, addedWord, removedTotal, removedWord,
		)
	}

	return strings.Join(lines, "\n") + "\n---\n" + summary + "\n"
}

// PrintList writes the list command output to w.
func PrintList(w io.Writer, bs []store.Bookmark) error {
	s := FormatList(bs)
	if s == "" {
		return nil
	}
	_, err := io.WriteString(w, s)
	return err
}

// PrintShow writes one bookmark's text block to w.
func PrintShow(w io.Writer, b store.Bookmark) error {
	_, err := io.WriteString(w, FormatShow(b))
	return err
}

// PrintShowJSON writes one bookmark as JSON to w.
func PrintShowJSON(w io.Writer, b store.Bookmark) error {
	s, err := FormatShowJSON(b)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}

// PrintShowJSONList writes a slice of bookmarks as a JSON array to w.
func PrintShowJSONList(w io.Writer, bs []store.Bookmark) error {
	s, err := FormatShowJSONList(bs)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, s)
	return err
}

// PrintAdd writes the add command summary line to w.
func PrintAdd(w io.Writer, r store.AddResult) error {
	_, err := io.WriteString(w, FormatAdd(r))
	return err
}

// PrintRemoved writes one rm command summary line to w.
func PrintRemoved(w io.Writer, b store.Bookmark) error {
	_, err := io.WriteString(w, FormatRemoved(b))
	return err
}

// PrintTagCounts writes the tag list table to w.
func PrintTagCounts(w io.Writer, tcs []store.TagCount) error {
	_, err := io.WriteString(w, FormatTagCounts(tcs))
	return err
}

// PrintRefresh writes the tag refresh output to w.
func PrintRefresh(w io.Writer, cs []store.RefreshChange, dryRun bool) error {
	s := FormatRefresh(cs, dryRun)
	if s == "" {
		return nil
	}
	_, err := io.WriteString(w, s)
	return err
}

// PrintSeparator writes the divider between blocks of `show unread`.
func PrintSeparator(w io.Writer) error {
	_, err := io.WriteString(w, strings.Repeat("=", 64)+"\n")
	return err
}
