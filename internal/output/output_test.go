package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/k16em/tsundoku/internal/store"
)

func TestSanitizeLine_RemovesEscapeControlByteButKeepsFollowingText(t *testing.T) {
	got := SanitizeLine("a\x1bb")
	want := "ab"
	if got != want {
		t.Errorf("SanitizeLine() = %q, want %q", got, want)
	}
}

func TestSanitizeLine_RemovesAnsiEscapeSequenceButKeepsItsPrintableTail(t *testing.T) {
	got := SanitizeLine("a\x1b[31mb")
	want := "a[31mb"
	if got != want {
		t.Errorf("SanitizeLine() = %q, want %q", got, want)
	}
}

func TestSanitizeLine_ReplacesNewlineAndTabWithSingleSpace(t *testing.T) {
	got := SanitizeLine("a\nb\tc")
	want := "a b c"
	if got != want {
		t.Errorf("SanitizeLine() = %q, want %q", got, want)
	}
}

func TestSanitizeLine_StripsLoneCarriageReturnWithoutInsertingSpace(t *testing.T) {
	got := SanitizeLine("a\rb")
	want := "ab"
	if got != want {
		t.Errorf("SanitizeLine() = %q, want %q", got, want)
	}
}

func TestSanitizeLine_CollapsesCrlfPairToOneSpace(t *testing.T) {
	got := SanitizeLine("a\r\nb")
	want := "a b"
	if got != want {
		t.Errorf("SanitizeLine() = %q, want %q", got, want)
	}
}

func TestSanitizeLine_ReplacesLoneLineFeedWithOneSpace(t *testing.T) {
	got := SanitizeLine("a\nb")
	want := "a b"
	if got != want {
		t.Errorf("SanitizeLine() = %q, want %q", got, want)
	}
}

func TestSanitizeLine_StripsDelAndC1ControlCharacters(t *testing.T) {
	input := "a" + string(rune(0x7f)) + "b" + string(rune(0x80)) + "c" + string(rune(0x9f)) + "d"
	got := SanitizeLine(input)
	want := "abcd"
	if got != want {
		t.Errorf("SanitizeLine() = %q, want %q", got, want)
	}
}

func TestSanitizeLine_PassesThroughOrdinaryMultibyteText(t *testing.T) {
	got := SanitizeLine("日本語")
	want := "日本語"
	if got != want {
		t.Errorf("SanitizeLine() = %q, want %q", got, want)
	}
}

func TestFormatListItem_UnreadBookmarkHasAsteriskMarker(t *testing.T) {
	b := store.Bookmark{
		ID:        12,
		URL:       "https://example.com/posts/go-concurrency",
		Read:      false,
		CreatedAt: "2026-09-08T00:00:00Z",
		Tags:      []string{"blog", "golang"},
	}
	got := FormatListItem(b)
	want := "* [12] 2026-09-08  #blog #golang\n       https://example.com/posts/go-concurrency"
	if got != want {
		t.Errorf("FormatListItem() = %q, want %q", got, want)
	}
}

func TestFormatListItem_ReadBookmarkAlignsWithSpaceInPlaceOfAsterisk(t *testing.T) {
	b := store.Bookmark{
		ID:        11,
		URL:       "https://example.org/a",
		Read:      true,
		CreatedAt: "2026-09-07T00:00:00Z",
	}
	got := FormatListItem(b)
	want := "  [11] 2026-09-07\n       https://example.org/a"
	if got != want {
		t.Errorf("FormatListItem() = %q, want %q", got, want)
	}
}

func TestFormatListItem_OmitsTagSectionAndLeavesNoTrailingWhitespaceWhenNoTags(t *testing.T) {
	b := store.Bookmark{
		ID:        11,
		URL:       "https://example.org/a",
		Read:      true,
		CreatedAt: "2026-09-07T00:00:00Z",
		Tags:      nil,
	}
	got := FormatListItem(b)
	firstLine := strings.SplitN(got, "\n", 2)[0]
	if strings.HasSuffix(firstLine, " ") {
		t.Errorf("first line has trailing whitespace: %q", firstLine)
	}
	want := "  [11] 2026-09-07\n       https://example.org/a"
	if got != want {
		t.Errorf("FormatListItem() = %q, want %q", got, want)
	}
}

func TestFormatListItem_SanitizesUrlControlCharactersWithoutBreakingLineStructure(t *testing.T) {
	b := store.Bookmark{
		ID:        7,
		URL:       "https://example.com/a\nb",
		Read:      true,
		CreatedAt: "2026-01-02T00:00:00Z",
	}
	got := FormatListItem(b)
	want := "  [7] 2026-01-02\n      https://example.com/a b"
	if got != want {
		t.Errorf("FormatListItem() = %q, want %q", got, want)
	}
	if strings.Count(got, "\n") != 1 {
		t.Errorf("FormatListItem() produced %d newlines, want 1: %q", strings.Count(got, "\n"), got)
	}
}

func TestFormatList_JoinsBlocksWithOneBlankLineAndNoTrailingBlankLine(t *testing.T) {
	bs := []store.Bookmark{
		{
			ID:        12,
			URL:       "https://example.com/posts/go-concurrency",
			Read:      false,
			CreatedAt: "2026-09-08T00:00:00Z",
			Tags:      []string{"blog", "golang"},
		},
		{
			ID:        11,
			URL:       "https://example.org/a",
			Read:      true,
			CreatedAt: "2026-09-07T00:00:00Z",
		},
	}
	got := FormatList(bs)
	want := "* [12] 2026-09-08  #blog #golang\n" +
		"       https://example.com/posts/go-concurrency\n" +
		"\n" +
		"  [11] 2026-09-07\n" +
		"       https://example.org/a\n"
	if got != want {
		t.Errorf("FormatList() = %q, want %q", got, want)
	}
}

func TestFormatList_ReturnsEmptyStringForNoBookmarks(t *testing.T) {
	got := FormatList(nil)
	if got != "" {
		t.Errorf("FormatList() = %q, want empty string", got)
	}
}

func TestFormatShow_RendersLabelsAlignedWithColon(t *testing.T) {
	b := store.Bookmark{
		ID:        12,
		URL:       "https://example.com/posts/go-concurrency",
		Read:      true,
		CreatedAt: "2026-09-08T12:34:56Z",
		Tags:      []string{"blog", "golang"},
	}
	got := FormatShow(b)
	want := "URL   : https://example.com/posts/go-concurrency\n" +
		"Tags  : blog, golang\n" +
		"Added : 2026-09-08T12:34:56Z\n" +
		"Read  : yes\n"
	if got != want {
		t.Errorf("FormatShow() = %q, want %q", got, want)
	}
}

func TestFormatShow_EmptyTagsLeavesTagsValueBlankButKeepsTheLine(t *testing.T) {
	b := store.Bookmark{
		ID:        1,
		URL:       "https://example.com/x",
		Read:      false,
		CreatedAt: "2026-01-01T00:00:00Z",
		Tags:      nil,
	}
	got := FormatShow(b)
	want := "URL   : https://example.com/x\n" +
		"Tags  : \n" +
		"Added : 2026-01-01T00:00:00Z\n" +
		"Read  : no\n"
	if got != want {
		t.Errorf("FormatShow() = %q, want %q", got, want)
	}
}

func TestFormatShow_SanitizesUrlControlCharactersWithoutBreakingLineStructure(t *testing.T) {
	b := store.Bookmark{
		ID:        1,
		URL:       "https://example.com/x\ty",
		Read:      false,
		CreatedAt: "2026-01-01T00:00:00Z",
	}
	got := FormatShow(b)
	want := "URL   : https://example.com/x y\n" +
		"Tags  : \n" +
		"Added : 2026-01-01T00:00:00Z\n" +
		"Read  : no\n"
	if got != want {
		t.Errorf("FormatShow() = %q, want %q", got, want)
	}
	if strings.Count(got, "\n") != 4 {
		t.Errorf("FormatShow() produced %d newlines, want 4: %q", strings.Count(got, "\n"), got)
	}
}

func TestFormatShowJSON_RendersAllFieldsInOrder(t *testing.T) {
	b := store.Bookmark{
		ID:        12,
		URL:       "https://example.com/posts/go-concurrency",
		Read:      true,
		CreatedAt: "2026-09-08T12:34:56Z",
		Tags:      []string{"blog", "golang"},
	}
	got, err := FormatShowJSON(b)
	if err != nil {
		t.Fatalf("FormatShowJSON() error = %v", err)
	}
	want := "{\n" +
		"  \"id\": 12,\n" +
		"  \"url\": \"https://example.com/posts/go-concurrency\",\n" +
		"  \"tags\": [\n" +
		"    \"blog\",\n" +
		"    \"golang\"\n" +
		"  ],\n" +
		"  \"read\": true,\n" +
		"  \"created_at\": \"2026-09-08T12:34:56Z\"\n" +
		"}\n"
	if got != want {
		t.Errorf("FormatShowJSON() = %q, want %q", got, want)
	}
}

func TestFormatShowJSON_EmptyTagsMarshalToEmptyArrayNotNull(t *testing.T) {
	b := store.Bookmark{
		ID:        5,
		URL:       "https://example.com/x",
		Read:      false,
		CreatedAt: "2026-09-01T00:00:00Z",
		Tags:      nil,
	}
	got, err := FormatShowJSON(b)
	if err != nil {
		t.Fatalf("FormatShowJSON() error = %v", err)
	}
	want := "{\n" +
		"  \"id\": 5,\n" +
		"  \"url\": \"https://example.com/x\",\n" +
		"  \"tags\": [],\n" +
		"  \"read\": false,\n" +
		"  \"created_at\": \"2026-09-01T00:00:00Z\"\n" +
		"}\n"
	if got != want {
		t.Errorf("FormatShowJSON() = %q, want %q", got, want)
	}
}

func TestFormatShowJSON_DoesNotSanitizeControlCharacters(t *testing.T) {
	b := store.Bookmark{
		ID:        1,
		URL:       "https://example.com/" + string(rune(0x1b)) + "[31mred" + string(rune(0x1b)) + "[0m",
		Read:      false,
		CreatedAt: "2026-09-01T00:00:00Z",
		Tags:      nil,
	}
	got, err := FormatShowJSON(b)
	if err != nil {
		t.Fatalf("FormatShowJSON() error = %v", err)
	}
	want := "{\n" +
		"  \"id\": 1,\n" +
		"  \"url\": \"https://example.com/\\u001b[31mred\\u001b[0m\",\n" +
		"  \"tags\": [],\n" +
		"  \"read\": false,\n" +
		"  \"created_at\": \"2026-09-01T00:00:00Z\"\n" +
		"}\n"
	if got != want {
		t.Errorf("FormatShowJSON() = %q, want %q", got, want)
	}
}

func TestFormatShowJSON_DoesNotEscapeDelAndC1ControlCharacters(t *testing.T) {
	b := store.Bookmark{
		ID:        1,
		URL:       "a" + string(rune(0x7f)) + "b" + string(rune(0x80)) + "c",
		Read:      false,
		CreatedAt: "2026-09-01T00:00:00Z",
		Tags:      nil,
	}
	got, err := FormatShowJSON(b)
	if err != nil {
		t.Fatalf("FormatShowJSON() error = %v", err)
	}
	if !strings.Contains(got, "\"url\": \"a"+string(rune(0x7f))+"b"+string(rune(0x80))+"c\"") {
		t.Errorf("FormatShowJSON() did not contain raw DEL/C1 bytes in url: %q", got)
	}
}

func TestFormatShowJSONList_EmptySliceMarshalsToEmptyArrayNotNull(t *testing.T) {
	got, err := FormatShowJSONList(nil)
	if err != nil {
		t.Fatalf("FormatShowJSONList() error = %v", err)
	}
	want := "[]\n"
	if got != want {
		t.Errorf("FormatShowJSONList() = %q, want %q", got, want)
	}
}

func TestFormatShowJSONList_RendersMultipleBookmarksAsArray(t *testing.T) {
	bs := []store.Bookmark{
		{
			ID:        12,
			URL:       "https://example.com/posts/go-concurrency",
			Read:      true,
			CreatedAt: "2026-09-08T12:34:56Z",
			Tags:      []string{"blog", "golang"},
		},
		{
			ID:        5,
			URL:       "https://example.com/x",
			Read:      false,
			CreatedAt: "2026-09-01T00:00:00Z",
			Tags:      nil,
		},
	}
	got, err := FormatShowJSONList(bs)
	if err != nil {
		t.Fatalf("FormatShowJSONList() error = %v", err)
	}
	want := "[\n" +
		"  {\n" +
		"    \"id\": 12,\n" +
		"    \"url\": \"https://example.com/posts/go-concurrency\",\n" +
		"    \"tags\": [\n" +
		"      \"blog\",\n" +
		"      \"golang\"\n" +
		"    ],\n" +
		"    \"read\": true,\n" +
		"    \"created_at\": \"2026-09-08T12:34:56Z\"\n" +
		"  },\n" +
		"  {\n" +
		"    \"id\": 5,\n" +
		"    \"url\": \"https://example.com/x\",\n" +
		"    \"tags\": [],\n" +
		"    \"read\": false,\n" +
		"    \"created_at\": \"2026-09-01T00:00:00Z\"\n" +
		"  }\n" +
		"]\n"
	if got != want {
		t.Errorf("FormatShowJSONList() = %q, want %q", got, want)
	}
}

func TestFormatAdd_CreatedListsAddedTagsInGivenOrder(t *testing.T) {
	r := store.AddResult{
		ID:        12,
		URL:       "https://example.com/posts/go-concurrency",
		Created:   true,
		AddedTags: []string{"blog", "golang"},
	}
	got := FormatAdd(r)
	want := "added   [12] https://example.com/posts/go-concurrency  #blog #golang\n"
	if got != want {
		t.Errorf("FormatAdd() = %q, want %q", got, want)
	}
}

func TestFormatAdd_CreatedWithNoTagsOmitsTrailingSection(t *testing.T) {
	r := store.AddResult{
		ID:        12,
		URL:       "https://example.com/x",
		Created:   true,
		AddedTags: nil,
	}
	got := FormatAdd(r)
	want := "added   [12] https://example.com/x\n"
	if got != want {
		t.Errorf("FormatAdd() = %q, want %q", got, want)
	}
}

func TestFormatAdd_UpdatedShowsAddedTagCountWithSingularWording(t *testing.T) {
	r := store.AddResult{
		ID:        12,
		URL:       "https://example.com/posts/go-concurrency",
		Created:   false,
		AddedTags: []string{"rust"},
	}
	got := FormatAdd(r)
	want := "updated [12] https://example.com/posts/go-concurrency  +1 tag\n"
	if got != want {
		t.Errorf("FormatAdd() = %q, want %q", got, want)
	}
}

func TestFormatAdd_UpdatedShowsAddedTagCountWithPluralWording(t *testing.T) {
	r := store.AddResult{
		ID:        12,
		URL:       "https://example.com/posts/go-concurrency",
		Created:   false,
		AddedTags: []string{"rust", "wasm"},
	}
	got := FormatAdd(r)
	want := "updated [12] https://example.com/posts/go-concurrency  +2 tags\n"
	if got != want {
		t.Errorf("FormatAdd() = %q, want %q", got, want)
	}
}

func TestFormatAdd_UpdatedWithZeroAddedTagsShowsPlusZeroTags(t *testing.T) {
	r := store.AddResult{
		ID:        12,
		URL:       "https://example.com/posts/go-concurrency",
		Created:   false,
		AddedTags: nil,
	}
	got := FormatAdd(r)
	want := "updated [12] https://example.com/posts/go-concurrency  +0 tags\n"
	if got != want {
		t.Errorf("FormatAdd() = %q, want %q", got, want)
	}
}

func TestFormatRemoved_RendersIdAndUrl(t *testing.T) {
	b := store.Bookmark{ID: 11, URL: "https://example.org/a"}
	got := FormatRemoved(b)
	want := "removed [11] https://example.org/a\n"
	if got != want {
		t.Errorf("FormatRemoved() = %q, want %q", got, want)
	}
}

func TestFormatTagCounts_CountColumnRightAlignedMinimumFiveWide(t *testing.T) {
	tcs := []store.TagCount{
		{Name: "blog", Count: 12},
		{Name: "golang", Count: 4},
		{Name: "日本語", Count: 3},
	}
	got := FormatTagCounts(tcs)
	want := "COUNT  NAME\n" +
		"   12  blog\n" +
		"    4  golang\n" +
		"    3  日本語\n"
	if got != want {
		t.Errorf("FormatTagCounts() = %q, want %q", got, want)
	}
}

func TestFormatTagCounts_WidensCountColumnForLargerValues(t *testing.T) {
	tcs := []store.TagCount{
		{Name: "blog", Count: 123456},
	}
	got := FormatTagCounts(tcs)
	want := " COUNT  NAME\n" +
		"123456  blog\n"
	if got != want {
		t.Errorf("FormatTagCounts() = %q, want %q", got, want)
	}
}

func TestFormatTagCounts_ThreeDigitCountStillAlignsToFiveWideMinimumWhileFiveDigitsFillItExactly(t *testing.T) {
	tcs := []store.TagCount{
		{Name: "blog", Count: 999},
		{Name: "golang", Count: 12345},
	}
	got := FormatTagCounts(tcs)
	want := "COUNT  NAME\n" +
		"  999  blog\n" +
		"12345  golang\n"
	if got != want {
		t.Errorf("FormatTagCounts() = %q, want %q", got, want)
	}
}

func TestFormatTagCounts_MultibyteTagNameDoesNotShiftCountColumn(t *testing.T) {
	tcs := []store.TagCount{
		{Name: "blog", Count: 12},
		{Name: "日本語", Count: 3},
	}
	got := FormatTagCounts(tcs)
	want := "COUNT  NAME\n" +
		"   12  blog\n" +
		"    3  日本語\n"
	if got != want {
		t.Errorf("FormatTagCounts() = %q, want %q", got, want)
	}
}

func TestFormatTagCounts_EmptySliceRendersHeaderOnly(t *testing.T) {
	got := FormatTagCounts(nil)
	want := "COUNT  NAME\n"
	if got != want {
		t.Errorf("FormatTagCounts() = %q, want %q", got, want)
	}
}

func TestFormatRefresh_RendersAddedAndRemovedTagsWithAddedFirst(t *testing.T) {
	cs := []store.RefreshChange{
		{ID: 12, URL: "https://example.com/blog/a", Added: []string{"blog"}},
		{ID: 15, URL: "https://news.example.org/x", Added: []string{"news", "blog"}},
		{ID: 18, URL: "https://example.org/z", Removed: []string{"rust"}},
	}
	got := FormatRefresh(cs, false)
	want := "[12] +blog        https://example.com/blog/a\n" +
		"[15] +news +blog  https://news.example.org/x\n" +
		"[18] -rust        https://example.org/z\n" +
		"---\n" +
		"3 bookmarks updated, 3 tags added, 1 tag removed\n"
	if got != want {
		t.Errorf("FormatRefresh() = %q, want %q", got, want)
	}
}

func TestFormatRefresh_OneBookmarkWithBothAddedAndRemovedTagsPutsAdditionsFirst(t *testing.T) {
	cs := []store.RefreshChange{
		{ID: 7, URL: "https://example.com/combined", Added: []string{"a"}, Removed: []string{"b"}},
	}
	got := FormatRefresh(cs, false)
	want := "[7] +a -b  https://example.com/combined\n" +
		"---\n" +
		"1 bookmark updated, 1 tag added, 1 tag removed\n"
	if got != want {
		t.Errorf("FormatRefresh() = %q, want %q", got, want)
	}
}

func TestFormatRefresh_DryRunWordingUsesWouldBeForAllThreeClauses(t *testing.T) {
	cs := []store.RefreshChange{
		{ID: 12, URL: "https://example.com/blog/a", Added: []string{"blog"}},
		{ID: 15, URL: "https://news.example.org/x", Added: []string{"news", "blog"}},
		{ID: 18, URL: "https://example.org/z", Removed: []string{"rust"}},
	}
	got := FormatRefresh(cs, true)
	want := "[12] +blog        https://example.com/blog/a\n" +
		"[15] +news +blog  https://news.example.org/x\n" +
		"[18] -rust        https://example.org/z\n" +
		"---\n" +
		"(dry-run) 3 bookmarks would be updated, 3 tags would be added, 1 tag would be removed\n"
	if got != want {
		t.Errorf("FormatRefresh() = %q, want %q", got, want)
	}
}

func TestFormatRefresh_SingularWording(t *testing.T) {
	cs := []store.RefreshChange{
		{ID: 5, URL: "https://example.com/a", Added: []string{"blog"}},
	}
	got := FormatRefresh(cs, false)
	want := "[5] +blog  https://example.com/a\n" +
		"---\n" +
		"1 bookmark updated, 1 tag added, 0 tags removed\n"
	if got != want {
		t.Errorf("FormatRefresh() = %q, want %q", got, want)
	}
}

func TestFormatRefresh_PluralRemovedWordingAcrossMultipleBookmarks(t *testing.T) {
	cs := []store.RefreshChange{
		{ID: 1, URL: "https://x.example/1", Removed: []string{"a"}},
		{ID: 2, URL: "https://x.example/2", Removed: []string{"b"}},
	}
	got := FormatRefresh(cs, false)
	want := "[1] -a  https://x.example/1\n" +
		"[2] -b  https://x.example/2\n" +
		"---\n" +
		"2 bookmarks updated, 0 tags added, 2 tags removed\n"
	if got != want {
		t.Errorf("FormatRefresh() = %q, want %q", got, want)
	}
}

func TestFormatRefresh_ReturnsEmptyStringForNoChanges(t *testing.T) {
	got := FormatRefresh(nil, false)
	if got != "" {
		t.Errorf("FormatRefresh() = %q, want empty string", got)
	}
}

type errWriter struct{ err error }

func (w errWriter) Write(p []byte) (int, error) { return 0, w.err }

func TestPrintList_WritesFormatListOutput(t *testing.T) {
	bs := []store.Bookmark{{ID: 1, URL: "https://example.com/a", CreatedAt: "2026-01-01T00:00:00Z"}}
	var buf bytes.Buffer
	if err := PrintList(&buf, bs); err != nil {
		t.Fatalf("PrintList() error = %v", err)
	}
	if buf.String() != FormatList(bs) {
		t.Errorf("PrintList() wrote %q, want %q", buf.String(), FormatList(bs))
	}
}

func TestPrintList_WritesZeroBytesWhenNoBookmarksEvenWithFailingWriter(t *testing.T) {
	w := errWriter{err: errors.New("boom")}
	if err := PrintList(w, nil); err != nil {
		t.Errorf("PrintList() error = %v, want nil", err)
	}
}

func TestPrintList_ReturnsErrorWhenWriterFails(t *testing.T) {
	bs := []store.Bookmark{{ID: 1, URL: "https://example.com/a", CreatedAt: "2026-01-01T00:00:00Z"}}
	wantErr := errors.New("boom")
	err := PrintList(errWriter{err: wantErr}, bs)
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintList() error = %v, want %v", err, wantErr)
	}
}

func TestPrintShow_WritesFormatShowOutput(t *testing.T) {
	b := store.Bookmark{ID: 1, URL: "https://example.com/a", CreatedAt: "2026-01-01T00:00:00Z"}
	var buf bytes.Buffer
	if err := PrintShow(&buf, b); err != nil {
		t.Fatalf("PrintShow() error = %v", err)
	}
	if buf.String() != FormatShow(b) {
		t.Errorf("PrintShow() wrote %q, want %q", buf.String(), FormatShow(b))
	}
}

func TestPrintShow_ReturnsErrorWhenWriterFails(t *testing.T) {
	b := store.Bookmark{ID: 1, URL: "https://example.com/a", CreatedAt: "2026-01-01T00:00:00Z"}
	wantErr := errors.New("boom")
	err := PrintShow(errWriter{err: wantErr}, b)
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintShow() error = %v, want %v", err, wantErr)
	}
}

func TestPrintShowJSON_WritesFormatShowJSONOutput(t *testing.T) {
	b := store.Bookmark{ID: 1, URL: "https://example.com/a", CreatedAt: "2026-01-01T00:00:00Z"}
	var buf bytes.Buffer
	if err := PrintShowJSON(&buf, b); err != nil {
		t.Fatalf("PrintShowJSON() error = %v", err)
	}
	want, err := FormatShowJSON(b)
	if err != nil {
		t.Fatalf("FormatShowJSON() error = %v", err)
	}
	if buf.String() != want {
		t.Errorf("PrintShowJSON() wrote %q, want %q", buf.String(), want)
	}
}

func TestPrintShowJSON_ReturnsErrorWhenWriterFails(t *testing.T) {
	b := store.Bookmark{ID: 1, URL: "https://example.com/a", CreatedAt: "2026-01-01T00:00:00Z"}
	wantErr := errors.New("boom")
	err := PrintShowJSON(errWriter{err: wantErr}, b)
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintShowJSON() error = %v, want %v", err, wantErr)
	}
}

func TestPrintShowJSONList_WritesFormatShowJSONListOutput(t *testing.T) {
	bs := []store.Bookmark{{ID: 1, URL: "https://example.com/a", CreatedAt: "2026-01-01T00:00:00Z"}}
	var buf bytes.Buffer
	if err := PrintShowJSONList(&buf, bs); err != nil {
		t.Fatalf("PrintShowJSONList() error = %v", err)
	}
	want, err := FormatShowJSONList(bs)
	if err != nil {
		t.Fatalf("FormatShowJSONList() error = %v", err)
	}
	if buf.String() != want {
		t.Errorf("PrintShowJSONList() wrote %q, want %q", buf.String(), want)
	}
}

func TestPrintShowJSONList_WritesEmptyArrayForNoBookmarks(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintShowJSONList(&buf, nil); err != nil {
		t.Fatalf("PrintShowJSONList() error = %v", err)
	}
	if buf.String() != "[]\n" {
		t.Errorf("PrintShowJSONList() wrote %q, want %q", buf.String(), "[]\n")
	}
}

func TestPrintShowJSONList_ReturnsErrorWhenWriterFails(t *testing.T) {
	wantErr := errors.New("boom")
	err := PrintShowJSONList(errWriter{err: wantErr}, nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintShowJSONList() error = %v, want %v", err, wantErr)
	}
}

func TestPrintAdd_WritesFormatAddOutput(t *testing.T) {
	r := store.AddResult{ID: 1, URL: "https://example.com/a", Created: true, AddedTags: []string{"blog"}}
	var buf bytes.Buffer
	if err := PrintAdd(&buf, r); err != nil {
		t.Fatalf("PrintAdd() error = %v", err)
	}
	if buf.String() != FormatAdd(r) {
		t.Errorf("PrintAdd() wrote %q, want %q", buf.String(), FormatAdd(r))
	}
}

func TestPrintAdd_ReturnsErrorWhenWriterFails(t *testing.T) {
	r := store.AddResult{ID: 1, URL: "https://example.com/a", Created: true}
	wantErr := errors.New("boom")
	err := PrintAdd(errWriter{err: wantErr}, r)
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintAdd() error = %v, want %v", err, wantErr)
	}
}

func TestPrintRemoved_WritesFormatRemovedOutput(t *testing.T) {
	b := store.Bookmark{ID: 1, URL: "https://example.com/a"}
	var buf bytes.Buffer
	if err := PrintRemoved(&buf, b); err != nil {
		t.Fatalf("PrintRemoved() error = %v", err)
	}
	if buf.String() != FormatRemoved(b) {
		t.Errorf("PrintRemoved() wrote %q, want %q", buf.String(), FormatRemoved(b))
	}
}

func TestPrintRemoved_ReturnsErrorWhenWriterFails(t *testing.T) {
	b := store.Bookmark{ID: 1, URL: "https://example.com/a"}
	wantErr := errors.New("boom")
	err := PrintRemoved(errWriter{err: wantErr}, b)
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintRemoved() error = %v, want %v", err, wantErr)
	}
}

func TestPrintTagCounts_WritesFormatTagCountsOutput(t *testing.T) {
	tcs := []store.TagCount{{Name: "blog", Count: 3}}
	var buf bytes.Buffer
	if err := PrintTagCounts(&buf, tcs); err != nil {
		t.Fatalf("PrintTagCounts() error = %v", err)
	}
	if buf.String() != FormatTagCounts(tcs) {
		t.Errorf("PrintTagCounts() wrote %q, want %q", buf.String(), FormatTagCounts(tcs))
	}
}

func TestPrintTagCounts_ReturnsErrorWhenWriterFails(t *testing.T) {
	wantErr := errors.New("boom")
	err := PrintTagCounts(errWriter{err: wantErr}, nil)
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintTagCounts() error = %v, want %v", err, wantErr)
	}
}

func TestPrintRefresh_WritesFormatRefreshOutput(t *testing.T) {
	cs := []store.RefreshChange{{ID: 1, URL: "https://example.com/a", Added: []string{"blog"}}}
	var buf bytes.Buffer
	if err := PrintRefresh(&buf, cs, false); err != nil {
		t.Fatalf("PrintRefresh() error = %v", err)
	}
	if buf.String() != FormatRefresh(cs, false) {
		t.Errorf("PrintRefresh() wrote %q, want %q", buf.String(), FormatRefresh(cs, false))
	}
}

func TestPrintRefresh_WritesZeroBytesWhenNoChangesEvenWithFailingWriter(t *testing.T) {
	w := errWriter{err: errors.New("boom")}
	if err := PrintRefresh(w, nil, false); err != nil {
		t.Errorf("PrintRefresh() error = %v, want nil", err)
	}
}

func TestPrintRefresh_ReturnsErrorWhenWriterFails(t *testing.T) {
	cs := []store.RefreshChange{{ID: 1, URL: "https://example.com/a", Added: []string{"blog"}}}
	wantErr := errors.New("boom")
	err := PrintRefresh(errWriter{err: wantErr}, cs, false)
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintRefresh() error = %v, want %v", err, wantErr)
	}
}

func TestPrintSeparator_WritesSixtyFourEqualsAndNewline(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintSeparator(&buf); err != nil {
		t.Fatalf("PrintSeparator() error = %v", err)
	}
	want := strings.Repeat("=", 64) + "\n"
	if buf.String() != want {
		t.Errorf("PrintSeparator() wrote %q, want %q", buf.String(), want)
	}
}

func TestPrintSeparator_ReturnsErrorWhenWriterFails(t *testing.T) {
	wantErr := errors.New("boom")
	err := PrintSeparator(errWriter{err: wantErr})
	if !errors.Is(err, wantErr) {
		t.Errorf("PrintSeparator() error = %v, want %v", err, wantErr)
	}
}
