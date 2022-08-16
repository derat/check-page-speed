// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	pso "google.golang.org/api/pagespeedonline/v5"
)

const (
	maxDetailLen = 40
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flag]... <url>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Prints analysis from PageSpeed Insights.\n\n")
		flag.PrintDefaults()
	}
	key := flag.String("key", "", "API key to use (empty for no key)")
	mobile := flag.Bool("mobile", false, "Analyzes the page as a mobile (rather than desktop) device")
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
		Category("PERFORMANCE", "BEST_PRACTICES", "ACCESSIBILITY", "SEO").
		Strategy(strategy).
		Do(opts...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed calling service:", err)
		os.Exit(1)
	}

	fmt.Println(res.Id)
	lhr := res.LighthouseResult
	cats := lhr.Categories
	for _, cat := range []*pso.LighthouseCategoryV5{
		cats.Performance,
		cats.BestPractices,
		cats.Accessibility,
		cats.Seo,
	} {
		fmt.Printf("%3d %s\n", score100(cat.Score), cat.Title)
		for _, ar := range cat.AuditRefs {
			audit, ok := lhr.Audits[ar.Id]
			if !ok {
				fmt.Fprintf(os.Stderr, "Missing audit %q\n", ar.Id)
				continue
			}
			score := score100(audit.Score)
			if score < 0 || score == 100 {
				continue
			}
			text := audit.Title
			if audit.DisplayValue != "" {
				text += ": " + audit.DisplayValue
			}
			fmt.Printf("    %3d %s\n", score, text)
			if det := formatDetails(audit.Details); len(det) > 0 {
				for _, s := range det {
					fmt.Printf("        %s\n", s)
				}
			}
		}
	}
}

func score100(score interface{}) int {
	f, ok := score.(float64)
	if !ok {
		return -1
	}
	return int(math.Round(f * 100))
}

func formatDetails(raw googleapi.RawMessage) []string {
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
		return []string{elide(string(raw), maxDetailLen)}
	}
	if len(details.Headings) == 0 || len(details.Items) == 0 {
		return nil
	}

	table := newTable()
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
	table.appendRow(headings)

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
		table.appendRow(row)
	}

	return table.format(2)
}