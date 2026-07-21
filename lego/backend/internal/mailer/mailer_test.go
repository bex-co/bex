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

package mailer

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/smtp"
	"strings"
	"testing"
)

func TestNewReturnsNilWithoutConfig(t *testing.T) {
	if m := New("", "from@bex.co", "", ""); m != nil {
		t.Error("empty addr must yield a nil mailer (no SMTP configured)")
	}
	if m := New("mail:25", "", "", ""); m != nil {
		t.Error("empty from must yield a nil mailer")
	}
}

// capture returns a mailer whose transport records the outbound message instead
// of dialing, plus a pointer to the captured fields.
type captured struct {
	addr string
	from string
	to   []string
	msg  []byte
}

func newCapture(t *testing.T) (*SMTP, *captured) {
	t.Helper()
	m := New("mailpit:1025", "bex <noreply@bex.co>", "", "")
	if m == nil {
		t.Fatal("mailer should be configured")
	}
	var c captured
	m.sendMail = func(addr string, _ smtp.Auth, from string, to []string, msg []byte) error {
		c.addr, c.from, c.to, c.msg = addr, from, to, msg
		return nil
	}
	return m, &c
}

func TestSendBuildsRFC5322AndTargetsRecipient(t *testing.T) {
	m, c := newCapture(t)
	// Empty HTML => the historical single-part text/plain message, byte-identical.
	if err := m.Send(context.Background(), "invitee@example.com", "You're invited", "line one\nline two", ""); err != nil {
		t.Fatalf("send: %v", err)
	}
	// MAIL FROM takes the BARE address — a display-name form there is a
	// syntax error the relay 501s (found live against Mailpit, w1/040);
	// the message's From: header keeps the display form.
	if c.addr != "mailpit:1025" || c.from != "noreply@bex.co" {
		t.Errorf("envelope: addr=%q from=%q, want the bare address", c.addr, c.from)
	}
	if len(c.to) != 1 || c.to[0] != "invitee@example.com" {
		t.Errorf("recipients = %v", c.to)
	}
	msg := string(c.msg)
	for _, want := range []string{
		"From: bex <noreply@bex.co>\r\n",
		"To: invitee@example.com\r\n",
		"Subject: You're invited\r\n",
		"Content-Type: text/plain; charset=\"utf-8\"\r\n",
		"line one\r\nline two", // body newlines normalized to CRLF
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q in:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "multipart") {
		t.Error("empty-HTML send must not be multipart")
	}
}

func TestPlainAuthSetWhenCredentialsGiven(t *testing.T) {
	if m := New("smtp.sendgrid.net:587", "f@bex.co", "apikey", "secret"); m == nil || m.auth == nil {
		t.Error("username+password must configure PLAIN auth")
	}
	if m := New("mailpit:1025", "f@bex.co", "", ""); m == nil || m.auth != nil {
		t.Error("no username must leave auth unset (Mailpit)")
	}
}

func TestMultipartCarriesBothPartsInOrder(t *testing.T) {
	m, c := newCapture(t)
	text := "plain body\nsecond line"
	html := "<html><body><p>hi &amp; welcome</p>\n" + strings.Repeat("x", 1200) + "</body></html>"
	if err := m.Send(context.Background(), "u@example.com", "Subject line", text, html); err != nil {
		t.Fatalf("send: %v", err)
	}

	parts := parseParts(t, c.msg)
	if len(parts) != 2 {
		t.Fatalf("want 2 alternative parts, got %d", len(parts))
	}
	// Text must come first so the least-capable client renders it.
	if ct := parts[0].contentType; !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("first part = %q, want text/plain", ct)
	}
	if ct := parts[1].contentType; !strings.HasPrefix(ct, "text/html") {
		t.Errorf("second part = %q, want text/html", ct)
	}
	if got := strings.ReplaceAll(parts[0].body, "\r\n", "\n"); got != text {
		t.Errorf("text part = %q, want %q", got, text)
	}
	// The HTML part decodes (quoted-printable) back to the input HTML, modulo
	// line-ending representation (MIME transfer encoding may rewrite it).
	if got := norm(parts[1].body); got != norm(html) {
		t.Errorf("html part round-trip mismatch:\n got %q\nwant %q", got, norm(html))
	}
	// The 1200-char line must have been soft-wrapped under the SMTP line limit.
	for _, line := range strings.Split(parts[1].raw, "\r\n") {
		if len(line) > 998 {
			t.Errorf("quoted-printable line exceeds 998 bytes: %d", len(line))
		}
	}
}

func TestBoundaryDiffersAcrossSends(t *testing.T) {
	m, c := newCapture(t)
	boundaryOf := func() string {
		if err := m.Send(context.Background(), "u@example.com", "s", "t", "<p>h</p>"); err != nil {
			t.Fatal(err)
		}
		_, params := mediaType(t, c.msg)
		return params["boundary"]
	}
	if b1, b2 := boundaryOf(), boundaryOf(); b1 == "" || b1 == b2 {
		t.Errorf("boundaries must be random and distinct: %q vs %q", b1, b2)
	}
}

func TestHeaderInjectionNeutralized(t *testing.T) {
	m, c := newCapture(t)
	// A CRLF-bearing recipient/subject must not open a NEW header line — the
	// injected text is flattened into the value it was smuggled through.
	if err := m.Send(context.Background(),
		"victim@example.com\r\nBcc: attacker@evil.com",
		"Hi\r\nX-Injected: yes",
		"body", ""); err != nil {
		t.Fatalf("send: %v", err)
	}
	msg := string(c.msg)
	headerBlock := msg[:strings.Index(msg, "\r\n\r\n")]
	lines := strings.Split(headerBlock, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Bcc:") || strings.HasPrefix(line, "X-Injected:") {
			t.Errorf("header injection not neutralized — smuggled header became a real line:\n%s", headerBlock)
		}
	}
	// Exactly the five intended headers, no more.
	wantHeaders := []string{"From: ", "To: ", "Subject: ", "MIME-Version: ", "Content-Type: "}
	if len(lines) != len(wantHeaders) {
		t.Fatalf("header line count = %d, want %d:\n%s", len(lines), len(wantHeaders), headerBlock)
	}
	for i, h := range wantHeaders {
		if !strings.HasPrefix(lines[i], h) {
			t.Errorf("header line %d = %q, want prefix %q", i, lines[i], h)
		}
	}
}

func TestNonASCIISubjectQEncodedASCIIUntouched(t *testing.T) {
	m, c := newCapture(t)
	if err := m.Send(context.Background(), "u@example.com", "Café ☕ workspace", "b", ""); err != nil {
		t.Fatalf("send: %v", err)
	}
	msg := string(c.msg)
	// Encoded-word form present; raw non-ASCII bytes absent from the header.
	if !strings.Contains(msg, "Subject: =?utf-8?q?") {
		t.Errorf("non-ASCII subject not RFC 2047 encoded:\n%s", msg)
	}
	dec := new(mime.WordDecoder)
	line := headerLine(msg, "Subject: ")
	got, err := dec.DecodeHeader(line)
	if err != nil {
		t.Fatalf("decode subject: %v", err)
	}
	if got != "Café ☕ workspace" {
		t.Errorf("decoded subject = %q", got)
	}
	// An ASCII subject stays literal (pinned by TestSendBuilds... above too).
	m, c = newCapture(t)
	if err := m.Send(context.Background(), "u@example.com", "Plain subject", "b", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(c.msg), "Subject: Plain subject\r\n") {
		t.Error("ASCII subject should not be encoded")
	}
}

// --- helpers -----------------------------------------------------------------

type part struct {
	contentType string
	body        string // decoded (quoted-printable applied)
	raw         string // as-on-the-wire, before transfer decoding
}

func mediaType(t *testing.T, msg []byte) (string, map[string]string) {
	t.Helper()
	rd, err := mail.ReadMessage(strings.NewReader(string(msg)))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	mt, params, err := mime.ParseMediaType(rd.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	return mt, params
}

func parseParts(t *testing.T, msg []byte) []part {
	t.Helper()
	rd, err := mail.ReadMessage(strings.NewReader(string(msg)))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	_, params, err := mime.ParseMediaType(rd.Header.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	mr := multipart.NewReader(rd.Body, params["boundary"])
	var out []part
	for {
		p, err := mr.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("next part: %v", err)
		}
		raw, _ := io.ReadAll(p)
		body := string(raw)
		if p.Header.Get("Content-Transfer-Encoding") == "quoted-printable" {
			dec, _ := io.ReadAll(quotedprintable.NewReader(strings.NewReader(string(raw))))
			body = string(dec)
		}
		out = append(out, part{contentType: p.Header.Get("Content-Type"), body: body, raw: string(raw)})
	}
	return out
}

func headerLine(msg, prefix string) string {
	for _, line := range strings.Split(msg, "\r\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	return ""
}

func norm(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
