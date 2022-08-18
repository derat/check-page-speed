// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	htemplate "html/template"
	"io"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
	ttemplate "text/template"
	"time"

	"gopkg.in/gomail.v2"
)

const (
	// SMTP connection info.
	mailHost = "localhost"
	mailPort = 25
)

// sendMail sends email to cfg.mailAddr with a summary of the supplied reports
// in the message body and a text attachment with the full reports.
func sendMail(reports []*report, cfg *reportConfig) error {
	text, html, err := generateBody(reports, cfg)
	if err != nil {
		return err
	}
	from, err := getMailFrom()
	if err != nil {
		return fmt.Errorf("couldn't get from address (consider setting $EMAIL): %v", err)
	}

	// Try to construct a subject like "example.com mobile page speed for Dec 7".
	subject := "Page speed report"
	if u, err := url.Parse(reports[0].URL); err == nil {
		subject = strings.TrimPrefix(u.Hostname(), "www.")
		if cfg.mobile {
			subject += " mobile"
		} else {
			subject += " desktop"
		}
		subject += " page speed"
	}
	subject += " for " + cfg.startTime.Format("Jan 2")

	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", cfg.mailAddr)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/plain", text)
	msg.AddAlternative("text/html", html)
	msg.Attach(fmt.Sprintf("page-speed-%s.txt", cfg.startTime.Format("20060102-030405")),
		gomail.SetCopyFunc(func(w io.Writer) error { return writeReports(w, reports, cfg) }),
		gomail.SetHeader(map[string][]string{"Content-Type": []string{"text/plain"}}),
	)

	// Make it easier to test generated messages during development.
	if cfg.mailAddr == "-" {
		_, err = msg.WriteTo(os.Stdout)
		return err
	}

	dialer := gomail.Dialer{Host: mailHost, Port: mailPort}
	if dialer.Host == "localhost" {
		// Try to work around "x509: certificate is not valid for any names, but wanted to match
		// localhost" errors, since we're just connecting to localhost anyway:
		// https://github.com/go-gomail/gomail#x509-certificate-signed-by-unknown-authority
		dialer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return dialer.DialAndSend(msg)
}

// getMailFrom tries to find an email address to use in the "From" header.
func getMailFrom() (string, error) {
	for _, name := range []string{
		"MAILFROM", // can be set in crontab
		"MAILTO",   // can be set in crontab
		"EMAIL",    // used by e.g. mutt
	} {
		if v := os.Getenv(name); v != "" {
			return v, nil
		}
	}

	// gomail complains if we leave the address blank or supply only a username,
	// so fall back to constructing an address with the username and hostname.
	user, err := user.Current()
	if err != nil {
		return "", err
	}
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v@%v", user.Username, host), nil
}

// generateBody generates text and HTML email message bodies.
func generateBody(reports []*report, cfg *reportConfig) (text, html string, err error) {
	// "Mon, 02 Jan 2006 15:04:05 -0700"
	startTime := cfg.startTime.Format(time.RFC1123Z)

	// Generate the text version.
	var sum bytes.Buffer
	if err := writeSummary(&sum, reports, cfg); err != nil {
		return "", "", err
	}
	tdata := &struct{ Summary, Time string }{strings.TrimSpace(sum.String()), startTime}
	if text, err = runTemplate(ttemplate.New(""), textTemplate, tdata); err != nil {
		return "", "", err
	}

	// Generate the HTML version.
	type column struct{ Text, Title, Href string }
	hdata := struct {
		Rows [][]column
		Time string
	}{
		Rows: [][]column{{{Text: "URL", Title: "URL"}}}, // first row is header
		Time: startTime,
	}
	for _, rep := range reports {
		// Add the categories from the first non-failed report to the heading row.
		if len(hdata.Rows[0]) == 1 && len(rep.Categories) > 0 {
			for _, cat := range rep.Categories {
				hdata.Rows[0] = append(hdata.Rows[0], column{
					Text:  cat.Abbrev,
					Title: cat.Title,
				})
			}
		}
		row := []column{column{Text: rep.URL, Href: rep.URL}}
		if !cfg.fullURLs {
			row[0].Text = urlPath(rep.URL)
		}
		for _, cat := range rep.Categories {
			row = append(row, column{Text: strconv.Itoa(cat.Score)})
		}
		hdata.Rows = append(hdata.Rows, row)
	}
	if html, err = runTemplate(htemplate.New(""), htmlTemplate, &hdata); err != nil {
		return "", "", err
	}

	return text, html, nil
}

// runTemplate makes tmpl parse text and then executes it using data.
func runTemplate(tmpl interface{}, text string, data interface{}) (string, error) {
	text = strings.TrimLeft(text, "\n")

	// Not sure if there's a less-awkward way to do this. Parse() returns
	// the typed template, so I don't think there's any way to put it in an
	// interface. I guess that generics are the way to go here...
	var b bytes.Buffer
	var err error
	switch t := tmpl.(type) {
	case *htemplate.Template:
		if _, err = t.Parse(text); err == nil {
			err = t.Execute(&b, data)
		}
	case *ttemplate.Template:
		if _, err = t.Parse(text); err == nil {
			err = t.Execute(&b, data)
		}
	default:
		return "", errors.New("didn't get template pointer")
	}
	return b.String(), err
}

const textTemplate = `
{{.Summary}}

Generated by https://github.com/derat/check-page-speed at
{{.Time}}.
`

const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1, minimum-scale=1">
    <title>check-page-speed</title>
  </head>
  <body>
    <table>
      {{- range $i, $row := .Rows}}
      <tr>
        {{- range $j, $col := $row}}
        {{if eq $i 0}}<th{{else}}<td{{end}}
            {{- if eq $j 0}} align="left"
            {{- else}} align="right" style="padding-left: 8px"
            {{- end}}{{if $col.Title}} title="{{$col.Title}}"{{end}}>
          {{- if $col.Href}}<a href="{{$col.Href}}">{{end}}{{$col.Text}}{{if $col.Href}}</a>{{end -}}
        {{if eq $i 0}}</th>{{else}}</td>{{end}}
        {{- end}}
      </tr>
      {{- end}}
    </table>
    <p>Generated by <a href="https://github.com/derat/check-page-speed">check-page-speed</a> at {{.Time}}.</p>
  </body>
</html>
`
