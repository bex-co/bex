# w1 · m57 — Codify static-site's manual prod fixes into gitops

**Worker:** worker1 **Goal:** the three hand-applied `kubectl` fixes keeping prod static sites alive (re-minted `bex-registry-pull`, the static-server ClusterRoleBinding, the hand-patched static-server Deployment) become git-owned — Argo-managed or bootstrap-codified — so a redeploy can no longer silently regress them **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                       | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Verify current prod state of the three hand-applied fixes; snapshot what still drifts                                                        | 30m | —          |
| t002 | Argo-manage ONE static-server from `config/staticserver` (placed per the Ingress topology, `BEX_BASE_DOMAIN` in `bex-static-config`)          | 45m | t001       |
| t003 | Codify `bex-registry-pull` custody; drop the hand-applied ClusterRoleBinding once the shipped manifests supersede it                         | 30m | t001       |
| t004 | Drift-proof: `scripts/samples-lifecycle.sh static-site` stays green through an operator/platform redeploy                                    | 30m | t002, t003 |
| t005 | Simplify pass over the manifests/scripts this milestone changed                                                                              | 20m | t004       |
| t006 | Test coverage: pin the codified invariants (single static-server, env completeness, pull-secret identity) in an automated check              | 30m | t004       |
| t007 | Closeout                                                                                                                                     | 15m | t006       |

## Definition of done

The evidence line from `w9/014`, verified live on prod: Argo shows the static-server app **Synced/Healthy** tracking the CI image; `kubectl` shows **one** static-server, config-complete (`BEX_BASE_DOMAIN` present via `bex-static-config`), with no hand-applied residue (no stale `bex-puller` identity in `bex-registry-pull`, no orphan hand-applied ClusterRoleBinding, no unmanaged `bex-system/bex-static-server` duplicate); and `scripts/samples-lifecycle.sh static-site` stays **GREEN through an operator/platform redeploy** — the drift scenario that motivated the note.

## Source + Goal linkage

- **Source:** promoted from `w9/014` (filed at the w9/m44 closeout merge 2026-07-18, from `w9/done/012.md`'s three manual prod interventions); handed to w1 by `/pm-brainstorm more work for w1` 2026-07-30 — w9 is drained and platform infra is w1's lane (cross-workstream placement precedent: m37, m44). The note moved to `w9/done/014.md` with a cross-ref.
- **Goal linkage:** [docs/ADR029-static-sites.md](../../../docs/ADR029-static-sites.md) (static sites reliably served) + [docs/ADR001-go-and-gitops.md](../../../docs/ADR001-go-and-gitops.md) (platform components are GitOps-owned in `deploy/gitops`, never hand-applied).
- **Expected outcome:** the next operator/platform redeploy cannot silently break prod static sites; the static-server tracks the CI image through Argo instead of a hand-patched Deployment.
- **Why now:** every redeploy since 2026-07-18 has been one Argo sync away from regressing all three fixes — a standing drift bomb on a production-serving feature.
- **Render parity omitted:** pure platform infra — no REST/GraphQL/MCP/UI surface change; the standing closing tasks are Simplify → Test coverage → Closeout only.
