package normalize

import (
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// URL validates and minimally normalizes a bookmark URL.
func URL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("normalize: invalid URL %q: %w", raw, err)
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("normalize: unsupported URL scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("normalize: URL %q has no host", raw)
	}

	schemeEnd := strings.IndexByte(raw, ':')
	authorityStart := schemeEnd + len("://")

	authorityEnd := len(raw)
	if rest := strings.IndexAny(raw[authorityStart:], "/?#"); rest >= 0 {
		authorityEnd = authorityStart + rest
	}

	hostStart := authorityStart
	if at := strings.LastIndexByte(raw[authorityStart:authorityEnd], '@'); at >= 0 {
		hostStart = authorityStart + at + 1
	}

	var b strings.Builder
	b.WriteString(strings.ToLower(raw[:schemeEnd]))
	b.WriteString("://")
	b.WriteString(raw[authorityStart:hostStart])
	b.WriteString(strings.ToLower(raw[hostStart:authorityEnd]))
	b.WriteString(raw[authorityEnd:])
	result := b.String()

	if u.Fragment == "" && strings.HasSuffix(result, "#") {
		result = result[:len(result)-1]
	}

	return result, nil
}

// Tag validates and normalizes a tag name.
func Tag(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", fmt.Errorf("normalize: tag is empty")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("normalize: tag %q contains a control character", raw)
		}
	}
	return trimmed, nil
}

// DedupTags removes duplicates while preserving input order.
func DedupTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}

// ClampLimit constrains a limit to the supported range.
func ClampLimit(n int) int {
	const (
		min = 1
		max = 1000
	)
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}
