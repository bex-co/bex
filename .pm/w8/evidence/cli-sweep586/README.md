# CLI sweep 586 — env precedence: service's own var beats the group's; `?name=` is exact

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixtures `qa586-web` (`srv-dbb8pcfo17is73furvog`, free busybox serving `V586=$V586`) and group `qa586-group` (`evg-dbb8pc5t4o5c73dqgfeg`, `V586=group`, `ownerId` explicit per the sweep-585 lesson). Both deleted: service 404, URL 404, group 404, zero `qa586` names. Follows up sweep 585's Unrun list.

- **Own var wins:** with the service's own `V586=own` set before linking, the live page read `V586=own` — the service-level value shadows the group's, matching Render's documented precedence (specific beats shared).
- **Removal uncovers the group value:** deleting the service's own var rolled the service and the page read `V586=group` within ~30s. Shadowing is by key presence, not a copy made at link time.
- **List filter semantics:** `GET /env-groups?name=qa586-group` returned the group; `?name=qa586` returned `[]`. The name filter is exact-match, not prefix/substring.

## Unrun

- Group secret file vs service-owned secret file with the same name (file precedence analogue).
- Multi-service fan-out: one group linked to two services, one var update rolling both.
- Precedence when the own var is set to an explicit empty string (shadowing with `""` vs falling through).
