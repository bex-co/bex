# w5 · m383 — Sandbox image imports cannot fill the shared registry or reach past it: a platform budget, fair workers, no foreign layers, no same-registry sources

**Worker:** worker5 **Goal:** no workspace, and no set of workspaces together, can fill the shared sandbox registry or starve other tenants' imports, and an import can bring in only bytes it actually copied. **Status:** blocked (t001–t007 done; code shipped; t008 Closeout waits on confirming the production registry volume's size, DoD 4)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Refuse a source image on the sandbox registry's own host, so no import can cross-repo mount another tenant's layer — **DONE** | 30m | — |
| [t002](done/t002.md) | Refuse a source image whose layers carry foreign URLs or are non-distributable — **DONE** | 30m | — |
| [t003](done/t003.md) | Import workers are fair: one workspace holds at most one of a replica's slots — **DONE** | 40m | — |
| [t004](done/t004.md) | A platform-wide budget on imported image bytes, sized below the registry volume, refused with a named 429 — **DONE** | 1h | — |
| [t005](done/t005.md) | Surface consistency — **DONE** | 15m | t001, t002, t003, t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 30m | t005, t006 |
| [t008](t008.md) | Closeout | 15m | t007 |

Total: **8 tasks, about 3h55m**.

## Definition of done

1. An import whose source is on the sandbox registry's host is refused before any registry write.
2. An import whose source manifest has a layer with `urls` or a non-distributable media type is refused before any registry write, naming the layer.
3. With a replica's import slots full, a second workspace's import starts as soon as a slot frees even while the first workspace has more queued; one workspace never holds more than one slot per replica.
4. A new import that would take the platform's imported bytes (stored and in flight, every workspace) past `BEX_SANDBOX_IMPORT_PLATFORM_BUDGET_GB`, or one workspace's past its share (a quarter), is refused with 429 `SANDBOX_IMPORT_CAPACITY` on REST, GraphQL and MCP; the default sits below the registry volume declared in `deploy/gitops/base/zot.yaml` (100 Gi), and the production volume's live size is confirmed before closeout (it may have been grown out of band).
5. Each is covered by a regression test that fails on the pre-milestone code.

## Source + Goal linkage

- **Source:** inbox note [w5/189](../../done/189.md), from security review round 24 ([ADR107](../../../../docs/ADR107-security-review-round24.md), the S-3 remainder and sandbox hardening notes). Promoted by `/loopx w5` on 2026-11-13.
- **Evidence:** the per-workspace cap (500 GB default) exceeds the shared Zot volume (`deploy/gitops/base/zot.yaml`, `storage: 100Gi`); `sandbox/images.go` runs `importWorkers = 2` per replica shared across tenants, and a slow source holds a slot for up to 30 minutes; `Images.copy` writes the source with `remote.Write`, which carries a layer's `urls` into the imported manifest and, for a source on the sink's own host, mounts blobs across repositories with the sink's credential.
- **Goal linkage:** sandboxes (w2/m358 imports, w2/m359 groups); the security-review lineage (ADR107).
- **Expected outcome:** imports cannot exhaust the registry other tenants' builds depend on, and an imported image holds only bytes bex copied.
- **Why now:** round 24's S-3 fix bounded each workspace, not the platform; this closes the remainder.
- **Surface consistency:** included (t005). The new refusals are new API errors on the import surfaces (REST, GraphQL, MCP); there is no Render or Devin counterpart for image import limits, so the check is consistency across bex's own surfaces.
- **Approach note:** the note asked to strip foreign URLs and to check Zot's cross-repo mount and dedupe behavior, both "needs a deployment test". Refusing foreign blobs (t002) needs none: a refused manifest never reaches containerd. Refusing a source on the registry's own host (t001) closes go-containerregistry's cross-repository mount. Whether Zot's dedupe links a blob another repository holds was filed as w5/192, which found that it does on any mount request and that every import sent one; fixed there (ADR107 S-4).

## Findings (2026-11-13)

- **Production registry volume not yet confirmed** (DoD 4, t004 step 4). This session has no production cluster access. The repo declares 100 Gi; the default budget is 80 GB. Someone with production access runs `kubectl -n bex-registry get pvc` (and the volume's usage) and records the size here; if it was grown, raise `BEX_SANDBOX_IMPORT_PLATFORM_BUDGET_GB` to stay below it.
- **Divergence recorded, awaiting sign-off:** ADR018 now lists the platform import budget beside the per-workspace storage cap.
- **Found by /simplify and fixed:** the copy could write a different image than the one checked (a manifest labeled image at the check and index at the copy); it now writes only the checked digest. A config blob's URLs are refused too. One workspace may hold only a quarter of the budget.

