# Operand dump and IMMV-sidecar consumer — 2026-10-31

The maintained restore executable consumes the existing directory dump and its original sibling sidecar as the caller's database identity. It applies both files in one transaction. TimescaleDB and large-object archives are explicitly refused before connecting, with no data silently omitted. The image installs the helper; export format, original complete gate and production pins are unchanged.

[Root review and byte custody](ROOT-REVIEW.json) bind all71 frozen payloads and four live source files. Eleven command/protocol tests and four meaningful mutation controls pass. Root's real PostgreSQL15.13 tests prove that a sidecar error rolls back all restored tables, success preserves Unicode/quote/dollar rows as app/non-superuser, and a real large-object dump is refused before target tables appear. The exact temporary container was removed. These controls do not establish pg_ivm maintenance, Timescale, CNPG or dual-architecture qualification.

The actual PG14 arm64 gate, run41196538568/job124586881316, fails when the ordinary owner calls timescaledb_pre_restore: its ALTER DATABASE restoring flag is denied. Complete raw logical restore remains mandatory under ADR092 D3; platform recovery is separate evidence. The complete raw job log is retained losslessly. Upstream invoker/GUC/catalog sources and the owner-portable protocol proposal remain in the frozen handoff. That alternative protocol has not been implemented or qualified and does not satisfy the unchanged raw restore criterion. No new restart decision is required.

Frozen non-JSON files use lossless gzip; ROOT-REVIEW maps each original SHA/size to its retained path. Original private files and all prior milestone evidence are preserved.
