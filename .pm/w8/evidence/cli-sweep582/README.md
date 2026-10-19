# CLI sweep 582 — create name boundaries, duplicate names, `--env-var` dialects

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Four owned fixtures, all created with explicit `--plan free`, all deleted with REST 404 and zero `qa582` names remaining. Closes the boundary-pair Unrun note from sweep 581.

- **Duplicate name:** a second create with the live name `qa582-web` fails `409 (CONFLICT): name "qa582-web" is already in use`, exit 1 — no shadow row created.
- **Name length boundary pair:** exactly 30 characters (`qa582-` + 24) creates successfully; 31 characters fails with the same named `RESOURCE_NAME_INVALID` grammar from sweep 581. The cap in the message is the cap enforced.
- **`--env-var` client dialects:** `JUSTKEY` (no `=`) and `=value` (empty key) fail client-side before any request: `invalid --env-var "…": expected KEY=VALUE`, exit 1, no fixture leaked.
- **Accepted forms read back exactly:** `KEY580=` creates the fixture and the env-var list shows `{"key":"KEY580","value":""}` — an explicit empty value is stored, not dropped. Duplicate keys `--env-var DUP=1 --env-var DUP=2` collapse to one row with the last value (`2`), matching last-wins flag semantics.

## Unrun

- Env-var key grammar beyond the `=` split (whitespace, lowercase, leading digit) at create time — partly covered at the REST boundary by sweep 577.
- Rename-to-duplicate via `services update` (create-side conflict proven here).
