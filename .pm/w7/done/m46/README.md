# w7 · m46 — Render CLI `psql [id|name]` end-to-end acceptance: flip the IP-allow-list-gated `[~]` to `[x]`

**Worker:** worker7 **Goal:** Prove the unmodified official Render CLI's `psql [id|name]` (and its `-c/--command` non-TTY path) connects end-to-end through bex, turning the last database-session `[~]` in `docs/cli-compatibility-checklist.md` into a `[x]` backed by a captured artifact. **Status:** done — t001–t006 **DONE** (2026-07-18)

## Tasks (in order)

| id   | title                                                                                                                                                            | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Confirm no bex code gap: `ipAllowList` on the `GET /v1/postgres/{id}` read shape + `externalConnectionString` on connection-info; document the psql gate mechanism — **DONE** | 30m | —          |
| t002 | Add a `psql-verify` leg to `scripts/cli-compat.sh` mirroring `pgcli-verify` (disposable public Postgres + self-IP `/32` allowlist, drive CLI `psql <id>`/`<name>` `-c`) — **DONE** | 1h  | t001       |
| t003 | Run the acceptance in a `BEX_DB_DOMAIN` + pg-sni-proxy env, capture `docs/render-artifacts/psql-cli.md`, flip the checklist row + `-c, --command` sub-item to `[x]` — **DONE** | 45m | t002       |
| t004 | Simplify: run `/simplify` over the `psql-verify` leg — **DONE** | 20m | t002       |
| t005 | Test coverage: `psql-compat-verify.test.sh` drives the real CLI against fake api+psql (id/name/deny/exit); gated live leg for `cli-compat.sh verify` — **DONE** | 30m | t002       |
| t006 | Closeout — **DONE**                                                                                                                                               | 10m | t005       |

## Definition of done

The checksum-verified, **unmodified** `render-oss/cli` v2.21.0 `psql <id>` and `psql <name>`, run through bex-api with `-c "SELECT 1 AS bex_psql_probe;"`, both resolve one disposable **public** bex Postgres and return the probe row through the real local `psql` binary in an environment that has `BEX_DB_DOMAIN` set + the pg-sni-proxy reachable + an `ipAllowList` containing the caller's `/32`. Evidence (redacted command captures, the SQL marker, the source-IP allowlist entry, cleanup proof) is recorded in `docs/render-artifacts/psql-cli.md`. The `docs/cli-compatibility-checklist.md` `psql [id|name]` row and its `-c, --command <SQL>` sub-item read `[x]`; the negative path (caller IP absent from the allowlist ⇒ the CLI's own `"IP address … not in allow list"` client-side refusal) is asserted, not just the happy path. Markdown passes `npx prettier@3.4.2 --write`.

## Source + Goal linkage

- **Source:** `docs/cli-compatibility-checklist.md:207-208` (the `[~] psql [id|name]` row) + the 2026-07-18 research pass into `render-oss/cli` (`cmd/psql.go` → `pkg/tui/views/psql.go`) and the bex-api Postgres surface (`internal/postgres/{rest,service,access}.go`, `operator/internal/controller/database_controller.go:797-854`).
- **Goal linkage:** Render parity — the fifth surface (Render's own official CLI pointed at bex-api, `DO_NOT_DO.md` line 30). This closes the last database-session `[~]` in the CLI-compat ledger, the direct continuation of the m45 CLI-compat sweep.
- **Expected outcome:** The CLI-compat checklist's Session block reads `pgcli [x]` **and** `psql [x]` — both database-shell entries live-proven — with a reusable `psql-verify` regression leg so the claim does not rot.
- **Why now:** m45 (the 2026-07-18 CLI-compat verification pass) left this the sole gated database row, and the research resolved the ambiguity: the block is a dev-9 **environment** limitation (no `BEX_DB_DOMAIN`, no allowlist, no SNI proxy) reproducing Render's own client-side IP-allowlist gate — **not** a bex code gap. So the work is a bounded acceptance against a wired environment (mirroring how the sibling `pgcli` row was already proven `[x]`), cheap and self-contained, before the checklist drifts further.
- **Render-parity closing task omitted:** this is a verification/acceptance + docs + test-harness milestone with **no** REST/GraphQL/MCP/UI surface change — the milestone's whole thesis (established in research) is that bex-api already emits the Render-compatible shape (`ipAllowList` on the read object, `externalConnectionString` gated on `Public && BEX_DB_DOMAIN`, allowlist enforced at the pg-sni-proxy). If t001 unexpectedly surfaces a wire-shape gap (e.g. `ipAllowList` missing from the read shape), that fix carries its own REST/GraphQL/MCP parity check as an added task before Closeout.
