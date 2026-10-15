# w1 · m173 — Free web services go to sleep again (auto-hibernate regression since 2026-10-07)

**Worker:** worker1 **Goal:** an idle free web service auto-hibernates after its idle window again, and the production tenant-view canary catches a regression instead of staying red. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                  | est | depends_on |
| ---- | ------------------------------------------------------------------------------------------------------ | --- | ---------- |
| t001 | Reproduce on dev-1: an idle free web App never hibernates; capture the operator line that keeps it awake | 45m | —          |
| t002 | Failing-before test through the real `patchAppMeta` path; bisect the 2026-10-06/07 operator commits     | 45m | t001       |
| t003 | Fix the root cause while keeping w5/096's guarantees                                                    | 1h  | t002       |
| t004 | Canary stage 3 prints the pod's age at query time (FUTURE-MAYBE w3/040)                                 | 20m | t003       |
| t005 | Production: three green scheduled canary runs with a real cold wake; count services that stayed awake   | 30m | t004       |
| t006 | Simplify                                                                                               | 15m | t005       |
| t007 | Test coverage                                                                                          | 30m | t005       |
| t008 | Closeout                                                                                               | 10m | t007       |

## Definition of done

- **The test proves it.** An operator test drives a free web App with no traffic past its idle window through the real metadata-patch path, including a status write earlier in the same pass, and asserts `Hibernated`. It fails with the fix reverted.
- **dev-1.** A free web App with `idleTTLSeconds: 60` and no traffic reaches `Hibernated` and emits `service_hibernated`; a request wakes it and emits `service_woken`.
- **Production.** After the fix is pinned, three consecutive scheduled `ssh-edge-liveness.yml` runs pass all four stages with a stage-1 wake that takes measurable time (not `200 in 0s`) and a fresh `service_woken` event. GitHub issue #87 is closed with the run ids.
- **Impact recorded.** The README records how many free web services stayed Running with no traffic between 2026-10-07 and the fix, read through the API, or states why the count could not be taken.

## Evidence

- `ssh-edge-liveness.yml` (tenant-view canary, service `srv-daif6dsmg29s73d1umvg`, `https://hello-go.onbex.co`) failed **25 scheduled runs in a row**, from `37581343426` (2026-10-07 06:24Z) through `38685083503` (2026-10-15 05:07Z). Every failing run reads `1/4 wake … ok: 200 in 0s` and `FAIL(4/4 events): the canary's events feed is empty for the probe window`. The last green runs (`37542778375` 10-06 22:47Z, `37512467892`, `37425932857`) woke the canary in 9–12s and found a fresh wake event. The service stopped sleeping.
- Platform pins in the window after the last green run: `9c42dbf9d` (pinned 23:14Z), `fd5d2693e` (w5/096, pinned 23:51Z), `1009b751e` (w5/108), `5fe6d5968` (w5/109), `b389580be` (w5/110), `71c6274da` (w5/112, API).
- Lead suspect, unproven: `fd5d2693e` moved `stampLastActive` onto `patchAppMeta`, which patches with `client.MergeFromWithOptimisticLock{}` under the resourceVersion the pass read (`lego/operator/internal/controller/metadata_patch.go:36-44`). `recentlyActive` treats any stamp error as "advancing last-active failed; keeping the service awake" and returns true (`activity.go:185-188`). If the pass's App is always one resourceVersion behind by then, every check conflicts and the service never sleeps.
- GitHub issue #87 ("A workflow is persistently red on main", 2026-10-08) has no comments. Issue #84 auto-closed on 10-15 after a pass that also read `200 in 0s` during a rollout.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w1` 2026-10-15, item 1 (approved by the user the same day); found by the CI/incident sweep and re-verified from the run logs above.
- **Goal linkage:** V0 roadmap 1 (deploy, suspend, sleep on `*.onbex.co`) and 5 (multi-tenant economics): free-tier sleep is what makes dense bin-packing of free services affordable.
- **Expected outcome:** free web services sleep after their idle window again, and the canary's four stages are a working regression alarm.
- **Why now:** a live production regression since 2026-10-07 that may be keeping every idle free web service running, and an alarm that has been red for 8 days hides anything else that breaks.
- **Render parity omitted:** operator-internal mechanism; no REST/GraphQL/MCP/UI shape changes. The behavior restored is Render's own free-tier sleep.
- **Related, not taken over:** `w4/blocked/m164` t003 (live idle journeys) cannot pass until this lands; `w1/blocked/124` is a different state (stuck Deploying). `DO_NOT_DO.md`'s "wakes itself" entry is the opposite symptom.
