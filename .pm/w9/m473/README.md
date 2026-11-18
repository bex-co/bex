# w9 · m473 — A private service reads Render's `serviceDetails.url` and an image service Render's docker-shaped `envSpecificDetails`, as Render's recorded responses show

**Worker:** worker9 **Goal:** three of w10/m373's open "needs a Render capture" entries are settled from Render's own recorded responses and conformed: private service `url`, image service `envSpecificDetails`, Postgres `project` **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | A private service's REST read carries Render's `serviceDetails.url`, `<slug>:<port>` | 40m | — |
| [t002](t002.md) | An image service's `envSpecificDetails` is Render's docker shape: empty `dockerContext`/`dockerfilePath`, `dockerCommand`, `preDeployCommand` | 1h | — |
| [t003](t003.md) | Postgres `project`: record that Render omits it too, with the recordings as evidence | 20m | — |
| [t004](t004.md) | Record the partial unblock on w10/m373 and rewrite ADR018's Partial serviceDetails projection row | 30m | t001, t002, t003 |
| [t005](t005.md) | Render parity | 30m | t004 |
| [t006](t006.md) | Simplify | 20m | t005 |
| [t007](t007.md) | Test coverage | 30m | t005 |
| [t008](t008.md) | Closeout | 15m | t006, t007 |

## Definition of done

- `GET /v1/services/{id}` for a private service carries `serviceDetails.url` = `<slug>:<port>`; for an image service, `envSpecificDetails` carries `dockerCommand`, `dockerContext`, `dockerfilePath` (empty strings when unset) and `preDeployCommand` when set; the pinned CLI still clones image services and image crons.
- `rendertest.Allowlist` no longer excuses private `url` or image `envSpecificDetails`; its Postgres `project` entries cite Render's own omission instead of an open question.
- w10/m373 records the dated partial unblock and waits only on build-source-linked `runtime`/`env`, Postgres `version` and its production gate.

## Source + Goal linkage

- **Source:** `/pm-brainstorm more for w9, preferrably one aspect of render parity` (2026-11-17), item 2, filed by `/pmx 1,2,3 to w9`. This milestone is **carved out of [w10/m373](../../w10/blocked/m373/README.md)**, not a duplicate: m373's gate (1) asks for "a Render capture of a private service, an image service and a build-source-linked service, plus a Postgres read" (w10/002's API-key gate). Render's own Terraform provider (`render-oss/terraform-provider-render` tip `06af456`) replays 46 recordings of Render's real API in its test suite; they settle three of m373's entries (each task cites its recordings).
- **Goal linkage:** [ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); ADR018's "Partial serviceDetails projection" row.
- **Expected outcome:** clients generated from Render's schema, the Terraform provider included, read a private service's address and an image service's details as on Render.
- **Why now:** the evidence is in hand, so these entries no longer wait on a Render API key.
- **Decision state:** treating these recordings as the Render capture m373 asks for is the proposal's **recommendation**, not a user decision. They come from Render's staging/testing hosts, not production, but Render's own test suite still asserts against them, and for private `url` and the image shape the pinned schema agrees. The remaining m373 entries keep their open sign-off.
- **Render parity task:** included. The milestone changes REST reads; t005 checks GraphQL, MCP and the dashboard's Connect panel.
