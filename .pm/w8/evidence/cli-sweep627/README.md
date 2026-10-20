# CLI sweep 627 — one group's generated key reaches two services identically

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `c8717afdf`, 2026-10-19. Owned fixtures `qa627-a` (`srv-dbbapmjjauss73bn3k50`) and `qa627-b` (`srv-dbbapn3uc61s73csch6g`), free busybox services serving `H=<first 16 hex of sha256($G627)>`, plus group `qa627-group` (`evg-dbbapmbuc61s73csch40`) with a `generateValue` key. All deleted: group 404, both services 404, zero `qa627` names. The secret itself was never printed anywhere — only its hash.

- Both services, linked to the same group, served the identical hash (`0956c70510fa380d`): a group-generated value is one shared secret, minted once per group and materialized into every linked service — not re-minted per service.
- Combined with sweep 626 (re-PUT rotates the group value), rotation would roll both services onto the same new value on their next deploys.

## Unrun

- Rotation fan-out timing: one group re-PUT rolling two live services (both observed converging).
