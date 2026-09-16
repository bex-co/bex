# w3 · m85 — Custom-domain certificate reason: tell the tenant why "Pending" is stuck

**Worker:** worker3 **Goal:** a tenant whose custom domain sits in "Pending" can read, on the domain row itself, the exact reason cert-manager cannot issue the certificate — and what to change to fix it. **Status:** done 2026-09-15

## Tasks (in order)

| id   | title                                                                             | est | depends_on |
| ---- | --------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Read the ACME failure reason next to `domainCertificateReady` — **DONE**           | 60m | —          |
| t002 | Project `certificateReason` across REST / GraphQL / MCP — **DONE**                 | 45m | t001       |
| t003 | Grant bex-api read access to cert-manager Challenge/Order/Certificate — **DONE**   | 30m | t001       |
| t004 | Dashboard: the pending certificate badge explains itself — **DONE**               | 60m | t002       |
| t005 | Docs + Render-parity record — **DONE**                                            | 20m | t002, t004 |
| t006 | Simplify — **DONE**                                                               | 20m | t004       |
| t007 | Closeout — **DONE**                                                               | 10m | t005, t006 |

## Definition of done

`DomainView` carries a nullable `CertificateReason` derived from the live ACME objects for the host's certificate — the `Challenge`'s `status.reason` first (the field that actually names "wrong status code '404'"), then the `Order`'s failure state, then the `Certificate`'s Ready-condition message — and it is populated **only** while the certificate is not yet issued. The reason reaches all three API surfaces (`certificateReason` on REST's `customDomain`, the GraphQL `CustomDomain` type, and the MCP projection) and the dashboard's `CertificateStatusBadge` renders it as a hint beneath a pending row, together with the ADR005 fix-it text (DNS-only, ALIAS/CNAME to `<name>.onbex.co`). A cluster with no cert-manager CRDs, or a bex-api without the new RBAC, degrades to today's behavior (empty reason, no error) rather than failing the read.

## Source + Goal linkage

- **Source:** inbox note [`w3/037`](../done/037.md) — `blockeden.xyz` sat in "Pending" for 25 days because the apex is Cloudflare-proxied and the HTTP-01 challenge answers 404. The reason existed in the `Challenge` object and in the `TenantCustomDomainCertNotReady` alert, but the tenant saw only a clock icon.
- **Goal linkage:** pillar 1 (the Render-alternative core) — custom domains are table stakes, and a silent 25-day stall is the failure mode that makes a tenant leave.
- **Expected outcome:** a tenant self-diagnoses a stuck custom domain from the dashboard, without an operator reading cluster objects for them.
- **Why now:** the diagnosis is already computed by cert-manager and already alerted on operator-side (`prometheus.yaml`'s info-level `TenantCustomDomainCertNotReady`, routed to the null receiver precisely because it is the tenant's own DNS). Only the tenant-facing projection is missing, so this is the cheap half of an already-understood problem.
- **Render parity:** additive. Render surfaces no `certificateReason` field on `customDomain`, so this is a bex extension in the same class as the existing `dnsRecord`/`ownershipDnsRecord` additions — a safe superset, never a changed Render field.

## Outcome (2026-09-15)

- `domainCertificateReason` (`lego/backend/internal/apps/domains.go`) resolves Challenge `status.reason` → failed Order → Certificate not-Ready condition, reading cert-manager unstructured on the `postgres/recovery.go` precedent. Every failure path returns an empty string, so a cluster with no cert-manager CRDs — proven by a test using the plain fake client, which has no such kinds registered — degrades to exactly the pre-m85 behavior.
- Populated only while the certificate is pending, on both the storeless (`domainView`) and managed-claim (`domainClaimView`) projections. An ownership-pending claim returns before the lookup: it is never projected into `spec.hosts`, so no Ingress and no ACME exchange exist to explain.
- `certificateReason` ships on REST (`omitempty`), GraphQL (nullable), and MCP (shares the REST wire type), with a three-surface parity test asserting one identical reason and a companion test proving an issued domain's wire shape is unchanged.
- RBAC is read-only and namespace-scoped on both `bex-tenant-api` and the legacy `bex-api-apps` Role. `lego/operator/config/api/rbac.yaml` was deliberately left alone — it is a ClusterRole, so a grant there would be the cluster-wide authority this milestone forbids (recorded in t003).
- Dashboard renders a `CertificateReasonHint` row beneath a blocked domain — cert-manager's text as plain escaped React content plus the ADR005 DNS-only/`<name>.onbex.co` fix-it line — with en/zh keys and three tests (with-reason, without-reason unchanged, issued shows nothing).

**Verification:** `go test ./internal/apps/...` green for every certificate-reason test; `bash scripts/gitops-validate.sh` PASS; dashboard `yarn typecheck && yarn lint && yarn test` green (419 files / 3348 tests). GraphQL codegen was run offline and reconciled — the `definitions.ts`/`schema-types.ts` diff contains nothing but the new field.

**Honestly owed:** no live probe. The behavior is proven on deterministic suites only — reproducing a real stalled ACME challenge needs either prod (off-limits without authorization) or a local cluster with cert-manager and a genuinely unreachable host. The `blockeden.xyz` fixture that motivated this is a real production domain, so a future `/qa-find-bugs` or prod-authorized session can confirm the reason renders on the live row.

**Pre-existing failures observed, NOT caused by this milestone** (both reproduced at HEAD before any edit): `TestShippedExampleBlueprintsValidate` fails locally because the repo-walking test picks up `bex-desktop/eden-cms-v2/bex.yml`, a **gitignored** local scratch file (`.gitignore:40`) that CI never sees; and `TestMigrationNumbersAreUnique` fails on a real duplicate `0121` migration number shared by `w4/089` and `w2/m96` — flagged to the user, not repaired here (it is another workstream's code and the safe renumber depends on prod's applied `schema_migrations` version).

## Notes / risks

- bex-api must not gain cluster-wide cert-manager authority: the grant belongs on the per-tenant `bex-tenant-api` ClusterRole (bound per `<ws>` namespace by the NamespaceReconciler), matching how the CNPG `backups`/`objectstores` reads were scoped in w6/m117.
- The reason is operational text from cert-manager, surfaced to a tenant. It describes the tenant's own DNS and carries no other tenant's data, but it must never be rendered as HTML.
- cert-manager types are not in the backend's scheme; read them unstructured, the way `postgres/recovery.go` reads CNPG.
