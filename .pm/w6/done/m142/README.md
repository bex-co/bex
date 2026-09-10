# w6 · m142 — ADR018 upstream drift re-baseline round 4

**Worker:** worker6 **Goal:** the Render-parity ledger is current against today's OpenAPI, changelog, and MCP tip, with every newly surfaced gap filed as owned board work **Status:** done — 2026-09-10

**Estimate:** ~2h implementation; ~3h including standing closing tasks (6 tasks). **Priority:** approved item 3 from `/pm-brainstorm for w6` 2026-09-09.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Diff Render OpenAPI ops (+ CLI regenerated client if past v2.26) vs w8/m33 baseline — **DONE** | 45m | — |
| t002 | Sweep Render changelog + render-mcp-server tip since round 3 — **DONE** | 30m | — |
| t003 | Re-baseline ADR018 (+ CLI checklist / pin only if upstream moved); file unowned gaps — **DONE** | 45m | t001, t002 |
| t004 | Simplify — **DONE** | 20m | t003 |
| t005 | Test coverage — **DONE** | 30m | t003 |
| t006 | Closeout — **DONE** | 10m | t004, t005 |

## Definition of done

- [x] `docs/ADR018-render-parity.md` carries a dated **round-4** re-baseline block: OpenAPI operation/shape diff vs the w8/m33 (2026-09-07) baseline, changelog + MCP-tip sweep, and classification of each delta as already ✅ / ◐ / ✖ / — (or filed follow-up).
- [x] Every real gap the round surfaces has an owned `.pm` inbox note or milestone — nothing silently accepted; no ADR018 deliberate `—` row or `.pm/DO_NOT_DO.md` anti-goal reopened without an explicit user decision.
- [x] If `render-oss/cli` has moved past the current pin, the pin + `docs/cli-compatibility-checklist.md` pin-conditional bullets are updated (or a follow-up note owns the bump); minting `bex-cli/v*` remains `/release`'s job.
- [x] The request-validator OpenAPI pin is not swapped casually — a needed refresh is filed as owned follow-up (w6/m96 gate discipline).

## Re-baseline summary (round 4, 2026-09-10)

- **OpenAPI:** live still **208** ops; request-validator pin still **207** (missing only `GET /services/{serviceId}/outbound-ips`). No new public paths vs round 3. Outbound-ips product surfaces already ✅ (w2/023 + w8/010). Pin refresh filed `.pm/w6/073.md`.
- **CLI pin:** `render-oss/cli` **v2.26.0 → v2.27.0** (`a764810a768202704e7206eb7b87a47211fcd98e`, module `v1.1.3-0.20260909214233-a764810a7682`). `bash scripts/bex-cli-validate.sh` + `cd lego/cli && go build/vet/test ./...` green. Seams: `RootCmd` exported, `CustomHelpTemplate` intact, `isRootVersionRequest` unchanged, no top-level/Bex-native collisions. Regenerated client +5 sandbox-snapshot methods only (`[-]` ea).
- **Changelog:** ea sandbox snapshots + create `--snapshot-id`; `blueprints validate` reports workflows (DO_NOT_DO — bex still refuses); x/crypto bump.
- **MCP tip:** `1cca753` → `a778e5a` — go.mod/go.sum only; contractual tool set unchanged.
- **Docs:** ADR018 round-4 block; checklist header + delta + disk pin-conditional re-check; UPSTREAM/validate/build/README/bex-cli/CLAUDE pin strings.
- **Not run:** live `scripts/cli-compat.sh verify` (environment carve-out as w8/m33). Minting `bex-cli/v*` is `/release`, out of scope.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w6` 2026-09-09 proposal 3 (labeled m144 in that proposal; materialized as next free **m142**). Precedent: `w2/m79` (round 2), `w8/m33` (round 3, 2026-09-07).
- **Goal linkage:** Render parity is the core product promise ([ADR006](../../../docs/ADR006-bex-api.md) / [ADR018](../../../docs/ADR018-render-parity.md)); an empty w6 queue is capacity for keeping the ledger honest as upstream moves.
- **Expected outcome:** parity claims are days-current instead of stale relative to round 3; new gaps have owners instead of silent drift.
- **Why now:** round 3 landed 2026-09-07; upstream OpenAPI/changelog/MCP tip move continuously; w6 has no other open work.
- **Render parity closing task omitted:** this milestone _is_ the cross-surface parity audit; drift it finds becomes filed follow-up work rather than a closing checkbox (same rationale as `w8/m33`).
- **Simplify / tests:** no product behavior change beyond the pin bump; existing hermetic launcher suite green; simplify N/A beyond comment clarity on analytics opt-out.
- **Anti-goals:** do not reopen PR previews, Managed OIDC, Bitbucket/GitLab, dedicated IPs, or other `.pm/DO_NOT_DO.md` / deliberate `—` rows.

## Gap analysis

- Round 3 already closed outbound-IPs client method awareness, `connectionPool` coverage, and filed `w8/011` (spec plan names; aliases since shipped). This round starts from that baseline — did not re-derive or re-file closed cells.
