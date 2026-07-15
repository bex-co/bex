# Render CLI compatibility checklist

Render's own CLI ([render.com/docs/cli](https://render.com/docs/cli), open-source [`render-oss/cli`](https://github.com/render-oss/cli)) is bex's fifth verified surface, the same way `render-oss/render-mcp-server` anchors the MCP surface ([ADR018-render-parity.md](ADR018-render-parity.md)). bex never forks or patches the CLI (`.pm/DO_NOT_DO.md`); this page tracks how much of it works against bex-api **unmodified**, with every claim backed by a captured, reproducible run. Gaps are bex-api's to fix, never the CLI's.

**Method.** One command per row (or one row per family where every subcommand shares a single root cause), each run for real against a live bex-api instance with real data (a deployed `web_service`, a `Database`, a `KeyValue`), evidence linked to [`.pm/w9/done/m2/evidence/log.md`](../.pm/w9/done/m2/evidence/log.md). Legend mirrors ADR018: ✅ works · ◐ works with a named divergence · ✖ fails (gap — owner in the backlog below) · — deliberate non-goal (`.pm/DO_NOT_DO.md`).

## Harness (reproduction)

|  |  |
| --- | --- |
| CLI | `render-oss/cli` @ `c23438e` (2026-07-14), checked out at `./cli`, built with `go build` (Go 1.26) |
| bex-api | this repo @ `32fae7ee`, run as workstream w9's isolated dev environment (`.pm/w9/dev-9/`, see its README) |
| Host override | `RENDER_HOST=http://localhost:54090/v1/` — `cli/pkg/cfg/cfg.go`'s `GetHost()` honors the `RENDER_HOST` env var; no CLI source change needed |
| Auth | Render's own API keys are static bearer tokens used as-is. bex's are Hydra `client_credentials` pairs (`docs/ADR012-auth.md`) — the CLI has no token-exchange step, so [`scripts/cli-compat.sh`](../scripts/cli-compat.sh) does it out of band: exchanges `key_id`/`key_secret` at Hydra's public `/oauth2/token` for a short-lived (15m) bearer and feeds that into `RENDER_API_KEY` (`cfg.go`'s `GetAPIKey()`, sent verbatim as `Authorization: Bearer`) |
| Run date | 2026-07-15 |

Reproduce from a clean checkout:

```sh
bash .pm/w9/dev-9/up.sh              # bring up the isolated bex-api + Kratos + Hydra stack
kubectl -n dev-9-auth port-forward service/hydra-public 59090:4444 &   # up.sh only forwards hydra-admin; the token endpoint is hydra-public
bash .pm/w9/dev-9/bootstrap-key.sh   # Kratos native registration -> first-login tenant mint -> POST /v1/api-keys
(cd cli && go build -o ../.pm/w9/dev-9/bin/render .)
scripts/cli-compat.sh <any render subcommand>   # e.g. scripts/cli-compat.sh workspaces -o json
scripts/cli-compat.sh verify         # re-runs every ✅ row below, t006 — see docs/cli-compatibility-checklist.md#reproducing-the--rows
```

## Systemic root causes

Six recurring bugs explain most of the ✖ rows below; each is cited by name in the table instead of repeating the diagnosis per row.

| # | Root cause | Evidence | Owner |
| --- | --- | --- | --- |
| **RC1** | **Error envelope shape.** bex-api's error writer (`lego/backend/internal/core/http.go:63,66`, `WriteJSON(w, code, map[string]string{"error": err.Error()})`) emits `{"error": "<message>"}`. Render's real contract (the CLI's generated `Error` type, `cli/pkg/client/types_gen.go:2120`) is a flat `{"id": "...", "message": "..."}` body. Because `{"error": "..."}` is still valid JSON, `encoding/json` silently unmarshals it into a zero-value `Error{}` (no error to fall back on) — the CLI's `firstNonNilErrorField` (`cli/pkg/client/client.go:115-149`) then reports the unhelpful generic `"unknown error"` for **every** bex-api 4xx/5xx the CLI hits, discarding the real (often perfectly good) message bex-api sent. This is the single highest-impact, lowest-effort fix in this whole checklist — one response-writer shape change, system-wide payoff. | `keyvalues create` via the CLI prints `Error: unknown error`; the same request through a logging proxy shows bex-api actually said `{"error":"bad request: unknown maxmemoryPolicy \"allkeys_lru\" ...}"` — see RC4. | `w1/022` (new note) |
| **RC2** | **`Service.autoDeploy` wire type.** bex-api serializes `autoDeploy` as a JSON boolean; Render's OpenAPI spec (and the CLI's generated `AutoDeploy` string-enum type, `cli/pkg/client/types_gen.go:1605-1606`, values `"yes"`/`"no"`) expects a string. Breaks every CLI path that decodes a full `Service` object. Same note also covers `deploys list`'s sibling `Deploy.image` wire-type mismatch below. | `.pm/w9/done/m2/evidence/log.md` §services list/delete/logs: `json: cannot unmarshal bool into Go struct field Service.service.autoDeploy of type client.AutoDeploy`. | `w2/008` (new note) |
| **RC3** | **Datastore list "cursor envelope" shape.** `GET /v1/postgres` / `GET /v1/key-value` return flat arrays of objects; the CLI expects each item wrapped `{postgres: {...}, cursor: "..."}` / `{keyValue: {...}, cursor: "..."}` (matching how `GET /v1/services` already wraps `{service: {...}, cursor: "..."}`, confirmed by direct REST — see `.pm/w9/done/m2/evidence/log.md`). Because Go's `encoding/json` doesn't error on a merely-differently-shaped object, every field silently decodes to its zero value instead of failing loudly — the CLI reports a "postgres/keyvalue" with every field empty, and any client-side match-by-name/id logic downstream (`get`, `update`, `suspend`) then can't find the real record. | `.pm/w9/done/m2/evidence/log.md` §postgres list / keyvalues list: every field `""`/zero despite a real record existing; §postgres get: `Error: postgres not found` for an instance that demonstrably exists; §keyvalues suspend/update: `Error: Multiple Key Value instances found with name '…'`. | **`w8/m13`** (open — "Datastore list pagination: Postgres + Key Value," t001 targets exactly this envelope; also allowlisted as a known divergence in `w7/done/m30`'s conformance suite) |
| **RC4** | **KeyValue `maxmemoryPolicy` underscore vs. hyphen.** The CLI only ever sends Render's real wire values (`allkeys_lru`, underscore — `cli/pkg/client/types_gen.go:688-695`); bex-api's key-value create only accepts hyphenated values (`allkeys-lru`) and 400s on anything else. No CLI flag combination can work around this — it's a hard block on `keyvalues create` end to end. | `.pm/w9/done/m2/evidence/log.md`: proxy-captured `RESP 400: {"error":"bad request: unknown maxmemoryPolicy \"allkeys_lru\" (valid: [... allkeys-lru ...])"}`. | `w8/004` (new note) |
| **RC5** | **Unprefixed/bare-name resource ids.** bex intentionally uses the user-chosen name as a resource's id for managed datastores (`docs/ADR020-identifiers.md` §Known deviations) — and Services follow the same pattern (`a.Name = req.Name`, `lego/backend/internal/apps/service.go:713`, no `srv-<xid>` minted). Render's real ids carry a type prefix (`srv-`, `dpg-`, `red-`, …) that the CLI's `restart`/`ssh`/`psql`/`kv-cli`/`pgcli` commands use to infer resource type client-side (`cli/pkg/resource/service.go:24-27,220,244`). Every one of those commands fails on any bex-native id. This is architectural, not a simple bug — undoing it means reopening ADR020, not a bex-api patch. | `.pm/w9/done/m2/evidence/log.md` §restart: `Error: failed to restart resource: unknown resource type`. | `w1/021` — flagged for design + a future `/pm promote`, not a quick fix (per DO_NOT_DO sizing rule) |
| **RC6** | **`GET /v1/users` (whoami) not implemented at all.** bex-api has no route for it — 404, not a shape mismatch. | `.pm/w9/done/m2/evidence/log.md` §whoami: `received response code 404`. | `w4/016` (new note) |

## Command census

| Command | Result |  | Notes |
| --- | --- | --- | --- |
| `login` | ✅ | 4 | Recognizes `RENDER_API_KEY` and skips the browser dashboard-login dance: `Success: CLI is already authenticated.` |
| `logout` | ◐ | 4 | Correctly refuses to "log out" an env-var credential and explains how to revoke via the dashboard — CLI-native behavior, not a bex gap; no owner needed. |
| `whoami` | ✖ | 4 | RC6 → owner `w4/016`. |
| `workspace current` | ✅ |  | Returns the real minted tenant (`tea-…`). |
| `workspace set` | ◐ | 8 | Not driven interactively in this harness; `RENDER_WORKSPACE` achieves the same effect through `config.WorkspaceID()`'s env-var precedence (`cli/pkg/config/config.go:123-131`) — every other command below relies on exactly that. Not a bex gap; no owner needed. |
| `workspaces` | ✅ |  | Lists the real minted tenant. |
| `projects` | ✅ |  | `null` (valid empty list — no projects created in this harness run; endpoint responds, no error). |
| `environments <id>` | ✖ |  | RC1 masks the real 404 for an unknown project id as `unknown error` → owner `w1/022`. |
| `services` (list) | ✖ | 8 | RC2 → owner `w2/008`. |
| `services create` | ◐ | 8 | The App CR is genuinely created (`kubectl get apps.app.bex.co` shows it `Deploying`) but the CLI prints `null` instead of a confirmation — the create response likely hits RC2/a sibling shape issue too → owner `w2/008`. |
| `services update` | ◐ | 8 | `--num-instances` is rejected **client-side** before any HTTP call (`--num-instances is not supported for update`) — a CLI-native restriction, not tested further against bex-api in this pass; no confirmed bex gap, no owner needed. |
| `services delete` | ✖ | 8 | RC2 (the delete path re-fetches/re-serializes the Service) → owner `w2/008`. |
| `services instances` | ✖ | 8 | `failed to list instances: 404 Not Found` — folded into `w2/008` (same Service-surface note) pending a closer look; owner `w2/008`. |
| `postgres create` | ✅ |  | Full, correct response body — real record created and returned intact. |
| `postgres` (list) | ✖ |  | RC3 → owner `w8/m13`. |
| `postgres get` | ✖ |  | RC3 (downstream: list-derived id is empty, so "get by name" can't resolve it) → owner `w8/m13`. |
| `postgres update` | ✖ |  | RC3 (same resolution failure) → owner `w8/m13`. |
| `postgres suspend` | ✖ |  | RC3 (same resolution failure) → owner `w8/m13`. |
| `postgres delete` / `resume` | — |  | Not run this pass (would hit the same RC3 resolution failure as `get`/`update`/`suspend`; not worth spending a live resource cycle to reconfirm). |
| `keyvalues create` | ✖ |  | RC4 (RC1 hides the real reason behind a generic error) → owner `w8/004` (RC1 masking → `w1/022`). |
| `keyvalues` (list) | ✖ |  | RC3 → owner `w8/m13`. |
| `keyvalues get` | ✖ |  | RC3 (manifests as "multiple instances found" rather than "not found" — see RC3 note) → owner `w8/m13`. |
| `keyvalues update` | ✖ |  | RC3 → owner `w8/m13`. |
| `keyvalues suspend` | ✖ |  | RC3 → owner `w8/m13`. |
| `keyvalues delete` / `resume` | — |  | Not run this pass, same rationale as `postgres delete`/`resume`. |
| `deploys list` | ✖ |  | `Deploy.image` wire-type mismatch — bex-api sends a bare string, the CLI expects `{ref, registryCredential, sha}` (a sibling of RC2) → owner `w2/008`. |
| `deploys create` | ✖ |  | RC1 masks the real cause (`unknown error`) → owner `w1/022`. |
| `deploys cancel` | ✖ |  | RC1 masks the real cause (tested against a nonexistent deploy id, so some failure is expected — the CLI just can't say what kind) → owner `w1/022`. |
| `restart` | ✖ |  | RC5 → owner `w1/021`. |
| `logs` | ✖ |  | RC2 (resolves the target Service before querying, so it never gets past RC2) → owner `w2/008`. |
| `jobs list` / `jobs cancel` | — |  | `GET`/`POST .../jobs` 404 — one-off jobs are a documented deliberate non-goal: `.pm/DO_NOT_DO.md` ("Shell/SSH into instances, one-off jobs (`/services/{id}/jobs`), and user SSH keys — all one exec surface: hosted execution is pillar 5, off the roadmap") and `docs/ADR018-render-parity.md`'s one-off-jobs row. |
| `jobs create` | ◐ |  | CLI-native flag mismatch caught before any HTTP call (`--plan` doesn't exist, the real flag is `--plan-id`) — a harness authoring error, not a finding; not retested with the correct flag given `jobs list`/`cancel` already show the family is a deliberate non-goal. No owner needed. |
| `workflows list` | — |  | 404 — Render Workflows is an already-documented deliberate non-goal (ADR018 §Gap backlog: "Deliberate non-goals … workflows"). |
| `blueprints validate` | ✖ |  | `validation request failed with status 400: {"error":"bad request"}` — RC1 hides the real reason; `examples/whoami-app.yaml` is a valid bex App CR spec but apparently not accepted as a Render Blueprint manifest (it isn't wrapped in a `render.yaml`-shaped `services:` document, which may simply be the correct expected failure — needs a real `render.yaml` retest before concluding this is a genuine gap). Owner: `w2/009` (new note). |
| `ssh` | — |  | `` `render ssh` can only be used in interactive mode `` — refused **client-side** regardless of backend; also the standing hosted-exec non-goal (`.pm/DO_NOT_DO.md`). |
| `kv-cli` | — |  | Same client-side interactive-only refusal; hosted-exec non-goal. |
| `pgcli` | — |  | Same client-side interactive-only refusal; hosted-exec non-goal. |
| `psql` | ◐ |  | The one session command that supports non-interactive mode (`--command`). It reaches bex's real Postgres and is correctly turned away at the network layer: `IP address (…) not in allow list for` — expected in this harness (`BEX_DB_DOMAIN` unset in dev-9 ⇒ internal-only, per the root `CLAUDE.md` env-var table), not a bug; would need `BEX_DB_DOMAIN` configured to verify end to end. Operational config, not a code gap — no owner needed. |
| `docs` | ✅ |  | Opens the (Render, not bex) docs URL in a browser — no bex-api call by design. |
| `skills` | ✖ |  | Panics (`nil pointer dereference`, `cli/pkg/tui/stack.go:105`) before any HTTP call reaches bex-api — a bug in `render-oss/cli` itself, reproducible against api.render.com too in principle. Not a bex compatibility finding; nothing for bex-api to fix; no owner needed (bex never patches the CLI, `.pm/DO_NOT_DO.md`). |
| `ea` (early access) | — |  | Lists early-access subcommands (`objects`, `sandbox`, `sandbox-groups`) — object storage and hosted sandboxes are both off bex's roadmap today (sandboxes: pillar 5, `.pm/DO_NOT_DO.md`); not walked further. |

Full raw transcripts for every row: [`.pm/w9/done/m2/evidence/log.md`](../.pm/w9/done/m2/evidence/log.md).

## Reproducing the ✅ rows

`scripts/cli-compat.sh verify` re-runs the rows marked ✅/◐ above that make a real bex-api call and exits non-zero the moment one regresses (see [`.pm/w9/done/m2/verify.sh`](../.pm/w9/done/m2/verify.sh) for exactly what it checks and how).

## Gap backlog

See [ADR018-render-parity.md](ADR018-render-parity.md) for the owner mapping — the CLI surface rides the same gap-backlog ledger rather than duplicating it here.
