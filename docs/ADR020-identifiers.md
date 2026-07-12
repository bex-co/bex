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

| Prefix | Resource           | Render |
| ------ | ------------------ | ------ |
| `tea-` | workspace (tenant) | teams  |
| `srv-` | service (app)      | srv    |
| `cdm-` | custom domain      | cdm    |

## Why this shape

- **Render parity.** bex-api is Render-compatible ([ADR006-bex-api.md](ADR006-bex-api.md)); a Render-targeting client expects `srv-…`, `tea-…`. Matching the prefixes is free compatibility.
- **DNS-/k8s-name safe → hyphen, not underscore.** bex ids flow into URLs, hostnames, and Kubernetes object names. A hyphen is legal in a DNS-1123 label; an **underscore is not** (RFC 952/1123 forbid `_` in hostnames, and k8s object names are DNS-1123). This is the decisive reason bex does **not** use Stripe's `tea_…` form — however nice underscores are for double-click-to-select, an underscore id cannot be a hostname or a k8s name. The guard test asserts every minted id is a valid DNS-1123 label, so this can't regress.
- **Typed + greppable.** The prefix says what an id refers to at a glance and in logs (`grep srv-`), and lets an adapter route or reject by kind (`id.KindOf`).
- **xid, not UUID.** [xid](https://github.com/rs/xid) is k-sortable (embeds a timestamp, so ids sort by creation time — friendly to Postgres b-tree locality), non-guessable, URL-safe (lowercase base32-hex, no padding), and 20 chars (vs UUID's 36 with hyphens). A random UUID is none of typed, sortable, or Render-shaped.

## id ≠ name

An **id** and a **name** are different things and must not be conflated:

- **ids** (`tea-…`/`srv-…`) are stable, opaque **keys** — the primary key in Postgres and the identifier in API URLs. A rename never changes an id, so references never break ([ADR009-postgresql-management.md](ADR009-postgresql-management.md) §4).
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

- **Managed datastores use the name as the id, not `dpg-<xid>`/`red-<xid>`.** `internal/postgres` exposes a `Database`'s user-chosen name as its id, and `internal/keyvalue` does the same for a `KeyValue` (both CRs are name-keyed). Render uses `dpg-…` / `red-…`. This is a conscious deviation — bex datastores are named CRs with no separate opaque key — recorded here rather than silently diverging, and applied uniformly so the two sibling datastore surfaces stay consistent (minting a `red-` id for key-value alone would split them). Revisit if datastores ever need rename-stable references.
- **API keys carry Hydra's `client_id`, not a bex id.** OAuth2 clients are minted by Ory Hydra ([ADR012-auth.md](ADR012-auth.md)); their id format is Hydra's, outside this convention by design.

## Alternatives considered

- **Underscore (`tea_…`, Stripe/GitHub style).** Rejected: illegal in hostnames and k8s names, and off-parity with Render. Its one real win (double-click-to-select) doesn't outweigh losing DNS-safety.
- **Bare UUIDs.** Rejected: untyped, not k-sortable, longer, not Render-shaped.
- **Sequential integer ids.** Rejected: guessable/enumerable (an information leak in URLs) and leaks row counts.
