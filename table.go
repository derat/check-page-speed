// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type tableCfg struct {
	spacing   int
	maxLines  int
	rightCols map[int]struct{}
}

type tableOpt func(cfg *tableCfg)

func tableSpacing(spaces int) tableOpt { return func(cfg *tableCfg) { cfg.spacing = spaces } }
func tableMaxLines(lines int) tableOpt { return func(cfg *tableCfg) { cfg.maxLines = lines } }
func tableRightCol(idx int) tableOpt   { return func(cfg *tableCfg) { cfg.rightCols[idx] = struct{}{} } }

func formatTable(rows [][]string, opts ...tableOpt) ([]string, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	cfg := tableCfg{
		spacing:   2,
		maxLines:  -1,
		rightCols: make(map[int]struct{}),
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	// Find the maximum width for each column.
	widths := make([]int, len(rows[0]))
	for i, row := range rows {
		if i > 0 && len(row) != len(rows[0]) {
			return nil, fmt.Errorf("row %d has %v column(s); want %v", i, len(row), len(rows[0]))
		}
		for j, val := range row {
			if width := utf8.RuneCountInString(val); width > widths[j] {
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

	if cfg.maxLines > 0 && len(lines) > cfg.maxLines {
		lines[cfg.maxLines-1] = fmt.Sprintf("[%d more]", len(lines)-cfg.maxLines+1)
		lines = lines[:cfg.maxLines]
	}

	return lines, nil
}
