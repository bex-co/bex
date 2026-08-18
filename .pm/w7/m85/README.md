# w7 · m85 — Close the digest-pinning inventory + fail-closed CI guard

**Worker:** worker7 **Goal:** every image bex builds from, runs, or ships resolves by digest — and a guard keeps it that way, so this stops coming back as a security-review finding. **Status:** todo

## Tasks (in order)

| id   | title                                                                    | est | depends_on                                                  |
| ---- | ------------------------------------------------------------------------ | --- | ----------------------------------------------------------- |
| t001 | Pin `dashboard/Dockerfile`'s two `node:22-alpine` FROMs                   | 25m | —                                                            |
| t002 | Pin the three KeyValue controller image constants                        | 30m | —                                                            |
| t003 | Remove the runtime `apk add age` from the plaintext-RDB stage             | 45m | w7/m85/t002                                                  |
| t004 | Pin the remaining manifest image refs (barman-cloud plugin and siblings)  | 30m | —                                                            |
| t005 | Fail-closed CI guard rejecting any unpinned image reference               | 45m | w7/m85/t001, w7/m85/t002, w7/m85/t003, w7/m85/t004           |
| t006 | Simplify the code this milestone changed                                 | 30m | w7/m85/t005                                                  |
| t007 | Test coverage for the shipped behavior                                   | 40m | w7/m85/t005                                                  |
| t008 | Closeout                                                                 | 15m | w7/m85/t006, w7/m85/t007                                     |

## Definition of done

Every image reference bex builds from, runs, or ships resolves by digest. The Key Value backup path installs nothing at runtime in the stage that reads the plaintext RDB. A fail-closed CI guard turns red on a reintroduced floating or version-only tag, and its own red/green self-test proves it does.

## Source + Goal linkage

- **Source:** [`.pm/w7/builder-issues.md`](../builder-issues.md) §3.9 (P8) — the **sixth** report of the same deferral: ADR055 F7 → ADR061 #1 → ADR063 #12 → ADR066 #7 → [ADR067](../../../docs/ADR067-security-review-round12.md) #8 → [ADR068](../../../docs/ADR068-security-review-round13.md) #9 (landed 2026-08-17 while this milestone was being written). ADR067 #8 extended it to the `apk add age` stage; **ADR068 #9 extends it further to the backup Job's `valkey:<version>` snapshot and `busybox:1.37` compress stages** (the upload image is already pinned) and names the durable fix: "a first-party reviewed backup helper image containing `valkey-cli` + `gzip` + `age`, digest-pinned, removes the runtime package install".
- **Goal linkage:** [`.pm/GOAL.md`](../../GOAL.md) #7 (security review) — supply-chain integrity, continuing w7's own track (`m10` Trivy, `m62` govulncheck, `m65` action SHA-pinning, `m75` infra pinning).
- **Expected outcome:** the deferral closes permanently rather than being re-triaged every review round. The guard is the durable half — without it this becomes a sixth report.
- **Why now:** it has been deferred six times because it reads like an open-ended inventory sweep. It is not: a code check found the build/push/sign/preparer images and **all five native base images already pinned**, leaving four files. The work is now small enough to finish in one pass, and each additional re-report costs triage time in a security review that has better things to do — ADR068 re-filed it the same day this milestone was created, which is the argument in miniature.
- **Render parity task omitted:** yes — this changes image references in Dockerfiles, operator constants and gitops manifests plus a CI guard. No REST, GraphQL, MCP or dashboard surface is touched, and tenant-visible behavior is unchanged.
