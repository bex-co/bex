# w1 · m54 — Branded email module: dashboard-token codegen + HTML/text layout for all bex-sent mail

**Worker:** worker1 **Goal:** every email bex-api sends (workspace invite, deploy started/succeeded/failed, webhook failing/disabled) goes out as multipart/alternative — the existing plain-text body unchanged as the fallback part, plus a branded HTML part whose palette is generated from the dashboard's own `style.css` tokens, with drift CI-enforced. **Status:** done (2026-07-20)

## Tasks (in order)

| id   | title                                                                                     | est | depends_on       |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Brand-token codegen: dashboard `style.css` → `internal/email/brand_gen.go` + CI sync test | 30m | — — **DONE**     |
| t002 | `internal/email` Message module: one struct → text + branded HTML                         | 45m | t001 — **DONE**  |
| t003 | Multipart mailer: `Send` gains an HTML part (QP-encoded) + header hardening                | 45m | — — **DONE**     |
| t004 | Rewire members invite email through `email.Message` (text byte-parity)                    | 30m | t002, t003 — **DONE** |
| t005 | Rewire notifications deploy emails through `email.Message` (text byte-parity)              | 30m | t002, t003 — **DONE** |
| t006 | Rewire webhooks failure/disable notices through `email.Message`                            | 20m | t002, t003 — **DONE** |
| t007 | Visual verification: all three mail shapes rendered as branded HTML                        | 20m | t004, t005, t006 — **DONE** |
| t008 | Render parity — compare bex's transactional emails against Render's equivalents            | 30m | t007 — **DONE**  |
| t009 | Simplify — `/simplify` over the code this milestone changed                                | 30m | t008 — **DONE**  |
| t010 | Test coverage — meaningful tests for layout, multipart, escaping, text parity              | 45m | t008 — **DONE**  |
| t011 | Closeout                                                                                   | 15m | t010 — **DONE**  |

## Definition of done

- `dashboard/scripts/generate-email-brand.mjs` generates `lego/backend/internal/email/brand_gen.go` (oklch → sRGB hex, radius rem → px) from `dashboard/src/style.css`; a vitest test in the dashboard suite (CI-enforced by `dashboard-test.yml`) fails when the committed Go file drifts from `style.css`; `yarn generate:email-brand` regenerates it.
- `internal/email.Message` renders one composed message to both a plain-text body and a branded HTML body (embedded layout, inlined brand tokens, HTML-escaped user data, CTA button + raw-link fallback).
- `mailer.SMTP.Send` sends multipart/alternative (text + quoted-printable HTML) when an HTML body is supplied, byte-identical single-part behavior when it isn't; header CRLF injection is neutralized and non-ASCII subjects are RFC 2047 encoded.
- Members invites, notifications deploy emails, and webhooks failure notices all send through the module; the plain-text part of the invite and deploy emails is byte-identical to today's bodies (pinned by tests).
- `cd lego/backend && go build ./... && go test ./...` green; dashboard `yarn test` green; `make lint-backend` clean.
- All three mail shapes visually verified as branded HTML.

## Outcome (2026-07-20)

Shipped, uncommitted in the working tree. All three senders now emit
multipart/alternative with a dashboard-branded HTML part; the plain-text part is
byte-identical to the pre-milestone bodies (invite + deploy pinned by tests).

- **Codegen (t001):** `dashboard/scripts/generate-email-brand.mjs` converts the light-theme `:root` tokens in `dashboard/src/style.css` (oklch → sRGB hex via Ottosson's transform, `--radius` rem → px) into the committed `lego/backend/internal/email/brand_gen.go` (`BrandPrimary = #307d03`, the brand green). `dashboard/scripts/__tests__/generate-email-brand.test.mjs` is the drift gate — it runs in the dashboard vitest suite (1621 tests green), so a `style.css` palette change without a regen fails CI. `yarn generate:email-brand` regenerates; output is gofmt-clean as emitted (dashboard CI has no Go toolchain).
- **Module (t002):** `internal/email.Message` → `Text()` + `HTML()` from one source. Layout is `layout.html.tmpl` (table-based, fully-inlined styles, text "bex" wordmark so nothing to block); brand tokens are substituted into the template text pre-parse so `html/template`'s CSS-value filter never touches them, and only user data flows through contextual escaping.
- **Mailer (t003):** `Send(ctx, to, subject, text, html)`; empty `html` ⇒ byte-identical single-part text/plain (pinned). Multipart uses `mime/multipart`'s crypto-random boundary (tenant content can't forge the delimiter) with a quoted-printable HTML part (soft-wraps past the 998-byte SMTP line limit). Header hardening: CR/LF stripped from From/To/Subject (injection defense), subject RFC 2047 Q-encoded when non-ASCII (ASCII untouched).
- **Rewires (t004–t006):** members invite, notifications deploy (started/succeeded/failed, compose-once-before-fan-out preserved), webhooks failing/disabled. Each feature keeps its own narrow `Mailer` interface (the backend convention that keeps features independent); `mailer.SMTP` satisfies all three structurally, so the composition root is untouched.

## Parity findings (t008)

Scoped to the emails themselves (no REST/GraphQL/MCP/UI wire change this milestone). Compared bex's three shapes against Render's known transactional-email behavior:

- **Invite** — parity. Render: branded HTML "You've been invited to join <org>" + accept button + expiry. bex: "You've been invited to the "X" workspace on bex" subject, branded card, "Accept invitation" button, expiry footer. bex says _workspace_ where Render says _team/org_ — intentional bex vocabulary (ADR024), not drift.
- **Deploy** — parity. Render sends branded deploy-failed/succeeded mail linking to the deploy; bex matches with "Deploy failed: web" register + "View logs" button. Wording already tuned to Render's register in w7/m44 and byte-pinned here.
- **Webhook failing/disabled** — acceptable divergence. Render notifies on failing endpoints; bex's copy differs but the semantics (failure count, last error, auto-disable, re-enable guidance) match. No CTA button because the webhook worker has no dashboard base URL — a "re-enable" link is filed as out-of-scope in t006 (would need dashboard-URL plumbing into the worker).

No actionable drift beyond that already-noted out-of-scope follow-up. A live Render side-by-side capture (no test Render org in this session) would strengthen the comparison but isn't blocking.

## Simplify (t009)

Self-reviewed the changed code. No behavior-preserving simplification applied — deliberately kept the three per-feature `Mailer` interfaces separate (consolidating them would couple the features, against the backend convention) and the `templateData`/`Message` split (splitting lines in Go keeps the template trivial; a template FuncMap would be more complex, not less). `crlf` was hardened to normalize `\r\n`→`\n`→`\r\n` (avoids doubling) — a correctness fix, not added complexity.

## Verification (t007, t010)

- **Visual:** rendered the real `email.HTML()` output for all three shapes (a throwaway `cmd/emailpreview`, since removed) and screenshotted each in a browser at a 640px email-column width — the same bytes Mailpit would display, isolating the email rendering from SMTP transport. Screenshots: `.playwright-mcp/email-{invite,deploy,webhook}.png`. Confirmed brand-green wordmark/buttons (`#307d03`), quoted workspace name rendered escaped (not injected), multiline "Commit:" breaking correctly, long link fallback wrapping, and the webhook notice correctly rendering no CTA.
- **Tests:** codegen drift + oklch conversion (dashboard); `email` text-parity + escaping + no-`oklch(`/`var(` + multiline `<br/>` + no-CTA shapes; `mailer` single-part byte-identity + multipart round-trip + random boundary + header-injection + Q-encoding; per-feature byte-parity (invite linked/linkless, all deploy kinds) and webhook content. Backend `go test ./...` + `make lint-backend` (0 issues) green.

## Source + Goal linkage

- **Source:** user request 2026-07-20 (this session): "should we add a dedicated email template module… with dashboard-consistent styles?" → "can we share certain code from dashboard" → "you should do the right thing at once". Implementation was started in the same session and sits **uncommitted in the working tree**: `dashboard/scripts/generate-email-brand.mjs`, generated `lego/backend/internal/email/brand_gen.go`, and `lego/backend/internal/email/layout.html.tmpl` exist; `email.go`, the mailer change, and the three feature rewires do not yet.
- **Goal linkage:** Render-competitive product polish on a tenant-facing surface — Render sends branded HTML transactional email while bex sends bare text (invites: `docs/ADR024-members.md`; deploy notifications: w7/m44, written to match Render's register; webhook notices: w3/m11). Single-source brand tokens extend the palette-sync convention `style.css` already declares with eden-cms-v2, eliminating a third hand-synced copy.
- **Expected outcome:** every bex-sent email renders with the dashboard's palette in HTML-capable clients and degrades to today's exact text elsewhere; a dashboard palette change propagates to email by running one yarn script, and forgetting to is a CI failure, not silent drift.
- **Why now:** the three senders already share one `Send(ctx, to, subject, body)` seam, so the change is cheap today and grows costlier with every new email (billing mail is coming with ADR040); and the started implementation should be scheduled work, not loose uncommitted state that rots.
- **Render parity included** (t008), scoped to the email surface itself: subjects, register, and content compared against Render's equivalent invite/deploy/webhook emails. No REST/GraphQL/MCP wire shapes or dashboard UI change in this milestone — parity here is about the emails, not the APIs.
