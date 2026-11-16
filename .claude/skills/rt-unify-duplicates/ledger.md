# rt-unify-duplicates ledger

Earlier runs' **Deferred** and **Skipped** findings, read at the start of every run and updated by its change (see the ground rules in [SKILL.md](SKILL.md)). One line per finding, newest first, at most 40 entries.

## Deferred

- 2026-11-15 App label keys (`app.bex.co/app`, `app.bex.co/workspace`, `app.bex.co/revision`, `app.bex.co/component`, `app.bex.co/environment-id`, `app.bex.co/network-isolation`, `bex.co/app-id`) — hand-spelled in backend `core/base.go` and in operator `controller/app_controller.go`, `execution/security.go`, `predeploy`, `drainguard`, `egressmeter`, `cmd/{kv,pg}-sni-proxy`, `cmd/registry-migrate`, plus backend `apps/clonesecret.go`; move them to `lego/types` beside `renderenv.LabelAppID` and migrate every copy (PR #170 was closed for guarding them with mirrored literal tests instead)

## Skipped

- 2026-11-15 `backend/internal/id` vs `operator/internal/identity` — different domains (minted typed resource ids vs workspace-scoped sink names)
- 2026-11-15 identity `secretName` vs `k8sname.stable` — unifying would rename live Secrets (253-char subdomain fast path vs 63-char always-hashed label)
- 2026-11-15 backend `namespaceFor`/`ownedBy` vs operator `canonicalNamespace` — complementary halves of the App CR boundary, not copies
- 2026-11-15 dashboard id regexes — per-feature validators in another language
- 2026-11-15 controller vs predeploy `startErrorMessage` — the two wordings (pre-deploy vs container start) are intentional; only the shared regex was unified (#169)
- 2026-11-13 build-source authorization composition in `apps` and `deploys` — an exported shared method becomes the recorded audit verb (`core.callerVerb` only skips unexported helpers), failing `deploys.TestBuildVerbsAuditUnderTheirOwnNames`; audit attribution wins (870834a3a)
