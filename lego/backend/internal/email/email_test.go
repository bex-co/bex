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

package email

import (
	"strings"
	"testing"
)

// TestTextReproducesInviteBody pins the plain-text layout against the exact
// bytes the members feature produced before this package existed (linked and
// linkless invite shapes). If Text() drifts, the invite byte-parity test in
// internal/members would fail too — this catches it at the source.
func TestTextReproducesInviteBody(t *testing.T) {
	linked := Message{
		Title:      "Workspace invitation",
		Paragraphs: []string{`You've been invited to join the "Acme" workspace on bex as a developer.`},
		CTA: &CTA{
			Lead:  "Sign up or log in with new@example.com to accept",
			Label: "Accept invitation",
			URL:   "https://dash.example/auth/sign-up?invite=tok123",
		},
		Footer: []string{"This invitation expires on 2026-01-02 15:04 UTC."},
	}
	wantLinked := `You've been invited to join the "Acme" workspace on bex as a developer.

Sign up or log in with new@example.com to accept:
https://dash.example/auth/sign-up?invite=tok123

This invitation expires on 2026-01-02 15:04 UTC.
`
	if got := linked.Text(); got != wantLinked {
		t.Errorf("linked invite text mismatch:\n got %q\nwant %q", got, wantLinked)
	}

	linkless := Message{
		Title: "Workspace invitation",
		Paragraphs: []string{
			`You've been invited to join the "Acme" workspace on bex as a developer.`,
			"Sign up or log in with new@example.com to accept the invitation.",
		},
		Footer: []string{"This invitation expires on 2026-01-02 15:04 UTC."},
	}
	wantLinkless := `You've been invited to join the "Acme" workspace on bex as a developer.

Sign up or log in with new@example.com to accept the invitation.

This invitation expires on 2026-01-02 15:04 UTC.
`
	if got := linkless.Text(); got != wantLinkless {
		t.Errorf("linkless invite text mismatch:\n got %q\nwant %q", got, wantLinkless)
	}
}

// TestTextReproducesDeployBody pins the deploy-notification text layout,
// including the multi-line "Commit:" paragraph and the "View logs" CTA.
func TestTextReproducesDeployBody(t *testing.T) {
	started := Message{
		Title:      "Deploy started: web",
		Paragraphs: []string{`A deploy of "web" has started. We'll email you when it finishes.`},
	}
	if got, want := started.Text(), "A deploy of \"web\" has started. We'll email you when it finishes.\n"; got != want {
		t.Errorf("started deploy text mismatch:\n got %q\nwant %q", got, want)
	}

	failed := Message{
		Title: "Deploy failed: web",
		Paragraphs: []string{
			`We encountered an error during the deploy process for "web". This means your deploy didn't complete successfully and your latest changes may not be live.`,
			"Commit:\nfix the thing",
		},
		CTA: &CTA{Lead: "View logs", Label: "View logs", URL: "https://dash.example/services/web/deploys/dep-1"},
	}
	want := `We encountered an error during the deploy process for "web". This means your deploy didn't complete successfully and your latest changes may not be live.

Commit:
fix the thing

View logs:
https://dash.example/services/web/deploys/dep-1
`
	if got := failed.Text(); got != want {
		t.Errorf("failed deploy text mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestHTMLEscapesUserDataAndInlinesBrand(t *testing.T) {
	m := Message{
		Title:      "Workspace invitation",
		Paragraphs: []string{`You've been invited to <script>alert(1)</script> & friends.`},
		CTA:        &CTA{Lead: "Accept", Label: "Accept invitation", URL: "https://dash.example/x?invite=a&b=c"},
		Footer:     []string{"Expires soon."},
	}
	html := m.HTML()

	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Error("user data not HTML-escaped — raw <script> present in output")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("expected the escaped form of the injected script tag")
	}
	// Brand tokens must be baked to literal values — no unresolved custom
	// properties or oklch() (which no email client understands).
	for _, forbidden := range []string{"oklch(", "var(--", "@primary@", "@font@"} {
		if strings.Contains(html, forbidden) {
			t.Errorf("HTML still contains unresolved token %q", forbidden)
		}
	}
	if !strings.Contains(html, BrandPrimary) {
		t.Errorf("brand primary %q not inlined", BrandPrimary)
	}
	// CTA renders as a button (label) AND a visible raw-link fallback; the URL
	// appears as an href and as link text. html/template encodes & as &amp; in
	// both attribute and text contexts.
	if !strings.Contains(html, "Accept invitation") {
		t.Error("CTA button label missing")
	}
	if strings.Count(html, "https://dash.example/x?invite=a&amp;b=c") < 2 {
		t.Errorf("CTA URL should appear as both button href and fallback link:\n%s", html)
	}
}

func TestHTMLJoinsMultilineParagraphsWithBreaks(t *testing.T) {
	m := Message{Paragraphs: []string{"Commit:\nfix the thing"}}
	html := m.HTML()
	if !strings.Contains(html, "Commit:<br />fix the thing") {
		t.Errorf("multi-line paragraph should join with <br />:\n%s", html)
	}
}

func TestReferenceRendersAsOneLinkedBlock(t *testing.T) {
	m := Message{
		Paragraphs: []string{"Deploy failed."},
		Reference:  &Reference{Label: "Commit", Token: "abc1234", URL: "https://github.com/acme/web/commit/abc1234def", Desc: "fix(api): correct the path"},
		CTA:        &CTA{Lead: "View logs", Label: "View logs", URL: "https://dash/logs"},
	}
	// Text: the reference is one block (label+token, desc, url), placed before
	// the CTA — the link lives with the commit, not as a trailing line.
	want := "Deploy failed.\n\nCommit abc1234\nfix(api): correct the path\nhttps://github.com/acme/web/commit/abc1234def\n\nView logs:\nhttps://dash/logs\n"
	if got := m.Text(); got != want {
		t.Errorf("text with reference:\n got %q\nwant %q", got, want)
	}
	// HTML: the SHA token is the link, the description shows below it.
	html := m.HTML()
	if !strings.Contains(html, `href="https://github.com/acme/web/commit/abc1234def"`) || !strings.Contains(html, ">abc1234</a>") {
		t.Errorf("commit SHA not linked in HTML:\n%s", html)
	}
	if !strings.Contains(html, "fix(api): correct the path") {
		t.Error("commit description missing from HTML")
	}
}

func TestReferenceWithoutURLRendersPlainToken(t *testing.T) {
	m := Message{Reference: &Reference{Label: "Commit", Token: "abc1234", Desc: "no link available"}}
	if got, want := m.Text(), "Commit abc1234\nno link available\n"; got != want {
		t.Errorf("plain reference text = %q, want %q", got, want)
	}
	if html := m.HTML(); strings.Contains(html, "<a href") {
		t.Errorf("reference with no URL must not render a link:\n%s", html)
	}
}

// TestCodeRendersAsCopyablePanel pins the one-time-code shape: text keeps the
// "Label: Value" line the committed Kratos plainBody carries, and HTML shows
// the code as one unbroken monospace text node in its own panel — the thing
// a reader has to spot and copy must not be a run of 14px prose.
func TestCodeRendersAsCopyablePanel(t *testing.T) {
	m := Message{
		Title:      "Verify your email",
		Paragraphs: []string{"Enter this code on the verification page:"},
		Code:       &Code{Label: "Verification code", Value: "812410", Desc: "It expires in 60 minutes."},
		CTA:        &CTA{Lead: "Or verify with one click", Label: "Verify email", URL: "https://auth.example/verify?code=812410"},
	}
	want := "Enter this code on the verification page:\n\nVerification code: 812410\nIt expires in 60 minutes.\n\nOr verify with one click:\nhttps://auth.example/verify?code=812410\n"
	if got := m.Text(); got != want {
		t.Errorf("code text:\n got %q\nwant %q", got, want)
	}
	html := m.HTML()
	// The value is a single text node inside the monospace span: nothing may
	// be interleaved between the digits (spans, zero-width characters), or a
	// copy would pick up more than the code.
	if !strings.Contains(html, ">812410</span>") {
		t.Errorf("code should be one unbroken text node ending the span:\n%s", html)
	}
	if !strings.Contains(html, "font-family:"+codeFontFamily+";") {
		t.Errorf("code should render in the monospace stack %q:\n%s", codeFontFamily, html)
	}
	if !strings.Contains(html, "user-select:all") {
		t.Error("code span should select as a unit on click")
	}
	// The panel sits on the muted background so it reads as a distinct block.
	if !strings.Contains(html, "background-color:"+BrandMuted+";") {
		t.Errorf("code panel should use the muted background %q", BrandMuted)
	}
	for _, s := range []string{"Verification code", "It expires in 60 minutes."} {
		if !strings.Contains(html, s) {
			t.Errorf("HTML missing %q", s)
		}
	}
	// The code is never a link — the CTA carries the URL.
	if strings.Contains(html, `href="812410"`) || strings.Contains(html, ">812410</a>") {
		t.Error("code must not render as a link")
	}
}

func TestHTMLWordmarkSplitsColorAndResistsAutolink(t *testing.T) {
	html := Message{Paragraphs: []string{"hi"}}.HTML()
	// Only ".co" carries the brand color; "bex" stays neutral (foreground).
	if !strings.Contains(html, `>bex<span style="color:`+BrandPrimary+`;">`) {
		t.Errorf("expected neutral 'bex' + a brand-colored '.co' span:\n%s", html)
	}
	// A zero-width space breaks the "bex.co" domain token so mail clients don't
	// auto-linkify the wordmark; the contiguous plain form must not appear.
	if !strings.Contains(html, "&#8203;.co</span>") {
		t.Errorf("expected a zero-width space before .co to defeat auto-linking:\n%s", html)
	}
	if strings.Contains(html, ">bex.co<") {
		t.Error("wordmark renders as contiguous 'bex.co' — clients will auto-link it")
	}
}

func TestHTMLRendersWithoutCTAOrFooter(t *testing.T) {
	m := Message{Title: "Deploy started: web", Paragraphs: []string{"A deploy has started."}}
	html := m.HTML()
	if !strings.Contains(html, "A deploy has started.") {
		t.Error("paragraph missing from minimal message")
	}
	// No CTA => no button label text, no "Or open this link" fallback.
	if strings.Contains(html, "Or open this link") {
		t.Error("CTA fallback rendered for a message with no CTA")
	}
	if !strings.HasPrefix(strings.TrimSpace(html), "<!doctype html>") {
		t.Error("expected a full HTML document")
	}
}
