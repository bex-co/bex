# m115 hosted acceptance (independent) + dup-env dialect — sweep475

**Headline: the shipped w8/m115 fix is live and independently verified.** Upstream completed m115 (moved to `done/` before this sweep; the sync conflict in the workstream README was resolved keeping upstream's done-line and this loop's m116 line). Independent acceptance on a fresh owned free web service: restrict `--ip-allow-list` to a non-client `/32`, two curls at 07:45:30Z answer 403, and one minute later `bex logs --type request --status-code 403` and `--path /m115recheck475` each return both refusal lines, with status discovery offering `["403"]` — every element of m115's definition of done, on a fixture the implementing worker never touched.

Passing coverage otherwise: duplicate `--env-var QA475DUP=first/second` at create resolves last-wins (single wire entry, `second`); a malformed saved-image ref is refused with the parser's exact reason (400, lowercase rule); `--previews` accepts only the client's closed `automatic|manual|off` enum (a `generation` guess refused client-side, no mutation).

Cleanup: exact-owned `srv-dba7hneonb0s73deel8g` deleted by ID (404-verified); baseline deltas are concurrent foreign churn, observed read-only. No plan mutations. Session remains active; next inline sweep is 476; m116 + evidence 465–475 remain uncommitted pending `/ship`.
