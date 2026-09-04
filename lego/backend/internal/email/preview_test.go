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
	"os"
	"path/filepath"
	"testing"
)

// TestPreview is a developer tool, not an assertion: with EMAIL_PREVIEW_DIR set
// it renders the bex-sent email shapes through the real layout and writes
// them as .html files to inspect in a browser; unset (CI, normal `go test`) it
// skips and writes nothing. It previews the layout, not the exact per-feature
// wording — that is byte-pinned by the members/notifications/webhooks tests.
//
//	EMAIL_PREVIEW_DIR=/tmp/bex-email go test ./internal/email -run Preview -count=1
//	open /tmp/bex-email/invite.html          # or: python3 -m http.server -d /tmp/bex-email
func TestPreview(t *testing.T) {
	dir := os.Getenv("EMAIL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set EMAIL_PREVIEW_DIR to render preview HTML files")
	}
	shapes := map[string]Message{
		"invite": {
			Title:      "Workspace invitation",
			Paragraphs: []string{`You've been invited to join the "Acme Corp" workspace on bex as a developer.`},
			CTA: &CTA{
				Lead:  "Sign up or log in with jordan@example.com to accept",
				Label: "Accept invitation",
				URL:   "https://dashboard.bex.co/auth/sign-up?invite=a1b2c3d4e5f6a1b2c3d4e5f6",
			},
			Footer: []string{"This invitation expires on 2026-01-27 15:04 UTC."},
		},
		"deploy": {
			Title: "Deploy failed: web",
			Paragraphs: []string{
				`We encountered an error during the deploy process for "web". This means your deploy didn't complete successfully and your latest changes may not be live.`,
			},
			Reference: &Reference{
				Label: "Commit",
				Token: "abc1234",
				URL:   "https://github.com/acme/web/commit/abc1234def5678",
				Desc:  "fix(api): correct the health-check path so readiness probes pass",
			},
			CTA: &CTA{Lead: "View logs", Label: "View logs", URL: "https://dashboard.bex.co/services/web/deploys/dep-abc123"},
		},
		"webhook": {
			Title: "Webhook delivery failing",
			Paragraphs: []string{
				`Deliveries to your webhook "prod-events" (https://hooks.example.com/bex) have failed 3 times in a row.`,
				"Last error: 502 Bad Gateway",
				"bex will keep retrying on an exponential backoff.",
			},
		},
		// The Kratos verification courier shape (kratos_templates.go) with a
		// concrete code in place of the sentinel, so the Code panel can be
		// eyeballed next to the other shapes.
		"verify": {
			Title:      "Verify your email",
			Paragraphs: []string{"Enter this code on the verification page to confirm your email address:"},
			Code:       &Code{Label: "Verification code", Value: "812410", Desc: "It expires in 60 minutes."},
			CTA: &CTA{
				Lead:  "Or verify with one click",
				Label: "Verify email",
				URL:   "https://auth.bex.co/self-service/verification?code=812410&flow=bdab73b4-be81-42bc-ba87-54bff484f317",
			},
			Footer: []string{"If this wasn't you, you can safely ignore this email —\nnothing changes without the code."},
		},
	}
	for name, msg := range shapes {
		path := filepath.Join(dir, name+".html")
		if err := os.WriteFile(path, []byte(msg.HTML()), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote %s", path)
	}
	// A gallery page that iframes all four so they can be viewed side by side.
	gallery := filepath.Join(dir, "all-email-templates.html")
	if err := os.WriteFile(gallery, []byte(galleryHTML), 0o644); err != nil {
		t.Fatalf("write %s: %v", gallery, err)
	}
	t.Logf("wrote %s (open this one)", gallery)
}

// galleryHTML is a static index that embeds invite/deploy/webhook/verify.html
// (written alongside it) in iframes — dark chrome so the light email cards
// stand out.
const galleryHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>bex email templates</title>
    <style>
      body { margin:0; font-family:system-ui,sans-serif; background:#0f1115; color:#e8eaed; }
      header { padding:20px 24px; border-bottom:1px solid #2a2d33; }
      h1 { margin:0; font-size:18px; font-weight:600; }
      p.sub { margin:4px 0 0; font-size:13px; color:#9aa0a6; }
      .grid { display:flex; flex-wrap:wrap; gap:24px; padding:24px; }
      .panel { flex:1 1 520px; min-width:380px; }
      .label { font-size:13px; font-weight:600; margin:0 0 8px; color:#c8cdd3; }
      .label span { color:#9aa0a6; font-weight:400; }
      iframe { width:100%; height:560px; border:1px solid #2a2d33; border-radius:8px; background:#f8f8fa; }
    </style>
  </head>
  <body>
    <header>
      <h1>bex email templates</h1>
      <p class="sub">Rendered from internal/email — invite · deploy · webhook · verify</p>
    </header>
    <div class="grid">
      <div class="panel"><p class="label">Workspace invite <span>· members</span></p><iframe src="invite.html" title="invite"></iframe></div>
      <div class="panel"><p class="label">Deploy failed <span>· notifications</span></p><iframe src="deploy.html" title="deploy"></iframe></div>
      <div class="panel"><p class="label">Webhook failing <span>· webhooks</span></p><iframe src="webhook.html" title="webhook"></iframe></div>
      <div class="panel"><p class="label">Verify your email <span>· kratos courier</span></p><iframe src="verify.html" title="verify"></iframe></div>
    </div>
  </body>
</html>
`
