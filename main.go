// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
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
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flag]... <url> <url>...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Prints analysis from PageSpeed Insights.\n\n")
		flag.PrintDefaults()
	}

	var cfg writeConfig
	flag.StringVar(&cfg.audits, "audits", auditsFailed,
		fmt.Sprintf("Audits to print (%q, %q, %q)", auditsFailed, auditsAll, auditsNone))
	flag.IntVar(&cfg.maxDetails, "details", 5, "Maximum details for each audit (-1 for all)")
	flag.IntVar(&cfg.detailWidth, "detail-width", 40, "Maximum audit detail column width (0 or -1 for no limit)")
	key := flag.String("key", "", "API key to use (empty for no key)")
	mobile := flag.Bool("mobile", false, "Analyzes the page as a mobile (rather than desktop) device")
	flag.BoolVar(&cfg.pathOnly, "path-only", false, "Just print URL paths in report")
	retries := flag.Int("retries", 2, "Maximum retries after failed calls to API")
	verbose := flag.Bool("verbose", false, "Log verbosely")
	workers := flag.Int("workers", 32, "Maximum simultaneous calls to API")
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	urls := flag.Args()

	vlogf := func(format string, args ...interface{}) {
		if *verbose {
			log.Printf(format, args...)
		}
	}

	os.Exit(func() int {
		vlogf("Creating service")
		svc, err := pso.NewService(context.Background(), option.WithoutAuthentication())
		if err != nil {
			log.Print("Failed creating service: ", err)
			return 1
		}
		apiSvc := pso.NewPagespeedapiService(svc)

		var apiOpts []googleapi.CallOption
		if *key != "" {
			apiOpts = append(apiOpts, googleapi.QueryParameter("key", *key))
		} else {
			vlogf("Anonymous access is unreliable; consider passing -key: " +
				"https://developers.google.com/speed/docs/insights/v5/get-started#key")
		}

		type job struct {
			url      string
			rep      *report
			err      error
			attempts int
		}
		jobs := make(chan job, len(urls))       // send jobs to workers
		results := make(chan job, len(urls))    // receive jobs from workers
		done := make(map[string]job, len(urls)) // completed jobs, keyed by URL

		for i := 0; i < *workers; i++ {
			go func() {
				for job := range jobs {
					vlogf("Starting attempt #%d for %v", job.attempts+1, job.url)
					job.rep, job.err = getReport(apiSvc, job.url, *mobile, apiOpts)
					vlogf("Finished attempt #%d for %v", job.attempts+1, job.url)
					job.attempts++
					results <- job
				}
			}()
		}
		for _, u := range urls {
			jobs <- job{url: u}
		}
		for len(done) < len(urls) {
			job := <-results
			if job.err != nil && job.attempts <= *retries {
				log.Printf("Will retry %v: %v", job.url, job.err)
				jobs <- job
			} else {
				done[job.url] = job
			}
		}
		close(jobs) // stop workers

		reports := make([]*report, len(urls))
		for i, url := range urls {
			if job := done[url]; job.err != nil {
				log.Printf("Failed getting %v: %v", url, job.err)
				reports[i] = &report{URL: url}
			} else {
				reports[i] = job.rep
			}
		}

		if err := writeSummary(os.Stdout, reports, &cfg); err != nil {
			log.Print("Failed writing summary: ", err)
			return 1
		}
		fmt.Fprintln(os.Stdout)

		for _, rep := range reports {
			fmt.Fprintln(os.Stdout, strings.Repeat("=", reportDividerLen))
			if err := writeReport(os.Stdout, rep, &cfg); err != nil {
				log.Printf("Failed writing report for %v: %v", rep.URL, err)
				return 1
			}
		}
		return 0
	}())
}

type writeConfig struct {
	pathOnly    bool   // print paths instead of full URLs in summary table
	audits      string // auditsFailed, auditsAll, auditsNone
	maxDetails  int    // max number of details to print per audit
	detailWidth int    // max width of each column in a detail
}

const (
	auditsFailed = "failed"
	auditsAll    = "all"
	auditsNone   = "none"
)

// getReport uses svc to fetch and read a report for url.
func getReport(svc *pso.PagespeedapiService, url string, mobile bool,
	opts []googleapi.CallOption) (*report, error) {
	strategy := "DESKTOP"
	if mobile {
		strategy = "MOBILE"
	}
	res, err := svc.Runpagespeed(url).
		Category("PERFORMANCE", "BEST_PRACTICES", "ACCESSIBILITY", "SEO", "PWA").
		Strategy(strategy).
		Do(opts...)
	if err != nil {
		return nil, err
	}
	return readReport(res)
}

// writeSummary writes a text table to w summarizing the category scores
// of each of the supplied reports.
func writeSummary(w io.Writer, reps []*report, cfg *writeConfig) error {
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

// writeReport writes rep to w in text format.
func writeReport(w io.Writer, rep *report, cfg *writeConfig) error {
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
				details, err := formatTable(aud.Details, tableSpacing(2))
				if err != nil {
					return fmt.Errorf("%q details: %v", aud.Title, err)
				}
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
