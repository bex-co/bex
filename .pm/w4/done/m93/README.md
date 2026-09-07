# w4 · m93 — Include linked environment groups in native builds

**Worker:** worker4 **Goal:** A linked group's environment value reaches a native build command, with service-local overrides preserved. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Project the effective group and service environment into native builds | 45m | w4/m92/t002 — **DONE** |
| t002 | Preserve build-secret ownership and cleanup across shared groups | 40m | t001 — **DONE** |
| t003 | Render parity | 20m | t001, t002 — **DONE** |
| t004 | Simplify | 15m | t003 — **DONE** |
| t005 | Test coverage | 35m | t003 — **DONE** |
| t006 | Closeout | 15m | t004, t005 — **DONE** |

## Outcome (2026-09-06)

Shipped. Native builds now receive the same effective Secret-backed environment the running pod does: `projectNativeBuildEnv` iterates the exact runtime source list from `envFromSources` (linked group Secrets in `spec.envFromSecrets` order, each optional; the service's own Secret last, required) and merges them into one App-owned `<name>-native-env` Secret beside the build Job, transported through the existing single BuildKit secret mount — so the m92 byte-exact codec is untouched and literal `spec.env` entries still win over every Secret source (preparer append order + decoder later-wins export = Kubernetes env precedence).

- **t002 ownership/cleanup:** the merged destination is a deterministic per-App name — never the shared `<evg>-env` — so two Apps linking one group get isolated inputs (`TestProjectNativeBuildEnvSharedGroupIsolation`) and the group source is never adopted or relabeled; `checkOwnedArtifact` refuses a foreign App-lifetime destination without clobbering it; the wholesale `Data` replace means an unlinked group's keys cannot linger; `"native-env-secret"` joined the artifact-component allowlist so the inventory reclaim sweeps it, and `nativeEnvSecretName` joined `knownBuildSecretNames` for the cross-namespace name-targeted finalizer loop.
- **Fingerprint:** `spec.EnvFromSecrets` now enters the native artifact identity (and `AppSpecIdentityClasses` classes it `IdentityArtifact | IdentityRelease`), so a direct CR edit linking/unlinking a group rebuilds exactly like an API link's `restartedAt` bump; Dockerfile/kpack artifacts are unaffected (guard-tested). Only native Apps with linked groups re-fingerprint on rollout — precisely the ones whose build env was wrong.
- **t003 parity:** all group-link surfaces (REST/GraphQL/MCP link + Blueprint seam + create-time prelinks) converge on the same App projection the operator reads, so no adapter change was needed; ADR013 gained a "Native build-time environment (w4/m93)" paragraph; ADR004:175's "native builder inputs also change its artifact" now covers group links consistently. Render's documented group distribution + service-local precedence is matched; upstream native build execution still not probed against an authenticated Render account.
- **t004/t005:** simplify pass single-sourced the source list through `envFromSources` (review found the hand-rolled duplicate); 8 new controller tests cover the group-only regression, merge ordering, optional-absent vs required-missing, protected-source refusal, foreign-owner refusal, shared-group isolation, unlink replacement, and the artifact-identity change. Operator suite (incl. envtest), backend suite (types identity table changed), and `make lint` all green.
- **Limit (t006):** the live linked-group and own-value-control journeys against the deployed fix require this ship to roll out — deferred to the next QA pass, same as m92's closeout. The filing pass's QA fixtures were already deleted at filing time.

## Definition of done

- Repeat t001's public Go service journey with no own MESSAGE and a linked group containing MESSAGE=qa-group-value. After the link-triggered deploy reaches Live, the actual build output says QA_BUILD_MESSAGE=qa-group-value, the running process says QA_RUNTIME_MESSAGE=qa-group-value, and external curl returns HTTP 200 with that body. A fresh reload and Manual Deploy → Deploy latest commit preserve that outcome.
- With that group still linked, set the service's own MESSAGE=qa-own-value. After the resulting deploy reaches Live, build and runtime both report qa-own-value and the public response contains qa-own-value. The group's API value remains qa-group-value.
- Delete both disposable resources; fresh service and environment-group lists no longer contain them. Preserve the workspace's preexisting resources.
- Complete t002/t005's mechanism and sibling verification with recorded evidence. These are implementation checks, not claims that the live hunt exercised those paths.

## Source + Goal linkage

- **Source:** continuous live `$qa-find-bugs w4`, 2026-09-06, loop pass 2. Major finding with two real builds, fresh-page reproduction, complete API probes, and own-value control in t001. Local evidence `.playwright-mcp/qa-groupbuild-missing-1.png` and `qa-groupbuild-own-control-1.png` was verified to exist; screenshots are gitignored, so the durable observations are also below.
- **Goal linkage:** ADR008 core hosting and Render compatibility; ADR004 native build contract, ADR013 secret custody, ADR018 Environment groups, ADR043 tenant namespaces.
- **Expected outcome:** moving configuration into a linked group no longer removes it from native compilation or asset generation while silently retaining it at runtime.
- **Why now:** observed on production; a group link and a later manual deploy both build without the saved key. Plain single-line values isolate this from the separately filed byte codec bug.
- **Render parity included:** existing REST/GraphQL/MCP group writers and dashboard links expose this environment promise. [Render documents group distribution, link-triggered deployment, and guaranteed service-local precedence](https://render.com/docs/configure-environment-variables). Exact upstream native build execution was not probed this run; do not claim an authenticated Render comparison.
- **Dedupe:** searched open and done board for group/build, native/group, EnvFromSecrets, and RuntimeEnvSecret; scanned all 13 currently open milestone paths and DO_NOT_DO. w6/done/m51/t008 addressed deploy-history tracking on group links, which passed here. w2/done/m73 addressed scoped editing/transactions, not native build projection. w6/done/m45 covered PATH and create-time env-store visibility. w4/m92/t002 explicitly excludes group projection; this is a separate input omission and waits for its codec work to avoid conflicting edits. Main 3488ce509 retains the omission; targeted history has no group build fix. No anti-goal or intentional group-at-runtime-only native contract found.
