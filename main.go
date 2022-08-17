// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	pso "google.golang.org/api/pagespeedonline/v5"
)

const (
	reportDividerLen = 80 // length of '=' dividers between URL reports
	catUnderlineLen  = 20 // length of '-' underlines below category names
	maxDetailLen     = 40
	maxDetailLines   = 5
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flag]... <url>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Prints analysis from PageSpeed Insights.\n\n")
		flag.PrintDefaults()
	}
	var cfg config
	flag.StringVar(&cfg.audits, "audits", auditsFailed,
		fmt.Sprintf("Audits to print (%q, %q, %q)", auditsFailed, auditsAll, auditsNone))
	flag.BoolVar(&cfg.details, "details", true, "Print audit details")
	key := flag.String("key", "", "API key to use (empty for no key)")
	mobile := flag.Bool("mobile", false, "Analyzes the page as a mobile (rather than desktop) device")
	flag.BoolVar(&cfg.pathOnly, "path-only", false, "Just print URL paths in report")
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	url := flag.Arg(0)

	svc, err := pso.NewService(context.Background(), option.WithoutAuthentication())
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed creating service:", err)
		os.Exit(1)
	}

	strategy := "DESKTOP"
	if *mobile {
		strategy = "MOBILE"
	}
	var opts []googleapi.CallOption
	if *key != "" {
		opts = append(opts, googleapi.QueryParameter("key", *key))
	}
	res, err := pso.NewPagespeedapiService(svc).Runpagespeed(url).
		Category("PERFORMANCE", "BEST_PRACTICES", "ACCESSIBILITY", "SEO", "PWA").
		Strategy(strategy).
		Do(opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed calling service:", err)
		os.Exit(1)
	}

	rep, err := readReport(res)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed reeading report for %v: %v\n", res.Id, err)
		os.Exit(1)
	}
	reports := []*report{rep}

	if err := writeSummary(os.Stdout, reports, &cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Failed writing summary:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout)

	for _, rep := range reports {
		fmt.Fprintln(os.Stdout, strings.Repeat("=", reportDividerLen))
		if err := writeReport(os.Stdout, rep, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Failed writing report for %v: %v", rep.URL, err)
			os.Exit(1)
		}
	}
}

type config struct {
	pathOnly bool
	audits   string
	details  bool
}

const (
	auditsFailed = "failed"
	auditsAll    = "all"
	auditsNone   = "none"
)

func writeSummary(w io.Writer, reps []*report, cfg *config) error {
	rows := [][]string{[]string{"URL"}}
	tableOpts := []tableOpt{tableSpacing(2)}
	for i, cat := range reps[0].Categories {
		rows[0] = append(rows[0], cat.Abbrev)
		tableOpts = append(tableOpts, tableRightCol(i+1))
	}
	for _, rep := range reps {
		var row []string
		if cfg.pathOnly {
			row = append(row, urlPath(rep.URL))
		} else {
			row = append(row, rep.URL)
		}
		for _, cat := range rep.Categories {
			row = append(row, strconv.Itoa(cat.Score))
		}
		rows = append(rows, row)
	}
	lines, err := formatTable(rows, tableOpts...)
	if err != nil {
		return fmt.Errorf("failed formatting table: %v", err)
	}
	for _, ln := range lines {
		fmt.Fprintln(w, ln)
	}
	return nil
}

func writeReport(w io.Writer, rep *report, cfg *config) error {
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

			if cfg.details {
				details, err := formatTable(aud.Details, tableSpacing(2), tableMaxLines(maxDetailLines))
				if err != nil {
					return fmt.Errorf("%q details: %v", aud.Title, err)
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
