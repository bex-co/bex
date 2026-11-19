# w10 · m513 — An agent session has Python 3.12, `uv`, `gitleaks` and a working `gh`, and a repository environment can install other versions, as on Devin

**Worker:** worker10 **Goal:** an agent session on either template (`agent`, `agent-4c-8g`) can run the checks common repositories ask for without network installs: Python 3.12 next to the image's 3.11, `uv`/`uvx` and `gitleaks`. `gh` reads the session repository's CI history and pull requests with no GitHub token in the sandbox. A repository environment's setup step can download a GitHub release asset, so a workspace can add a tool or version the image lacks. **Status:** done (2026-11-19; every task done, the Definition of done verified on production pin `c81d2d447bd8`, see § Live check)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Bake Python 3.12, `uv`/`uvx` and `gitleaks` into the agent image, pinned and checked at build — **DONE** | 1h15m | — |
| [t002](done/t002.md) | GitHub's release-asset host joins every session's GitHub baseline, so a setup step can install another version — **DONE** | 30m | — |
| [t003](done/t003.md) | Design: `gh` reads the session repository's GitHub API with no token in the sandbox; ADR090 and ADR047 amended — **DONE** | 45m | — |
| [t004](done/t004.md) | Gateway: a read-only GitHub API route for the session's repository, the token injected on the upstream hop — **DONE** | 2h | t003 |
| [t005](done/t005.md) | `gh` in the image, wired to that route; the overlay says what it reads and that pull requests stay with the `bex` tools — **DONE** | 1h | t001, t004 |
| [t006](done/t006.md) | Live check: a production session runs the tools, a setup step fetches a release asset, `gh` reads CI — **DONE** | 45m | t002, t005 |
| [t007](done/t007.md) | Render parity (Devin) — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
| [t010](done/t010.md) | Closeout — **DONE** | 10m | t009 |

Total: **10 tasks, about 8h**.

## Definition of done

1. **The tools are in the image.** `lego/agent-image/Dockerfile` ships `python3.12` (with `venv` and `pip`), `uv` and `uvx`, and `gitleaks`, each pinned by version and digest or SHA-256. `python3` stays Debian's 3.11. The image build fails unless each tool reports its pinned version and `uv venv --python 3.12` finds the baked interpreter with Python downloads disabled. Both templates run the one image (`sandboxTemplateRegistry`, `lego/backend/cmd/api/main.go`), so both have the tools. deploy.yml's CRITICAL Trivy gate passes on the image.
2. **A setup step can install another version.** `release-assets.githubusercontent.com`, where GitHub now redirects release downloads, is in every session's GitHub baseline (`githubDomains`, `lego/backend/internal/sessionegress/policy.go`). A registries-tier session can download a GitHub release asset (for example `uv python install 3.13`, or a pinned `gitleaks` tarball) in a repository environment's setup step.
3. **`gh` reads CI, with no token in the sandbox.** In a repo session, `gh run list`, `gh run view --log-failed`, `gh pr checks` and `gh api repos/{owner}/{repo}/...` GET calls answer for the session's repository. No GitHub token is in the sandbox's environment or files. A write (`gh pr create`, `gh api -X POST`), and any other repository, are refused with a message that names the `bex` MCP tools. ADR090 and ADR047 record the change. If keeping "no raw token in the sandbox" turns out impossible, giving the sandbox a token is a reversal of a security-review invariant: t003 parks the milestone with that question for the user instead of shipping it.
4. **Live.** On production, once a deploy carries the milestone: a session on `agent-4c-8g` in the `bex` workspace runs `python3.12 --version`, `uv --version`, `gitleaks version` and `gh run list` for its repository. A bex-co/beancount-io automation run then runs the checks its reports said it skipped (`make check-all`, the skills `ci-check.py` customer-`bea` leg, the secret scan).

## Source + Goal linkage

- **Source:** inbox note [w10/018](../018.md), filed from the agent-automation runs of 2026-11-15..18 (bex-co/beancount-io #188, #189, #208, #212), promoted by `/loopx w10` on 2026-11-19.
- **Triage evidence (2026-11-19):**
  - **The image has only Debian's tools.** `lego/agent-image/Dockerfile` installs bookworm's `python3` (3.11), `python3-pip` and `python3-venv`, Go and Node. There is no `uv`, `gitleaks` or `gh`. ADR090 left `gh` out on purpose ("`gh` is not installed, so there is no second PR path to guard"), and the overlay says "the `gh` CLI is not authenticated" (`lego/agent-image/driver/src/overlay.ts`).
  - **A repository environment cannot add them either.** Setup steps (ADR109, snapshots in ADR111) run as uid 10001, so `apt` is out. The registries tier's egress (`sessionegress/policy.go`: `githubDomains` + `registryDomains`) reaches PyPI, so `pip install uv` works, but not `release-assets.githubusercontent.com`. Python builds (`uv python install`), `gitleaks` and `gh` release downloads all redirect there, which is the "release-asset hosts are unreachable" the agents reported.
  - **An unauthenticated `gh` does nothing.** `gh` refuses every API command without a token, so adding the binary alone does not let `rt-flaky-test-fixer` read CI history. The sandbox holds no GitHub token by design (ADR047: the gateway injects the installation token on the upstream hop).
- **Reference product (Devin):**
  - Devin's base image comes with some tools preinstalled (JDK 17, Rust and Cargo, the AWS CLI, direnv, nvm). A blueprint's `initialize` step installs the rest, for example Python 3.12 through `actions/setup-python` or `uv` through its installer ([Template library](https://docs.devin.ai/onboard-devin/environment/templates), [Blueprint reference](https://docs.devin.ai/onboard-devin/environment/blueprint-reference), read 2026-11-19). bex's equivalent of `initialize` is the ADR109 setup step, and DoD 2 makes those installs reachable. DoD 1 bakes in the tools common repositories expect, so sessions with no environment have them too.
  - "Devin's machine can carry a GitHub CLI (`gh`) token that lets Devin use GitHub features directly through the GitHub API", and a security profile can remove it ([Security profiles](https://docs.devin.ai/product-guides/security-profiles), read 2026-11-19). bex reaches the same observable result, a `gh` that works, through the gateway, keeping its no-token invariant. Reads come first because they are what the evidence needs. A write path for `gh` would be a second pull-request path next to the `bex` MCP tools, so it is a separate follow-up, not part of this milestone.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../../docs/ADR047-cloud-coding-agent-sessions.md)). An agent that cannot run a repository's checks pushes work only CI verifies, and cannot reproduce a red CI run.
- **Expected outcome:** automation and session reports stop saying "relying on CI" for Python 3.12, `uv` and secret-scan checks. A routine that reads CI history (`rt-flaky-test-fixer`) can use `gh`. A workspace whose repository needs another runtime version installs it in a setup step.
- **Why now:** every bex-co/beancount-io automation run from 2026-11-16 to 2026-11-18 skipped at least one of these checks, and 17 scheduled automations run in the `bex` workspace.
- **Render parity included (reference product: Devin):** sessions are a tenant-facing surface. t007 compares the tools a bex session has with Devin's base image and blueprint model, and `gh` with Devin's token, and files any gap (for example Java or Rust) as follow-up work.

## Gate (2026-11-19)

Everything implementable shipped in `52f5db3b6`. What remains is t006, the live check, which needs production to run that commit, and then t010.

- **Clears by itself (pin must carry `52f5db3b6`):** the deploy.yml run that carries it builds the amd64 agent image (its build check runs there, and the Trivy CRITICAL gate) and pins bex-api, the ssh gateway and the image digest. Check: `deploy/gitops/base/values/opensandbox-controller.values.yaml` (or the `bex-api` deployment's agent image) names a digest built from a commit that contains `52f5db3b6`.
- **Then (t006):** in the `bex` workspace, a repo session on `agent-4c-8g` runs `python3 --version`, `python3.12 --version`, `uv --version`, `uvx --version`, `gitleaks version`, `gh run list --limit 3` (answers) and `gh pr create` (refused, naming the bex tools); a repository environment whose setup step runs `uv python install 3.13` builds on a registries-tier workspace; and the next bex-co/beancount-io automation report no longer says it skipped the Python 3.12, `uv` or secret-scan checks. The production App (`bex-co`) already holds Actions and Commit statuses, so its read tokens carry them.
- **Ready for QA when deployed:** no user decision is needed. Use `/unblock w10` only if t006 finds this milestone's own defect.

## Unblock (2026-11-19)

- **Affected:** t006, t010. **Source:** verified evidence, no user decision.
- **The pin gate holds.** origin/main's newest pin, `aefb144dc` (`chore(deploy): pin platform images to c81d2d447bd8`), carries `52f5db3b6`: `git merge-base --is-ancestor 52f5db3b6 c81d2d447bd8` exits 0. Production runs it: `bex-system/bex-api` (`hetzner-prod`) runs `bex-operator@sha256:697fe9c095d7…` with `BEX_AGENT_SESSION_IMAGE=ghcr.io/bex-co/bex-agent-sandbox@sha256:56a857676917…`, the digests that pin writes into `deploy/gitops/base/bex.yaml` and `lego/operator/config/api/deployment.yaml`.
- **Before:** the last automation on the old image, `ags-dbvj6h6lkors73fppii0` (rt-shipped-feature-inliner, 17:35Z), reported "`gitleaks` is not installed in this sandbox, so the PR's secret-scan CI is the gate".
- **Next:** t006's live check in the `bex` workspace, then t010 if the Definition of done holds. No gate remains.

## Live check (t006, production, 2026-11-19)

Workspace `bex` (network tier `custom`, which is `registries` plus two hosts), owner's CLI login, deploy pin `c81d2d447bd8`. deploy.yml run 44579886499 built that pin, and its `CVE scan — agent sandbox image CRITICAL (gate)` step passed.

**1. A session has the tools, and `gh` reads CI.** `ags-dbvjucvi2lkc73a9hajg` (repo `bex-co/beancount-io`, template `agent-4c-8g`, agent `muse`, 18:26–18:35Z). Its sandbox Pod ran `bex-agent-sandbox@sha256:56a857676917…`, the pinned image. Every command exited 0 unless noted:

- `python3 --version` → 3.11.2 and `python3.12 --version` → 3.12.16.
- `uv --version` and `uvx --version` → 0.13.9; `gitleaks version` → 8.30.1; `gh --version` → 2.105.0.
- `uv venv --python 3.12` found `/usr/local/bin/python3.12`.
- `gh run list --limit 3` listed the repository's three newest runs. `gh run view <newest failed run> --log-failed` printed that run's failed-step log. `gh pr list --state all` listed #232, #231 and #230, and `gh pr checks 232` showed its four checks passing. `gh api 'repos/{owner}/{repo}/commits?per_page=1'` returned `adb290b19…`.
- `gh api -X POST repos/{owner}/{repo}/issues` → HTTP 403: "bex: gh in this session only reads bex-co/beancount-io (CI runs and logs, checks, pull requests and contents). Open or update the pull request with the open_or_update_draft_pr tool of the bex MCP server, and reply on it with reply_on_pull_request."
- A second turn ran `gh pr create --draft --head bex-agent/ags-dbu0pfuc1l2s739d3n5g --base main …`, an existing branch with no pull request, so the request reached the `createPullRequest` mutation. It failed with the same 403 message from `https://api.github.com/graphql`, and no pull request was opened (`gh pr list -R bex-co/beancount-io` afterwards: #232 is still the newest).
- `gh api repos/cli/cli` → HTTP 403 "gh in this session reads only bex-co/beancount-io; other repositories and GitHub API paths are not reachable from here".
- **No token in the sandbox.** The environment's `GH_TOKEN` starts `bex-ga…`, the gateway placeholder. A grep of `~` and `/etc` for `gh[pousr]_…` and `github_pat_` matched only muse's own log of that grep command and its redaction script.
- `uv python install 3.13` downloaded and installed CPython 3.13.18 in the session itself. `gitleaks git --log-opts=-n 20 .` reported "no leaks found".

**2. A setup step installs another version.** An environment for `bex-co/bex` was saved at 18:30Z with one setup step, `uv python install 3.13 && uv python find 3.13 && "$(uv python find 3.13)" --version`. That was after the 18:30Z `rt-unify-duplicates` automation had dispatched, and well before the next `bex-co/bex` fire at 20:00Z. The save queued build `arb-dbvk0cfi2lkc73a9hakg` (`environment_saved`). It reached `success` at 18:33:53Z (3 m 38 s; head `efcb3b649`, 2.05 GB). Its step exited 0 in 4.0 s: "Downloaded cpython-3.13.18-linux-x86_64-gnu … Python 3.13.18", the download GitHub redirects to `release-assets.githubusercontent.com`. `release-assets` is in the GitHub baseline of every tier, so a `registries` workspace reaches it the same way.

**3. The automations run the checks they used to skip.**

- `ags-dbvkc2bospoc73b2rq00` (`beancount-rt-useless-test-pruner`, 18:55–19:19Z, `agent-4c-8g`) succeeded and opened draft beancount-io#233. Its sandbox had `/usr/local/bin/uv`, `/usr/local/bin/python3.12` and `/usr/local/bin/gitleaks`. Its report's gates include "`scripts/scan-secrets.py`: no leaks. `gitleaks protect --staged`: no leaks." The runs before the deploy said "`gitleaks` is not installed … relying on the PR's secret-scan CI". Its change was backend-v2 TypeScript, so the `cli` gate was not among its checks.
- `ags-dbvkq3rospoc73b2rrqg` (`beancount-rt-logic-bugfixer`, from 19:25Z) changed `cli/`. It ran the `cli` gate's tools under Python 3.12: `uv run … pytest` on the report and price modules, `ruff check` and `mypy`. The 15:25Z run of the same routine had said "`make check-all` … not run … this sandbox has only Python 3.11 … no uv". At 20:13Z, before its final report, its sandbox was lost. The gateway logged "unable to upgrade connection: container not found", after several "exec gateway did not answer" reads while it and this check's own session `ags-dbvko3rospoc73b2rrm0` (below) both ran test suites on the one sandbox node, and the session failed with "sandbox terminated before completion". That is w9/m349's open gate, sandbox node capacity (its t006), and its known lost-reason gap, not this milestone's. Draft beancount-io#234 keeps what it had pushed.
- `ags-dbvko3rospoc73b2rrm0` (a check session on `agent-4c-8g`, 19:20Z) ran the repository's own gates:
  - `cd cli && uv sync --all-groups` exited 0.
  - `python3 scripts/skills/ci-check.py` exited 0. Its customer leg ran as "Customer bea: uv run --project /workspace/cli bea (PATH without bean-*)".
  - `python3 scripts/scan-secrets.py` exited 0 (gitleaks: "no leaks found").
  - `make check-all` passed `lint`, `deadcode`, `format-check` and `typecheck`, which make runs before `test`, and ran `test` under 3.12 to 26 %. It was stopped there at 20:15Z to take the load off the shared sandbox node.

**Cleanup.** The environment was deleted at 18:34Z (`GET /v1/agent-repo-environments` → `[]`, the build's `imageAvailable: false`), and the check sessions were archived. They created no branch push and no pull request.

**Observation (not this milestone's defect).** All nine bex-co/beancount-io automations in the `bex` workspace still tell the agent "`gh` is not installed … use another available route". That is their own task text, so a routine that reads CI history keeps avoiding `gh` until the workspace edits those overrides. Filed as [w10/030](../../030.md).
