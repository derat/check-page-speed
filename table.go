// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type table struct {
	rows [][]string
}

func newTable() *table {
	return &table{}
}

func (t *table) appendRow(row []string) {
	if len(t.rows) > 0 && len(row) != len(t.rows[0]) {
		panic(fmt.Sprintf("Row has wrong number of columns (got %v; want %v)", len(row), len(t.rows[0])))
	}
	t.rows = append(t.rows, row)
}

func (t *table) format(maxLines, spacing int) []string {
	if len(t.rows) == 0 {
		return nil
	}

	// Find the maximum width for each column.
	widths := make([]int, len(t.rows[0]))
	for _, row := range t.rows {
		for i, val := range row {
			if width := utf8.RuneCountInString(val); width > widths[i] {
				widths[i] = width
			}
		}
	}

	lines := make([]string, len(t.rows))
	for i, row := range t.rows {
		for j, val := range row {
			width := widths[j]
			if width == 0 {
				continue // skip completely-empty columns
			}
			lines[i] += val
			if j < len(row)-1 {
				// TODO: Support right alignment.
				lines[i] += strings.Repeat(" ", width-utf8.RuneCountInString(val)+spacing)
			}
		}
	}

	if len(lines) > maxLines {
		lines[maxLines-1] = fmt.Sprintf("[%d more]", len(lines)-maxLines+1)
		lines = lines[:maxLines]
	}

	return lines
}
