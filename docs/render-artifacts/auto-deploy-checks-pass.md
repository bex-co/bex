# Render: auto-deploy after CI checks pass (`autoDeployTrigger: checksPass`)

Captured 2026-10-17 for `w1/m181` from Render's public documentation. No live Render deploy was run.

## Sources

| Source | Fetched | What it settles |
| --- | --- | --- |
| [render.com/docs/deploys](https://render.com/docs/deploys) | 2026-10-17 | The three Auto-Deploy choices, which CI signals count, pass conditions, zero-check and failure behavior |
| Render public API (pinned `render-public-api-1.json`) and Blueprint schema (`render.yaml.json`) | pinned in repo | `autoDeployTrigger` enum `off` · `commit` · `checksPass` |

## Contract

- **The choices.** Settings → Auto-Deploy offers **On Commit** ("Render triggers a deploy as soon as you push or merge a change to your linked branch", the default), **After CI Checks Pass**, and **Off**. The API and Blueprint spell them `commit`, `checksPass` and `off`.
- **What counts as a check (GitHub).** Render detects "GitHub Actions" and other tools that use the "GitHub checks API", for example CircleCI. Commit statuses (the older Status API) are not mentioned, so bex reads check runs only.
- **Pass condition.** Render deploys "only after all of your repo's CI checks pass". A check passes when its conclusion is `success`, `neutral` or `skipped`.
- **Zero checks.** Render doesn't deploy when "Zero checks are detected for the new commit"; the docs recommend On Commit for repositories without CI.
- **Failure.** Render doesn't deploy when "At least one CI check fails for the new commit". The running service is untouched.
- **Newer commits.** The page decides per commit and doesn't describe a newer push while an older commit's checks are still pending. _Inferred default (`.pm/w1/README.md` rule):_ only the branch's latest pushed commit can deploy. A newer push supersedes the older pending one, so a late result for an older commit never rolls back past the newer one.

## bex mapping

- `checksPass` keeps `spec.autoDeploy: true` and sets `spec.autoDeployTrigger: checksPass`. `commit` and `off` behave as before.
- A push to a `checksPass` service passes the same filters as `commit` (branch, skip phrase, root directory, build filters). Instead of deploying, it records the pushed commit as awaiting checks. Competing pending commits are ordered by the authoritative branch tip rather than delivery arrival time.
- bex lists the commit's check runs right away (a commit CI already checked on another branch, then fast-forwarded, gets no further suite) and again on every completed GitHub `check_suite` for it. The suite's own branch isn't matched, since the awaited commit already pins the push. All complete and all `success`/`neutral`/`skipped` deploys it once, claimed with an optimistic-lock update so two suites finishing together deploy it once.
- A failed check doesn't deploy, and the commit stays awaited: a re-run that passes deploys it, and a newer push replaces it. _bex decision (Render's docs don't cover re-runs)._ Runs still in progress keep it waiting.
- This needs the App to hold **Checks: read** and the installation to receive **Check suite** events. Either missing refuses `checksPass` by name (on the trigger verb, on create, and on a Blueprint sync that turns it on) rather than silently never deploying. The legacy `autoDeploy` yes/no leaves the gate as it is on every surface; only `autoDeployTrigger` sets or clears it. A branch deletion, or switching away from `checksPass`, forgets the awaited commit.

## Dispatch recovery (w1/m181 correctness extension, 2026-10-20)

The pending acceptance records its repository, branch and commit metadata on the App. Suite callbacks and a bounded periodic worker invoke the same dispatch path. A resourceVersion-guarded App write consumes that acceptance **with** the requested release and a durable history receipt; database recovery records the receipt idempotently by App and release generation before clearing it. Rollback-image cleanup is conditional on a source-specific row revision; the projector retains the release image through the handoff and rejects stale row snapshots afterward. Unrelated settings changes do not invalidate cleanup, and a later identical-image rollback remains protected. Failed intake returns 503 and releases its replay claim for webhook redelivery before ordinary services in that workspace are redeployed; already accepted work recovers without another external event. GitHub [does not automatically redeliver failures](https://docs.github.com/en/webhooks/using-webhooks/handling-failed-webhook-deliveries), so a refused delivery requires an explicit redelivery by the operator or an existing redelivery integration. A failure never restores an old commit over a newer pending one. Pending work is checked against the current source, suspension/deletion and auto-deploy settings; when the Git resolver is available, the current branch tip is authoritative, including force pushes. Source changes invalidate pending acceptance. Legacy pending SHAs without a source record keep the old current-source interpretation until settled; new acceptances always carry the binding.

These recovery, receipt/idempotency and latest-source rules are **bex correctness choices**, not verified Render guarantees. Render's [current deploy documentation](https://render.com/docs/deploys#integrating-with-ci) documents the CI gate and accepted conclusions, but not crash recovery, exactly-once dispatch, re-run behavior or pending-commit supersession. No claim is made about Render using a queue or this receipt mechanism. The public REST, GraphQL, MCP and dashboard gate vocabulary is unchanged. A real production passing/failing/rapid-push walkthrough still needs the GitHub App owner to enable the Check suite subscription; local fault and recovery tests do not replace that gate.
