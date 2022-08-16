// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/url"
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
	key := flag.String("key", "", "API key to use (empty for no key)")
	mobile := flag.Bool("mobile", false, "Analyzes the page as a mobile (rather than desktop) device")
	pathOnly := flag.Bool("path-only", false, "Just print URL paths in report")
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

	rep := report{URL: res.Id}
	lhr := res.LighthouseResult
	for _, lhrCat := range []*pso.LighthouseCategoryV5{
		// This matches the order in Chrome DevTools.
		lhr.Categories.Performance,
		lhr.Categories.Accessibility,
		lhr.Categories.BestPractices,
		lhr.Categories.Seo,
		lhr.Categories.Pwa,
	} {
		cat := category{
			Title:  lhrCat.Title,
			Abbrev: categoryAbbrev(lhrCat.Id),
			Score:  score100(lhrCat.Score),
		}
		for _, ar := range lhrCat.AuditRefs {
			lhrAudit, ok := lhr.Audits[ar.Id]
			if !ok {
				log.Printf("%v category %q is missing audit %q", rep.URL, cat.Title, ar.Id)
				continue
			}
			cat.Audits = append(cat.Audits, audit{
				Title:   lhrAudit.Title,
				Score:   score100(lhrAudit.Score),
				Details: getDetails(lhrAudit.Details),
			})
		}
		rep.Categories = append(rep.Categories, cat)
	}

	reports := []report{rep}

	if err := writeSummary(os.Stdout, reports, *pathOnly); err != nil {
		fmt.Fprintln(os.Stderr, "Failed writing summary:", err)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stdout)

	for _, rep := range reports {
		fmt.Fprintln(os.Stdout, strings.Repeat("=", reportDividerLen))
		if err := writeReport(os.Stdout, rep); err != nil {
			fmt.Fprintf(os.Stderr, "Failed writing report for %v: %v", rep.URL, err)
			os.Exit(1)
		}
	}
}

func writeSummary(w io.Writer, reps []report, pathOnly bool) error {
	rows := [][]string{[]string{"URL"}}
	tableOpts := []tableOpt{tableSpacing(2)}
	for i, cat := range reps[0].Categories {
		rows[0] = append(rows[0], cat.Abbrev)
		tableOpts = append(tableOpts, tableRightCol(i+1))
	}
	for _, rep := range reps {
		var row []string
		if pathOnly {
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

func writeReport(w io.Writer, rep report) error {
	fmt.Fprintln(w, rep.URL)
	fmt.Fprintln(w)

	for _, cat := range rep.Categories {
		fmt.Fprintf(w, "%3d %s\n", cat.Score, cat.Title)
		fmt.Fprintln(w, strings.Repeat("-", catUnderlineLen))
		for _, aud := range cat.Audits {
			if aud.Score < 0 || aud.Score == 100 {
				continue
			}
			text := aud.Title
			if aud.Value != "" {
				text += ": " + aud.Value
			}
			fmt.Fprintf(w, "%3d %s\n", aud.Score, text)
			details, err := formatTable(aud.Details, tableSpacing(2), tableMaxLines(maxDetailLines))
			if err != nil {
				return fmt.Errorf("%q details: %v", aud.Title, err)
			}
			for _, det := range details {
				fmt.Fprintf(w, "    %s\n", det)
			}
		}
		fmt.Fprintln(w)
	}
	return nil
}

func score100(score interface{}) int {
	f, ok := score.(float64)
	if !ok {
		return -1
	}
	return int(math.Round(f * 100))
}

func getDetails(raw googleapi.RawMessage) [][]string {
	if len(raw) == 0 {
		return nil
	}
	var details struct {
		Type     string `json:"type"`
		Headings []struct {
			Key      string `json:"key"`
			Text     string `json:"text"`
			Label    string `json:"label"`
			ItemType string `json:"itemType"`
		} `json:"headings"`
		Items []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(raw, &details); err != nil {
		return [][]string{{string(raw)}}
	}
	if len(details.Headings) == 0 || len(details.Items) == 0 {
		return nil
	}

	var headings, keys, units []string // names, keys, and units for each column
	for _, h := range details.Headings {
		var name string
		if h.Text != "" {
			name = h.Text
		} else if h.Label != "" {
			name = h.Label
		}
		headings = append(headings, strings.TrimSpace(name))

		var un string
		switch h.ItemType {
		case "ms", "bytes":
			un = h.ItemType
		}
		units = append(units, un)

		keys = append(keys, h.Key)
	}

	rows := [][]string{headings}
	for _, item := range details.Items {
		var row []string
		for i, key := range keys {
			var val string
			if v, ok := item[key]; ok {
				switch vt := v.(type) {
				case string:
					val = strings.TrimSpace(vt)
				case float64:
					val = strings.TrimSuffix(fmt.Sprintf("%.1f", vt), ".0")
					if un := units[i]; un != "" {
						val += " " + un
					}
				case map[string]interface{}:
					if s, ok := vt["snippet"].(string); ok {
						val = s
					} else if s, ok := vt["url"].(string); ok {
						val = s
					} else {
						val = fmt.Sprint(vt)
					}
				default:
					val = fmt.Sprint(vt)
				}
			}
			row = append(row, elide(val, maxDetailLen))
		}
		rows = append(rows, row)
	}

	return rows
}

type report struct {
	URL        string
	Categories []category
}

type category struct {
	Title  string
	Abbrev string
	Score  int // [0, 100]
	Audits []audit
}

type audit struct {
	Title   string
	Score   int // [0, 100] or -1 if unset
	Value   string
	Details [][]string
}

func categoryAbbrev(id string) string {
	switch id {
	case "accessibility":
		return "A11Y"
	case "best-practices":
		return "Best"
	case "performance":
		return "Perf"
	case "pwa":
		return "PWA"
	case "seo":
		return "SEO"
	}
	return id
}

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
