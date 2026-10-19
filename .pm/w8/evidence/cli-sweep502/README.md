# CLI sweep 502 — Key Value create validation dialects

**Verdict: no bug filed.** Passing functional coverage. Same isolated human device-login session as sweep 501, `bex-canary` (`tea-…s3g`), HEAD checkout build (`97a170c1b`, `vdev`, Render pin v2.27.0 `a764810a7682`), 2026-10-19T08:56–08:57Z. Deployed production revision not observable.

Every invalid create refused by name before creating anything — `keyvalues list` stayed empty across all negative probes, and the one successful free fixture was deleted by ID with a named not-found on re-`get`.

| step | observed result |
| --- | --- |
| `--plan bogusplan` | Exit `1`, server `400: invalid request body at /plan`. The client deliberately does not enum-validate the plan (help: "Custom enterprise plan names are also accepted"), so a named server refusal is the correct layer. The message does not enumerate valid plans; not filed — Render's API rejects unknown plan IDs the same terse way and no valid input is refused. |
| `--region atlantis`, `--memory-policy boguspol`, `--persistence-mode bogusmode` | Exit `1` client-side, each flag error names the accepted values; no request sent. |
| `--name 'Bad_Name!'`, 70-char name | Exit `1`, server `400 (RESOURCE_NAME_INVALID)` spelling the rule (lowercase/digits/hyphens, ≤30 chars, no leading/trailing hyphen). |
| `--plan free --memory-policy cache` create | Exit `0`; `creating → available` in ~30 s; the `cache` alias reads back `maxmemoryPolicy: allkeys_lru`; default `persistenceMode: journal_snapshot`, `version: 8`. |
| Duplicate create with the same name | Exit `1`, `409 (CONFLICT): a key-value store named "…" already exists in this workspace` — no second resource. |
| `keyvalues get` by name | Resolves to the same ID with `available`. |
| Delete by ID, re-`get`, `list` | Named not-found (`No Key Value with ID 'red-…'`); list empty again. |

## Safety and unrun checks

- Cleanup: exact-owned `red-dbatma7bcrps73dmoa3g` deleted by ID and verified absent as above. No plan mutations; paid plan names were not probed (a create must never risk a purchase), so spec-based paid-plan acceptance stays with the compat ledger / w8-011 alias record. Secrets unrecorded; connection info never printed.
- The isolated CLI session and this run's Kratos jars remain active for subsequent sweeps in this loop.
- Unrun: `kv-cli` data path on this fixture (covered by sweeps 463/389), `--ip-allow-list` malformed-CIDR dialect on create (allowlist round trip covered 460), `--environment`/`--project` scoping dialects.
- Deploy lag means deployed revisions can precede local `main`.
