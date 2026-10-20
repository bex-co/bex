# CLI sweep 674 — service outbound IPs (live, 2026-10-19)

Read-only against the baseline `hello-go` service; nothing mutated.

- `GET /services/{id}/outbound-ips` → `{"type":"shared", ips:[…]}` with 4 shared tenant-pool egress IPs (count only recorded here), matching ADR018's truthful shared-egress semantics (no `dedicatedIpId`).
- Synthetic service id → 404 `NOT_FOUND`.

No bug filed.
