# CLI sweep 1295 — distinct free-form names that share a slug (live, 2026-10-21)

Production `https://api.bex.co/v1/`, pin `f64d4c85f2cd`, bex-canary, HEAD-built pinned CLI. `bex services create --type web_service --plan free --image docker.io/ealen/echo-server:0.9.2 --name <n>`:

| `--name` | Result |
| --- | --- |
| `QA Dup c9900c` | 201, `slug: qa-dup-c9900c` |
| `qa dup c9900c` (case only) | 409 `name "qa-dup-c9900c" is already in use` |
| `QA_Dup_c9900c` (underscores) | 409, same |
| `QA Dup c9900c` (exact repeat) | 409, same |

Selecting by the exact free-form name (`services instances "QA Dup c9900c"`) works.

Not filed as new. The mechanism (per-workspace `(tenant_id, name)` uniqueness on the derived slug, and a 409 that names the slug rather than the typed name) is w4/385. That item's fix covers only the empty-slug fallback (`resource`), so these case- and punctuation-variant collisions remain once it ships. Record this extension there when w4/385 is picked up. The fixture `srv-dbcevpf4vasc73dpoo5g` was deleted (204 → 404).
