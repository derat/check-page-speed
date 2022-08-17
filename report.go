// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"google.golang.org/api/googleapi"
	pso "google.golang.org/api/pagespeedonline/v5"
)

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

func readReport(res *pso.PagespeedApiPagespeedResponseV5) (*report, error) {
	rep := &report{URL: res.Id}
	lhr := res.LighthouseResult
	for _, lhrCat := range []*pso.LighthouseCategoryV5{
		// This matches the order in Chrome DevTools.
		lhr.Categories.Performance,
		lhr.Categories.Accessibility,
		lhr.Categories.BestPractices,
		lhr.Categories.Seo,
		lhr.Categories.Pwa,
	} {
		if lhrCat == nil {
			continue
		}
		cat := category{
			Title:  lhrCat.Title,
			Abbrev: categoryAbbrev(lhrCat.Id),
			Score:  score100(lhrCat.Score),
		}
		for _, ar := range lhrCat.AuditRefs {
			lhrAudit, ok := lhr.Audits[ar.Id]
			if !ok {
				return nil, fmt.Errorf("category %q is missing audit %q", cat.Title, ar.Id)
			}
			cat.Audits = append(cat.Audits, audit{
				Title:   lhrAudit.Title,
				Score:   score100(lhrAudit.Score),
				Details: getDetails(lhrAudit.Details),
			})
		}
		rep.Categories = append(rep.Categories, cat)
	}

	return rep, nil
}

func score100(score interface{}) int {
	f, ok := score.(float64)
	if !ok {
		return -1
	}
	return int(math.Round(f * 100))
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
