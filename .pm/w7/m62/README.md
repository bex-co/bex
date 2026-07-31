# w7 · m62 — Security-chores round 2: govulncheck CI + sandbox-prune keying + publish-credential scope

**Worker:** worker7 **Goal:** three filed sub-hour security chores land as one shippable chunk (the m10/m37 grouping precedent): Go call-graph vuln scanning joins the CI guard family, a config-toggle rollback can no longer reap live sandbox namespaces, and the static-site publish Job stops holding a wildcard registry credential **Status:** todo

## Tasks (in order)

| id   | title                                                                                     | est | depends_on       |
| ---- | ------------------------------------------------------------------------------------------ | --- | ---------------- |
| t001 | Pinned `govulncheck` CI job over the Go workspace + baseline triage + gate policy (`013`)   | 45m | —                |
| t002 | Gate `<ws>-sandbox` prune on workspace existence, not the config toggle (`011`)             | 45m | —                |
| t003 | Repo-scoped publish-Job extract credential: investigate, fix or record (ADR045 Finding 7)   | 45m | —                |
| t004 | Simplify pass over the changed code                                                         | 20m | t001, t002, t003 |
| t005 | Test coverage: prune regression, guard durability, credential-scope assertion               | 45m | t001, t002, t003 |
| t006 | Closeout                                                                                    | 10m | t005             |

## Definition of done

- `govulncheck` runs in CI over all three workspace modules, pinned by version + sha256 (the gitleaks/trivy pattern), with the gate policy (fail on reachable-vuln-with-available-fix, warn otherwise) documented next to the job and the current baseline triaged before the gate is on.
- Flipping `BEX_TENANT_SANDBOX_NAMESPACES` off no longer deletes a live workspace's `<ws>-sandbox` namespace — prune reaps only when the workspace itself is gone — proven by a regression test.
- The static-site publish Job's extract initContainer pulls with a repo-scoped credential instead of the wildcard `bex-builder` identity, **or** ADR045 Finding 7's row carries an evidence-backed disposition explaining why it cannot (with the trigger to revisit).
- Inbox notes `w7/011` and `w7/013` retired to `done/`; backend + operator suites and lint green.

## Source + Goal linkage

- **Source:** groups `w7/011` (ADR045 Finding 6), `w7/013` (govulncheck, CI security-guard sweep 2026-07-30), and ADR045 Finding 7 — whose "folded into w1/m57" cross-reference was checked during `/pm-brainstorm more for w7` round 2, 2026-07-30 and found **not executed**: m57 codified the wildcard `bex-registry-pull`/`bex-builder` credential into gitops as-is (its t004 asserts identity "still `bex-builder`"), so the repo-scoped recommendation was still unowned.
- **Goal linkage:** security hygiene / least privilege — the m10 (CI scanning), m36/m39 (registry least-privilege), and w3/m31 (namespace lifecycle) lineages.
- **Expected outcome:** the CI guard family covers the one scanning axis it lacked (Go call-graph); an operator config rollback is no longer destructive; no short-lived platform pod holds a wildcard registry credential (or the exception is documented with evidence); the w7 inbox drains to the single deliberate record-don't-build note (`008`).
- **Why now:** all three are already-filed audit/board debt with concrete fixes; grouped they clear the backlog in one pass per the m10/m37 sub-hour-chores precedent.
- **Render parity:** omitted — CI + operator/store mechanism, no REST/GraphQL/MCP/UI surface change.
