# CLI sweep 501 — request-log filter dialects (`--status-code`/`--method`/`--limit`)

**Verdict: no bug filed.** Passing functional coverage. Isolated human device login (QA account) into `bex-canary` (`tea-…s3g`) at `https://api.bex.co/v1/`, 2026-10-19T08:49–08:55Z. Installed `/opt/homebrew/bin/bex` v0.2.1 for device `login` (browser-approved; its post-login Render-release banner is the known stale-install artifact fixed in bex-cli v0.3.0, not a filing); a HEAD checkout build (`465c28bbc`, self-identifies `vdev`, Render pin v2.27.0 `a764810a7682`) for all journeys. Deployed production revision not observable.

Setup note: `scripts/qa-login.sh --login-challenge` answered `HTTP 400 … /self-service/login/browser` twice against this run's fresh seeded jar — the already-filed [w8/124](../../124.md) failure, reproduced once more (no new filing). The browser-approval path (`--serve` + dashboard device page) completed the same device code.

## Journey

One free image web service (`docker.io/ealen/echo-server:0.9.2`) created → `live` (~30 s) → five planted requests (`GET 200 ×2`, `POST 200`, `GET 404` and `POST 201` via `?echo_code=`) verified over HTTPS → `bex logs -r <id> --type request -o json` filter matrix → deleted by ID.

| step | observed result |
| --- | --- |
| `--limit 50` | Exactly the 5 planted rows, newest first; method/statusCode labels truthful. |
| `--status-code 404` / `200` / `2xx` | 1 / 3 / 4 rows — exact sets, including the `2xx` class form. |
| `--method POST` and lowercase `post` | Both return exactly the 2 POSTs. |
| `--limit 1` | Only the newest row. |
| `--limit -1`, `--limit 100000` | Exit `1`, server `400`: `limit must be an integer between 1 and 100` — Render's own spec bounds (`limit` minimum 1, maximum 100, default 20 in the vendored Render OpenAPI). |
| `--limit abc`, `--method BOGUS`, `--level boguslevel` | Exit `1` client-side with named flag errors before any request. |
| `--status-code abc` / `99` / `600` | Exit `0`, honestly empty. Correct per contract: Render's spec types `statusCode` as an array of free strings (no pattern/enum), so an unmatched value is a valid filter that matches nothing. |
| `--limit 0` | Exit `0`, full page. This is upstream client behavior, not a server dialect: the pinned CLI maps `Limit == 0` to its default (`pkg/tui/views/logloader.go:136-137` → `DefaultLogLimit = 100`) before sending, so the same command behaves identically against Render. |
| First queries within ~20 s of planting | Two filtered queries briefly returned empty while an unfiltered query already showed rows; after ingestion settled, every filter returned its exact set. Recorded as ingestion lag, not a finding (not reproducible once settled). |
| Log JSON shape | `-o json` emits a stream of pretty-printed objects (JSONL-style), the pinned upstream rendering. |

## Safety and unrun checks

- Cleanup: exact-owned `srv-dbatjbpkv6rc73en2ch0` deleted by ID; absence verified by `deploys list` 404, former-URL 404, and a baseline diff showing all nine baseline rows intact (the one extra service is a concurrent hunt's fixture, observed read-only). No plan mutations; secrets unrecorded.
- The isolated CLI session and this run's Kratos jars remain active for subsequent sweeps in this loop; revocation happens when the user stops the loop.
- Unrun: `--instance`/`--task-id` filters, app-log level filtering on a native build, combined multi-filter intersections.
- Deploy lag means deployed revisions can precede local `main`.
