# CLI sweep 702 — image update staging, then deploy (live, 2026-10-19)

Disposable free web service `qa702-web` (`srv-dbbbs3dfsg2s73ako9dg`), live at 200. Deleted and verified absent (REST 404).

- `bex services update --image gcr.io/google-samples/hello-app:9.9.9-nonexistent` → 200; `imagePath` read back as the new (bad) ref, but **no deploy was minted** — the deploy list still held only the original `live` deploy on the good image. This is the documented staging contract (ADR032: setImage stages; the next deploy applies it).
- A subsequent explicit `POST /services/{id}/deploys {}` picked up the staged ref, ran `update_in_progress` for ~2 minutes, and ended `update_failed`; the public URL served 200 from the old version throughout.

No bug filed.
