// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"strings"
	"unicode/utf8"
)

type tableCfg struct {
	spacing   int
	rightCols map[int]struct{}
}

type tableOpt func(cfg *tableCfg)

func tableSpacing(spaces int) tableOpt { return func(cfg *tableCfg) { cfg.spacing = spaces } }
func tableRightCol(idx int) tableOpt   { return func(cfg *tableCfg) { cfg.rightCols[idx] = struct{}{} } }

// formatTable formats the supplied rows as lines of aligned columns.
// Rows can have different numbers of columns.
func formatTable(rows [][]string, opts ...tableOpt) []string {
	if len(rows) == 0 {
		return nil
	}

	cfg := tableCfg{
		spacing:   2,
		rightCols: make(map[int]struct{}),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Find the maximum width for each column.
	widths := make([]int, len(rows[0]))
	for _, row := range rows {
		for j, val := range row {
			if width := utf8.RuneCountInString(val); j >= len(widths) {
				widths = append(widths, width)
			} else if width > widths[j] {
				widths[j] = width
			}
		}
	}

	lines := make([]string, len(rows))
	for i, row := range rows {
		for j, val := range row {
			width := widths[j]
			if width == 0 {
				continue // skip completely-empty columns
			}
			pad := strings.Repeat(" ", width-utf8.RuneCountInString(val))
			_, right := cfg.rightCols[j]
			if right {
				lines[i] += pad
			}
			lines[i] += val
			if j < len(row)-1 {
				if !right {
					lines[i] += pad
				}
				lines[i] += strings.Repeat(" ", cfg.spacing)
			}
		}
	}
	return lines
}
