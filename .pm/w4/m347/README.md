# w4 · m347 — A new service's credential for the shared registry works without restarting the registry, so a first build starts in seconds, not minutes

**Worker:** worker4 **Goal:** a new service's per-App registry credential is accepted as soon as it is written. A first build or a first registry-hosted rollout then starts as fast as any later one, and creating a service never restarts the registry that every other tenant is pushing to and pulling from. **Status:** todo (t001–t004 done; t005, the production replay, runs after the deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Measure activation on the mock cluster: the wait under concurrent new Apps, and what a bounce does to in-flight pushes and pulls — **DONE** | 45m | — |
| [t002](done/t002.md) | Activate a per-App credential without a registry restart, and amend ADR022 — **DONE** | 90m | t001 |
| [t003](done/t003.md) | Simplify — **DONE** | 15m | t002 |
| [t004](done/t004.md) | Test coverage — **DONE** | 30m | t002 |
| [t005](t005.md) | Closeout (live replay on production) | 30m | t003, t004 |

## Outcome (t001–t004, 2026-11-14)

### t001: activation measured on the mock cluster

Local CAPD mock cluster, zot `v2.1.22` at the production digest. A throwaway htpasswd user and ACL entry were written into the live `zot-htpasswd` and `zot-config` Secrets, then probed through NodePort 30500 every 0.5 s (`GET /v2/<repo>/tags/list`). zot was never restarted: the pod's UID and restart count were the same before and after.

| Case | Accepted after | Notes |
| --- | --- | --- |
| N = 1, Secret write only, no nudge | 16.6 s | the kubelet's periodic pod sync, which can take up to about a minute |
| N = 1, write, then annotation nudge on `zot-0` | 1.2 s | the kubelet re-projects at once; zot's 1 s file poll reloads |
| N = 6, written back to back, each followed by a nudge | all within 4.9 s of the first write | includes the twelve serial Secret writes |
| Isolation: user c0 on c1's repository | `403` | ACL enforced after hot reload |
| Revocation, then nudge | refused (`401`) in 0.8–1.1 s | was "at the next restart" |

- **Restart effect on in-flight push/pull:** not re-measured, because that would have meant restarting the shared mock registry. Round 190's production evidence above stands: an upload failed with `EOF`, then `connection refused` for at least 10 s, longer than skopeo's 3-try budget.
- **Round 180 overrun:** only partly explained. Any restart activates every credential written before it, so one App's credential wait is bounded by:
  - the 30 s grace;
  - whatever remains of the 2 min cooldown;
  - zot's own restart, since the pod must be Ready again before a probe passes;
  - up to one 10 s requeue.
  That is roughly 3 min. The rest of the 6 m 30 s most likely came before 08:46:07, which was not sampled, probably the build-capacity wait (ADR060 §D6). t005's 5 s sampling separates the two. With hot reload, the credential part drops to seconds either way.
- **Upstream proof** of the mechanism: `TestHTPasswdWatcherAtomicReplace` and `TestConfigReloaderKubernetesConfigMapUpdate` (tag `search`) pass at zot `v2.1.22`. `v2.1.18`'s watchers react to `Write` events only, which matches ADR022's original finding.

### t002: what shipped

- **Hot reload replaces the restart.** Every credential write (new App, rotation, revocation, snapshot and sandbox-meter users) ends with `registry.nudgeZot`: a merge patch of `bex.co/registry-credentials-written` on the zot pod. RBAC adds `pods: patch` with `resourceNames: [zot-0]`. The nudge is best effort; if it fails, the kubelet's periodic sync still delivers the write.
- **The probe gate stays.** A pending credential re-probes every 3 s (was 10 s) and re-nudges at most every 5 s.
- **The restart is a last resort that alerts.** A credential still rejected 3 min after its write (was 30 s) restarts zot, still capped at one restart per 2 min. It logs an error and increments `bex_registry_activation_restarts_total`. `RegistryCredentialActivationRestarted` alerts on any increase (promtool test `registry_activation_alerts_test.yml`), the series is in the `bex-operator` scrape keep-list, and the Cluster capacity dashboard graphs it.
- **ADR022 § Credential activation is amended** with the mechanism, the round 180 and 190 evidence, and the measurements above. ADR060 and ADR097 had sentences that assumed restarts; both are corrected.

## Definition of done

All on production in `bex-canary`:

1. **First build:** create a Free Go web service from the dashboard (Public Git URL `https://github.com/bex-co/bex`, Root Directory `examples/hello-go`, Runtime Go).
   - The log shows `==> Preparing registry credentials` for no longer than 10 s, sampled every 5 s from the deploy's creation.
   - When the log narrates no other wait (`==> Waiting for platform build capacity`, `==> Waiting for a build slot`), the first build starts within 60 s.
   - Repeat with three services created within one minute: no first deploy shows the credentials line for longer than 10 s.
2. **Later builds unchanged:** a later deploy of the same service still starts within about 60 s, as round 180's did.
3. **No restart:** creating those services restarts no registry pod (t001's measurement method, read from the platform side).
4. Fixtures deleted afterwards.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 180, 2026-11-12 08:41–08:58 UTC (`muse.env`, workspace `bex-canary`; pin `3d66946c9e16`). The finding is below.
- **Governing doc:** [ADR022 § Credential activation](../../../docs/ADR022-tenant-isolation.md) (w9/m43). It accepted a rate-limited registry restart per new credential to fix the 2026-07-17 `ImagePullBackOff` incident.
- **Goal linkage:** Render parity for the first deploy ("create a service, watch it build") and platform reliability. A shared registry that restarts whenever any tenant creates a service interrupts everyone's pushes and pulls.
- **Expected outcome:** a new service's first build starts as fast as a repeat build, and the registry restarts only for operator-intended changes.
- **Why now:** major. On a busy platform a first build waited 6 m 30 s before it even started, still explained only as "Preparing registry credentials" 4 m 46 s in. Every service anyone creates adds another registry restart.
- **Render parity task omitted:** no REST, GraphQL, MCP or dashboard surface changes. The deploy log already narrates the wait (`waitLineRegistryCreds`).

## Finding (round 180, 2026-11-12)

### Fixture

- Free web service `qa-20261112-r180-go` (`srv-dbqnnca6krac73d83ce0`), created through the dashboard wizard by pasting `https://github.com/bex-co/bex/tree/main/examples/hello-go` into Public Git URL (w4/576's split filled Repository, Branch and Root Directory) and choosing Runtime Go.
- It was deleted afterwards (DELETE 204, then GET 404).

### Probe

Deploy rows (`GET /v1/services/{id}/deploys`) and the deploy page's log, in UTC:

| Deploy | Created | Log while waiting | Build started | Wait |
| --- | --- | --- | --- | --- |
| First, `dep-dbqnnca6krac73d83ceg` | 08:41:21 | `==> Build queued`, then **`==> Preparing registry credentials`** (sampled once, at 08:46:07, 4 m 46 s into the wait); REST `phase: Building`, deploy `queued` | 08:47:51 | **6 m 30 s** in all |
| Second, `dep-dbqnsu26krac73d83eg0`, `POST …/deploys {"clearCache":"do_not_clear"}` | 08:53:22 | `==> Build queued` | 08:54:21 | 59 s |

- The first deploy went `live` at 08:52:40 and served 200.
- A third deploy, `POST …/deploys` with the same `commitId` `2d30e30` at 08:58:15, sat `queued` with `==> Waiting for platform build capacity`, the cluster build ceiling (ADR060 §D6), still waiting at 09:02:26. It was canceled when the fixture was deleted. So the capacity wait is narrated separately.
- What the first deploy waited on before 08:46:07 was not sampled. Only the part spent on `Preparing registry credentials` belongs to this item.
- Other sessions' first builds sat `queued` at the same time:
  - `qa-20261112-81af-env-rust`, queued from 08:37:29;
  - `qa-20261112-l256-go`, queued from 08:35:54.

### Root cause

- **The registry cannot load a new credential while it runs.**
  - Zot loads htpasswd and `accessControl` only at process start (ADR022 § Credential activation, verified against v2.1.18; the pin is now `v2.1.22`, `deploy/gitops/base/zot.yaml:27`).
  - The operator therefore holds a new App before its build Job (`app_controller.go:1122-1134`) or its first registry-hosted rollout (`deployRegistryGate`, `:2928-2935`) in `awaitRegistryCredActive` (`:6731-6745`). That sets `RegistryCredsPending` and requeues every 10 s.
  - The backend narrates the hold as `==> Preparing registry credentials` (`lego/backend/internal/logs/progress.go:170`, `:177-188`).
- **Activation is a registry restart, rate-limited across the platform.** `EnsureActive` probes zot with the App's credential, and past `defaultActivationGrace = 30s` deletes the zot pod, at most once per `defaultBounceCooldown = 2m` across all Apps (`lego/operator/internal/registry/verify.go:46-47`, `recordRejection` `:149-170`).
- **Consequences:**
  - Every new App waits at least the grace, plus whatever is left of the platform-wide cooldown.
  - Zot is one StatefulSet replica with a PVC (`zot.yaml`), so each bounce takes the only registry down for every tenant's pushes and pulls.
- **Not traced:** how much of the 6 m 30 s was the credential wait. If all of it, it exceeded one grace plus one cooldown (2 m 30 s). Candidates:
  - other Apps' credentials written after a bounce began, needing the next one;
  - the zot pod's restart time;
  - the 10 s requeue interacting with the cooldown;
  - a platform-capacity wait before 08:46:07, which t001 separates by sampling the log every 5 s.
- **Unverified:**
  - Whether a bounce makes an in-flight push or pull fail. Reasoned from the single replica, not probed; t001 measures it.
  - Whether zot `v2.1.22` can reload htpasswd or `accessControl` from a regular file write. ADR022's test used a Kubernetes Secret's symlink swap. t002 checks it first.

### Not this item

- Round 179's image deploys from Docker Hub (`traefik/whoami`, `busybox`) waited 1–4 min with REST `phase` empty. Those images are not registry-hosted (`registryHosted`, `app_controller.go:2192-2194`), so `deployRegistryGate` does not hold them. That wait is recorded on w21/012, cause unverified.

### Dedupe

- w9/m43 (done) introduced the bounce.
- w8/m156 (open) restarts zot once to load a retention config; that is a different trigger.
- `grep -rli "bounce\|htpasswd\|Preparing registry credentials" .pm` finds no open item about the activation wait.
- Not deploy lag: nothing after `3d66946c9` touches `registry/verify.go` or `awaitRegistryCredActive`.

### Also verified this round (all pass)

- **w4/576:**
  - The wizard split the `/tree/` URL into Repository `https://github.com/bex-co/bex`, Branch `main`, Root Directory `examples/hello-go` and Name `bex`.
  - `…/blob/main/README.md` is marked invalid ("Use the repository URL, not a file page.", with `aria-invalid=true`), and Deploy Service is disabled.
  - The REST refusal waits on w4/595, which is blocked on Render's answer. `POST /v1/services?dryRun=true` still answers 200 for both URLs.
  - The DoD's "hello from bex" is a wrong expectation: `examples/hello-go` answers `OK` unless `MESSAGE` is set (`main.go:18-21`). The service answered `OK`.
- **w4/582:** a native build narrates `==> Uploading build...`, `==> Uploaded in 23s`, then `==> Build successful 🎉`, in that order.
  - Its cache-reuse line was not verified. The repeat build on a new commit (`2d30e30`) ran the command in 11 s, with no "Reusing the cached result" line.
  - The same-commit repeat waited on platform capacity until the fixture was deleted.
- **Seen, not filed:**
  - The Public Git URL tab runs no runtime detection: `use-new-service-form.ts:111-116` passes `repo: null` outside the GitHub tab, and `repoRuntimeDetection` answers `null` for `bex-co/bex`. The wizard kept Runtime Node with `npm install` / `npm start` for a Go folder. Render's behavior there is unverified.
  - A Go build logs `==> Using Node 24 (default)`, as Python builds did in earlier rounds.

## Evidence — QA loop round 190 (2026-11-12, pin `a5be11e4e517`): an in-flight push failed while the registry was unreachable

This answers part of t001's step 2 from the customer side. The platform side (which App's credential, whether a bounce) was not read.

- **Fixture:** static site `qa-20261112-r190-st` (`srv-dbqq04a8vjhs73do9e9g`, `octocat/Spoon-Knife`), deleted afterwards (DELETE 204, then GET 404).
- **Deploy:** its first build with a build command, `dep-dbqq251n6kgc73837380`, created 11:20:52. The build log shows no `==> Preparing registry credentials` line.
- **Build log (UTC):**

  | Time | Line |
  | --- | --- |
  | 11:21:53 | `==> Running build command …` |
  | 11:22:22 | `==> Uploading build...` |
  | 11:22:31 | `Failed, retrying in 1s ... (1/3). Error: writing blob: Put "http://zot.bex-registry.svc:5000/v2/<ws>/<ws>-qa-20261112-r190-st/blobs/uploads/…": EOF` |
  | 11:22:33, 11:22:36 | `(2/3)`, `(3/3)`: `pinging container registry zot.bex-registry.svc:5000: Get "http://zot.bex-registry.svc:5000/v2/": dial tcp 10.106.213.240:5000: connect: connection refused` |
  | 11:22:41 | `level=fatal msg="…connection refused"` |
  | 11:22:56 | `==> Using Node 24 (default)`: the Job ran the whole build again, with no line saying why |
  | 11:23:58 – 11:24:11 | `==> Uploading build...`, `==> Uploaded in 13s`, `==> Build successful 🎉` |
  | 11:24:44 | `==> Your service is live 🎉` |

- **What this shows:**
  - The single zot replica stopped answering mid-upload. The upload hit EOF, then the Service's ClusterIP refused connections for at least 10 s, consistent with its pod being deleted. skopeo's three retries span 10 s, which is shorter than a pod restart, so the push failed. The build then succeeded only because the Job's backoff reran the whole build.
  - The deploy took 3 m 14 s (the deploy page's Duration) instead of the ~1 m 20 s of this site's next build (11:26:52 → 11:28:15, `Uploaded in 9s`).
- **Unverified:**
  - That a credential-activation bounce (`recordRejection`) caused this outage, rather than another restart. Other sessions were creating services in the same minutes (`qa-20261112-1cee-web` was live in `bex-canary`).
  - Whether pulls failed in the same window.
- **For t002's guarantee:** a registry restart that any tenant's new service can trigger fails other tenants' in-flight pushes. Removing the bounce removes that failure class. Until then, the push's retry budget (3 attempts over 10 s) is shorter than a zot restart.
- **Separate, not this milestone:** the raw skopeo records that name `zot.bex-registry.svc:5000` and `10.106.213.240` are w8/286 (`a177508f3`, on `main` after this pin). Deploys wait for the 6-hour pin age (`.github/workflows/deploy.yml:20-24`).

## Evidence — QA loop round 201 (2026-11-12, pin `f018ce9455c8`)

- **Another push met an unreachable registry.** A native Node build (`qa-20261112-r201-commit`, `dep-dbqt0t2mfp7s73dv7qb0`) began `==> Uploading build...` at 14:44:18. It logged `retrying (1 of 3)` at 14:44:19 and `(2 of 3)` at 14:44:21, then `Uploaded in 22s` at 14:44:40. The upload took 22 s, against round 190's 9–13 s.
- **Unverified:** whether a credential-activation bounce caused it. Other sessions were creating services (`qa-20261112-6aa7-sd` was present), and this service's own first build at 14:38–14:42 had waited about a minute in `queued` before building.
- **The narration is the tenant-facing text** since w8/286. The retry budget held this time, but the round 190 outage (at least 10 s) had exhausted it.
- **Correction to round 201's entry:** the push retries at 14:44:19–21 happened while the platform rolled out pin `ca7fbbe606c8` (commit `8e18e630f` at 14:41:54), so a platform rollout is a candidate cause beside a credential-activation bounce. Neither was confirmed.
