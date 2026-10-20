# CLI sweep 653 — service rename semantics (live, 2026-10-19)

Disposable free web service `qa653-web` (`srv-dbbbcodfsg2s73ako7hg`). Deleted and verified absent (REST 404).

Via the pinned CLI (`services update <id> --name …`):

| Probe | Result |
| --- | --- |
| rename to `hello-go` (existing baseline name) | 409 `CONFLICT: name "hello-go" is already in use` |
| rename to `bad name!` | 200 accepted as display label |
| rename to `qa653-renamed` | 200; `name=qa653-renamed`, `slug`/`immutableName` stay `qa653-web` |

Routing follows the immutable slug: `qa653-web.onbex.co` kept returning 200 after both renames; `qa653-renamed.onbex.co` returned 404. This matches the sweep-583 classification: rename edits the display label only, uniqueness is enforced with a named 409, and punctuation in a display label is not a routing or slug hazard. No bug filed.
