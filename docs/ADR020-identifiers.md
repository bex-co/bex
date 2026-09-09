# Resource identifiers (ADR): typed, opaque, hyphenated `<prefix>-<xid>`

**Status: decided + enforced.** Every bex-minted resource id is a **typed opaque string** of the form `<prefix>-<xid>` — e.g. `srv-c185th5c2rvvnhbfiltg`. One package mints and validates them (`lego/backend/internal/id`), one guard test pins the format, and this ADR is the rationale. The decision this file records: **the separator is a hyphen, never an underscore**, and ids are minted, never hand-built.

## The id shape

```
srv-c185th5c2rvvnhbfiltg
└┬┘ └────────┬─────────┘
 │           └ 20-char xid: k-sortable (time-ordered), non-guessable, lowercase base32-hex (0-9a-v)
 └ 2-4 char type prefix, Render-style
```

Registered prefixes (the `id.Kind` registry — Render's public-API spellings, so bex ids are drop-in for Render-shaped clients):

| Prefix | Resource                    | Render |
| ------ | --------------------------- | ------ |
| `tea-` | workspace (tenant)          | teams  |
| `srv-` | service (app)               | srv    |
| `dpg-` | managed Postgres database   | dpg    |
| `red-` | managed key-value store     | red    |
| `cdm-` | custom domain               | cdm    |
| `evt-` | service event (**derived**) | evt    |
| `crr-` | cron-job run (**derived**)  | —      |

Service **instances** are not a top-level Kind. They are compound ids `<service-id>-<opaque>` from `id.ServiceInstanceID` / `id.DeriveServiceInstance` (see [ADR035](ADR035-ssh.md)): the opaque suffix is a name-derived hash so live listing, metrics, and logs agree without exposing Kubernetes pod names. Pre-m87 live SSH selectors hashed the Pod UID; `id.MatchServiceInstance` still accepts those for the same Ready pod.

## Why this shape

- **Render parity.** bex-api is Render-compatible ([ADR006-bex-api.md](ADR006-bex-api.md)); a Render-targeting client expects `srv-…`, `tea-…`. Matching the prefixes is free compatibility.
- **DNS-/k8s-name safe → hyphen, not underscore.** bex ids flow into URLs, hostnames, and Kubernetes object names. A hyphen is legal in a DNS-1123 label; an **underscore is not** (RFC 952/1123 forbid `_` in hostnames, and k8s object names are DNS-1123). This is the decisive reason bex does **not** use Stripe's `tea_…` form — however nice underscores are for double-click-to-select, an underscore id cannot be a hostname or a k8s name. The guard test asserts every minted id is a valid DNS-1123 label, so this can't regress.
- **Typed + greppable.** The prefix says what an id refers to at a glance and in logs (`grep srv-`), and lets an adapter route or reject by kind (`id.KindOf`).
- **xid, not UUID.** [xid](https://github.com/rs/xid) is k-sortable (embeds a timestamp, so ids sort by creation time — friendly to Postgres b-tree locality), non-guessable, URL-safe (lowercase base32-hex, no padding), and 20 chars (vs UUID's 36 with hyphens). A random UUID is none of typed, sortable, or Render-shaped.

## Minted vs derived ids

Most ids are **minted**: `id.New(kind)` draws a fresh xid, and the id is stored alongside the row it names. A few resources have no row to store an id in — they are **projections**, computed at read time from rows the store already holds. The service-events feed is the first ([ADR006-bex-api.md](ADR006-bex-api.md) § Service events): an event is a view over a `deploys` or `audit_events` row, never a table of its own.

For those, `id.Derive(kind, parts…)` mints the id as a **deterministic function of the source row or mechanism identity** — same parts in, same id out, forever — instead of a fresh xid:

```go
id.Derive(id.Event, "dep-c185th5c2rvvnhbfiltg:started") // → evt-… , identical on every read
id.Derive(id.CronRun, "nightly-run-a1b2c3d4")             // → crr-… , identical on every read
```

Determinism is the requirement, not a nicety: a client pages with an event/run cursor, re-fetches it, and dedupes on its id. `id.New` would hand out a different id for the same projection on every request, which is exactly wrong. Cron runs use the backing Job name as the derivation input so the ID survives Job deletion without exposing that mechanism name. The output is 100 bits of SHA-256 in base32-hex, so a derived id is **shape-identical to a minted one** (`WellFormed`/`KindOf` accept it and it is a valid DNS-1123 label) — the distinction is where the entropy comes from, not what the id looks like. `Derive` lives in the same closed registry as `New` and panics the same way on an unregistered kind, so this is a second mint path, not a bypass of the one place ids are made.

## id ≠ name

An **id** and a **name** are different things and must not be conflated:

- **ids** (`tea-…`/`srv-…`/`dpg-…`/`red-…`) are stable, opaque **keys** — the primary key in Postgres and the identifier in API URLs. A rename never changes an id, so references never break ([ADR009-postgresql-management.md](ADR009-postgresql-management.md) §4).
- **names** (a workspace's name, an app's name) are **user-chosen DNS labels** (`[a-z0-9-]`, ≤30 chars) that become part of a CR name (`<workspace>-<app>`). They're human-facing and mutable.

Render's own workspace names are freeform; bex constrains them to DNS labels because they land in CR/host names. That divergence is noted where it's enforced (`internal/workspaces`, `internal/store/api.go`).

## Enforcement (why this won't drift)

Prose rules rot; these don't — the enforcement is layered, compiler first:

1. **The registry is closed at COMPILE TIME.** `id.Kind`'s fields are unexported, so no code outside `internal/id` can fabricate a kind — `id.Kind{prefix: "zz"}` is a compile error ("cannot refer to unexported field"). The only kinds that exist are the package's registered vars (`id.Workspace`, …); a new one is added there and nowhere else. `id.New` additionally panics fail-fast on the one thing a caller _can_ pass — the zero `Kind{}` — rather than minting a malformed id.
2. **One mint path.** `id.New(kind)` is the only way to make an id — nothing concatenates a prefix by hand. The store delegates to it.
3. **Lint forbids the bypass** (`lego/backend/.golangci.yml`, depguard). Only `internal/id` may import `github.com/rs/xid` — the sole source of the random suffix. Import xid anywhere else (i.e. hand-roll an id like `"tea-" + xid.New()...`) and `make lint-backend` fails. This is the layer the compiler and the guard test can't reach: a hand-rolled id at some other call site. Run via `make lint` (both modules) or `make lint-backend` (backend only).
4. **A guard test** (`internal/id/id_test.go`, in the spirit of `TestAuthzGuardsEveryVerb`) machine-checks, for every registered kind: the format, prefix uniqueness, **DNS-1123 label safety**, and that the **underscore form is rejected**. This catches what neither the compiler nor lint can — editing the separator literal in `New` from `-` to `_`: flip it and the suite goes red.
5. **Boundary validation.** `id.WellFormed` / `id.KindOf` let adapters reject a malformed id as a 400 and parse a kind back out, instead of string-slicing prefixes ad hoc. This is the only layer that sees runtime id _values_ (from a client, the DB, another service) — lint and the compiler see code, not data.

## Known deviations (deliberate, documented)

- ~~Legacy Postgres and Key Value ids are grandfathered~~ — **closed by w1/m56 t009 (2026-07-28)**: production and maintained dev reported zero non-`dpg-`/`red-` ids and zero missing display names. Their CRDs now require `spec.name`, readers no longer substitute `metadata.name`, and the one-time backfill scripts are gone. A restored pre-normalization snapshot must be upgraded through the preceding release before the current CRD/code is applied.
- **API keys carry Hydra's `client_id`, not a bex id.** OAuth2 clients are minted by Ory Hydra ([ADR012-auth.md](ADR012-auth.md)); their id format is Hydra's, outside this convention by design.
- ~~GraphQL `Service.id` returns the App name~~ — **closed by w1/m46 (2026-07-16)**: GraphQL now emits the minted `srv-…` id like REST/MCP (`internal/apps/graphql.go`), and dashboard URLs follow (`/services/srv-…`). No verb-layer change was needed — `core.Base.AuthorizeApp`/`GetApp` already resolve `LabelAppID` first with a `LabelServiceName` fallback, so name-shaped args and pre-flip URLs keep resolving. Legacy hand-applied CRs without `LabelAppID` keep their name as id (`publicID` fallback).
- **Blueprint ids are `blp-…`, where Render's are `exs-…`** (and blueprint-sync runs are `bsr-…` to Render's `exe-…`). Minted with bex's own mnemonic in `w2/m15`, before the Render OpenAPI compatibility gate existed, so this was never a considered divergence — it was an unrecorded gap, and it had a live cost. `internal/api/render_openapi.go` validates every `/v1/*` request against the embedded copy of Render's spec, which pins `blueprintId` to `^exs-[0-9a-z]{20}$`; no bex id has ever matched that and none can, so from the day the gate shipped until **w6/m96 (2026-08-26)** every REST call to `GET`/`PATCH`/`DELETE /v1/blueprints/{id}` and `GET /v1/blueprints/{id}/syncs` was rejected `400 invalid path parameter "blueprintId"` before authz or lookup ran. The prefix is **not** changing — ids are durable and the stored ones are `blp-`. Instead `renderPathParameterPatternCompatibility` widens that one parameter to `^(?:blp|exs)-[0-9a-z]{20}$`, alongside the file's three existing compatibility maps; a Render-shaped `exs-` id therefore still passes the gate and gets an honest 404 rather than a syntax error about Render's own spelling. `TestEveryConstrainedIDPathParameterAcceptsItsBexID` now checks every pattern-constrained id path parameter on a bex-served route against the prefix that addresses it, so this class of mismatch fails CI instead of shipping silently — the other four in the intersected surface (`dsk-`, `whk-`, `evt-`, `job-`) all match Render's patterns today and are locked in by that test.

  The sync id (`bsr-` vs Render's `exe-`) is **unverified rather than confirmed safe**: no path parameter in the currently-embedded spec is constrained to a sync id — sync runs appear only in response bodies, never as an addressable path segment — so the same gate cannot currently reject one. If a future spec refresh introduces such a parameter on a route bex serves, the guard test above fails and this entry is where to record the outcome.

- **REST and GraphQL `Domain.id` return the hostname, not the minted `cdm-…` id.** Deliberate: domains are addressed by hostname in most Render flows (DNS instructions, the custom domain itself), making the hostname the honest identity contract. Both REST (`internal/apps/domains.go::toRenderCustomDomain`) and GraphQL (`internal/apps/graphql.go::customDomainGQLType`) emit `d.Name` consistently — no surface divergence.

## Alternatives considered

- **Underscore (`tea_…`, Stripe/GitHub style).** Rejected: illegal in hostnames and k8s names, and off-parity with Render. Its one real win (double-click-to-select) doesn't outweigh losing DNS-safety.
- **Bare UUIDs.** Rejected: untyped, not k-sortable, longer, not Render-shaped.
- **Sequential integer ids.** Rejected: guessable/enumerable (an information leak in URLs) and leaks row counts.
