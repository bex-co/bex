# w8 · m23 — Blueprint resource ownership: stop silent cross-blueprint overwrite

**Worker:** worker8 **Goal:** a resource adopted or created by one blueprint carries an ownership marker, and a second blueprint whose manifest names the same resource gets a loud coded conflict (with an explicit takeover confirmation) instead of silently adopting and overwriting it — safer than Render's documented "unpredictable behavior, last sync wins", with the divergence documented. **Status:** done

## Tasks (in order)

| id   | title                                                                        | est | depends_on |
| ---- | ----------------------------------------------------------------------------- | --- | ---------- |
| t001 | Ownership record: mark resources with the blueprint that manages them — **DONE**         | 45m | —          |
| t002 | Conflict detection + coded refusal with explicit takeover confirmation — **DONE**        | 45m | t001       |
| t003 | Render parity check (conflict shape across surfaces; divergence documented) — **DONE**   | 30m | t002       |
| t004 | Simplify (`/simplify` over the changed code) — **DONE**                                  | 30m | t003       |
| t005 | Test coverage (ownership stamping, conflict refusal, takeover, disconnect) — **DONE**    | 45m | t003       |
| t006 | Closeout — **DONE**                                                                      | 15m | t005       |

## Definition of done

After blueprint A creates or adopts a service/database/keyvalue, that resource records A as its managing blueprint (visible on the blueprint's `resources[]` and survivable across syncs). A create/sync of blueprint B whose manifest names the same resource fails pre-write with one coded conflict error (naming the resource and owning blueprint) identical across REST/GraphQL/MCP, and the dashboard surfaces it; passing the explicit takeover confirmation transfers ownership to B and proceeds. `DisconnectBlueprint` clears the marker (resources become unmanaged, per Render disconnect semantics). Manually created resources without a marker still adopt freely (unchanged m19-era behavior). Integration tests cover all four paths.

## Source + Goal linkage

- **Source:** blueprint lifecycle-semantics verification 2026-08-16, item 5: resources carry no blueprint-ownership label (`core/base.go` label constants have no blueprint entry), and apply resolves purely by name (`deploy.go` adoption path) — so two blueprints declaring the same name silently fight over one resource, each sync overwriting the other's spec with no signal. Render docs warn "max one Blueprint per resource; multi-Blueprint management ⇒ unpredictable behavior, last sync wins" — Render doesn't enforce either, so refusing is a deliberate, documented improvement, consistent with ADR049's fail-closed philosophy. (Render's name-suffixing on deliberate same-yaml replication is intentionally out of scope — takeover confirmation covers the intent.)
- **Goal linkage:** tenant safety on the blueprint surface (the ADR045→ADR060 lineage's recurring theme: shared sinks resolved by name) + Render parity ledger honesty.
- **Expected outcome:** cross-blueprint clobbering becomes impossible to do accidentally; the failure mode is a clear conflict message instead of mysteriously flapping specs.
- **Why now:** m21/m22 both increase blueprint creation volume (dashboard completion + generate-adopt funnel), which multiplies the chance of two blueprints referencing one resource; the m20 transactional apply (`RunGroupingTx` shape) just made a pre-write check cheap to slot in. Render parity task included: new coded error + `resources[]`/dashboard surface changes across REST/GraphQL/MCP/UI.
- **DO_NOT_DO constraints honored:** no resource deletion added anywhere (disconnect still only clears markers); no excluded features touched.
