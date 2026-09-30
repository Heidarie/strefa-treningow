package app

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

func sendSMTP(c *smtp.Client, host string, auth smtp.Auth, from string, p map[string]string) error {
	if ok, _ := c.Extension("STARTTLS"); ok {
		if e := c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); e != nil {
			return e
		}
	} else if env("SMTP_REQUIRE_TLS", "false") == "true" {
		return fmt.Errorf("SMTP TLS required")
	}
	if auth != nil {
		if e := c.Auth(auth); e != nil {
			return e
		}
	}
	for _, v := range []string{from, p["to"], p["subject"]} {
		if strings.ContainsAny(v, "\r\n") {
			return errInvalid
		}
	}
	if e := c.Mail(from); e != nil {
		return e
	}
	if e := c.Rcpt(p["to"]); e != nil {
		return e
	}
	w, e := c.Data()
	if e != nil {
		return e
	}
	_, e = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", from, p["to"], p["subject"], p["body"])
	if e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	return c.Quit()
}
