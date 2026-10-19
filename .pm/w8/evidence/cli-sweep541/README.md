# CLI sweep 541 — Key Value rename and per-kind name namespaces

**Verdict: no bug filed.** Same isolated session, `bex-canary`, HEAD build at `df689525f`, 2026-10-19T10:45–10:47Z.

Setup fact first: a second free Key Value create refused with `400 (WORKSPACE_RESOURCE_LIMIT): workspace is limited to 1 key-value store` — the free-tier cap that also shaped sweeps 502/513/520, named precisely. The rename legs therefore ran on the single owned store:

| step | observed result |
| --- | --- |
| `keyvalues update <id> --name …-renamed` | Exit `0`, same `red-…` ID, new name reads back. |
| `keyvalues get <new name>` / `get <old name>` | New name resolves to the same ID; old name answers the resolver's named not-found (with the documented upstream `render workspace set` runtime-copy residual, sweeps 468/504). |
| Rename onto `hello-go` (a baseline *service* name) | Exit `0` — names are per-kind namespaces (sweep 479's model); `keyvalues get hello-go` resolves within the Key Value kind to the same ID, and the baseline service was never touched. |
| Rename back, delete by ID | Exit `0`; list empty; `get hello-go` (Key Value) not-found again. |

Cleanup: exact-owned `red-dbav9vnmnaes738pfp10` deleted by ID.

- Unrun: same-kind rename collision (needs a second store, which the free cap forbids in this workspace; the create-side 409 for exact duplicate names was sweep 502), rename to an invalid name on update (create-side name rule covered 502).
- Deploy lag means deployed revisions can precede local `main` (production pin `8e57d4eaa121` at sweep time).
