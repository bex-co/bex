# rt-unify-duplicates ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

(none open — the 2026-11-18 postgres/keyvalue free-slot quartet unified into `core.FreeSlot*` + `core.FreePlanID` on 2026-11-19)

## Skipped

- 2026-11-26 backend `namespaceFor` vs operator `canonicalNamespace` re-verified — both files changed since 2026-11-23 but the split stands (desired-state projection vs live-object membership check, different signatures and purposes)
- 2026-11-23 postgres/keyvalue `CheckFreeLimit`/`FreeSlots`/`refuseSecondFree*` thin per-kind adapters — shared rules already live in `core.FreeSlot*`; merging the typed projections further is below the bar
- 2026-11-23 backend `core.DecodeJSON` vs operator `httpclient.DecodeJSON` — different signatures (`*http.Request` vs `io.Reader`), different layers, not copies
- 2026-11-23 backend `objectgw.NewCachedResolver` vs operator `staticserver.NewCachedResolver` — different domains (S3 upstream vs k8s-reader base domain), not copies
- 2026-11-23 `backend/internal/id` vs `operator/internal/identity` re-verified — still different domains (minted typed resource ids vs workspace-scoped sink names)
- 2026-11-23 backend `namespaceFor`/`ownedBy` vs operator `canonicalNamespace` re-verified — still complementary halves of the App CR boundary, not copies
- 2026-11-21 `FilterOf` in backend audit/deploys/events — intentionally different strictness (strict 400 vs permissive-onto-default-window), each documented at its site; per-domain rules, not copies
- 2026-11-21 operator `JobName` in publish/predeploy/build — thin per-kind delegates to `appv1alpha1.*JobName`, already unified at the contract layer
- 2026-11-21 `Register(mux, h)` in backend settingsguard/opsrole/identityclaims — per-handler readiness gates (different required fields and methods); 5-line shapes below the bar
- 2026-11-19 postgres/keyvalue restart-free helpers (`restartFreePending`/`markRestartFree`/`annotationRestartFreeGeneration`) — intentionally different per-datastore restart semantics (CNPG cert-reload and allow-list layers vs Valkey eviction policy, `externalAccessRestarts` and the SNI-proxy layers); the field allow-lists must stay per datastore, comments already cross-reference
- 2026-11-18 `pg-sni-proxy`/`kv-sni-proxy` mains — deliberately parallel binaries (Postgres SSLRequest preamble vs plain TLS); twins maintained with backport comments, merging two mains is out of the bar
- 2026-11-18 `bandwidth`/`buildminutes` Sweepers — parallel enforcement policies with intentionally different stages (spend-limit stage only in buildminutes); shared sweep primitives already live in `core`/`store`
- 2026-11-18 `sandboxexec`/`shellticket` Mint+Verify — thin shells over the canonical `codec`+`hmacticket` envelope; per-claims required-field checks are inherent
- 2026-11-18 `events`/`webhooks` eventvocab alias blocks — identical sets over canonical `eventvocab.DatastoreAuditTypes`; alias convention per #181
- 2026-11-17 `bex.co/*` OpenSandbox vocabulary, remaining single-site backend-only keys (`owner`, `network-policy`, `plan`, …) — no cross-module copy left after the 2026-11-23 `LabelSandboxID`/`LabelAgentSession`/`LabelTenant`/`LabelNodePool` move to `lego/types/v1alpha1`
- 2026-11-17 agentsession/sandbox `bex.co/workspace` vs `v1alpha1.LabelWorkspace` (`app.bex.co/workspace`) — different domains (OpenSandbox create metadata vs App CR labels), not copies
- 2026-11-16 `*_test.go` fixtures hand-writing the unified label values — fixtures model external input; aliases keep same-package tests compiling unchanged
- 2026-11-16 `lego/operator/config/*` YAML hand-writing label values (`app.bex.co/component` etc.) — manifests cannot import Go constants
- 2026-11-15 `backend/internal/id` vs `operator/internal/identity` — different domains (minted typed resource ids vs workspace-scoped sink names)
- 2026-11-15 identity `secretName` vs `k8sname.stable` — unifying would rename live Secrets (253-char subdomain fast path vs 63-char always-hashed label)
- 2026-11-15 backend `namespaceFor`/`ownedBy` vs operator `canonicalNamespace` — complementary halves of the App CR boundary, not copies
- 2026-11-15 dashboard id regexes — per-feature validators in another language
- 2026-11-15 controller vs predeploy `startErrorMessage` — the two wordings (pre-deploy vs container start) are intentional; only the shared regex was unified (#169)
- 2026-11-13 build-source authorization composition in `apps` and `deploys` — an exported shared method becomes the recorded audit verb (`core.callerVerb` only skips unexported helpers), failing `deploys.TestBuildVerbsAuditUnderTheirOwnNames`; audit attribution wins (870834a3a)
