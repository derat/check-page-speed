// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	reportDividerLen = 80 // length of '=' dividers between URL reports
	catUnderlineLen  = 20 // length of '-' underlines below category names
)

// writeSummary writes a text table to w summarizing the category scores
// of each of the supplied reports.
func writeSummary(w io.Writer, reps []*report, cfg *reportConfig) error {
	// Add a heading row to the table, using categories from the first non-failed report.
	rows := [][]string{[]string{"URL"}}
	tableOpts := []tableOpt{tableSpacing(2)}
	for _, rep := range reps {
		if len(rep.Categories) > 0 {
			for i, cat := range rep.Categories {
				rows[0] = append(rows[0], cat.Abbrev)
				tableOpts = append(tableOpts, tableRightCol(i+1))
			}
			break
		}
	}

	for _, rep := range reps {
		var row []string
		if cfg.fullURLs {
			row = append(row, rep.URL)
		} else {
			row = append(row, urlPath(rep.URL))
		}
		for _, cat := range rep.Categories {
			row = append(row, strconv.Itoa(cat.Score))
		}
		rows = append(rows, row)
	}
	for _, ln := range formatTable(rows, tableOpts...) {
		fmt.Fprintln(w, ln)
	}
	return nil
}

// writeReports calls writeReport, printing a divider line between each report.
func writeReports(w io.Writer, reps []*report, cfg *reportConfig) error {
	for _, rep := range reps {
		fmt.Fprint(w, strings.Repeat("=", reportDividerLen)+"\n\n")
		if err := writeReport(w, rep, cfg); err != nil {
			return fmt.Errorf("%v: %v", rep.URL, err)
		}
	}
	return nil
}

// writeReport writes rep to w in text format.
func writeReport(w io.Writer, rep *report, cfg *reportConfig) error {
	fmt.Fprintln(w, rep.URL)
	fmt.Fprintln(w)

	for _, cat := range rep.Categories {
		fmt.Fprintf(w, "%3d %s\n", cat.Score, cat.Title)
		if cfg.audits == auditsNone {
			continue
		}
		fmt.Fprintln(w, strings.Repeat("-", catUnderlineLen))
		for _, aud := range cat.Audits {
			if cfg.audits == auditsFailed && (aud.Score < 0 || aud.Score == 100) {
				continue
			}

			var ln string
			if aud.Score >= 0 {
				ln += fmt.Sprintf("%3d", aud.Score)
			} else {
				ln += "  ."
			}
			ln += " " + aud.Title
			if aud.Value != "" {
				ln += ": " + aud.Value
			}
			fmt.Fprintln(w, ln)

			if len(aud.Details) > 0 && cfg.maxDetails != 0 {
				// Elide long values.
				if cfg.detailWidth > 0 {
					for _, row := range aud.Details {
						for j, val := range row {
							row[j] = elide(val, cfg.detailWidth)
						}
					}
				}
				details := formatTable(aud.Details, tableSpacing(2))
				if cfg.maxDetails > 0 && len(details) > cfg.maxDetails {
					details[cfg.maxDetails-1] = fmt.Sprintf("[%d more]", len(details)-cfg.maxDetails+1)
					details = details[:cfg.maxDetails]
				}
				for _, det := range details {
					fmt.Fprintf(w, "    %s\n", det)
				}
			}
		}
		fmt.Fprintln(w)
	}
	return nil
}
