# CLI sweep 713 — CLI service-create validation (live, 2026-10-19)

Refusal-only; nothing created.

| Probe | Result |
| --- | --- |
| `--env-var NOEQUALS` | client `invalid --env-var "NOEQUALS": expected KEY=VALUE` |
| `--env-var 1BAD=x` | server 400 `ENVIRONMENT_VARIABLE_INVALID` (same coded refusal as sweep 647) |
| `--name hello-go` (baseline's name) | 409 `CONFLICT: name "hello-go" is already in use` |
| `--name 'Bad Name!'` | 400 `RESOURCE_NAME_INVALID: name must use lowercase letters, digits, and hyphens, be at most 30 characters, and not start or end with a hyphen` |

Create-time names are slug-strict (contrast sweep 653: rename only edits the free-form display label). No bug filed.
