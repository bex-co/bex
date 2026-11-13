# w5 · m371 — Security review round 24: the agent, sandbox and hosting surfaces shipped since round 23

**Worker:** worker5 **Goal:** every tenant-facing surface shipped since the last broad review (round 23, ADR085, 2026-08-29) has a recorded security verdict; confirmed findings are fixed with regression tests or filed with an owner; ADR107 records the round in the ADR028 lineage. **Status:** blocked (t001–t009 done; fixes shipped; t010 Closeout waits on the production deploy that carries them)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Scope and threat-model the surfaces shipped since round 23 — **DONE** | 45m | — |
| [t002](done/t002.md) | Audit the cloud-agent plane — **DONE** | 2h | t001 |
| [t003](done/t003.md) | Audit the sandbox plane — **DONE** | 1h30m | t001 |
| [t004](done/t004.md) | Audit the hosting surfaces — **DONE** | 1h30m | t001 |
| [t005](done/t005.md) | Fix the confirmed findings with regression tests; file the overflow — **DONE** | 2h | t002, t003, t004 |
| [t006](done/t006.md) | Record the round in ADR107 and the security lineage — **DONE** | 40m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 20m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 30m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 9h40m**.

## Definition of done

1. Every surface in t001's scope table has a verdict (no finding, finding fixed, finding filed, or accepted residual with its DO_NOT_DO anchor) in the round-24 ADR (`docs/ADR107-security-review-round24`, a Markdown file t006 creates; take the next free number if 107 is gone by then).
2. Every confirmed Critical or High finding is fixed, covered by a regression test that fails without the fix, and deployed; each Medium or Low is fixed or filed as a board item with severity and owner.
3. ADR107 is linked from the security lineage in `docs/AGENTS.md` and the root `AGENTS.md` key entry points.
4. No accepted residual in `.pm/DO_NOT_DO.md` (#PSL, #CI-RUNNERS, #RUNNER-HOSTS, #ENV-REVIEWERS, #APPROVAL-MODE) is re-filed as a finding.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w5` round 9, item 3 (2026-11-12), filed by `/pmx 1,2,3,4` with w5 chosen. Re-checked at filing against `origin/main` `98b3587a8`: no open security-review item exists on the board; w3/m365 (push protection and branch rules, done 2026-11-13) is repository settings, not these surfaces.
- **Last broad review:** round 23 ([ADR085](../../../../docs/ADR085-security-review-round23.md), codex-security `5PePz5` at `3cbb7b8e`, accepted 2026-08-29). Since then only the static-site rewrite proxy has had a focused review ([ADR103](../../../../docs/ADR103-security-review-static-rewrite-proxy.md), 2026-11-10).
- **Shipped since** (ADR086–ADR106 and recent milestones): account deletion (ADR086); the obs.bex.co ops gate (ADR088); agent-owned delivery, where the agent commits, pushes and opens PRs (ADR090); Postgres extensions (ADR092); the Cloud Agents MCP (ADR094); agent attachments (ADR095); build sources and cross-service image copies (ADR097); agent session secrets (ADR098); workspace skills (ADR099); scheduled automations (ADR101); child sessions (ADR102); PR feedback and `/bex` takeover (`agentsessions/pr_feedback.go`, `pr_takeover.go`); sandbox Render parity, groups and OCI image import (ADR104, w2/m358, w2/m359); the bare-metal sandbox node (ADR105); the CLI's agent ssh, handoff and pickup (ADR106); Postgres credential rotation (w5/m265); Key Value internal auth (w5/m268); Blueprint plan and approve over MCP (w5/m283); readable webhook secrets (w5/m285).
- **Goal linkage:** GOAL item 7 (security review) and the ADR028 review lineage.
- **Expected outcome:** the 75 days of new attack surface, much of it running untrusted code or holding GitHub tokens, get a verdict before open signup.
- **Why now:** cards are on file for every workspace and charged since 2026-11-12 (ADR075 D7, w6/m362), Render's sandboxes went public beta on 2026-11-11 and bex's match them, and the agent plane now acts on GitHub with installation tokens and on PR comments.
- **Render parity: included, conditionally.** Fixes are likely to change refusals on REST, GraphQL or MCP; t007 checks them against render.com for hosting surfaces and Devin for agent surfaces, or records that no surface changed.

## Findings

### 2026-11-13 — round 24 (loopx w5)

- **Record:** [ADR107](../../../../docs/ADR107-security-review-round24.md). The skill's run-1 artifacts (`architecture.md`, `REPORT.md`, `findings.json`, validated against the skill's schema) are outside the repository at `~/security-audit-skill/bex-w5/run-1`.
- **t001:** scope and threat model for 22 surfaces in three planes.
- **t002–t004:** three parallel hunts. Cloud agents: no exploitable finding. Sandboxes: S-1 (high), S-2 and S-3 (medium). Hosting: H-1 (medium). Each finding was re-read at the audited revision (`5bfc00af7`). S-1's paging contract was checked against the pinned opensandbox-server 0.2.2 wheel.
- **t005 fixes:**
  - H-1: `members.Invite`, `ResendInvite` and `ChangeRole` take `AuthorizeMintClass`, the operations are classified mint, a webhook repoint takes it too, and a delegated token reads the secret redacted.
  - S-1: paged `sandbox.Client.List`, refusing a list that moved; public-id lookups use the server's metadata filter.
  - S-2: the declared config size is checked before the read; source manifests are capped.
  - S-3: in-flight imports count at their declared size, a new import is checked in the build transaction under a workspace lock, and imports take the billing gates. Migration 0247 aligns the index.
- **t006:** ADR107, the `docs/AGENTS.md` lineage and the root `AGENTS.md` entry points.
- **t007 (Render parity):** each fix answers the same on REST, GraphQL and MCP because the three share one service verb: `members`, `webhooks.Update`/`Get`/`List`, `sandbox.ImportImage`, and the sandbox list consumers. Render has no third-party OAuth clients (its API keys are user-level), so H-1's refusal has no Render counterpart to drift from. A sandbox import's new 429 `SANDBOX_STORAGE_LIMIT` and its billing refusals are bex's existing codes for sandbox storage and creation.
- **t008 (Simplify):** three reviewers.
  - Applied: `RequireBillingFor`; one `coretest.PlatformClients` fake in place of five; a stable-total check with one re-read, a page bound and a guard for a full unpaged page; the metadata-filter lookup; `core.Identity.MintClass` for webhook reveals (no token introspection per read); the cap check inside `store.CreateSandboxBuild` (concurrency- and retry-safe); a redirect-aware, path-exact manifest cap with its own error; failed builds clearing their declared size; one 429 helper.
  - Not applied: an audit row for a refused mint-class call (API-key creation has the same gap, so it is a separate decision); a paged version of the stateful `fakeOpenSandbox`.
- **t009 (Test coverage):** the regression tests ADR107 § Verification lists, each failing on the pre-fix code; the sandbox ones run against real Postgres.
- **Follow-ups filed:** w5/186–190 (hardening, ADR107 § Residuals).
