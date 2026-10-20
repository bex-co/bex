# CLI sweep 646 — static-site creation dialect (live, 2026-10-19)

No fixture created.

Via the pinned CLI (`services create --type static_site`, no `--plan`):

| Probe | Result |
| --- | --- |
| with `--image …` | 400 `a static_site builds from a Git repo; a prebuilt image is not supported` |
| neither repo nor image | client-side: `either repo or image is required` |

The repo-only constraint for static sites is enforced server-side with a named refusal; the missing-source case is caught client-side. No bug filed.
