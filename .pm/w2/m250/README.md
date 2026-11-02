# w2 · m250 — Pro Plus, Pro Max and Pro Ultra services start, as on Render

**Worker:** worker2 **Goal:** a customer on any workspace plan who picks Pro Plus, Pro Max or Pro Ultra gets a running service, because autoscaled serving pools with nodes large enough for those tiers exist, and only those tiers use them. **Status:** todo

## Tasks (in order)

| id   | title                                                                                          | est | depends_on |
| ---- | ---------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Two serving pools in the CAPI overlay (cx53 min 0/max 4, ccx43 min 0/max 2), each with its own taint, plus validator guards | 1h  | —          |
| t002 | Operator: App pods of the large tiers tolerate and prefer their pool (from `tiers.yaml`); admission and build-only rules still hold | 1h  | —          |
| t003 | Production: pro-plus and pro-ultra services go live on new nodes and the pools scale back to 0 | 45m | t001, t002 |
| t004 | Docs: ADR053/ADR060 pools, ADR018 instance-type row                                            | 20m | t003       |
| t005 | Render parity                                                                                  | 15m | t004       |
| t006 | Simplify                                                                                       | 15m | t005       |
| t007 | Test coverage                                                                                  | 30m | t005       |
| t008 | Closeout                                                                                       | 10m | t006, t007 |

## Definition of done

- On production, a `pro_plus` web service reaches `live` on a node of the cx53 pool and a `pro_ultra` one on a node of the ccx43 pool, created by the autoscaler from zero; after both are deleted, both pools return to 0 nodes.
- `starter`…`pro` services, builds and datastores never schedule on either pool (taints), and the build-only pool is unchanged.
- `scripts/clusterapi-validate.sh` fails if a large tier no longer fits its pool's allocatable, or if a pool loses its taint or min 0.
- The Scale workspace whose pro-plus/pro-ultra services failed (`tea-db9f0dkfcf2c73c2mq4g`) can redeploy them successfully, or its owner is told the types now work.

## Source + Goal linkage

- **Source:** promoted from inbox note [w2/063](source-w2-063.md) (found in w2/061's production check, 2026-11-01); user decisions "A" (2026-11-01) and "Two pools" (2026-11-02).
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) Render parity first: Render sells and starts these instance types on every plan.
- **Expected outcome:** every compute tier bex sells actually runs; nodes for them cost money only while such a service exists.
- **Why now:** a Scale customer's pro-plus and pro-ultra services already fail, and w2/061 just let Hobby workspaces pick these tiers too.
- **Render parity included:** the instance types on REST, GraphQL, MCP, CLI and the dashboard go from "never starts" to "starts".
