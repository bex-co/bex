/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package mailer is the bex-api SMTP sender — the transport behind the members
// feature's invite emails (and any future bex-sent mail). It is deliberately
// provider-agnostic: point it at the same SMTP relay Kratos's courier uses
// (SendGrid in prod, Mailpit locally, docs/ADR012-auth.md §Email) via BEX_SMTP_ADDR, so
// a self-hoster configures one relay, not two. Auth is optional (Mailpit needs
// none); when a username is set, PLAIN auth over the connection is used.
//
// A message may be text-only (the historical shape) or text + HTML: when an
// HTML body is supplied Send emits multipart/alternative (text part first, so a
// client that renders neither still shows the text; HTML preferred where
// supported). With no HTML body the wire output is byte-identical to the
// single-part text/plain message this package always sent (internal/email
// composes both bodies from one source, so they never disagree).
package mailer

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
)

// SMTP sends mail over an SMTP relay. It satisfies the per-feature Mailer
// interfaces (members/notifications/webhooks). Zero value is not usable —
// construct with New.
type SMTP struct {
	addr     string // host:port
	from     string // header From (may carry a display name)
	envelope string // bare address for MAIL FROM
	auth     smtp.Auth
	sendMail func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// New returns an SMTP mailer for the relay at addr (host:port) sending as from.
// from may carry a display name (`bex <no-reply@bex.co>`, the prod
// BEX_SMTP_FROM shape): the full form goes in the message's From: header, but
// the SMTP envelope MAIL FROM takes only the bare address — relays reject the
// display form there with a 501 (found live against Mailpit, w1/040 walk).
// username/password enable PLAIN auth (host derived from addr); leave username
// empty for an unauthenticated relay (Mailpit). Returns nil when addr or from is
// empty — the "no SMTP configured" case, which leaves the members feature
// recording invites without emailing them.
func New(addr, from, username, password string) *SMTP {
	if addr == "" || from == "" {
		return nil
	}
	var auth smtp.Auth
	if username != "" {
		host := addr
		if i := strings.LastIndex(addr, ":"); i >= 0 {
			host = addr[:i]
		}
		auth = smtp.PlainAuth("", username, password, host)
	}
	envelope := from
	if a, err := mail.ParseAddress(from); err == nil {
		envelope = a.Address
	}
	return &SMTP{addr: addr, from: from, envelope: envelope, auth: auth, sendMail: smtp.SendMail}
}

// Send delivers a message with a plain-text body and, when html is non-empty, a
// branded HTML alternative. The context bounds nothing today (net/smtp has no
// context hook) but is kept in the signature so a future context-aware
// transport is a drop-in.
func (s *SMTP) Send(_ context.Context, to, subject, text, html string) error {
	msg := buildMessage(s.from, to, subject, text, html)
	if err := s.sendMail(s.addr, s.auth, s.envelope, []string{to}, msg); err != nil {
		return fmt.Errorf("mailer: send to %s: %w", to, err)
	}
	return nil
}

// buildMessage assembles an RFC 5322 message. With no HTML body it is a single
// text/plain part (byte-identical to this package's original output); with an
// HTML body it is multipart/alternative. Address and subject headers are
// stripped of CR/LF (header-injection defense) and the subject is RFC 2047
// encoded when it carries non-ASCII (ASCII subjects pass through unchanged).
func buildMessage(from, to, subject, text, html string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", headerSafe(from))
	fmt.Fprintf(&b, "To: %s\r\n", headerSafe(to))
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", headerSafe(subject)))
	b.WriteString("MIME-Version: 1.0\r\n")

	if html == "" {
		b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
		b.WriteString("\r\n")
		b.WriteString(crlf(text))
		return []byte(b.String())
	}

	// multipart/alternative — mime/multipart mints a crypto-random boundary, so
	// tenant-controlled part content (workspace names, commit messages, URLs)
	// cannot forge the delimiter. Text part first: least-capable clients render
	// the last part they understand, and every client understands text.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if tp, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type": {`text/plain; charset="utf-8"`},
	}); err == nil {
		tp.Write([]byte(crlf(text)))
	}
	if hp, err := mw.CreatePart(textproto.MIMEHeader{
		"Content-Type":              {`text/html; charset="utf-8"`},
		"Content-Transfer-Encoding": {"quoted-printable"},
	}); err == nil {
		// HTML lines routinely exceed SMTP's 998-byte limit; quoted-printable
		// soft-wraps them while keeping the source human-readable.
		qw := quotedprintable.NewWriter(hp)
		qw.Write([]byte(html))
		qw.Close()
	}
	mw.Close()

	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n", mw.Boundary())
	b.WriteString("\r\n")
	b.Write(body.Bytes())
	return []byte(b.String())
}

// headerSafe removes CR and LF so a value carrying them cannot inject
// additional headers (or a premature body) into the message.
func headerSafe(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

// crlf normalizes body newlines to SMTP's CRLF line endings.
func crlf(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\n", "\r\n")
}
