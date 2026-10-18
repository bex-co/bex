# CLI sweep 488 — env-group link surface (API-only upstream) and strict body discipline

**Verdict: no bug filed.** One free web service and one env group were created, linked, unlinked, and deleted by ID; both final GETs returned HTTP `404` and the baseline was restored modulo the concurrent hunt's churn.

## Observations

| probe | observed result | assessment |
| --- | --- | --- |
| `bex env-groups --help` | `unknown command` | The pinned Render CLI has no env-group subcommand; env groups are an API-only surface upstream. Harness guess, not a defect. |
| Link via `POST /v1/env-groups/{id}/services/{serviceId}` with `{}` body | HTTP `400` "`request body is not allowed for this operation`" | Correct strictness — the operation is bodyless by design (handler at [`rest.go:187`](../../../../lego/backend/internal/envgroups/rest.go)). |
| Same link, bodyless | HTTP `200`, `serviceLinks` populated, revision (`egr1_…`) minted | Matches the documented Render contract (link returns the updated env group; w8/063 note in the handler comment). |
| `GET /v1/services/{id}/env-vars` after link | `[]` | Service-own vars only; inherited group vars are not flattened into this list — Render-consistent. |
| Service wire after link (list row) | no `linkedEnvGroups` field | Render carries links on the group side (`serviceLinks`); the CLI service view matching that is consistent, not a gap. (Correction from sweep 490: `bex services get <id>` is not a real subcommand upstream — cobra falls through to the list; the row read was the fresh service at the head of that list.) |

## Safety and unrun checks

- No plan mutation; only exact owned IDs were mutated/deleted; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.
