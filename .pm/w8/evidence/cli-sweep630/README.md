# CLI sweep 630 — secret file size ceilings: a per-service total cap under a global body cap

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture `qa630-web` (`srv-dbbauf2vfpec73dm0hfg`, free busybox), deleted at close: REST 404, zero `qa630` names. Contents were filler bytes, nothing sensitive. REST boundary (the CLI has no secret-file verb).

- **Accepted sizes:** single files of 1KB, 16KB, 64KB, and 256KB each stored with `201`.
- **Per-service total cap:** with ~341KB already stored, a 512KB addition → `400 total secret file size limit of 524288 bytes exceeded` — the limit is a cumulative per-service budget (512KiB), stated in bytes in the error. A standalone 1MB file hit the same cap message.
- **Global body cap above it:** 2MB and 5MB payloads → `413 request body exceeds 2097152 bytes` (`payload_too_large`) — the transport limit fires before content validation at that size, and both ceilings are named with their numbers.

## Unrun

- The exact boundary (a file landing precisely on 524288 total bytes).
- Whether env-group secret files share the same per-group budget.
