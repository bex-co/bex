# w2 · m66 — RequiresSshKey: SSH-key onboarding gate for SSH affordances

**Worker:** worker2 **Goal:** no SSH affordance ever hands a user a doomed action — a `RequiresSshKey` gate detects zero registered keys and swaps the raw ssh/`zed://` payload for an in-product "add your key" CTA (with a browser-terminal alternative where one exists), turning the off-surface `Permission denied (publickey)` dead-end into a ~20s round-trip. **Status:** done (2026-08-10 — all tasks DONE; dashboard 300 test files + lint + typecheck green; live Playwright walk confirmed no-key → CTA → deep-linked add-key form; shipped via `/ship`).

## Tasks (in order)

| id   | title                                                                                     | est | depends_on          |
| ---- | ----------------------------------------------------------------------------------------- | --- | ------------------- |
| t001 | `RequiresSshKey` gate primitive + one cached `hasSSHKey` selector (fail-open)              | 40m | —                   | — **DONE** |
| t002 | Settings deep-link: `?addKey` opens the form (SSR-safe) + `returnTo` round-trip            | 40m | t001                | — **DONE** (used the `addKey` query param + `#ssh-public-keys` scroll rather than a `#addSshPublicKey` hash — a hash can't drive SSR-consistent state without a post-hydration effect) |
| t003 | Gate the agent-session Open-in-Zed control (swap payload → CTA)                            | 30m | t001, t002          | — **DONE** |
| t004 | Gate the service Connect→SSH menu + Shell page (two-door: add key / browser terminal)      | 40m | t001, t002          | — **DONE** |
| t005 | Funnel instrumentation + document the gateway `rejected_key`/`accepted` proxy metric       | 30m | t003, t004          | — **DONE** |
| t006 | Simplify pass over the milestone's changes                                                 | 30m | t005                | — **DONE** (shared `AddSshKeyCta`; effect-free SSR-safe open; fail-open-on-no-client) |
| t007 | Test coverage for gate states + deep-link round-trip + fail-open                           | 40m | t005                | — **DONE** |
| t008 | Closeout (on `/ship`)                                                                      | 15m | t007                | — **DONE** |

## Definition of done

A caller with **zero** registered SSH keys who opens an agent session's Connect control or a service's Connect→SSH menu / Shell page sees, instead of the raw `ssh …` command and live `zed://` link, a CTA that deep-links to `/settings#addSshPublicKey` with the add-key form focused; after saving a key, a `returnTo` round-trip returns them to the originating page with the real affordance now live. Where a browser terminal exists (paid services, the shipped Web Shell), the gate offers it as a second, zero-setup door. A caller **with** ≥1 key sees the affordance unchanged. If the `sshKeys` query errors or is still loading, the gate **fails open** (shows the real affordance — never hides a working feature). No keypair is ever generated in the browser. Component tests cover all three gate states (has-key / no-key / query-error) and the deep-link round-trip; the dashboard suite + lint + typecheck are green.

## Source + Goal linkage

- **Source:** live-testing failure during w2/m65 (2026-08-10): the author clicked Open in Zed and hit `Permission denied (publickey)` in Zed's own dialog — root-caused via the gateway metric (`rejected_key=2`, `rejected_target=0`) to **no registered SSH key**. PM discussion the same day defined the gate's principles (swap-payload-not-hide, fail-open, two-door, deep-link round-trip, activation-nudge-not-guarantee).
- **Goal linkage:** pillar 5 activation — makes the w2/m65 "Open in Zed" differentiator (and the older w2/m39 running-instance SSH + w2/m55 Web Shell surfaces) actually reach first-time users instead of dead-ending off-surface. Turns bex's missing SSH-onboarding moment into a reusable primitive.
- **Expected outcome:** the dominant first-run SSH failure (zero keys) is caught in-product with a one-click path to the fix; the gateway's `rejected_key` share for `ags-…`/`srv-…` usernames trends down as `accepted` rises.
- **Why now:** w2/m65 just shipped the feature that exposes the gap to production users, and the author already reproduced the exact dead-end. Every day it ships un-gated, first-time users bounce off the flagship editor with a cryptic error and no recovery path. The reusable surfaces (`ssh-keys` feature, service SSH/Shell, agent-session header, Web Shell) all already exist, so the marginal cost is low.
- **Render parity task OMITTED:** this is dashboard-only activation UX that **reuses the existing `sshKeys` query** (no new REST/GraphQL/MCP surface) and has **no Render counterpart** (Render surfaces SSH keys but ships no such precondition gate). If t001 instead adds a backend `viewer.hasSSHKey` convenience field, add a parity task before closeout. Simplify + Test coverage + Closeout are retained.
