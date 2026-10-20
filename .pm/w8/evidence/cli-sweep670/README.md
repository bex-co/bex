# CLI sweep 670 — persistent-disk dialect and plan gate (live, 2026-10-19)

Disposable free web service `qa670-web` (`srv-dbbbhutfsg2s73ako8ag`). Deleted and verified absent (REST 404); `GET /disks` → `[]` throughout.

- Dialect: disks are top-level. `GET|POST /services/{id}/disks` → 404; `GET /disks` and `GET /disks?serviceId=…` → 200 `[]` (matches Render's flat `/disks`).
- `POST /disks {"serviceId","name":"qa670-disk","mountPath":"/data","sizeGB":1}` on the free plan → 400 `disks require a paid instance type`. The same gate answers before field validation (relative `mountPath` and `sizeGB:0` got the identical gate refusal). No paid plan was submitted.
- Synthetic `serviceId` → 404 `NOT_FOUND`.

No bug filed.
