# w9 · m56 — Official CLI service create/update parity

**Worker:** worker9 **Goal:** the unmodified official Render CLI v2.21.0 can create and update bex services with every supported flag without silent field loss, with repeatable evidence for the paths the dev-9 audit could not exercise and explicit classification of upstream-client and platform non-goals. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Freeze the official CLI contract and supported-parity boundary | 40m | — — **DONE** |
| t002 | Persist service IP allowlist descriptions without breaking old Apps | 60m | t001 — **DONE** |
| t003 | Carry service allowlist entries across REST, GraphQL, and MCP | 60m | t002 — **DONE** |
| t004 | Preserve allowlist descriptions in the dashboard networking editor | 40m | t003 — **DONE** |
| t005 | Add a live official-CLI create/update parity verifier | 60m | t001, t003 — **DONE** |
| t006 | Regrade the checklist from captured official-CLI evidence | 30m | t004, t005 — **DONE** |
| t007 | Render parity: audit every service create/update surface | 30m | t006 — **DONE** |
| t008 | Simplify: `/simplify` over the changed code | 20m | t007 — **DONE** |
| t009 | Test coverage: storage compatibility, adapters, UI, and CLI harness | 45m | t007 — **DONE** |
| t010 | Closeout | 15m | t008, t009 — **DONE** |

## Implementation notes (2026-07-18, closeout)

- **One description-preserving contract:** new Apps store ordered `IPAllowEntry` values in `spec.ipAllowListEntries`; flat-only legacy Apps still decode and enforce through deterministic fallback, while every new write clears the legacy field so an explicit clear cannot resurrect stale CIDRs. REST uses Render's `{cidrBlock,description}` shape; GraphQL/MCP retain tested flat aliases and reject conflicting dual inputs; Blueprint uses the same Core request; the operator projects CIDRs only.
- **One dashboard editor:** service, Postgres, and Key Value networking now reuse `IPAllowListEditor` for add, inline edit, ordered move, remove, clear, duplicate/invalid blocking, and description-preserving submit. This was the main simplify finding; the other conversion paths already reduce to one helper per layer and were retained for compatibility.
- **Official CLI evidence:** unmodified v2.21.0 passed `scripts/cli-compat.sh verify`, `services-parity-verify configured`, and `services-parity-self-test`. The configured run used disposable OpenBao plus auth-enabled persistent Zot: env var and secret file matched exact readback, anonymous private pull failed, create bound credential A, update replaced it with B, and the kubelet pulled the private image to a Running App. Runtime update failed in the upstream client before PATCH, previews reached bex and failed explicitly, and a non-Render platform region required an explicit clone region. Redacted bodies and the complete flag disposition are in `docs/render-artifacts/cli-services-create-update.md`.
- **Validation:** `make test` (operator/types and codegen), backend `go test ./...`, offline dashboard GraphQL codegen, dashboard typecheck/lint, all 241 dashboard files / 1,519 tests, focused editor tests, shell `bash -n`, verifier omission census, Markdown Prettier, and `git diff --check` passed. Verifier Apps and registry credentials were absent after cleanup; the operator was restored to one replica; disposable API/port-forward processes were stopped.

## Definition of done

Using the unmodified official Render CLI v2.21.0 against a configured bex environment, a scripted run creates and updates representative `web_service`, `cron_job`, and `static_site` services and proves every bex-supported create/update flag either round-trips exactly or fails explicitly—never as a silent no-op. In particular, `cidrBlock` and `description` survive service create, update, readback, GraphQL, MCP, and the dashboard editor while the operator enforces only the CIDRs; legacy Apps containing the existing flat `spec.ipAllowList` remain readable and enforce identically. The verifier covers the dev-9 audit's environment-blocked `--env-var`, `--secret-file`, `--registry-credential`, and native `--cron-command` paths in an environment with their dependencies enabled and cleans up what it creates.

`docs/cli-compatibility-checklist.md` is regraded only from captured official-CLI evidence. `--runtime` update is identified as an upstream CLI guard, `--previews` remains the documented bex non-goal, and clone/region behavior is stated truthfully: no bex task forks the CLI or invents a Render region for a deployment whose configured `BEX_REGION` is outside the CLI's closed enum. `cd lego/backend && go test ./...`, operator/types tests and codegen checks, dashboard typecheck/lint/tests, and the new CLI verifier are green.

## Source + Goal linkage

- **Source:** user request 2026-07-18 (`$pm fix services create and services update parity in docs/cli-compatibility-checklist.md for w9`) plus the exhaustive official-CLI v2.21.0 flag audit recorded in `docs/cli-compatibility-checklist.md`. The audit found one true supported-field loss (service IP allowlist descriptions), four configured-dependency evidence gaps, and three deliberately limited paths (`--from` with a non-Render region, CLI-blocked runtime switching, and preview environments).
- **Goal linkage:** bex's fifth compatibility surface is the official Render CLI running unmodified against bex-api. This closes flag-level service mutation drift while preserving the architecture rule that REST, GraphQL, MCP, and dashboard all ride one Core contract.
- **Expected outcome:** operators can run one cleanup-safe verifier and see supported service create/update flags pass end to end; users no longer lose allowlist descriptions; the checklist distinguishes product bugs from environment limitations, upstream client guards, and explicit platform non-goals.
- **Why now:** w9/m2 and w9/m4 established command-family compatibility and regression coverage, but the 2026-07-18 flag-by-flag sweep exposed gaps those broad checks could not see. Fixing the shared allowlist representation now also prevents the REST-only loss from spreading further into GraphQL/MCP/dashboard. Render parity is included because this changes tenant-facing REST, GraphQL, MCP, and UI behavior.
