# CLI sweep 1239 — deploy-hook contract against Render's (live, 2026-10-20)

One free image-backed web service (`qa-20261020-e13edc-hook`, `srv-dbbh88ecbnls73997chg`, `gcr.io/google-samples/hello-app:1.0`) created with `bex services create`, then deleted (GET 404). The hook URL came from `GET /v1/services/{id}/deploy-hook`. It was handled privately, never recorded, and its local copy was removed.

- GET and POST → 200 `{"deploy":{"id":"dep-…"}}`. PUT and DELETE → 405. A same-repository `imgURL=…:2.0` deployed 2.0 while `imagePath` stayed `:1.0`. Rapid triggers superseded each other (`Superseded by dep-…`), and the newest went live.
- A different-repository `imgURL` → 400 (Render's same-image rule). A malformed `imgURL` → 400. `ref` on an image service → 400. A suspended service → 409. These match Render's documented codes.
- A wrong or missing `key` → 404, and a burst → 429 with `Retry-After`. Both are recorded drift/protection in ADR018's “Secret deploy hook” row (uniform 404; 6/min, burst 2).
- **Finding → [w8/141](../../141.md):** those 400s name `commitId`/`imageUrl` (the REST body fields) instead of the hook's `ref`/`imgURL`.
- Minor doc nit (not filed): ADR018 describes the open endpoint as `/v1/deploy-hooks/{token}`, while the URL Bex hands out is `/v1/deploy-hooks?key=…`.
