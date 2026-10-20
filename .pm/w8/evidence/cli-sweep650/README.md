# CLI sweep 650 — service restart endpoint (live, 2026-10-19)

Disposable free web service `qa650-web` (`srv-dbbbbp5fsg2s73ako7e0`). Deleted and verified absent (REST 404).

- Service reached HTTP 200. `POST /services/{id}/restart` with no body → 200 with the service object.
- Same call with a `{}` body → 400 `request body is not allowed for this operation` (consistent with sweep 648 suspend/resume).
- Public URL answered 200 on the first probe after restart; instance list showed a live instance and the latest deploy `live`.

No bug filed.
