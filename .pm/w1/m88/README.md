# w1 · m88 — Service-disk REST parity with Render

**Worker:** worker1 **Goal:** every remaining service-disk REST divergence from Render is either closed or written down as deliberate — none left accidental. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Accept a disk at service-create time, or refuse it in writing | 1h | — |
| t002 | Match Render's snapshot restore and list response shapes | 45m | — |
| t003 | Honor Render's documented list-disks filters | 45m | — |
| t004 | Resolve the `sizeGB` default asymmetry across surfaces | 30m | — |
| t005 | Render parity check | 30m | t001, t002, t003, t004 |
| t006 | Simplify pass | 30m | t005 |
| t007 | Test coverage | 45m | t005 |
| t008 | Closeout | 30m | t007 |

## Definition of done

A Render-shaped `POST /v1/services` carrying `serviceDetails.disk` either attaches the disk or returns a named, documented error — never "unknown field". Restore and list-snapshots responses match Render or are recorded divergences, and REST/GraphQL/MCP agree with each other. Every filter Render documents on `GET /v1/disks` either filters or is rejected; none is silently ignored, and repeated `serviceId` filters on all values. The `sizeGB` default is consistent across the four surfaces or its difference is documented. ADR082's divergence list and ADR018's disk row both match the shipped behavior.

## Source + Goal linkage

- **Source:** [.pm/w1/076.md](../done/076.md) — four divergences with file:line evidence, from the w1/m86 cross-surface audit.
- **Goal linkage:** ADR006/ADR018 Render compatibility. bex-api's contract is that a Render client works unmodified; the silently-ignored list filters break that quietly, which is the worst way to break it.
- **Expected outcome:** a Render user's existing tooling gets correct answers from bex's disk routes, or a clear refusal — never a wrong answer delivered confidently.
- **Why now:** the audit evidence is fresh and precise, and every item is small. Left alone these calcify into "how bex has always behaved" and get much more expensive to change once clients depend on them.
- **Render parity task included** (t005): the milestone is entirely REST-surface work.
