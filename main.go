// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	pso "google.golang.org/api/pagespeedonline/v5"
)

const keyEnv = "PAGE_SPEED_API_KEY"

type reportConfig struct {
	startTime   time.Time
	mobile      bool   // generate reports for mobile rather than desktop
	mailAddr    string // email address to send to ("-" to dump to stdout)
	fullURLs    bool   // print full URLs instead of paths in summary table
	audits      string // auditsFailed, auditsAll, auditsNone
	maxDetails  int    // max number of details to print per audit
	detailWidth int    // max width of each column in a detail
}

const (
	auditsFailed = "failed"
	auditsAll    = "all"
	auditsNone   = "none"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flag]... <url> <url>...\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Prints analysis from PageSpeed Insights.\n\n")
		flag.PrintDefaults()
	}

	cfg := reportConfig{startTime: time.Now()}
	flag.StringVar(&cfg.audits, "audits", auditsFailed,
		fmt.Sprintf("Audits to print (%q, %q, %q)", auditsFailed, auditsAll, auditsNone))
	flag.IntVar(&cfg.maxDetails, "details", 10, "Maximum details for each audit (-1 for all)")
	flag.IntVar(&cfg.detailWidth, "detail-width", 40, "Maximum audit detail column width (-1 for no limit)")
	flag.BoolVar(&cfg.fullURLs, "full-urls", false, "Print full URLs (instead of paths) in report")
	key := flag.String("key", os.Getenv(keyEnv), fmt.Sprintf("API key to use (can also set %v)", keyEnv))
	flag.StringVar(&cfg.mailAddr, "mail", "", "Email address to mail report to (write report to stdout if empty)")
	flag.BoolVar(&cfg.mobile, "mobile", false, "Analyzes the page as a mobile (rather than desktop) device")
	retries := flag.Int("retries", 2, "Maximum retries after failed calls to API")
	verbose := flag.Bool("verbose", false, "Log verbosely")
	workers := flag.Int("workers", 8, "Maximum simultaneous calls to API")
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
					job.rep, job.err = getReport(apiSvc, job.url, cfg.mobile, apiOpts)
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
				// The API fails often, so make retries silent.
				vlogf("Will retry %v: %v", job.url, job.err)
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

		if cfg.mailAddr != "" {
			vlogf("Sending mail to %v", cfg.mailAddr)
			if err := sendMail(reports, &cfg); err != nil {
				log.Print("Failed sending mail: ", err)
				return 1
			}
		} else {
			if err := writeSummary(os.Stdout, reports, &cfg); err != nil {
				log.Print("Failed writing summary: ", err)
				return 1
			}
			fmt.Fprintln(os.Stdout)
			if err := writeReports(os.Stdout, reports, &cfg); err != nil {
				log.Print("Failed writing reports: ", err)
				return 1
			}
		}
		return 0
	}())
}

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
