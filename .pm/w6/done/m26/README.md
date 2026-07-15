# w6 · m26 — Official CLI `services instances`: live service-instance listing

**Worker:** worker6 **Goal:** make the unmodified Render CLI's `render services instances <service>` command work against bex by serving Render's authenticated `GET /v1/services/{serviceId}/instances` contract from the service's live Kubernetes Pods. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Project live Kubernetes Pods into Render service instances | 45m | — — **DONE** |
| t002 | Register the Render REST route and exact error semantics | 35m | t001 — **DONE** |
| t003 | Add authorization, lifecycle, and wire-shape regressions | 40m | t002 — **DONE** |
| t004 | Verify the unmodified CLI live and add the harness guard | 35m | t003 — **DONE** |
| t005 | Render parity | 30m | t004 — **DONE** |
| t006 | Simplify | 30m | t005 — **DONE** |
| t007 | Test coverage | 40m | t005, t006 — **DONE** |
| t008 | Closeout | 15m | t007 — **DONE** |

**Completed 2026-07-14.** `ListInstances` authorizes through `AuthorizeApp(can_view)`, projects eligible Deployment Pods into the exact Render two-field shape, and serves both `/v1/services` and `/v1/apps` aliases. The unmodified CLI passed JSON/text, suspended-empty, and missing-service checks twice against rebuilt `dev-6`; the self-cleaning full compatibility verifier passed with no leftover resources. Backend build/tests and `make lint-backend` are green; ADR006, ADR018, the CLI ledger, and the retired `w2/008` ownership note agree on the final REST-only scope.

## Definition of done

Against a live one-replica service, `./cli/render services instances <service> -o json` exits zero and returns a non-null array whose entries decode as Render `ServiceInstance` objects (`id`, `createdAt`); suspended/scaled-to-zero services return `[]` with HTTP 200; nonexistent and cross-workspace services preserve bex's non-leaking authorization/error semantics. The route is covered by meaningful fake-client tests, the real CLI path is guarded by `scripts/cli-compat.sh verify`, and `docs/cli-compatibility-checklist.md` marks the row ✅ with reproducible evidence.

## Source + Goal linkage

- **Source:** user `$pm for w6`, 2026-07-15, from the live-reproduced `services instances` ✖ row in `docs/cli-compatibility-checklist.md`; transfers the last open slice of `w2/008` after that note's `Service.autoDeploy` and `Deploy.image` bugs shipped.
- **Goal linkage:** Render API/official-CLI compatibility (ADR008's Render-alternative core and ADR006's one-core/thin-adapters contract).
- **Expected outcome:** operators can enumerate a service's actual running instances with Render's official CLI unchanged; empty, missing, and unauthorized cases are deterministic and regression-tested.
- **Why now:** this is the only independently confirmed, directly fixable REST-route failure remaining in the CLI's core service census, and the route is a small, bounded projection over labels bex already places on every workload Pod. Render parity task included because this adds a tenant-facing REST surface; GraphQL/MCP/UI equivalence must be checked and any intentional REST-only scope documented rather than assumed.
