// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"testing"
)

func TestElide(t *testing.T) {
	for _, tc := range []struct {
		in   string
		max  int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello there", 1, "…"},
		{"hello there", 2, "h…"},
		{"hello there", 9, "hello th…"},
		{"hello there", 10, "hello the…"},
		{"hello there", 11, "hello there"},
		{"hello there", 12, "hello there"},
		{"https://example.org/dir/file.html", 20, "https://example.org…"},
		{"https://example.org/dir/file.html", 21, "https://example.org/…"},
		{"https://example.org/dir/file.html", 22, "https://example.org/d…"},
		{"https://example.org/dir/file.html", 23, "https://example.org/d…l"},
		{"https://example.org/dir/file.html", 24, "https://example.org/di…l"},
		{"https://example.org/dir/file.html", 25, "https://example.org/di…ml"},
		{"https://example.org/dir/file.html", 26, "https://example.org/dir…ml"},
		{"https://example.org/dir/file.html", 27, "https://example.org/dir…tml"},
		{"https://example.org/dir/file.html", 28, "https://example.org/dir/…tml"},
		{"https://example.org/dir/file.html", 29, "https://example.org/dir/…html"},
		{"https://example.org/dir/file.html", 30, "https://example.org/dir/f…html"},
		{"https://example.org/dir/file.html", 31, "https://example.org/dir/f….html"},
		{"https://example.org/dir/file.html", 32, "https://example.org/dir/fi….html"},
		{"https://example.org/dir/file.html", 33, "https://example.org/dir/file.html"},
		{"https://example.org/dir/file.html", 34, "https://example.org/dir/file.html"},
		{"https://example.org/dir/file.html", 35, "https://example.org/dir/file.html"},
	} {
		if got := elide(tc.in, tc.max); got != tc.want {
			t.Errorf("elide(%q, %v) = %q; want %q", tc.in, tc.max, got, tc.want)
		}
	}
}

func TestUrlPath(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"https://www.example.org", ""},
		{"https://www.example.org/", "/"},
		{"https://www.example.org/foo/bar.html", "/foo/bar.html"},
		{"https://www.example.org:443/foo/bar.html", "/foo/bar.html"},
		{"foo", "foo"},
		{"", ""},
	} {
		if got := urlPath(tc.in); got != tc.want {
			t.Errorf("urlPath(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}
