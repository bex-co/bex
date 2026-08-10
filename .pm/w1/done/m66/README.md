# w1 · m66 — Security-scan fast path: close the six cheap, confirmed findings

**Worker:** worker1 **Goal:** every Tier-1 finding of the 2026-08-10 codex-security scan is fixed at root cause with a regression test that fails on the pre-fix code — including the one live data bug the scan under-rated. **Status:** done

## Tasks (in order)

| id   | title                                                                             | est | depends_on             |
| ---- | --------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Canonicalize the secret-file write path (F10 — live data bug)                      | 40m | — — **DONE**           |
| t002 | Disable ambient proxy inheritance on the webhook transport (F9)                    | 20m | — — **DONE**           |
| t003 | Require exact PKCE `S256` at both consent gates (F8)                               | 30m | — — **DONE**           |
| t004 | Pin + lock the Packer hcloud plugin (F12)                                          | 30m | — — **DONE**           |
| t005 | Authenticate the CI SSH host key before reading `admin.conf` (F7)                  | 45m | — — **DONE**           |
| t006 | No runtime package install in the backup encryption stage; digest-pin images (F16) | 45m | — — **DONE**           |
| t007 | Render parity                                                                      | 30m | t001, t003 — **DONE**  |
| t008 | Simplify                                                                           | 20m | t007 — **DONE**        |
| t009 | Test coverage                                                                      | 45m | t007 — **DONE**        |
| t010 | Closeout                                                                           | 10m | t009 — **DONE**        |

## Definition of done

- `SetSecretFile` writes under the same canonical OpenBao key every read/delete/purge path uses; a file written by `srv-…` id is readable by every addressing form, and a deleted service leaves no recoverable id-keyed path. ✅
- The outbound-webhook HTTP client never consults `HTTP_PROXY`/`HTTPS_PROXY`; a test with the proxy env set proves a private/metadata target is refused at dial time. ✅
- Both consent entry points refuse any non-device authorization-code request whose `code_challenge_method` is not exactly `S256`. ✅
- The Packer hcloud plugin is pinned to an exact version and verified against a repository-pinned checksum before it executes in the credentialed job. ✅ (revised — see below)
- `scripts/fetch-app-kubeconfig.sh` fails closed on an unknown/changed control-plane host key whenever the pinned known-hosts input is supplied, and both production workflows supply it. ✅ mechanism; **activation is an operator step** (below)
- Neither backup CronJob installs a package at runtime, and every backup image is referenced by `@sha256:` digest. ✅

## Outcome (2026-08-10)

**F10 — secret-file keying (`lego/backend/internal/secrets/`).** `SetSecretFile` was the only one of seven call sites missing `storeServiceName` canonicalization, so a `PUT …/secret-files/{name}` addressed by the stable `srv-…` id wrote to `services/<srv-id>/files` — a path no read, delete, or purge path consults. The file materialized into the pod once and then was invisible to GET/LIST/DELETE, and it outlived the service. Fixed at the call site; `PurgeApp` now also destroys legacy id-keyed paths (a `srv-` id is DNS-safe and under the 30-char name limit, so a later service could legally take that name and read the dead one's secrets). A **class sweep** (`TestEveryStoreKeyIsCanonicalized`) parses the package and fails any verb that authorizes an App and then builds a store key from the raw request token, so the divergence cannot return in a new verb.

**F9 — webhook transport (`internal/webhooks/worker.go`, `operator/internal/httpclient`).** `DefaultTransport.Clone()` carried `ProxyFromEnvironment` while only `DialContext` was replaced, so with a proxy configured `SafeDialContext` would validate the *proxy* while the proxy fetched the tenant-controlled target. Both dial-time-guarded transports now set `Proxy = nil`. The regression test records what the transport actually dials: pre-fix it dialed the proxy, post-fix the (blocked) tenant target.

**F8 — PKCE (`dashboard/src/common/server-fn/hydra-consent.ts`).** `missingPKCE` tested only for the presence of `code_challenge`, contradicting its own doc comment. One shared `pkceSatisfied` now backs both entry points and requires exactly one `code_challenge` plus `code_challenge_method` exactly `S256`; duplicates, `plain`, omitted, lowercase, trailing-whitespace, unknown methods, and unparseable authorize URLs are all refused. The RFC 8628 device exception (the official Render CLI login) is preserved and pinned by test.

**F12 — Packer plugin.** Revised from the planned lock file: **Packer ≥1.14 no longer writes `.packer.lock.hcl`** (verified against the local 1.15.4 — `packer init` installs and records nothing), so a committed lock is not a durable control. Instead the template pins `= 1.7.2` exactly and `scripts/packer-plugin-install.sh` downloads that release artifact, verifies it against the SHA-256 committed in `infra/packer/plugin-checksums.txt` (copied from the upstream release's SHA256SUMS, so the value is under code review rather than fetched beside the artifact), and only then installs it. `snapshot.yml` calls the installer instead of `packer init`, so nothing unverified executes in the `HCLOUD_TOKEN` job. Verified locally end to end (checksum OK → install → `packer validate` passes) plus a negative run (corrupted pin ⇒ exit 1, nothing installed).

**F7 — CI SSH host key.** The seam already existed and no caller used it. New shared `scripts/lib/ssh-hostkey.sh` gives both `fetch-app-kubeconfig.sh` and `verify-substrate.sh` one policy: pin supplied ⇒ `StrictHostKeyChecking=yes` + pinned `UserKnownHostsFile` + global known-hosts ignored (fails closed); pin absent ⇒ `accept-new` as before, but announced on stderr so the weaker mode is never silent; pin supplied but missing/empty ⇒ hard error, never a silent downgrade. Both production workflows materialize a new `BEX_SSH_KNOWN_HOSTS` secret to a file when present. **⚠️ ACTIVATION IS PENDING AN OPERATOR STEP:** the control is inert until someone captures the control-plane host keys (`ssh-keyscan`, fingerprint checked out of band) into that GitHub secret. Capture procedure and the re-capture-on-rotation caveat are in `docs/ADR019-infra-credentials.md`.

**F16 — backup CronJobs.** All five images (`registry.k8s.io/etcd`, `busybox`, `alpine`, `amazon/aws-cli`, `openbao/openbao`) are now `@sha256:`-pinned with digests resolved from the live registries, and `apk add --no-cache age` is gone: the encrypt stage fetches the pinned `age` v1.3.1 release artifact and verifies it against a SHA-256 committed in the chart before executing it, so a tampered artifact fails the Job (no upload) rather than producing an unencrypted one. The exact `age -r … -o out in` invocation was round-tripped locally against the real binary. `scripts/gitops-validate.sh` grew a guard enforcing both invariants (digest-pinned images, no runtime package manager, age download always checksum-verified), proven to bite on a tag-only image.

**Verification.** Backend suite green (`go test ./...`, all packages); dashboard 301 files / 2046 tests green (a pre-existing local failure in 3 files was a stale `node_modules` missing declared `@tiptap/*` deps — `yarn install` cleared it, unrelated to this milestone); new shell suites `scripts/ssh-hostkey.test.sh` + `scripts/packer-plugin-install.test.sh` wired into `.github/workflows/scripts.yml` and passing; `github-actions-validate.sh` green; every changed markdown prettier-clean. Each finding's test was checked by reverting the fix: F10 (both the specific test and the class sweep), F9, and F8 (12 cases) all fail pre-fix.

**Render parity (t007).** Secret files reach one core verb from all three adapters (`rest.go:184`, `graphql.go:271`, `mcp.go:192`), so F10's fix lands identically on REST, GraphQL, and MCP — and it moves bex *toward* Render, whose API addresses secret files by `srv-` id, which was exactly the broken addressing form. F8 changes only a refusal for non-S256 clients; the device flow the Render CLI uses is unaffected (pinned by test). No parity drift, no ledger row changed.

**Simplify (t008).** The shapes this milestone created are already single-sourced: one `pkceSatisfied` + one `pkceRefusal` for both consent paths, one `scripts/lib/ssh-hostkey.sh` for both SSH entry points, one guard block for both backup charts. No further behavior-preserving cleanup was warranted.

**Lint note.** `make lint` reports 18 pre-existing issues in the operator module (`internal/controller/`, `cmd/activator`, `cmd/kv-sni-proxy`, `internal/sniproxy`, `internal/staticserver`) — none in any file this milestone touched; the backend module lints clean. Not introduced here and not fixed here.

**Uncommitted pending `/ship`.**

## Source + Goal linkage

- **Source:** the 2026-08-10 codex-security repository scan (`~/.codex/state/plugins/codex-security/scans/bex/codex-security-bex-qmBeaW/report.md`, revision `855b0ce7`, 18 findings — 6 medium, 12 low) and the same-day triage that verified all 18 against source and re-rated F10 upward. Fourth audit in the ADR028 → `w1/m53` → `w1/m65` lineage.
- **Goal linkage:** ADR008's multi-tenant PaaS credibility — tenant data isolation (F10), SSRF/egress discipline (F9), OAuth correctness (F8, `docs/ADR012-auth.md` §7), and the privileged-automation supply chain (F7, F12, F16 → `docs/ADR031-platform-data-backup.md`, `docs/ADR050-encrypted-platform-backups.md`).
- **Expected outcome:** six confirmed findings retired in roughly a day; the secret-files API stops silently losing id-addressed writes; the consent gate enforces the invariant its own doc comment already claims.
- **Why now:** these were the highest value-per-hour items in the scan — five one-file changes plus one live product bug. Doing them first shrank the report to the four that needed design (→ m67).
- **Render parity task included** because F10 changes secret-file behavior across REST/GraphQL/MCP and F8 changes an OAuth consent refusal the CLI/MCP clients observe.
