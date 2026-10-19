# CLI sweep 587 — precedence edges: empty own var still shadows; own secret file beats the group's

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `4303334d2`, 2026-10-19. Owned fixtures `qa587-web` (`srv-dbb8qdvo17is73furvug`, free busybox serving `V=[%s] SF=[%s]`) and group `qa587-group` (`evg-dbb8qd7o17is73furvt0`, `V587=group`, file `qa587.txt=group-file`). Both deleted: service 404, URL 404, group 404, zero `qa587` names. Completes sweep 586's Unrun pair.

- **Empty-string shadowing:** the service's own `V587` was set to an explicit `""` before linking. The live page read `V=[] SF=[own-file]` — the process received the empty value, not the group's `group`. Shadowing is by key presence; an explicit empty is a deliberate override, not a fall-through. (Consistent with sweep 582: `KEY=` stores `""` rather than dropping the var.)
- **File precedence matches var precedence:** with both the service and its group providing `qa587.txt`, the mounted `/etc/secrets/qa587.txt` contained `own-file`. Service-specific material wins over shared group material on both channels.

## Unrun

- Multi-service fan-out: one group linked to two services, one var update rolling both.
- Group var update while the linked service is suspended (roll on resume?).
- Precedence between two groups when a service is linked to both and both define the same key.
