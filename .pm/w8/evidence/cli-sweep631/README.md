# CLI sweep 631 — env-group secret files share the 512KiB cumulative cap

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixture: env group `qa631-group` (`evg-dbbav12vfpec73dm0hk0`, `ownerId` explicit), deleted at close (204, then 404). Completes sweep 630's Unrun item.

- First 256KB group file → `200`. A second 256KB file — which would land the group total on exactly 524288 bytes — → `400 total secret file size limit of 524288 bytes exceeded`. A 64KB file still fit afterward (320KB cumulative), and a 512KB file was refused. So the same cumulative per-owner budget governs group files, and the check treats reaching the stated cap as exceeding it (no file lands the total exactly on 524288 in these probes; the boundary arithmetic is the server's).
- Error wording is identical to the service-side cap (sweep 630), one shared implementation.

## Unrun

- A single file of exactly 524288 bytes on an empty service/group (the pure boundary).
- Whether a service's own files and its groups' files draw on separate budgets when both mount (expected separate; unprobed).
