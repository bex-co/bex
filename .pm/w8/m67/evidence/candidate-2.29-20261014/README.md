# Candidate Bex on Render CLI v2.29.0 — local qualification, 2026-10-14

A candidate `bex` built from the current checkout, with the pin moved to `github.com/render-oss/cli@af9bdd180e90` (tag v2.29.0, Go 1.27.2), ran the unmodified sweep272 fixtures. Each fixture used only its client list, narrowed to this one binary. Everything ran against loopback servers with synthetic config and token; there was no production traffic. Afterwards the pin bump was reverted: nothing about it is committed.

- `go build ./...` and `go test ./...` in `lego/cli` passed on the candidate pin.
- [Recovery fixture](sweep272-tail-recovery-results.json): in all three faults (abrupt close before the first line, abrupt close after A, 1011 after A) the client delivered A then B exactly once and stayed alive until the harness SIGINT. Its stderr reported `Unable to connect to logs. Retrying in 1s.` and then `Log connection restored.`
- [Close fixture](sweep272-tail-close-results.json): all five scenarios hit the 8-second harness timeout with no native exit, after four connection attempts. That includes **`normal1000_after_line`** and **`handshake503`**. The delivered line was printed once, and stderr showed backoff of 1s, 2s, 4s and then 8s.

## Blocking lifecycle finding

Upstream v2.29 `isRetryable` (`pkg/logs/tail.go`) retries close codes 1000, 1001, 1006, 1011, 1012 and 1013, plus handshake 429/5xx and network EOFs. Any other close code becomes a terminal `Err`, which `cmd/logs.go` returns as a non-zero exit. No server signal produces a clean exit.

Bex ends a build tail that has no running build with one `==>` refusal line followed by a **1000** close (`lego/backend/internal/logs/rest.go`, `subscribeWebSocket`; w8/done/020). That covers both the queued-past-wait and the finished-build cases. Under v2.29 that tail would reconnect forever with a growing backoff, capped at 30s, instead of exiting. Each reconnect would print a fresh refusal line, since every reconnect gets a new timestamp. A logs-unavailable refusal behaves the same way.

Adopting the pin therefore needs a decision on what a terminal server-side end of a tail should look like:

- **Option A: non-retryable close.** Keep the 1000 close for the pinned 2.27 clients, and send a non-retryable application close code (for example 4000+) for named refusals. v2.29 then exits non-zero and prints the reason. A finished build tail exits as an error.
- **Option B: accept the retry policy.** Accept indefinite retry for build tails under v2.29 and document it as a limitation.
- **Option C: ask upstream.** Ask upstream for a "stream complete" signal before adopting the pin.
