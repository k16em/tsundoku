package normalize

import "testing"

func TestURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name: "a plain https URL passes through unchanged",
			raw:  "https://example.com/a",
			want: "https://example.com/a",
		},
		{
			name: "an uppercase scheme and host are lowercased",
			raw:  "HTTPS://Example.COM/a",
			want: "https://example.com/a",
		},
		{
			name: "the path is not lowercased",
			raw:  "https://example.com/A",
			want: "https://example.com/A",
		},
		{
			name: "an empty fragment is dropped",
			raw:  "https://example.com/a#",
			want: "https://example.com/a",
		},
		{
			name: "a non-empty fragment is kept",
			raw:  "https://example.com/a#frag",
			want: "https://example.com/a#frag",
		},
		{
			name: "a trailing slash is not normalized away",
			raw:  "https://example.com/a/",
			want: "https://example.com/a/",
		},
		{
			name: "query parameters are preserved as-is",
			raw:  "https://example.com/a?b=2&a=1",
			want: "https://example.com/a?b=2&a=1",
		},
		{
			name:    "an ftp scheme is rejected",
			raw:     "ftp://example.com/a",
			wantErr: true,
		},
		{
			name:    "a file scheme is rejected",
			raw:     "file:///tmp/a",
			wantErr: true,
		},
		{
			name:    "a missing scheme is rejected",
			raw:     "example.com/a",
			wantErr: true,
		},
		{
			name:    "an empty host is rejected",
			raw:     "https:///a",
			wantErr: true,
		},
		{
			name:    "an empty string is rejected",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "an invalid percent-escape is rejected by the URL parser itself",
			raw:     "https://example.com/%zz",
			wantErr: true,
		},
		{
			name: "percent-encoded userinfo is preserved verbatim",
			raw:  "https://%41@example.com/a",
			want: "https://%41@example.com/a",
		},
		{
			name: "mixed-case percent-encoding in the path is preserved verbatim",
			raw:  "https://example.com/%7ea%2Fb",
			want: "https://example.com/%7ea%2Fb",
		},
		{
			name: "mixed-case percent-encoding and a plus sign in the query are preserved verbatim",
			raw:  "https://example.com/a?x=%3D+%2b",
			want: "https://example.com/a?x=%3D+%2b",
		},
		{
			name: "only the host is lowercased, leaving userinfo case alone",
			raw:  "https://user:PassWord@Example.COM/a",
			want: "https://user:PassWord@example.com/a",
		},
		{
			name: "the host is lowercased but the path is not, and the port is kept",
			raw:  "https://Example.COM:8443/A",
			want: "https://example.com:8443/A",
		},
		{
			name: "a bare authority with no terminator is returned unchanged with no trailing slash added",
			raw:  "https://example.com",
			want: "https://example.com",
		},
		{
			name: "a bare uppercase authority with no terminator is lowercased",
			raw:  "HTTPS://EXAMPLE.COM",
			want: "https://example.com",
		},
		{
			name: "a bare authority with a port and no terminator lowercases the host and keeps the port",
			raw:  "https://Example.COM:8443",
			want: "https://example.com:8443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := URL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("URL(%q) = %q, nil; want error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("URL(%q) returned unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("URL(%q) = %q; want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestTag(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name: "surrounding whitespace is trimmed and the tag is lowercased",
			raw:  "  Blog  ",
			want: "blog",
		},
		{
			name: "a Japanese tag passes through since lowercasing only affects ASCII",
			raw:  "日記",
			want: "日記",
		},
		{
			name:    "an empty string is rejected",
			raw:     "",
			wantErr: true,
		},
		{
			name:    "a whitespace-only string is rejected",
			raw:     "   ",
			wantErr: true,
		},
		{
			name:    "a tag containing a newline is rejected",
			raw:     "blog\ntech",
			wantErr: true,
		},
		{
			name:    "a tag containing an escape control character is rejected",
			raw:     "blog\x1b",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Tag(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Tag(%q) = %q, nil; want error", tt.raw, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Tag(%q) returned unexpected error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("Tag(%q) = %q; want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDedupTags(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want []string
	}{
		{
			name: "duplicates are removed while preserving input order",
			tags: []string{"a", "b", "a", "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "an empty slice does not panic and returns no tags",
			tags: []string{},
			want: []string{},
		},
		{
			name: "a nil slice does not panic and returns no tags",
			tags: nil,
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DedupTags(tt.tags)
			if len(got) != len(tt.want) {
				t.Fatalf("DedupTags(%v) = %v; want %v", tt.tags, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("DedupTags(%v) = %v; want %v", tt.tags, got, tt.want)
				}
			}
		})
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want int
	}{
		{name: "zero is clamped up to the minimum of 1", n: 0, want: 1},
		{name: "a negative number is clamped up to the minimum of 1", n: -5, want: 1},
		{name: "a value within range is left unchanged", n: 20, want: 20},
		{name: "a value above the maximum is clamped down to 1000", n: 5000, want: 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampLimit(tt.n)
			if got != tt.want {
				t.Fatalf("ClampLimit(%d) = %d; want %d", tt.n, got, tt.want)
			}
		})
	}
}
