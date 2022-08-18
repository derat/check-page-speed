# check-page-speed

[![Build Status](https://storage.googleapis.com/derat-build-badges/ed044c39-7d4b-431b-b015-0a3864a38b45.svg)](https://storage.googleapis.com/derat-build-badges/ed044c39-7d4b-431b-b015-0a3864a38b45.html)

`check-page-speed` is a command-line program that uses Google's free [PageSpeed
Insights API] to generate [Lighthouse] reports about one or more web pages,
optionally sending them via email. Its main purpose is automated monitoring of a
website's performance, accessibility, adherence to best practices, SEO, and PWA
support.

[PageSpeed Insights API]: https://developers.google.com/speed/docs/insights/v5/get-started
[Lighthouse]: https://github.com/GoogleChrome/lighthouse

## Usage

To compile and install the `check-page-speed` executable, run `go install` from
the root of this repository. You will need to have [Go] installed.

[Go]: https://go.dev/

The program accepts flags to control its behaior followed by one or more URLs to
analyze:

```
Usage: check-page-speed [flag]... <url>...
Analyzes web pages using PageSpeed Insights.

  -audits string
        Audits to print ("failed", "all", "none") (default "failed")
  -detail-width int
        Maximum audit detail column width (-1 for no limit) (default 40)
  -details int
        Maximum details for each audit (-1 for all) (default 10)
  -full-urls
        Print full URLs (instead of paths) in report
  -key string
        API key to use (can also set PAGE_SPEED_API_KEY)
  -mail string
        Email address to mail report to (write report to stdout if empty)
  -mobile
        Analyzes the page as a mobile (rather than desktop) device
  -pwa
        Perform Progressive Web App audits (default true)
  -retries int
        Maximum retries after failed calls to API (default 2)
  -verbose
        Log verbosely
  -workers int
        Maximum simultaneous calls to API (default 8)
```

By default, `check-page-speed` prints a summary with each URL's score in the
_Performance_, _Accessibility_, _Best Practices_, _Search Engine Optimization_,
and _Progressive Web App_ Lighthouse categories, followed by lengthier reports
listing the failed (non-100) audits for each URL:

```
$ check-page-speed https://web.dev/ https://web.dev/about/ https://web.dev/blog/
URL      Perf  A11Y  Best  SEO  PWA
/         100    97   100   83   89
/about/   100   100   100   83   89
/blog/     99    91   100   92   89

================================================================================

https://web.dev/

100 Performance
--------------------
 99 Largest Contentful Paint
 89 Reduce unused CSS
    URL                                       Transfer Size  Potential Savings
    https://www.gstatic.com/recaptch…ltr.css  25173          25160
    https://web.dev/css/next.css?v=b56043e5   20381          15157
 74 Reduce unused JavaScript
    URL                                       Transfer Size  Potential Savings
    https://www.gstatic.com/recaptch…__en.js  158237         93490
    https://web.dev/js/index-8b0d1eca.js      89791          74601
...
```

If the `-mail` flag is used to supply an address, `check-page-speed` will
instead email the summary with attached full reports via a local SMTP server at
`localhost:25`.

The PageSpeed Insights API frequently fails when called anonymously, so you
probably want to [create an API key] and pass it via the `-key` flag or the
`PAGE_SPEED_API_KEY` environment variable. As of mid-2022, [the API appears to
still be free].

[create an API key]: https://developers.google.com/speed/docs/insights/v5/get-started#key
[the API appears to still be free]: https://support.google.com/webmasters/thread/57358176/
