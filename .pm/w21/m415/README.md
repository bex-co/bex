# w21 · m415 — Object storage core and Render's `ea objects` over Wasabi: buckets as rows, quotas, presigned put, get, list and delete

**Worker:** worker21 **Goal:** the unchanged Render CLI and SDKs store, read, list and delete workspace objects in Wasabi, isolated per workspace and bounded by quotas. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Migration, the `obk` id kind and the store methods for `object_buckets` and `object_storage_usage` | 45m | — |
| t002 | `internal/objectstorage`: the region map, constants, name and key validation, and the key layout | 50m | — |
| t003 | The Wasabi client over `internal/s3client`, a fake S3 for tests, and the two read-write credential variables | 60m | t002 |
| t004 | Credentials rollout: IAM policies, `object-storage-s3-credentials.sh`, deployment wiring, ADR019 rows and the AGENTS inventory | 60m | t003 |
| t005 | The bucket registry, the system bucket `render-<region>`, the quota gate and the stream limiter | 60m | t001, t002, t003 |
| t006 | Contract A routes: list, put, get and delete replace the not-offered refusals | 60m | t005 |
| t007 | Authorization and scope classes for Contract A, with a test for each | 40m | t006 |
| t008 | The pinned Render CLI test for `ea objects`, the launcher pass-through and the checklist rows | 50m | t006, t007 |
| t009 | Docs: the ADR018 object storage row, its dated divergences and `bex-cli.md` | 30m | t008 |
| t010 | Production walk for Contract A: install the credentials, round trips in both regions, isolation | 60m | t004, t008, t009 |
| t011 | Render parity | 30m | t010 |
| t012 | Simplify | 20m | t011 |
| t013 | Test coverage | 45m | t011, t012 |
| t014 | Closeout | 10m | t013 |

## Definition of done

1. With the credentials installed, the pinned `render ea objects` put, get, list and delete round-trip 0 B, 1 B and 5 MiB objects byte-identically in `oregon` and `frankfurt`, and every other region answers 400.
2. A key in one workspace is unreachable from another, a `bex.read`-only OAuth token cannot mint a GET URL, and a put above a cap is refused before a URL is minted.
3. Without the credentials the four routes answer today's named 404, so shipping the code changes nothing until the credential is installed.
4. The ADR018 row records the shipped behavior with each bex-chosen value dated and marked unsigned.
5. The pinned-CLI contract test runs in CI, and the production walk (t010) has recorded its results, including whether Wasabi enforces the signed `Content-Length`.

## Source + Goal linkage

- **Source:** [ADR112](../../../docs/ADR112-object-storage.md), filed by `/pmx adr 112 to w21` on 2026-11-15; ADR112 tasks t001, t002, t004 and the Contract A parts of t008 and t009, reorganized into three milestones.
- **Goal linkage:** ADR008 Render parity first: `render ea objects` is a Render capability ADR018 listed as not offered, and the user re-opened that row on 2026-11-14.
- **Expected outcome:** A customer who points the unchanged Render CLI at bex stores and reads files, and the same store is ready for wrangler (w21/m416).
- **Why now:** The user's 2026-11-14 instruction replaced the parity gap with a store; every later milestone (wrangler, public reads) sits on the registry, client and quotas built here.
- **Render parity task:** included. This milestone changes a customer-facing REST surface; the reference is render.com's early-access object storage as the pinned CLI and SDK show it.

## Open decisions

- Recorded recommendation, not a user decision: the regions, caps, key rule and lifetimes are bex's own and unverified against Render; ADR018 needs the user's sign-off on each before the divergence stands (ADR008 Render parity first).
- User decision 2026-11-15: tenant data shares the platform's existing Wasabi account (ADR112 D1).
- Not filed: ADR112 t010 (pricing). It waits for the user to set a price; v1 only records shadow meters.
