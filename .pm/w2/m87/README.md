# w2 · m87 — New Service wizard runtime auto-detection

**Worker:** worker2 **Goal:** picking a repo (or changing Root Directory) in the New Service wizard pre-selects the correct runtime, matching Render's repo-inference behavior — closing the parity ledger's last open unowned gap-backlog row **Status:** todo

## Tasks (in order)

| id   | title                                                                                    | est | depends_on |
| ---- | ----------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Backend repo-tree probe: fetch the repo tree at a Root Directory via the GitHub connection | 45m | —          |
| t002 | Detection heuristic: manifest → runtime mapping mirroring Render's precedence              | 30m | t001       |
| t003 | Expose detection as a dashboard-facing GraphQL query                                       | 30m | t002       |
| t004 | Wizard integration: auto-select on repo pick, re-infer on Root Directory change            | 45m | t003       |
| t005 | Live verification against real repos (one per runtime + an ambiguous case)                 | 30m | t004       |
| t006 | Render parity                                                                              | 30m | t005       |
| t007 | Simplify                                                                                   | 20m | t006       |
| t008 | Test coverage                                                                              | 45m | t006       |
| t009 | Closeout                                                                                   | 15m | t007, t008 |

## Definition of done

- Picking a connected repo in the New Service wizard pre-selects the runtime implied by its manifests (Dockerfile, `go.mod`, `package.json`, `requirements.txt`/`pyproject.toml`, `Gemfile`, `mix.exs`, `Cargo.toml`); editing Root Directory re-runs detection against that subtree.
- An explicit user runtime choice is never overwritten by a later detection result.
- Detection failure (probe error, unrecognized tree, rate limit) degrades silently to today's manual selection — no error surface, no blocked wizard.
- The ADR018 gap-backlog row ("New Service wizard runtime auto-detection — unowned, open") is marked done with this milestone as owner.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w2` 2026-09-01 #2; `docs/ADR018-render-parity.md` gap backlog — the only row marked "open (divergence, not a regression)" and "unowned — surfaced by `w6/m45` t004" (closing it "needs a repo-tree probe … scoped by Root Directory").
- **Goal linkage:** pillar 1 Render compatibility and the git-push-to-URL developer experience (ADR008); ADR026 GitHub integration provides the connection/token plumbing this rides.
- **Expected outcome:** the parity ledger's gap backlog becomes fully owned; wizard UX matches Render's language inference.
- **Why now:** it is the final unowned parity-backlog item; w2's queue is empty and holds most of the parity-closure lineage.
- **Render parity:** **included** (t006) — feature dev touching a backend query surface + dashboard UI; the parity pass compares Render's inference precedence/behavior and decides whether the probe stays deliberately dashboard-only (Render has no public REST for it) or warrants REST/MCP exposure.
- **Coordination:** the wizard area neighbors `ServiceSourcePicker`, which open `w8/m31` (in progress, blocked on live verification) also edits — rebase over m31 if it lands first; the detection hook itself lives in the form layer (`use-new-service-form.ts`), not the picker.
