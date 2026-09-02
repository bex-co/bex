# ADR076: Security review round 16 — J1JBjc disposition

- **Status**: Accepted (2026-08-19)
- **Scan**: codex-security `J1JBjc`, repository revision `7456c4c7` (2026-08-19), 13 findings (1 high, 7 medium, 5 low)
- **Lineage**: sixteenth pass in the ADR028 → ADR045 → ADR055 → ADR056 → ADR057 → ADR072 → ADR061 → ADR063 → ADR064 → ADR066 → ADR067 → ADR068 → ADR069 → ADR073 lineage

## Summary

Twelve findings are fixed in place with regression tests; one is a standing accepted residual (onbex.co PSL). Finding 9's overwrite impact and finding 11's export/`apk add age` surfaces were closed in a follow-up pass; the URL remaining in argv and the wider ADR060 §D7 inventory stay documented.

| # | Finding | Severity | Disposition |
| --- | --- | --- | --- |
| 1 | Unsigned installer checksums | high | **Fixed** — `install-bex.sh` verifies `checksums.txt.sigstore.json` with the same Fulcio workflow identity as `bex upgrade` |
| 2 | Rollback bypasses billing gate | medium | **Fixed** — `RequireBillingMutation` before any rollback write |
| 3 | Concurrent last-admin demotion/removal | medium | **Fixed** — tenant advisory lock + in-transaction admin count on role update/remove |
| 4 | Stale platform-client mint cache | medium | **Fixed** — `IsPlatformClientFresh` at `AuthorizeMintClass` |
| 5 | Contributor rollback selects images | medium | **Fixed** — rollback gates on `can_create`; added to the m68 executable-selection class |
| 6 | Web Push SSRF | medium | **Fixed** — `SafeDialContext`, no proxy, no redirects; HTTPS-only registration |
| 7 | Demoted contributor sandbox exec | medium | **Fixed** — caller-selected sandbox commands require `can_create` |
| 8 | Cross-site invite auto-redemption | medium | **Fixed** — dashboard requires explicit Accept; navigation alone creates no membership |
| 9 | Snapshot PUT in argv | low | **Fixed (partial)** — create-once `If-None-Match:*` on the presigned PUT + matching hibernate `curl` header; URL still in argv until a broker/helper |
| 10 | Shared onbex.co tenant suffix | low | **Accepted residual** — onbex.co PSL (eleventh report); `.pm/DO_NOT_DO.md` `#PSL` forbids in-repo "remediation" |
| 11 | Mutable datastore export tooling | low | **Fixed (partial)** — CNPG export images pinned per CRD enum; KeyValue encrypt uses the pinned FiloSottile/age release (no `apk add`); wider inventory remains ADR060 §D7 |
| 12 | Unbounded git webhook replay ledger | low | **Superseded fix** — migration `0104`: post-match claims, exact per-workspace cap, signing-epoch leases, safe retired-epoch purge |
| 13 | API-key revoke ignores UnbindKey | low | **Fixed** — unbind before Hydra delete; fail closed on unbind errors |

## Finding 1 (high) — installer authenticates checksums

`scripts/install-bex.sh` fetched `checksums.txt` and the archive from the same unauthenticated origin and treated SHA-256 equality as authenticity. The Go updater already verified the Sigstore bundle.

**Fix.** Download `checksums.txt.sigstore.json` and run `cosign verify-blob` with the pinned Fulcio issuer and `cli-release.yml@refs/tags/bex-cli/v*` identity before parsing checksums. Missing `cosign` or a failed verification aborts before install. Documented in `docs/bex-cli.md`.

## Finding 2 (medium) — rollback shares Trigger's billing gate

`Trigger` called `RequireBillingMutation`; `Rollback` did not, so a workspace in enforcing/enforced/recovering could still change the running image while the App was not yet suspended.

**Fix.** Call `RequireBillingMutation` after authorization and before any store or Kubernetes write.

## Finding 3 (medium) — last-admin invariant is transactional

`guardLastAdmin` counted admins in a standalone query; `UpdateMemberRole` / `RemoveMember` mutated later. Two concurrent demotions of a two-admin workspace could both pass and leave zero admins.

**Fix.** Both mutations take `pg_advisory_xact_lock(hashtext(tenant_id))`, re-read the member `FOR UPDATE`, re-count admins inside the transaction, and refuse with `store.ErrLastAdmin` when the change would leave zero. Service-level `guardLastAdmin` remains as a fast path.

## Finding 4 (medium) — mint class re-reads Hydra

`AuthorizeMintClass` trusted the 30s `platformClients` positive cache. A recently declassified bearer could mint an API key, SSH key, or deploy-hook credential.

**Fix.** `PlatformClientResolver.IsPlatformClientFresh` always hits Hydra; `AuthorizeMintClass` uses only that path. Cached `IsPlatformClient` remains for audience/scope classification.

Tests: `TestCreateAPIKeyUsesFreshPlatformClientClassification`.

## Finding 5 (medium) — rollback is executable selection

`Rollback` authorized `can_operate` then wrote a caller-chosen prior `ResolvedImage` into `App.spec.image`. Contributors hold `can_operate` but not `can_create`.

**Fix.** Gate on `can_create`. Add "deploy rollback" to `execSelectionCases` and pin `*deploys.Service.Rollback` in `representativeVerbRelations`.

## Finding 6 (medium) — Web Push uses the safe dialer

Registration accepted every HTTPS host and HTTP loopback; delivery used the default `http.Client` (ambient proxy, redirects).

**Fix.** Default Web Push client clones the transport with `netutil.SafeDialContext`, `Proxy: nil`, and `CheckRedirect → ErrUseLastResponse` (same as outbound webhooks). Registration is HTTPS-only.

## Finding 7 (medium) — sandbox exec is create-like

`dialGateway` authorized `can_operate` then minted a ticket wrapping a caller-selected `/bin/sh -c` command. A demoted contributor who still owned a sandbox kept choosing commands.

**Fix.** Require `can_create` for the public exec verb; keep the agent-session `can_view_sensitive` second gate. Suspend/resume stay on `can_operate`.

## Finding 8 (medium) — invite acceptance is intentional

Authenticated dashboard mount auto-called `acceptWorkspaceInvite` from a pending token, so cross-site navigation to a valid invite URL silently joined the attacker's workspace.

**Fix.** `useInviteRedemption` peeks and shows a confirmation banner; only an Accept click consumes the token and mutates. Decline clears storage. Tests assert mount alone does not call the mutation.

## Finding 9 (low) — hibernate PUT create-once

Restore already verified SHA-256 + size (ADR073 #7). The remaining demonstrated impact was a same-UID process retaining the 15-minute argv URL and overwriting the snapshot object after a legitimate hibernate.

**Fix (partial).** `PrepareUpload` signs `If-None-Match: *` into the PUT; `hibernateScript` sends the matching header. Keys stay unique per mint, so the first write always creates; a second PUT against the same key fails closed. The URL still appears in `/bin/sh -c` argv (gateway sandbox-exec has no stdin) — a create-only upload broker or isolated helper remains the full fix for credential-in-argv.

## Finding 10 (low) — onbex.co PSL (eleventh report)

Unchanged and **must not** be "fixed" in-repo. `.pm/DO_NOT_DO.md` `#PSL` forbids unsetting `BEX_BASE_DOMAIN` and forbids submitting to the PSL yet. Operator tracking: `.pm/w1/050.md`.

## Finding 11 (low) — export + age toolchain pins

**Fix (partial).** Logical-export `pg-dump` Jobs resolve `cnpgExportImage(version)` — every CRD-permitted major is digest-pinned, with a Valkey-style enum-derived guard test. KeyValue backup encrypt no longer `apk add`s age; it downloads the same reviewed FiloSottile/age v1.3.1 amd64 artifact (SHA-256 pinned) as the etcd/OpenBao backup charts. Remaining digest-pinning inventory (Dockerfile FROMs, kpack, barman, CNPG _Cluster_ serving tags, first-party signed backup helper image) stays ADR060 §D7.

## Finding 12 (low) — git webhook replay retention

Successful distinct signed bodies accumulated forever in `git_webhook_replays`.

**Superseded fix.** The original 90-day purge was removed because a captured body remains valid for as long as its HMAC key does; deleting its claim by wall-clock age reopened replay. Migration `0104` instead assigns claims to a tenant scope and authenticated signing-secret epoch. The handler claims only after repository/branch matching and enforces an exact 100,000-row cap per workspace across live epochs. Every API replica leases each epoch it accepts; maintenance purges a rotated epoch only after no live lease remains, so storage is bounded without expiring a claim whose signature is still accepted. Pre-`0104` rows remain in a finite legacy partition because their scope and epoch cannot be reconstructed safely.

## Finding 13 (low) — revoke fails closed on unbind

`RevokeAPIKey` deleted the Hydra client then discarded `UnbindKey` errors, so residual tenant mapping / FGA authority could outlive a "successful" revoke for the token TTL.

**Fix.** Unbind first; surface unbind errors; only then delete the Hydra client. Tests: `TestRevokeAPIKeyFailsClosedWhenUnbindFails`.

## Deferred and follow-up

- **onbex.co PSL submission** (finding 10, eleventh report): `.pm/w1/050.md` — do not unset `BEX_BASE_DOMAIN` (`.pm/DO_NOT_DO.md` `#PSL`).
- **Digest-pinning inventory** (finding 11 residual): ADR060 §D7 — Dockerfile FROMs, kpack, barman, CNPG Cluster serving tags, first-party signed backup helper (removing the age download hop).
- **Hibernate URL out of argv** (finding 9 residual): create-only upload broker or isolated helper; create-once already blocks overwrite.
