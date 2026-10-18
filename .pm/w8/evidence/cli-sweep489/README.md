# CLI sweep 489 — projects and environments from the CLI

**Verdict: no bug filed.** One project, one environment, and one free web service were created, linked, unlinked by deletion, and removed by ID; all three final GETs returned HTTP `404` and the baseline was restored modulo the concurrent hunt's churn.

## Journey

| step | observed result | assessment |
| --- | --- | --- |
| `bex projects -o json` after creating a project via API | lists it with owner (`bex-canary`, `tea-…`), `environmentIds: []` | Expected workspace-scoped round trip. |
| `bex environments <project>` on an empty project | literal `null` | Same null-vs-`[]` wire class closed in earlier sweeps as Render-consistent; noted, not filed. |
| Guessed route `POST /v1/projects/{id}/environments` | HTTP `404` | Not the contract; environments are created at `POST /v1/environments` with `projectId` ([`environments/rest.go:180`](../../../../lego/backend/internal/environments/rest.go)). Harness error. |
| Guessed `PATCH /v1/services/{id}` with `environmentId` | no attachment | Attachment is owned by the environment via `PUT /v1/environments/{id}/service-links` whole-replacement — Render-consistent. Harness error. |
| Correct create + link | `bex environments` lists the env with default `0.0.0.0/0`/`::/0` IP allow list, `protectedStatus: unprotected`; the service row in `bex services -o json` reports the `environmentId` | Full CLI-visible round trip. (Correction from sweep 490: `bex services get <id>` is not a real subcommand upstream — cobra falls through to the list; the row read here was the fresh service at the head of that list.) |

## Safety and unrun checks

- No plan mutation; only exact owned IDs were mutated/deleted; secrets unrecorded.
- Deploy lag means deployed revisions can precede local `main`.
