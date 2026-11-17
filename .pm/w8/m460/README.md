# w8 · m460 — A static site's first deploy goes live when a publish setting is saved during its build, and an IP rule saved meanwhile takes effect

**Worker:** worker8 **Goal:** saving a static site's publish directory or root directory while its first deploy builds no longer hangs that deploy for ~21 minutes and fails it with a health-gate message. The deploy goes live on the settings it started with, and an IP rule saved in the meantime blocks within seconds, as Render's do. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                            | est | depends_on |
| ---- | -------------------------------------------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Reproduce the hang in a test and name the exact mechanism                                                                        | 45m | —          |
| t002 | A static site's first deploy settles on its own release when a release input is saved during the build                          | 45m | t001       |
| t003 | An IP rule saved while a static site's first deploy is unsettled or failed takes effect on the content it serves within seconds | 30m | t001       |
| t004 | Blast radius: other release inputs, service types, save surfaces and the owed release                                           | 30m | t002, t003 |
| t005 | Render parity                                                                                                                    | 20m | t004       |
| t006 | Simplify                                                                                                                         | 15m | t005       |
| t007 | Test coverage                                                                                                                    | 30m | t005       |
| t008 | Closeout                                                                                                                         | 15m | t007       |

## Definition of done

Verified on production after a roll that carries the change, with bex CLI v0.7.0 or later, in `bex-canary` (a Scale workspace):

1. Create two static sites:

   ```sh
   bex services create --type static_site --name <fresh> --repo https://github.com/octocat/Spoon-Knife --branch main --publish-directory out --build-command 'mkdir -p out out2 && echo m-out > out/marker.txt && echo m-out2 > out2/marker.txt' --confirm -o json
   ```

   While each first deploy reads `build_in_progress`:

   - on the first site, run `bex services update <id> --publish-directory out2 --confirm -o json`;
   - on the second, run `--root-directory sub` then `--root-directory ''`.

2. Each first deploy reaches `live` within the usual ~4 minutes, not `update_failed` after ~21. GraphQL `service { phase }` reads `Running`. The first site serves the publish directory its deploy started with (`/marker.txt` → `m-out`). A following `bex deploys create <id> --wait` publishes `out2` (`m-out2`). The record of what t002 chose (snapshot, or the saved value when the build had not started, w21/051) matches what is served.
3. On a third fresh static site, save `--ip-allow-list 'cidr=203.0.113.0/24,description=doc'` during the first build, together with `--publish-directory`. Within 30 s of the save, a request from outside that range gets `403 Forbidden`, while the deploy is in flight and after it settles.
4. Controls, unchanged:
   - a static first deploy with no mid-build save goes live;
   - a mid-build rename or IP-rule-only save goes live, and the rule answers 403;
   - a non-first deploy with a mid-build root-dir save goes live;
   - a native web service's first deploy with mid-build saves goes live.
5. Delete every fixture by ID and confirm 404s.

## Finding

**Source:** `/qa-find-bugs-cli` sweep `0bbe`, 2026-11-17 07:20–08:10 UTC, production `https://api.bex.co/v1/`, pin `21762a1df1fb`, bex CLI v0.7.0 (Render CLI v2.36.0), human device OAuth, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`, Scale plan). **Severity:** major. A deploy hangs and is then reported failed while the site serves. A rule the owner saved to lock the site down is not enforced for at least 36 minutes. **Attribution:** operator (static-site first release); the exact mechanism is unverified (t001).

### Repro

1. Static site `qa-20261117-0bbe-st` (`srv-dbu00mgmlqgs73cq4hdg`, since deleted) was created at 07:20:58 with `--publish-directory .` and a build command writing `root.txt` and `out/marker.txt`.
2. During its first build (07:21–07:22:38), `bex services update` saved, in order:
   - publish directory `out`;
   - `''`, refused "publishPath must not be empty";
   - `../x`, `/etc`, then `out` again;
   - build command cleared, then restored;
   - auto-deploy off;
   - a build filter;
   - root directory `sub`, then `''`;
   - `--ip-allow-list 'cidr=203.0.113.0/24,description=qa-doc-range'`;
   - a rename.
3. Build log: `07:23:56 ==> Uploaded in 37s` and `==> Build successful 🎉`. The deploy `dep-dbu00mgmlqgs73cq4he0` then read `update_in_progress` from 07:24:06.
4. From about 07:29 the site served the create-time publish: `/` → 200 (the repo's `index.html`), `/root.txt` → `qa-root-0bbe`, `/marker.txt` → 404. Requests came from a client outside `203.0.113.0/24`. GraphQL read `{"phase":"Deploying","publishPath":"out","revision":"rev-1","undeployedChanges":false}`, and `deploy { stallReason }` was empty.
5. 07:42:07: the deploy closed `update_failed` with `failureReason: "the deploy did not become healthy within the health-gate window; check the service logs"`. Phase stayed `Deploying`. At about 07:44, `/` still answered 200 to the same client.
6. Recovery: `bex deploys create srv-dbu00mgmlqgs73cq4hdg --confirm` at 07:54:37 went `live` at 07:58:14, phase `Running`, revision `rev-14`. Only then did `/` and `/marker.txt` answer `403 Forbidden`.

### Bisect

Fresh static sites from the same repo, each with one change:

| Site | Mid-build save | First deploy |
| ---- | -------------- | ------------ |
| `ctl` (`srv-dbu09rbvsrns73ebrpag`) | none | `live` in ~3.5 min |
| `ip` (`srv-dbu09rgmlqgs73cq4hmg`) | IP rule only | `live`; then `403 Forbidden` |
| `nm` (`srv-dbu09rrvsrns73ebrpd0`) | rename only | `live` |
| `pp` (`srv-dbu0bsgmlqgs73cq4hpg`) | `--publish-directory out` → `out2` | `update_in_progress`, served `out` (create-time), `update_failed` at 08:06:10 (created 07:44:50) |
| `rd` (`srv-dbu0bsgmlqgs73cq4hs0`) | `--root-directory sub` then `''` (net unchanged) | `update_in_progress`, `update_failed` at 08:05:54 |
| `ctl`, second deploy | `--root-directory sub` then `''` | `live` |

A native web service's first deploy took mid-build saves of root directory, branch and source in sweep `0594` and went live. So the hang needs a static site, its first release, and a mid-build save of a publish input.

### Cause (unverified, t001)

- Publish path and root directory are release identity inputs: `lego/operator/internal/controller/release_identity.go:62,152` (`RootDir`) and `:89,207` (`PublishPath`). A save of either, or a set-and-clear of either, bumps the App's generation during the first release.
- `reconcileStaticSite` (`app_controller.go:4942-5045`):
  - publishes `rev-1`;
  - records it, then requeues for the first-publish handover (`:5020-5027`);
  - reaches `PhaseRunning`, `ObservedGeneration` and `convergeStaticRouting` only at `:5029-5037`.
- The App stayed `Deploying` on `rev-1` while serving it. So a later pass returns before `:5029`. One lead is the first-release and pending-slot rules in `release_identity.go:743-786` (`sourceUpdatePending`, `firstReleaseReadsSavedSource`, `pendingReleaseOwed`, `requestedReleaseGeneration`). A newer release generation may be owed or required that no deploy request will ever bring, because settings saves don't deploy.
- The IP middleware for a static host converges in `convergeStaticRouting`, which the stuck pass never reaches. ADR018's w32/m419 entry promises that a rule saved over a failed deploy "takes effect within seconds on the release that still serves" for web services. The static path has no equivalent.
- The deploy row is closed by bex-api's gate, not the operator. It gives the generic line `types/v1alpha1/deploy_reasons.go:174` through `failureReasonFor` (`lego/backend/internal/store/reconciler.go:2296`), because the App reports no specific verdict.

### Render

- Render's update-service API says a settings change is not deployed automatically: "Changes will **not** be deployed automatically" ([update-service](https://api-docs.render.com/reference/update-service), read 2026-11-17). An in-flight deploy finishes on what it started with.
- Inbound IP rules: "Your new rules take effect within a few seconds" and "Disallowed IPs are automatically blocked with a `403 Forbidden` response". Static sites are covered on Scale and Enterprise ([inbound-ip-rules](https://render.com/docs/inbound-ip-rules), read 2026-11-17).

### Dedupe

- **w4/515 and w8/235** (done): a static deploy never closed because the publish directory was missing. Here the directory exists, and the trigger is a mid-build save.
- **w32/028** (done): a missing branch on a no-build static site.
- **w32/m419** (done): IP rules over failed image deploys, web services.
- **w21/051** (done): a first release that has not started building reads the saved source.
- No open item matches (`rg` over `.pm` for static first-deploy hangs, `update_in_progress` with static, and mid-build publish saves). No newer operator commit touches static first releases since pin `21762a1df1fb`.

### Unverified

- The exact early return (t001).
- Whether other release inputs reproduce it on a static site: environment variables, branch, headers or routes saved mid-build. Headers and routes are read live, so they probably don't.
- Whether a Blueprint sync or a dashboard save during the build hangs it the same way.
- Whether the workspace and environment IP rule layers are also unenforced on a stuck static site.
- Not filed: an absolute (`/etc`) or `../` publish path is accepted on a built site. The extract step already runs the tenant's own image (`publish.go:614-630`), the publish pod mounts no service-account token, and the guard for the clone-only path sits in `publish.go:466-471`.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` sweep `0bbe` (2026-11-17), a `bex services update` round trip on a static site. Its other flags passed: publish directory set (empty refused), build command clear and restore, auto-deploy, build filters, root directory, IP rules (403 once live), rename. Refused as expected: previews (a non-goal), `--image`, `--health-check-path`, `--plan`.
- **Goal linkage:** ADR008 core hosting and Render parity first; ADR029 static sites; ADR018's inbound IP allowlist row (w32/m419).
- **Expected outcome:** editing a new static site's settings while it first builds is safe. The deploy goes live in minutes, the edit applies on the next deploy, and an IP rule protects the site within seconds.
- **Why now:** a first deploy is exactly when people adjust a new site's publish directory. Today that hides the site's real state behind a false health-gate failure for ~21 minutes. Meanwhile it ignores an access rule the owner believes is in force.
- **Render parity included:** the deploy status reaches REST, GraphQL, MCP and the dashboard, and IP rules are a Render-documented behavior.
