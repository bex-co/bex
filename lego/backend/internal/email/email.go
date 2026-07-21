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

// Package email composes the bex-sent transactional emails (workspace invites,
// deploy notifications, webhook failure notices) into BOTH a plain-text body
// and a branded HTML body from one Message, so the two renderings can never
// drift. The HTML layout is a table-based, fully-inlined design (the shape
// email clients tolerate) whose palette is generated from the dashboard's own
// style.css — see brand_gen.go and dashboard/scripts/generate-email-brand.mjs.
//
// The text body is the source of compatibility: it is byte-identical to the
// bodies bex sent before this package existed, and it is the part every client
// (and the mailer's degraded single-part path) falls back to. HTML is additive.
package email

import (
	"bytes"
	_ "embed"
	"html/template"
	"strconv"
	"strings"
)

//go:embed layout.html.tmpl
var layoutTemplate string

// layout is the parsed HTML layout. The brand tokens are substituted into the
// template TEXT before parsing (a strings.Replacer over the raw source), never
// as template actions: they land inside style="" attributes, where
// html/template's CSS-value filter would otherwise mangle a bare hex/px/font
// value. Only user data flows through the template's own contextual escaping.
var layout = template.Must(template.New("email").Parse(
	strings.NewReplacer(
		"@background@", BrandBackground,
		"@card@", BrandCard,
		"@foreground@", BrandForeground,
		"@mutedfg@", BrandMutedForeground,
		"@primary@", BrandPrimary,
		"@primaryfg@", BrandPrimaryForeground,
		"@border@", BrandBorder,
		"@radius@", strconv.Itoa(BrandRadiusPx),
		"@font@", BrandFontFamily,
	).Replace(layoutTemplate),
))

// CTA is a call-to-action: a lead-in line, a button label, and the URL both
// point at. In text it renders as "Lead:\nURL"; in HTML as a branded button
// plus a visible raw-link fallback (many clients strip or distrust buttons).
type CTA struct {
	Lead  string
	Label string
	URL   string
}

// Reference is a labeled reference whose token links out, followed by an
// optional (possibly multi-line) description — e.g. a deploy's commit: Label
// "Commit", Token "abc1234" linking to the commit page, Desc the commit
// message. With no URL the token renders as plain text (an image-backed deploy
// shows the message without a link). It is rendered as a single block, so the
// link lives with the thing it describes rather than as a separate line.
type Reference struct {
	Label string
	Token string
	URL   string
	Desc  string
}

// Message is one composed email, independent of how it is rendered. A feature
// fills it and hands Text()+HTML() to the mailer. Paragraphs and Footer lines
// may contain "\n"; in HTML those become <br/>, in text they are preserved.
type Message struct {
	// Title is an HTML-only heading (and the document <title>). The text body
	// has no heading — it opens on the first paragraph, as bex's mails always
	// have — so Title must not carry information the paragraphs don't.
	Title      string
	Paragraphs []string
	// Reference is an optional labeled/linked reference shown after the
	// paragraphs and before the CTA (e.g. the deploy commit).
	Reference *Reference
	CTA       *CTA
	Footer    []string
}

// Text renders the plain-text body: paragraphs, then the CTA as "Lead:\nURL",
// then the footer as one block — sections joined by a blank line, with a single
// trailing newline. This layout reproduces bex's historical mail bodies exactly
// (the members/notifications byte-parity tests pin it).
func (m Message) Text() string {
	sections := make([]string, 0, len(m.Paragraphs)+3)
	sections = append(sections, m.Paragraphs...)
	if r := m.Reference; r != nil {
		head := r.Label
		if r.Token != "" {
			head += " " + r.Token
		}
		parts := []string{head}
		if r.Desc != "" {
			parts = append(parts, r.Desc)
		}
		if r.URL != "" {
			parts = append(parts, r.URL)
		}
		sections = append(sections, strings.Join(parts, "\n"))
	}
	if m.CTA != nil {
		sections = append(sections, m.CTA.Lead+":\n"+m.CTA.URL)
	}
	if len(m.Footer) > 0 {
		sections = append(sections, strings.Join(m.Footer, "\n"))
	}
	return strings.Join(sections, "\n\n") + "\n"
}

// HTML renders the branded HTML body. A template-execution error (practically
// unreachable with this data shape) yields "" — the mailer then sends the text
// part alone rather than a broken message.
func (m Message) HTML() string {
	var b bytes.Buffer
	if err := layout.Execute(&b, templateData{
		Title:      m.Title,
		Paragraphs: splitLines(m.Paragraphs),
		Reference:  refData(m.Reference),
		CTA:        m.CTA,
		Footer:     splitLines(m.Footer),
	}); err != nil {
		return ""
	}
	return b.String()
}

// templateData is the layout's view of a Message: each paragraph/footer entry
// pre-split into lines so the template can join them with <br/>.
type templateData struct {
	Title      string
	Paragraphs [][]string
	Reference  *refView
	CTA        *CTA
	Footer     [][]string
}

// refView is the template's view of a Reference — Desc pre-split into lines.
type refView struct {
	Label string
	Token string
	URL   string
	Desc  []string
}

func refData(r *Reference) *refView {
	if r == nil {
		return nil
	}
	v := &refView{Label: r.Label, Token: r.Token, URL: r.URL}
	if r.Desc != "" {
		v.Desc = strings.Split(r.Desc, "\n")
	}
	return v
}

func splitLines(blocks []string) [][]string {
	if len(blocks) == 0 {
		return nil
	}
	out := make([][]string, len(blocks))
	for i, s := range blocks {
		out[i] = strings.Split(s, "\n")
	}
	return out
}
