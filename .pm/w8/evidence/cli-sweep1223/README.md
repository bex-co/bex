# CLI sweep 1223 — webhook URLs must be absolute HTTPS; SSRF is blocked at dial time (live, 2026-10-20)

Disposable webhooks created and deleted (204, verified absent). No new bug filed.

- `http://`, `ftp://`, hostless `https://`, and non-URL values → 400 with code `WEBHOOK_URL_INVALID` (`url must be an absolute HTTPS URL`, per [service.go](/Volumes/nvme4tbfish/projects/bex8/lego/backend/internal/webhooks/service.go:845)).
- `https://127.0.0.1/hook` → 201 at create. That is not an SSRF hole: the delivery worker blocks loopback/private/link-local destinations at dial time (per the worker's guard and its tests in `lego/backend/internal/webhooks/worker.go`), so create validates the URL's shape and delivery enforces reachability policy — defense in two layers. Recorded with the source evidence; not filed.
- Webhook list rejects `name` as an unsupported query parameter (400 naming both offending parameters).
