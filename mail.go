// Copyright 2022 Daniel Erat.
// All rights reserved.

package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"gopkg.in/gomail.v2"
)

const (
	// SMTP connection info.
	mailHost = "localhost"
	mailPort = 25
)

func sendMail(reports []*report, cfg *reportConfig) error {
	text, html, err := generateBody(reports, cfg)
	if err != nil {
		return err
	}
	from, err := getMailFrom()
	if err != nil {
		return fmt.Errorf("couldn't get from address (consider setting $EMAIL): %v", err)
	}

	subject := "Page speed report"
	if u, err := url.Parse(reports[0].URL); err == nil {
		subject = u.Hostname() + " page speed report"
	}
	subject += " for " + cfg.startTime.Format(time.RFC1123Z)

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
	// Generate the text version.
	var tb bytes.Buffer
	if err := writeSummary(&tb, reports, cfg); err != nil {
		return "", "", err
	}
	fmt.Fprint(&tb, "\nSent by https://github.com/derat/check-page-speed.\n")

	// Generate the HTML version.
	type column struct{ Text, Title, Href string }
	data := struct{ Rows [][]column }{
		Rows: [][]column{{{Text: "URL", Title: "URL"}}}, // first row is header
	}
	for _, rep := range reports {
		// Add the categories from the first non-failed report to the heading row.
		if len(data.Rows[0]) == 1 && len(rep.Categories) > 0 {
			for _, cat := range rep.Categories {
				data.Rows[0] = append(data.Rows[0], column{
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
		data.Rows = append(data.Rows, row)
	}

	var hb bytes.Buffer
	tmpl := template.Must(template.New("").Parse(strings.TrimLeft(htmlMailTemplate, "\n")))
	if err := tmpl.Execute(&hb, &data); err != nil {
		return "", "", err
	}
	return tb.String(), hb.String(), nil
}

const htmlMailTemplate = `
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
    <p>Sent by <a href="https://github.com/derat/check-page-speed">check-page-speed</a>.</p>
  </body>
</html>
`
