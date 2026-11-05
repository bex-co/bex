# w2 · m263 — Sandbox flags the Render CLI v2.35 sends: honor or refuse each by name

**Worker:** worker2 **Goal:** every sandbox flag the pinned Render CLI v2.35.0 sends behaves as on render.com, or is refused by name, instead of a generic `unknown field` 400 or a silently ignored filter. **Status:** blocked — t003, t004, t006–t008 done; t001, t002, t005 await the user's sign-off

## Tasks (in order)

| id   | title                                                              | est | depends_on                   |
| ---- | ------------------------------------------------------------------ | --- | ---------------------------- |
| t001 | `idleTimeoutSeconds` on sandbox create and detail                  | 1h  | —                            |
| t002 | `--timeout=0` creates a sandbox with no timeout                    | 30m | —                            |
| t003 | Sandbox plan names `1c-2g` … `8c-16g` — **DONE**                              | 45m | —                            |
| t004 | Named refusals for sandbox groups and images on create and list — **DONE**    | 45m | —                            |
| t005 | `ea blueprints` next-step hints name `bex`, not `render`           | 45m | —                            |
| t006 | Render parity — **DONE**                                                      | 15m | t001, t002, t003, t004, t005 |
| t007 | Simplify — **DONE**                                                           | 15m | t006                         |
| t008 | Test coverage — **DONE**                                                      | 30m | t006                         |
| t009 | Closeout                                                           | 10m | t007, t008                   |

## Definition of done

- With the bex CLI at v2.35.0 (w2/m256), `bex sandboxes create --idle-timeout N`, `--timeout=0`, and `--plan 1c-2g` each do what Render documents, and detail output shows the real idle timeout.
- `--group`/`--sandbox-group` on create and list, `--image` on create, and `sandbox-groups create` / `images import|get` answer a 400 or 404 that names the unsupported feature, unless sandbox groups ship. A list never silently ignores a filter.
- `bex ea blueprints create|plan|approve|update` next steps, in both text output and JSON/YAML `meta.next`, point at `bex`. Otherwise ADR018 records the residual with the user's sign-off.
- REST, GraphQL, MCP and ADR018 agree.

## Source + Goal linkage

- **Source:** the w2/m256 pin bump re-diff (2026-11-04), which compared the v2.29.0 and v2.35.0 upstream sources and spot-checked the built launcher. Upstream CHANGELOG entries: v2.31.0 `--idle-timeout`; v2.32.0 `--group`/`--sandbox-group`, `sandbox-groups create`, `--wait`; v2.34.0 `--image`, `images import|get`, `--timeout=0` = no timeout. Server side: `lego/backend/internal/sandbox/rest.go` decodes strictly, so these fields 400 as `unknown field`; `sandboxGroupId` on list is ignored; plans accept only `starter|standard|pro`; timeout 0 maps to the 86400 s cap (w5/m99).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first). The Render CLI is bex's CLI (DO_NOT_DO: no first-party CLI), so gaps the CLI exposes are bex-api work.
- **Expected outcome:** customers on the v2.35 bex CLI never see an opaque `unknown field` error. Each sandbox flag works or explains itself, and agents following `meta.next` stay on bex.
- **Why now:** shipping m256 puts these flags in every customer's `bex sandboxes --help`. Before the bump the pinned CLI (v2.29.0) could not send them.
- **Render parity included:** REST/MCP sandbox surface and CLI output change.

## Work log — 2026-11-05 (`/loopx w2`)

- **t003, served:** plans `1c-2g`/`2c-4g`/`4c-8g`/`8c-16g` size the container as named. `starter`/`standard`/`pro` stay as aliases at 0.5/512Mi, 1/2Gi and 2/4Gi; before this, every plan ran at 0.5/512Mi. Billing follows the resources. An unknown plan gets `400 SANDBOX_PLAN_INVALID`.
- **t004, named refusals:**
  - `sandboxGroupId` on create and list → `SANDBOX_GROUPS_UNSUPPORTED`.
  - `buildId`/`buildName`/`image` → `SANDBOX_IMAGES_UNSUPPORTED`.
  - `/v1/sandbox-groups/**` and the CLI's snapshot routes answer named 404s.
  - The `status` filter now applies on REST, GraphQL and MCP. With no filter, terminated sandboxes are hidden, as on Render.
- **t001, partial:** `idleTimeoutSeconds: 0` is honored and every sandbox reports `0`. A positive value is refused by name with `SANDBOX_IDLE_TIMEOUT_UNSUPPORTED`. Honoring it needs idle tracking and wake-on-exec, which OpenSandbox lacks (ADR042 D5, ADR014 D4).
- **Tests and checks:** `cli_flags_test.go`, `sandbox_create_flags_gate_test.go`, the real-DB sandbox/api suite and `make lint` pass.

## Blocked (2026-11-05)

Three divergences, recorded in ADR018 § Sandbox create flags and § CLI runtime copy, need the user's call. Run `/unblock w2`.

1. **t001, idle suspension:** accept "no idle suspension" as a signed-off divergence, or approve building idle tracking plus wake-on-exec (a new milestone). Render suspends after 900 s by default.
2. **t002, `--timeout=0`:** Render means no timeout, with a 30-day maximum on paid plans. bex keeps w5/m99's 24 h cap, which exists because an orphan ran 403 h and cost $88.50. Options: allow an explicit `0` for paid workspaces, raise paid workspaces to 30 days, or sign off the cap.
3. **t005, `ea blueprints` next steps:** the hints name `render` in text and in `meta.next`. Fixing them needs a decision to rewrite upstream run-time output with a stdout wrapper like `brandLoginOutput`, which docs/bex-cli.md currently defers for the whole class.
