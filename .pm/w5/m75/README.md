# w5 · m75 — ADR075 revision: claim flow, dashboard workspace scoping, verifier fail-at-start

**Worker:** worker5 **Goal:** an already-installed GitHub installation can be bound to a workspace through the OAuth claim flow (§3a), every dashboard GitHub surface acts on the selected workspace (§6), and a missing OAuth verifier fails at connect-start with an actionable error instead of a post-GitHub-round-trip 503 (§7) **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend claim flow: StartClaim verb + claim branch in the callback                        | 90m | —          |
| t002 | Verifier fail-at-start: StartConnect/StartClaim preflight + startup warning               | 20m | t001       |
| t003 | Surfaces: claimGit GraphQL mutation + POST /v1/git/claim REST                             | 30m | t002       |
| t004 | Dashboard: ownerId threading on every git surface + claim CTA on the recovery affordance  | 60m | t003       |
| t005 | Render parity — cross-surface consistency; record why claim is REST/GraphQL-only          | 20m | t004       |
| t006 | Simplify — `/simplify` over the milestone's changed code                                  | 30m | t005       |
| t007 | Test coverage — claim resolution matrix, preflight, ownerId threading                     | 45m | t005       |
| t008 | Closeout — live claim-flow walk on production, sync status, move to done/                 | 30m | t007       |

## Definition of done

The three 2026-08-20 revision sections of [docs/ADR075-github-workspace-connections.md](../../../docs/ADR075-github-workspace-connections.md) hold, verified live where marked:

1. **§3a claim flow (live):** with the `puncsky` installation already present on GitHub and unbound, `claimGit(ownerId: tian-personal)` returns a GitHub OAuth authorize URL; completing it binds installation 154851602 to `tea-da2isimlm39c739m4ofg` (DB row confirms), with no reinstall and no `missing_state`. Zero claimable ⇒ bounded `no_claimable_installation`; several ⇒ bounded `ambiguous_installation`; an installation bound elsewhere is never claimable.
2. **§6 workspace scoping (live):** with `tian-personal` selected, the Settings GitHub card shows *its* connection set (not `bex`'s); the repo picker, connect, claim, and disconnect all carry `ownerId` = the selected workspace; switching workspaces refetches.
3. **§7 fail-at-start (test):** with the verifier unconfigured, `connectGit`/`claimGit` refuse immediately with a bounded, actionable error and no transaction row is minted; bex-api logs the half-configured warning at startup.
4. All three proofs of the connect flow hold on the claim path (nonce transaction + initiator match + per-installation admin proof); backend + dashboard suites and `make lint` green.

## Source + Goal linkage

- **Source:** the 2026-08-20 revision of [docs/ADR075-github-workspace-connections.md](../../../docs/ADR075-github-workspace-connections.md), produced by the failed live verification walk of w5/m74 (open note `w5/046`): GitHub strips `state` for already-installed accounts (Configure → `github.com/settings/installations/<id>`), the dashboard git hooks pass no `ownerId` (verified live: tian-personal selected, bex's connection shown; an explicit-`ownerId` API probe returned the correct empty set), and production's missing `client-id`/`client-secret` Secret keys made every binding 503 only at the callback.
- **Goal linkage:** pillars 3–4 (repo connect + push-to-deploy) and the ADR018 Render-parity divergence: the claim flow is the workspace-bound model's answer to Render's "direct github.com install just works".
- **Expected outcome:** the original incident's user story finally completes — puncsky (already installed) binds to tian-personal from the dashboard with no uninstall; no GitHub surface can act on the wrong workspace; a misconfigured deployment says so at connect-start.
- **Why now:** m74 shipped the multi-connection model but its live verification is **blocked** on exactly these three defects; the wrong-workspace bind (§6) is an active correctness hazard on production today.
- **Render parity task included:** yes — new REST/GraphQL verbs + dashboard changes. The claim flow is deliberately absent from MCP (it is a browser OAuth ceremony an agent cannot complete; the parity task records this).

## Deployment prerequisite (ops, not a repo task)

§3a/§7 verification requires the production `bex-system/bex-github-app` Secret to gain the `client-id` + `client-secret` keys (from the GitHub App's settings page — operator action, credentials out-of-band per ADR019) and a bex-api restart. Until then every binding — install or claim — fails closed at the verifier.

## Security invariants (must hold throughout)

Three-proof one-principal connect (w1/m67 F3) extended, never bypassed: the claim path consumes the same single-use `github_connect_transactions` nonce, requires initiator == caller and fresh `can_manage`, and proves per-installation admin from the code-exchanged user token (`GET /user/installations` + the existing admin check) — the callback accepts **no client-supplied installation id** on the claim branch. One-workspace-per-installation (w1/m65 F2), the connection quota, and `account_login` refresh apply identically to claimed bindings. No new callback auth exemptions.
