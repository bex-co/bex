# w1 · m88 — Service-disk REST parity with Render

**Worker:** worker1 **Goal:** every remaining service-disk REST divergence from Render is either closed or written down as deliberate — none left accidental. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Accept a disk at service-create time, or refuse it in writing | 1h | — | — **DONE**
| t002 | Match Render's snapshot restore and list response shapes | 45m | — | — **DONE**
| t003 | Honor Render's documented list-disks filters | 45m | — | — **DONE**
| t004 | Resolve the `sizeGB` default asymmetry across surfaces | 30m | — | — **DONE**
| t005 | Render parity check | 30m | t001, t002, t003, t004 | — **DONE**
| t006 | Simplify pass | 30m | t005 | — **DONE**
| t007 | Test coverage | 45m | t005 | — **DONE**
| t008 | Closeout | 30m | t007 | — **DONE**

## Closeout

All four divergences closed by implementing rather than documenting-away, and each decision is now recorded in ADR082 D6 so none of them is accidental any more.

- **t001** — `serviceDetails.disk` accepted on create for web/private/worker, routed through the same `validateCreateDisk` the Blueprint path uses. It also settled t004: Render's `serviceDisk` requires only name + mountPath while its `add-disk` body requires `sizeGB`, so the asymmetry is Render's own and bex reproduces it (default 10 at create, required on the standalone route) instead of unifying it.
- **t002** — restore answers Render's 200 with the disk, and GraphQL/MCP return the disk too, so all three surfaces agree; GraphQL's `Boolean` had told a caller only that nothing threw. `list-snapshots` matches Render's documented 201 for that GET — the oddity is Render's, and a generated client switches on the code, so a tidier 200 would leave its result empty.
- **t003** — every documented filter now filters. Multiple `serviceId` values are unioned by asking the **authorized** verb once per service, so naming more services can only narrow what a caller may read, never widen it. Within a multi-value filter a stale id matches nothing rather than failing the whole request; a single unknown id is still a 404.
- Two shared helpers grew rather than being forked: `core.NewUnavailableError` (the 503 class had no coded constructor) and `core.TimeWindow.ContainsTime` (for views holding `time.Time` instead of Render's wire string).

Gates: full backend suite on a fresh Postgres + OpenFGA, dashboard 2600 tests, `make lint` clean on all four modules.

## Definition of done

A Render-shaped `POST /v1/services` carrying `serviceDetails.disk` either attaches the disk or returns a named, documented error — never "unknown field". Restore and list-snapshots responses match Render or are recorded divergences, and REST/GraphQL/MCP agree with each other. Every filter Render documents on `GET /v1/disks` either filters or is rejected; none is silently ignored, and repeated `serviceId` filters on all values. The `sizeGB` default is consistent across the four surfaces or its difference is documented. ADR082's divergence list and ADR018's disk row both match the shipped behavior.

## Source + Goal linkage

- **Source:** [.pm/w1/076.md](../done/076.md) — four divergences with file:line evidence, from the w1/m86 cross-surface audit.
- **Goal linkage:** ADR006/ADR018 Render compatibility. bex-api's contract is that a Render client works unmodified; the silently-ignored list filters break that quietly, which is the worst way to break it.
- **Expected outcome:** a Render user's existing tooling gets correct answers from bex's disk routes, or a clear refusal — never a wrong answer delivered confidently.
- **Why now:** the audit evidence is fresh and precise, and every item is small. Left alone these calcify into "how bex has always behaved" and get much more expensive to change once clients depend on them.
- **Render parity task included** (t005): the milestone is entirely REST-surface work.
