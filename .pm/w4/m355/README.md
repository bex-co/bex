# w4 · m355 — Between two linked environment groups, bex gives the key to the group linked last, while Render gives it to the group created last; relinking an older group silently flips the value a service runs

**Worker:** worker4 **Goal:** when two linked environment groups define the same env var or secret file, a service runs the value from the **most recently created** group, as Render documents. Relinking, or linking in a different order, no longer changes the winner. A service's own value still beats every group. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Order a service's group refs by group creation on every link writer | 1h15m | — |
| [t002](t002.md) | Existing services: put their group refs in creation order at their next release, never by reconcile | 40m | t001 |
| [t003](t003.md) | Environment page: name the creation rule in the override tooltip (en, zh) | 25m | t001 |
| [t004](t004.md) | Amend ADR013 § Precedence and the ADR018 environment-groups row | 20m | t001, t002 |
| [t005](t005.md) | Render parity | 20m | t003, t004 |
| [t006](t006.md) | Simplify | 15m | t005 |
| [t007](t007.md) | Test coverage | 40m | t005 |
| [t008](t008.md) | Closeout (live replay on production) | 20m | t006, t007 |

## Definition of done

All on production, workspace `bex-canary`.

**Fixture:** a Free web service from `POST /v1/services`:

- `image.imagePath: docker.io/library/busybox:1.36`;
- service env vars `QA_SVC=svc1` and `QA_BOTH=from-service`;
- the round 247 `dockerCommand`, which writes `QA_GRP`, `QA_SVC`, `QA_BOTH`, `/etc/secrets/qa-grp.txt` and `ls /etc/secrets` into `/w/index.html`, then runs `exec httpd -f -p $PORT -h /w`.

Then create two groups in this order, a few seconds apart:

- **A:** `QA_GRP=a`, `QA_BOTH=from-group`, secret file `qa-grp.txt` = `a-file`;
- **B:** `QA_GRP=b`, secret file `qa-grp.txt` = `b-file`.

1. **The newer group wins, whatever the link order.**
   - Link A, then B, each from the dashboard. After each deploy is Live, `curl https://<name>.onbex.co/` reads `QA_GRP=b` and `b-file`.
   - Unlink A from the service's Environment page, wait for Live, then link A again. After Live, the page still reads `QA_GRP=b` and `b-file`. Round 247 read `QA_GRP=grp2` and `grp-file-1`, the older group's values.
   - Link B first and A second on a fresh fixture: the page reads `b` and `b-file`.
2. **The service's own value still wins.** Throughout, the page reads `QA_SVC=svc1` and `QA_BOTH=from-service` (w2/m94).
3. **Every surface names the same winner.**
   - `GET /v1/services/<id>` returns `linkedEnvGroupIds` in precedence order, lowest first, so B's id comes last after any link sequence.
   - GraphQL `service { linkedEnvGroupIds }` returns the same list.
   - The Environment page lists B first. It marks A's `QA_GRP` and `qa-grp.txt` "Overridden by `<B>`, which was created later", and the zh copy matches.
4. **A gated service follows the same rule.** On a repo-backed service with Auto-Deploy off, link B, then A. Both links are staged, and no deploy opens (w4/m239). After a Manual Deploy, the service runs `b`.
5. **Fixtures.** Every fixture is deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 247, 2026-11-13 00:53–01:07 UTC (`muse.env`, workspace `bex-canary`; pin `c328714af1dd`, deployed 00:39 UTC). The probes are under § Finding.
- **Governing docs:**
  - [ADR013](../../../docs/ADR013-secrets.md) § Precedence and redeploy, rule 2 (line 102): "Between groups, the last linked wins".
  - [ADR018](../../../docs/ADR018-render-parity.md) environment-groups row (line 160) calls last-linked-wins "a **deliberate divergence** … adopting that would silently flip which value already-running services serve".
  - Both come from w1/091 and w2/m94 (2026-09-14/15), before the parity-first rule. Neither records the user's sign-off.
- **Goal linkage:** Render parity is the top product principle ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first); AGENTS.md: "A divergence stands only when ADR018 records it with the user's sign-off", and "Never preserve the divergence because an older document chose it").
  - [Render: environment variables](https://render.com/docs/configure-environment-variables), read 2026-11-13: "Render _does not guarantee_ its precedence behavior when multiple linked environment groups define the same environment variable. Currently, Render uses the value from the _most recently created_ environment group."
- **Expected outcome:**
  - The value a service runs depends only on which groups it links, not on the order or history of its link and unlink clicks.
  - A user who reads Render's docs, or migrates a Render setup with the same groups, predicts the right value.
- **Why now:**
  - Relinking a group is an ordinary edit, and today it silently changes the value a service runs on its next deploy.
  - The divergence has no sign-off, so the rule requires amending it.
  - The "would flip running services" concern is met by t002: an existing service changes order only at a release its owner starts.
- **Render parity task included (t005):** the precedence shows on REST, GraphQL and MCP (`linkedEnvGroupIds`) and on the dashboard's Environment page.
- **If the user prefers to keep last-linked-wins:** record their sign-off in the ADR018 row and close this milestone as superseded. Per AGENTS.md ("Never park parity work on an optional decision"), the parity choice is filed meanwhile.

## Finding (round 247)

**Fixture:** `qa-20261113-r247-env` (`srv-dbr5ve7oj40s73fkt8q0`), created 00:54:16 by `POST /v1/services`. It was Free, `docker.io/library/busybox:1.36`, with service vars `QA_SVC=svc1` and `QA_BOTH=from-service` and service secret file `qa-svc.txt`. It was deleted at the end of the round, together with both groups (`DELETE` 204, then `GET` 404).

The groups were created from the dashboard's Create environment group dialog, each linked at creation:

| Group | id | Created | Contents |
| --- | --- | --- | --- |
| `qa-20261113-r247-grp` (A) | `evg-dbr60a95t55c738j9nmg` | 00:56:09.820 | `QA_GRP=grp1`, `QA_BOTH=from-group`, `qa-grp.txt` = `grp-file-1` |
| `qa-20261113-r247-grp2` (B) | `evg-dbr61inoj40s73fktaig` | 00:58:50 | `QA_GRP=g2`, `qa-grp.txt` = `grp2-file` |

1. **A linked.** Deploy `dep-dbr60a95t55c738j9nn0` (`service_updated`) was Live at 00:57:19. The page read `QA_GRP=grp1`, `QA_BOTH=from-service`, `grp-file-1`, and both files were listed.
2. **A's `QA_GRP` saved as `grp2` with "Save only"** at 00:58:04. No deploy opened. The toast read "Environment saved without a deploy. Use a standard deploy to apply any pending changes.". GraphQL then read `undeployedChanges: true`, and the service banner read "Saved changes aren't live yet.".
3. **B created and linked.** Deploy `dep-dbr61inoj40s73fktaj0` was Live at 00:59:36. The page read `QA_GRP=g2` and `grp2-file`, so B won (created later and linked later).
   - The Environment page marked A's `QA_GRP` and `qa-grp.txt` "(Overridden by qa-20261113-r247-grp2, which is linked later)".
4. **A unlinked, then relinked.**
   - Unlinked from the Environment page at 01:01:08; deploy `dep-dbr62l7oj40s73fktaqg` was Live at 01:02:05.
   - Relinked at 01:02:06; deploy `dep-dbr633noj40s73fktb3g` was Live at 01:03:05.
   - `GET /v1/services/srv-dbr5ve7oj40s73fkt8q0` → `linkedEnvGroupIds: ["evg-dbr61inoj40s73fktaig","evg-dbr60a95t55c738j9nmg"]` (B, then A).
   - The page read **`QA_GRP=grp2`** and **`grp-file-1`**: A's values, although B was created later. On Render the service would run `g2` and `grp2-file`.
   - The Environment page now marked B's keys "(Overridden by qa-20261113-r247-grp, which is linked later)", which describes bex's rule correctly.

### Mechanism

- The precedence is `spec.envFromSecrets` order. Kubernetes lets the last `envFrom` source win a key (`envFromSources`, `lego/operator/internal/controller/app_controller.go:5891`), and the last projected source wins a file path (`secretFileMounts`, `:5985`). The service's own Secret is always placed last (ADR013 rule 1).
- Every link writer **appends** the group's refs:
  1. `rollLinked`, link branch: `addString(a.Spec.EnvFromSecrets, envSecretName(gid))` (`lego/backend/internal/envgroups/service.go:1194-1195`). `addString` is `append` (`:1824-1829`).
     - Every link path reaches it through `linkFetched`: `LinkService` (`:1047`), `LinkEnvGroup` by name (`:1584`), the Blueprint and deploy paths (`apps/deploy.go:1224`, `apps/blueprint_build_sources.go:380`) and build sources (`apps/build_sources.go:250`).
  2. Create-time links (the dialog's Linked Services, `initial_links.go:103-104`), with the same `addString`.
  3. A gated service's staged link: `StageGroupLink` → `refDelta.apply` appends at fold time (`lego/backend/internal/rollout/grouplinks.go:60-68`, `:106-128`).
- An unlink removes the refs, so a relink puts the group last, whatever its age.
- `linkedEnvGroupIDs` (`lego/backend/internal/apps/service.go:6411-6420`) reports that order on REST and GraphQL. The dashboard ranks by it (`dashboard/src/features/services/components/env-groups-panel.tsx:161-219`) and words it as "linked later" (`dashboard/src/features/services/locales/en.ts:1097-1105`, `zh.ts:1075-1084` "已被后链接的 {group} 覆盖").

### Also verified this round (all pass), env groups journey (4)

- **Create dialog.** Create with variables, a secret file and a linked service toasts "Created …". The image-backed linked service deploys at once, as the Linked Services copy says ("every other service deploys it now").
- **Service over group (w2/m94).** The service's `QA_BOTH` wins over the group's. The Environment page strikes the group's key through with "(Overridden by this service's own QA_BOTH environment variable)".
- **Group save modes.** The group editor offers Save, rebuild, and deploy; Save and deploy; and Save only.
  - Save only opens no deploy and is reported honestly (step 2).
  - Save and deploy of `QA_GRP=grp3` at 01:04:45 toasted "Environment saved and deployment started", opened `dep-dbr64bfoj40s73fktb80` (Live 01:05:49), and the page read `QA_GRP=grp3`.
- **Unlink and relink toasts** say a repo-backed service with auto-deploy off picks the change up at its next deploy, and any other service deploys now. Both deploys opened.

## Unverified

- **A gated repo-backed service** (Auto-Deploy off) was not probed this round. It reaches the same append through `refDelta.apply` (DoD 4).
- **Blueprint `fromGroup` links and Render-shaped REST link calls** were reasoned from the shared `linkFetched` path, not probed.
- **Secret files on Render.** Render documents the creation rule for env vars only, and it documents nothing for group-vs-group secret files. This milestone applies the same rule to files, as bex already applies one rule to both.
- **Render's tie-break** when groups share a creation second was not captured. Use the store's `createdAt`, then the id.

## Dedupe

- [w1/091](../../w1/done/091.md) (done) found the collision and chose to keep last-linked-wins and record it. [w2/m94](../../w2/done/m94/README.md) (done) shipped `linkedEnvGroupIds`, the panel ranking and the ADR wording. [w4/322](../done/322.md) and [w4/480](../done/480.md) (done) fixed the panel's refresh and accessibility.
- None of them had the user's sign-off, and all of them predate the parity-first rule (2026-10-17). This milestone amends their rule. It does not reopen their display work.
- `grep -rln "most recently created\|created later" .pm` finds no open item about it.
- Not deploy lag: `git log c328714af1dd..origin/main -- lego/backend/internal/envgroups lego/backend/internal/rollout dashboard/src/features/services/components/env-groups-panel.tsx docs/ADR013-secrets.md` is empty.
