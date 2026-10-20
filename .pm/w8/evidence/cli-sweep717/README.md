# CLI sweep 717 — env-var total-size cap (live, 2026-10-19)

Disposable free web service `qa717-web` (`srv-dbbc1blfsg2s73ako9tg`). Deleted and verified absent (REST 404).

Single-key `PUT /env-vars` with value length N:

| N | Result |
| --- | --- |
| 4096 / 65536 / 262144 | 200 |
| 500000 / 520000 / 524000 / 524200 | 200 |
| 1000000 | 400 `total environment variable size limit of 524288 bytes exceeded` |

The 512 KiB total cap (same family as the secret-file cap bisected in sweep 632) is enforced with a named refusal; individual values have no smaller per-value cap below it.

No bug filed.
