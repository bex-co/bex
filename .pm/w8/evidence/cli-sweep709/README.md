# CLI sweep 709 — operations against a suspended service (live, 2026-10-19)

Disposable free web service `qa709-web` (`srv-dbbbv9ivfpec73dm0lbg`), suspended mid-creation. Deleted while suspended and verified absent (REST 404).

While `suspended`:

| Verb | Result |
| --- | --- |
| `PUT env-vars` | 200 accepted (held for resume — same held-change semantics as sweep 590/w4 m234) |
| `POST deploys` | 409 `conflict: service "…" is suspended` |
| `POST restart` | 409 same |
| `POST scale {"numInstances":1}` | 202 accepted (config plane) |
| `PUT autoscaling` (disabled body) | 200 |
| `DELETE service` | succeeds |

The suspend also canceled the still-`created` initial deploy (deploy list showed it `canceled`). Runtime verbs refuse with a named conflict; configuration verbs apply. No bug filed.
