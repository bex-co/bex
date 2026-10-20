# CLI sweep 665 — update enums, repo/branch validation, `restart` dialects (live, 2026-10-19)

Disposable free web service `qa665-web` (`srv-dbbbgctfsg2s73ako7t0`). Deleted and verified absent (REST 404).

`services update` validation (all named):

| Probe | Result |
| --- | --- |
| `--previews bogus` | client enum (`automatic`, `manual`, `off`) |
| `--runtime bogus` | client enum (docker, elixir, go, image, node, python, ruby, rust) |
| `--branch 'bad branch!'` | server 400 `invalid branch` |
| `--repo not-a-url` | server 400 `repo must be an https URL, or a GitHub SSH URL (…)` |

`bex restart` (documented as by-resource-ID):

| Target | Result |
| --- | --- |
| service name `qa665-web` | `unknown resource type` (names not resolved; cryptic but ID-only per help) |
| real `srv-…` id | `"… restarted successfully"` |
| synthetic `srv-…` id | 404 `NOT_FOUND` |
| synthetic `red-…` id | `key / value stores cannot be restarted` |
| synthetic `dpg-0000000000000000000000` | **`"… restarted successfully"` despite the 404** — exact re-confirmation of the already-tracked blocked finding [w8/blocked/084](../../blocked/084.md) (upstream Render CLI swallows the Postgres restart 404). Not refiled. |

No new bug filed.
