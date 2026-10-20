# CLI sweep 704 — service port range (live, 2026-10-19)

Disposable free web service `qa704-web` (`srv-dbbbu1ivfpec73dm0l20`). Deleted and verified absent (REST 404).

`PATCH {"serviceDetails":{"port":N}}`:

| Port | Result |
| --- | --- |
| 0, -1, 65536, 99999 | 400 `port must be 1024-65535 — a tenant container has no NET_BIND_SERVICE capability, so it cannot bind a port below 1024 even as root …` |
| 8080 | 200; readback `"port":8080` |
| `"abc"` (wrong type) | 400 `request body does not match the Render API schema` |

The sub-1024 floor is a bex platform constraint, and the refusal explains it instead of letting the pod fail to bind later. No bug filed.
