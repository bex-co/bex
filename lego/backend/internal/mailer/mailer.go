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
// (SendGrid in prod, Mailpit locally, docs/auth.md §Email) via BEX_SMTP_ADDR, so
// a self-hoster configures one relay, not two. Auth is optional (Mailpit needs
// none); when a username is set, PLAIN auth over the connection is used.
package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTP sends plain-text mail over an SMTP relay. It satisfies members.Mailer.
// Zero value is not usable — construct with New.
type SMTP struct {
	addr     string // host:port
	from     string // envelope + header From
	auth     smtp.Auth
	sendMail func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

// New returns an SMTP mailer for the relay at addr (host:port) sending as from.
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
	return &SMTP{addr: addr, from: from, auth: auth, sendMail: smtp.SendMail}
}

// Send delivers a plain-text message. The context bounds nothing today
// (net/smtp has no context hook) but is kept in the signature so a future
// context-aware transport is a drop-in.
func (s *SMTP) Send(_ context.Context, to, subject, body string) error {
	msg := buildMessage(s.from, to, subject, body)
	if err := s.sendMail(s.addr, s.auth, s.from, []string{to}, msg); err != nil {
		return fmt.Errorf("mailer: send to %s: %w", to, err)
	}
	return nil
}

// buildMessage assembles an RFC 5322 message with the minimal headers a relay
// needs. Body newlines are normalized to CRLF (SMTP line endings); the subject
// is kept single-line (callers pass short subjects).
func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", strings.ReplaceAll(subject, "\n", " "))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return []byte(b.String())
}
