# w7 · m85 — Close the digest-pinning inventory + fail-closed CI guard

**Worker:** worker7 **Goal:** every image bex builds from, runs, or ships resolves by digest — and a guard keeps it that way, so this stops coming back as a security-review finding. **Status:** done (2026-08-21)

## Tasks (in order)

| id   | title                                                                   | est | depends_on                                         |
| ---- | ----------------------------------------------------------------------- | --- | -------------------------------------------------- |
| t001 | Pin `dashboard/Dockerfile`'s two `node:22-alpine` FROMs                 | 25m | — — **DONE**                                       |
| t002 | Pin the three KeyValue controller image constants                       | 30m | — — **DONE**                                       |
| t003 | Remove the runtime `apk add age` from the plaintext-RDB stage           | 45m | w7/m85/t002 — **DONE**                             |
| t004 | Pin the remaining manifest image refs (barman-cloud plugin and siblings) | 30m | — — **DONE**                                       |
| t005 | Fail-closed CI guard rejecting any unpinned image reference             | 45m | w7/m85/t001, t002, t003, t004 — **DONE**           |
| t006 | Simplify the code this milestone changed                                | 30m | w7/m85/t005 — **DONE**                             |
| t007 | Test coverage for the shipped behavior                                  | 40m | w7/m85/t005 — **DONE**                             |
| t008 | Closeout                                                                | 15m | w7/m85/t006, w7/m85/t007 — **DONE**                |

## Definition of done — verified clause by clause

| clause | evidence |
| --- | --- |
| Every image reference bex builds from, runs, or ships resolves by digest | `scripts/image-pin-validate.sh` → **78 references, all digest-pinned or enumerated as exempt**. `--verify-digests` additionally asked each registry whether the pinned digest exists in the repository its reference names: **60/60 present** (after the skopeo fix below). |
| The Key Value backup path installs nothing at runtime in the stage that reads the plaintext RDB | The encrypt stage runs `/backup-encrypt` — a first-party entrypoint of the bex image — with plain file arguments. No shell, no package manager, no download. Snapshot and compress were already pinned and install nothing. Asserted by `TestKeyValueBackupEncryptStepWhenKeyConfigured`, which fails on `apk`, `wget`, `curl`, `http(s)://` or any `sh -c` appearing in that stage. |
| A fail-closed CI guard turns red on a reintroduced floating or version-only tag, and its own red/green self-test proves it does | `scripts/image-pin-validate.test.sh` proves red on all four syntaxes and green on the fixed tree; both run in `.github/workflows/scripts.yml`. Meta-verified: making `pinned()` return true unconditionally turns the self-test red with 8 failures. |

## What was actually open, and what closed it

The register implied a broad sweep. Measured against the tree, most of it had already closed one milestone at a time (`w1/m66` F16, `w1/m73`, round-14 #5, round-16 #11, `w7/m58`, `w7/m65`, `w7/m75`) — including **t001 and t002 in full**, which is recorded honestly in those task files rather than claimed as work done here.

What was genuinely unpinned, all fixed:

| reference | where | why it mattered |
| --- | --- | --- |
| `docker.io/library/alpine:3` | `BEX_SANDBOX_IMAGE` default, `cmd/api/main.go` | The base sandbox template image. No deployment sets this variable, so the floating default is what actually runs. |
| `ghcr.io/bex-co/bex-agent-sandbox:latest` | `BEX_AGENT_SESSION_IMAGE` default, same file | Production overrides it from the api Deployment (which deploy.yml digest-pins), but the in-code floor was `:latest`. |
| `postgres:17`, `openfga/openfga:latest` | `.github/workflows/backend-test.yml` | The backend integration suite certifies real SQL and real authz against these two containers. |
| `axllent/mailpit:v1.21.8` | gitops local overlay + `scripts/dev-env` | Version-only, two copies. |
| `quay.io/skopeo/stable@sha256:c7d3c512…` | `internal/build/build.go` | **A live defect** — see below. |

Everything else was already pinned by one of four mechanisms, which is the classification t004 asked for: FROM digests, Go constants, kustomize `images:` write-backs, and OCI chart `targetRevision` digests — plus the barman-cloud plugin and its injected sidecar, pinned by a kustomize patch over a byte-for-byte vendored upstream manifest and asserted by `scripts/gitops-validate.sh` on the **rendered** output.

## The defect the verification found

`--verify-digests` does not compare a digest against what its tag resolves to today — a moved tag is the pin working. It asks whether the digest exists in that repository at all, which is the one failure a code review cannot see: a wrong 64-hex string looks exactly like a right one.

`quay.io/skopeo/stable@sha256:c7d3c512…` **did not exist**. Quay stores an OCI image index for `v1.22.2`, converts it to a Docker manifest list on request, and reports the **converted** digest in `Docker-Content-Digest` — a digest it never stores and cannot serve. The build plane's credential-isolated push stage was pinned to an image the registry would not return. Repinned to the stored index, `sha256:64ac45c5…`, in `build.go`, `toolchain-freshness.json` and `scripts/verify-build-isolation.sh`.

That is also a caution about method: resolving a pin from `Docker-Content-Digest` is not sufficient on its own.

## The encrypt stage, and the helper image that was not built

[ADR068](../../../docs/ADR068-security-review-round13.md) #9 named "a first-party reviewed backup helper image containing `valkey-cli` + `gzip` + `age`, digest-pinned" as the durable fix. It was deliberately not built, and the reasoning is recorded in [ADR050](../../../docs/ADR050-encrypted-platform-backups.md) and [ADR067](../../../docs/ADR067-security-review-round12.md) finding 8.

The bex image is **already** the reviewed, cosign-signed, SBOM-attested, digest-pinned first-party artifact. So `age` became part of it: `lego/operator/cmd/backup-encrypt` compiles `filippo.io/age` v1.3.1 into a `/backup-encrypt` entrypoint, and the encrypt stage execs it. Publishing a second image to carry three binaries would have added a registry artifact to scan and sign, a sixth generated digest to the deploy write-back transaction, and a bootstrap fallback path — for a stage the bex image can run itself.

The operator learns which image that is by reading its own Pod (`internal/selfimage`, `POD_NAME` via the downward API), so the derived stage always matches the digest the operator was rolled out with — no second thing to keep in sync, and nothing in the source pretending to know a digest only kustomize can supply.

**The trade-off, stated plainly:** the encrypt stage now depends on that resolution succeeding. With a public key configured and no image resolved, the reconcile **fails closed** — it refuses to converge a CronJob rather than dropping the encrypt stage and uploading a nightly plaintext backup to a bucket it was told to encrypt into. Unencrypted backups do not depend on it at all.

Interop was proved, not assumed: a `.rdb.gz.age` from `/backup-encrypt` decrypts byte-identically with the upstream `age` v1.3.1 CLI — the exact binary `scripts/lib/restore.sh` invokes — so no restore-script change was needed.

## The durable half

`scripts/image-pin-validate.sh` re-derives the inventory on every CI run across **four** syntaxes: Dockerfile `FROM`/`--from=`, Go string literals, manifest `image:`/`*Image:` keys plus kustomize `images:` edits and Helm `repository:`+`tag:` pairs, and `docker run` inside CI workflows. The fourth was added after the sweep found `openfga/openfga:latest` started that way — the three syntaxes the task named would have missed it.

Fail-closed is the design, not a slogan:

- An unclassifiable reference **fails**. A Dockerfile `FROM ${BASE}` is reported, not skipped, because "the guard cannot see it" is precisely how this inventory drifted through six rounds.
- Exemptions are enumerated `<path>\t<reference>\t<reason>` rows. The same string in a different file fails until someone writes a reason for it, and exemptions apply **only** to the canonical tree — the self-test proves a fixture spelling `controller:latest` still fails.
- A scan matching nothing fails. "Clean tree" and "broken scan" produce the same silence otherwise.
- On bash < 4 it refuses to run: macOS's bash 3.2 crashes the per-file subshells partway through and quietly returns a **third** of the inventory, which is worse than no guard. Found by running it under `/bin/bash` rather than assuming.

`scripts/gitops-validate.sh` stays complementary: it renders the gitops tree and checks the **effective** references a cluster would pull, which is what backs the barman exemption.

## Test coverage

- Guard red on each of the four syntaxes and green on the fixed tree; plus red for an unparseable `FROM`, for exemption leakage, for an empty scan, and for an exemption row with no reason.
- `cmd/backup-encrypt`: round-trip through the public age format, not-armored, and five failure modes (no recipient, malformed recipient, an SSH recipient, missing input, wrong arity) each asserting **no output file is left for the upload stage to ship** and **the plaintext survives for the retry**.
- `internal/selfimage`: reads the manager container by name (a sidecar at index 0 must not become the image), and five ways resolution can fail — none returning a usable-looking image. 100% coverage.
- `TestKeyValueBackupEncryptionFailsClosedWithoutHelperImage`: encryption configured + no image ⇒ reconcile errors, **no CronJob exists**, and the error names both remedies; encryption off ⇒ unaffected, CronJob created, pipeline still snapshot+compress.

Suites green: operator `make test`, `cd lego/backend && go test ./...`, `dashboard/yarn test` (353 files / 2461 tests), `make lint` across all four modules (0 issues), `shellcheck` on the three new scripts, and the existing `github-actions-validate.sh`, `gitops-validate.sh` and `build-toolchain-freshness.sh` guards.

## Left open, deliberately

Filed as [`.pm/w7/041.md`](../041.md): the etcd/OpenBao static backup CronJobs still fetch the checksum-verified `age` release at run time (no operator in the loop to resolve a first-party image for them); developer/e2e shell harnesses under `scripts/` start containers by tag; and `toolchain-freshness.json` holds digests in a fifth syntax the guard does not read (coupled to Go source by a cross-check test). The re-resolve **cadence** for pinned digests remains [`030.md`](../030.md).

## Source + Goal linkage

- **Source:** [`.pm/w7/builder-issues.md`](../builder-issues.md) §3.9 (P8) — the **sixth** report of the same deferral: ADR055 F7 → ADR061 #1 → ADR063 #12 → ADR066 #7 → [ADR067](../../../docs/ADR067-security-review-round12.md) #8 → [ADR068](../../../docs/ADR068-security-review-round13.md) #9. Both ADR067 #8 and ADR068 #9 are now marked resolved, and the lineage is retired in ADR067's deferred-with-evidence register.
- **Goal linkage:** [`.pm/GOAL.md`](../../GOAL.md) #7 (security review) — supply-chain integrity, continuing w7's own track (`m10` Trivy, `m62` govulncheck, `m65` action SHA-pinning, `m75` infra pinning).
- **Render parity task omitted:** yes — image references, an operator entrypoint, and a CI guard. No REST, GraphQL, MCP or dashboard surface is touched, and tenant-visible behavior is unchanged.
