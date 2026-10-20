# CLI sweep 762 — service `urls` at creation time (live, 2026-10-19)

Disposable free web service `qa762-web` (`srv-dbbcnoq720pc73c0kmlg`). Deleted and verified absent (REST 404).

- Immediately after create, the service object has **no `urls` key yet** (it appears once the first deploy assigns the ingress — sweep 641's readback had it). Adding a pending custom domain does not create one either; the domain lists separately with all three statuses `pending`.

Recorded as lifecycle timing, not a bug. No bug filed.
