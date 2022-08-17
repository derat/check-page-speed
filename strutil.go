// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"net/url"
	"regexp"
	"unicode/utf8"
)

const elideURLMinPath = 5

func elide(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}

	ms := elideURLRegexp.FindStringSubmatch(s)
	if ms != nil && utf8.RuneCountInString(ms[1]) < max {
		url, end := []rune(ms[1]), []rune(ms[2])
		url = append(url, end[:(max-len(url))/2]...)
		url = append(url, '…')
		if rem := max - len(url); rem > 0 {
			url = append(url, end[len(end)-rem:]...)
		}
		return string(url)
	}

	return string(r[:max-1]) + "…"
}

// Extracts the '[scheme]://[authority]/' part and remainder of a URL.
var elideURLRegexp = regexp.MustCompile(`^([^/]+://[^/]+/)(.+)$`)

func urlPath(full string) string {
	url, err := url.Parse(full)
	if err != nil {
		return full
	}
	url.Scheme = ""
	url.User = nil
	url.Host = ""
	return url.String()
}
