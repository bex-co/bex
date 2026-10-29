# w8 · m122 — Report the image digest each prebuilt-image deploy ran

**Worker:** worker8 **Goal:** Every live deploy of an image-backed service reports Render's `image.sha`, the digest its tag resolved to when the deploy ran, on REST, MCP, GraphQL, the dashboard and the pinned CLI. Today the field is never sent, so `bex deploys list` prints `SHA:` blank and a user cannot tell which bytes a `:latest` deploy actually ran. **Status:** todo（2026-10-29 解除阻塞，待线上验收；原记录：t001–t006 done 2026-10-20; t007 live check on production after the release pipeline deploys the operator and bex-api

## Tasks (in order)

| id   | title                                                                       | est | depends_on                 |
| ---- | --------------------------------------------------------------------------- | --- | -------------------------- |
| t001 — **DONE** | Operator: record the digest the serving release's container actually runs  | 45m | —                          |
| t002 — **DONE** | Store: keep each live deploy's image digest on its own row                  | 45m | w8/m122/t001               |
| t003 — **DONE** | Surfaces: REST/MCP `image.sha`, GraphQL `imageSha`, dashboard deploy detail | 40m | w8/m122/t002               |
| t004 — **DONE** | Render parity: deploy image object across REST, GraphQL, MCP, UI and CLI    | 20m | w8/m122/t003               |
| t005 — **DONE** | Simplify the changed code                                                   | 15m | w8/m122/t004               |
| t006 — **DONE** | Test coverage: digest capture, per-row attribution and projection           | 30m | w8/m122/t004               |
| t007 | Close out the milestone                                                     | 30m | w8/m122/t005, w8/m122/t006 |

## Definition of done

Live on production after the release pipeline deploys the operator and bex-api, with an owned free fixture deleted by ID and a 404 absence proof:

- A fresh free web service created with `bex services create --type web_service --plan free --image docker.io/ealen/echo-server:0.9.2 -o json --confirm` goes live. `GET /v1/services/{id}/deploys` returns `"image": {"ref": "docker.io/ealen/echo-server:0.9.2", "sha": "sha256:<64 hex>"}` on the live row, and that digest equals `skopeo inspect --format '{{.Digest}}' docker://docker.io/ealen/echo-server:0.9.2` for the platform's architecture (or the index digest, whichever the kubelet records; t001 decides and documents which).
- The pinned CLI's deploy list (`bex deploys list <id>` in a TTY) shows that digest after `SHA:`. `bex deploys list <id> -o json` carries it.
- A second deploy (`bex deploys create <id> --wait`) records its own digest on its own row. A deploy that never goes live (bad tag → `update_failed`) carries no `sha`, never the previous release's.
- A service created by digest (`…@sha256:…`) reports that same digest. A Git-backed deploy still has no `image` object (Render: "Not present for Git-backed deploys").
- GraphQL `Deploy.imageSha` and MCP `get_deploy`/`list_deploys` carry the same value as REST. The dashboard deploy detail shows it next to the image reference.

## Source + Goal linkage

- **Source:** `/qa-find-bugs-cli` sweep 1228, 2026-10-20 ([evidence](../evidence/cli-sweep1228/README.md)). Reproduction below.
- **Goal linkage:** [ADR018](../../../docs/ADR018-render-parity.md) "List / get deploy objects" row: deploy history must describe what ran. [ADR006](../../../docs/ADR006-bex-api.md): REST and MCP use Render's deploy shape.
- **Expected outcome:** for a mutable tag (`:latest`), the deploy record says exactly which image bytes each live deploy ran. Render's pinned contract field is populated, so the CLI's `SHA:` line stops being blank.
- **Why now:** `w4/m236` just cleaned the deploy and event image projection. This is the last missing member of Render's three-field `image` object that bex can honor truthfully (`registryCredential` is a separate, existing field question). Render parity is included because the fix changes the deploy shape on REST, GraphQL, MCP and the dashboard.

## Reproduction (sweep 1228, 2026-10-20)

- **Versions:** HEAD checkout build `94533d795` (`bex vdev`), upstream pin Render CLI v2.27.0 (`a764810a7682`), production API `https://api.bex.co/v1/`, deployed platform pin `94533d795067`. Human device-login grant, workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`), PTY 160×45 for the TUI.
- **Repro:** `bex services create --name qa-20261020-451090-tail --type web_service --image docker.io/ealen/echo-server:0.9.2 --plan free --env-var QA_MARK=<marker> -o json --confirm` → exit 0 in 2.0 s, `srv-dbbfv719a20c7385r9m0`, live in 43 s. Then `bex services` (interactive) → search → the service → **deploys list**.
- **Actual (CLI):** the deploy row renders `Image: docker.io/ealen/echo-server:0.9.2 SHA:` with an empty value.
- **Actual (wire):** `GET /v1/services/srv-dbbfv719a20c7385r9m0/deploys?limit=5` → 200:

  ```json
  [
    {
      "deploy": {
        "id": "dep-dbbfv719a20c7385r9mg",
        "serviceId": "srv-dbbfv719a20c7385r9m0",
        "status": "live",
        "trigger": "other",
        "bexTrigger": "create",
        "image": { "ref": "docker.io/ealen/echo-server:0.9.2" },
        "createdAt": "2026-10-20T05:43:56.212309Z",
        "updatedAt": "2026-10-20T05:44:29.843446Z",
        "startedAt": "2026-10-20T05:43:59.819034Z",
        "finishedAt": "2026-10-20T05:44:29.843445Z"
      },
      "cursor": "dep-dbbfv719a20c7385r9mg"
    }
  ]
  ```

- **Expected:** Render's deploy schema (pinned `pkg/client/types_gen.go:2088–2098`) defines `image` as `{ref, registryCredential, sha}`, where `sha` is the "SHA that the image reference was resolved to when creating the deploy". Its consumer, `pkg/deploy/tui.go:54–56`, prints `SHA` beside `Image` for every image deploy. Render's [prebuilt-image docs](https://render.com/docs/deploying-an-image) treat the digest as the deploy's identity ("the image and digest used for the deploy you roll back to must be available").
- **Cleanup:** the fixture was deleted with `bex services delete srv-dbbfv719a20c7385r9m0 --confirm`, and a later GET returned 404.

## Root cause

- **Producer:** `lego/backend/internal/deploys/rest.go:49–56`. `renderDeployImage` has only `Ref`, and the comment says "bex only tracks the resolved image string, reported as Ref". `rest.go:134–136` fills it from `store.Deploy.Image`. GraphQL `Deploy.image` (`deploys/graphql.go:39`) is the same bare string, and MCP shares REST's `renderDeploy`.
- **No digest exists to report.** For an external prebuilt image the operator runs `app.Spec.Image` verbatim (`release_identity.go:642–643`). `pinBuiltImage` (`app_controller.go:1463–1482`) pins only images in the platform registry, and `pullPolicyFor` (`:1509–1514`) gives a mutable tag `PullAlways`. So neither `App.status.image` nor the deploy row's `ResolvedImage` (`store/reconciler.go:892–903`) ever holds a digest for these services.
- **Truthful source:** once a release is Ready, the kubelet reports the bytes it runs in `pod.status.containerStatuses[name=app].imageID` (`<repo>@sha256:…`). The operator already lists the Deployment's pods (`stuckPodMessage`, `app_controller.go:5298–5330`). Built images already carry `@sha256:` in `status.image`.

## Fix (target)

- **t001 (operator):** when a release converges Ready, record the serving container's digest on App status, attributed to that release generation (for example `status.imageDigest` next to `status.image`). Read it from a Ready pod of the active revision's `app` container `imageID`. If `status.image` is already digest-pinned, parse the digest from it instead. Record nothing when no Ready pod of that revision reports one. Never carry the previous release's digest forward.
- **t002 (store):** add a nullable `deploys.image_sha` column. Backfill it beside `ResolvedImage` only on a genuine live convergence for the matching release (`matchesObservedRelease` / `servesGeneration`, `reconciler.go:896–903`). Failed, canceled and superseded rows stay empty.
- **t003 (surfaces):** REST `renderDeployImage.Sha` (`json:"sha,omitempty"`), which MCP inherits, plus GraphQL `Deploy.imageSha`. The dashboard deploy detail shows the short digest with copy. Omit the field when unknown; never fake it.
- **Out of scope:** changing what runs. Bex keeps running the tag with `PullAlways`. Whether a deploy should pin and run `ref@digest`, so that later restarts and scale-outs cannot pick up a moved tag, is a separate product decision with registry-auth cost. Render's own docs are ambiguous for restart, and its rollback re-pulls the tag. Record it in ADR004 as an open question; do not implement it here.

## Blast radius

- Readers of the deploy image: `deploys/rest.go` (REST + MCP), `deploys/graphql.go`, the dashboard deploy detail and history rows, and the pinned CLI's TUI and JSON output.
- Every runtime with prebuilt images is affected: web, private, worker and cron (CronJob runs reuse the release image). Static sites built from images have no runtime container. t001 must state what it records for them (nothing).
- Rollback and Restart are untouched: they keep selecting `ResolvedImage`.

## Unverified

- Whether the kubelet's `imageID` on production nodes is the platform-specific manifest digest or the index digest for multi-arch tags. t001 must observe and document it, and the DoD's `skopeo` comparison uses whichever it is.
- `registryCredential` (the third `image` field) was not exercised: no private-registry credential fixture was used.

## Dedupe

`rg -i 'image\.sha|"sha"|SHA:|imageDigest|resolved to when'` over `.pm` (open, blocked, done) and `docs`: no matching item. Sweep 483 verified that digest references are stored verbatim at create; it did not cover the deploy read. ADR004's rollback table records a separate retained divergence about which image a rollback selects. `git log -S'renderDeployImage'`: the struct has had `Ref` only since it was introduced. There is no fix on main, so this is not deploy lag.


## 解除阻塞记录（2026-10-29，`/unblock w8`）

- **涉及 ID：** w8/m122
- **结论来源：** 本次复核证据 + 现有规则（loopx：线上验收由 worker 用 `scripts/qa-login.sh` / `bex` CLI 自行完成）。
- **复核结果：** 原卡点「发布流水线部署后再做线上 QA」已不成立。生产当前运行 pin `b33dd43c4428`（2026-10-29 06:51 UTC 由 deploy.yml + Argo 自动发布），本条目在 main 上已合入的修复都包含在内。线上验收不需要用户操作。
- **剩余工作：** 只剩线上验收（待验证，未完成）。部署不等于验收通过。
- **worker 下一步：** loopx 按原 DoD 和上方 BLOCKED 记录里的检查清单在生产执行线上验收：通过则按 done 规则关闭；不通过则修复后复验。fixture 按 ID 删除并留 404 证据。
- **用户门槛：** 无。
