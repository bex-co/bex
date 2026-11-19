# w10 · m513 — An agent session has Python 3.12, `uv`, `gitleaks` and a working `gh`, and a repository environment can install other versions, as on Devin

**Worker:** worker10 **Goal:** an agent session on either template (`agent`, `agent-4c-8g`) can run the checks common repositories ask for without network installs: Python 3.12 next to the image's 3.11, `uv`/`uvx` and `gitleaks`. `gh` reads the session repository's CI history and pull requests with no GitHub token in the sandbox. A repository environment's setup step can download a GitHub release asset, so a workspace can add a tool or version the image lacks. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Bake Python 3.12, `uv`/`uvx` and `gitleaks` into the agent image, pinned and checked at build | 1h15m | — |
| [t002](t002.md) | GitHub's release-asset host joins every session's GitHub baseline, so a setup step can install another version | 30m | — |
| [t003](t003.md) | Design: `gh` reads the session repository's GitHub API with no token in the sandbox; ADR090 and ADR047 amended | 45m | — |
| [t004](t004.md) | Gateway: a read-only GitHub API route for the session's repository, the token injected on the upstream hop | 2h | t003 |
| [t005](t005.md) | `gh` in the image, wired to that route; the overlay says what it reads and that pull requests stay with the `bex` tools | 1h | t001, t004 |
| [t006](t006.md) | Live check: a production session runs the tools, a setup step fetches a release asset, `gh` reads CI | 45m | t002, t005 |
| [t007](t007.md) | Render parity (Devin) | 30m | t006 |
| [t008](t008.md) | Simplify | 20m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 10m | t009 |

Total: **10 tasks, about 8h**.

## Definition of done

1. **The tools are in the image.** `lego/agent-image/Dockerfile` ships `python3.12` (with `venv` and `pip`), `uv` and `uvx`, and `gitleaks`, each pinned by version and digest or SHA-256. `python3` stays Debian's 3.11. The image build fails unless each tool reports its pinned version and `uv venv --python 3.12` finds the baked interpreter with Python downloads disabled. Both templates run the one image (`sandboxTemplateRegistry`, `lego/backend/cmd/api/main.go`), so both have the tools. deploy.yml's CRITICAL Trivy gate passes on the image.
2. **A setup step can install another version.** `release-assets.githubusercontent.com`, where GitHub now redirects release downloads, is in every session's GitHub baseline (`githubDomains`, `lego/backend/internal/sessionegress/policy.go`). A registries-tier session can download a GitHub release asset (for example `uv python install 3.13`, or a pinned `gitleaks` tarball) in a repository environment's setup step.
3. **`gh` reads CI, with no token in the sandbox.** In a repo session, `gh run list`, `gh run view --log-failed`, `gh pr checks` and `gh api repos/{owner}/{repo}/...` GET calls answer for the session's repository. No GitHub token is in the sandbox's environment or files. A write (`gh pr create`, `gh api -X POST`), and any other repository, are refused with a message that names the `bex` MCP tools. ADR090 and ADR047 record the change. If keeping "no raw token in the sandbox" turns out impossible, giving the sandbox a token is a reversal of a security-review invariant: t003 parks the milestone with that question for the user instead of shipping it.
4. **Live.** On production, once a deploy carries the milestone: a session on `agent-4c-8g` in the `bex` workspace runs `python3.12 --version`, `uv --version`, `gitleaks version` and `gh run list` for its repository. A bex-co/beancount-io automation run then runs the checks its reports said it skipped (`make check-all`, the skills `ci-check.py` customer-`bea` leg, the secret scan).

## Source + Goal linkage

- **Source:** inbox note [w10/018](../done/018.md), filed from the agent-automation runs of 2026-11-15..18 (bex-co/beancount-io #188, #189, #208, #212), promoted by `/loopx w10` on 2026-11-19.
- **Triage evidence (2026-11-19):**
  - **The image has only Debian's tools.** `lego/agent-image/Dockerfile` installs bookworm's `python3` (3.11), `python3-pip` and `python3-venv`, Go and Node. There is no `uv`, `gitleaks` or `gh`. ADR090 left `gh` out on purpose ("`gh` is not installed, so there is no second PR path to guard"), and the overlay says "the `gh` CLI is not authenticated" (`lego/agent-image/driver/src/overlay.ts`).
  - **A repository environment cannot add them either.** Setup steps (ADR109, snapshots in ADR111) run as uid 10001, so `apt` is out. The registries tier's egress (`sessionegress/policy.go`: `githubDomains` + `registryDomains`) reaches PyPI, so `pip install uv` works, but not `release-assets.githubusercontent.com`. Python builds (`uv python install`), `gitleaks` and `gh` release downloads all redirect there, which is the "release-asset hosts are unreachable" the agents reported.
  - **An unauthenticated `gh` does nothing.** `gh` refuses every API command without a token, so adding the binary alone does not let `rt-flaky-test-fixer` read CI history. The sandbox holds no GitHub token by design (ADR047: the gateway injects the installation token on the upstream hop).
- **Reference product (Devin):**
  - Devin's base image comes with some tools preinstalled (JDK 17, Rust and Cargo, the AWS CLI, direnv, nvm). A blueprint's `initialize` step installs the rest, for example Python 3.12 through `actions/setup-python` or `uv` through its installer ([Template library](https://docs.devin.ai/onboard-devin/environment/templates), [Blueprint reference](https://docs.devin.ai/onboard-devin/environment/blueprint-reference), read 2026-11-19). bex's equivalent of `initialize` is the ADR109 setup step, and DoD 2 makes those installs reachable. DoD 1 bakes in the tools common repositories expect, so sessions with no environment have them too.
  - "Devin's machine can carry a GitHub CLI (`gh`) token that lets Devin use GitHub features directly through the GitHub API", and a security profile can remove it ([Security profiles](https://docs.devin.ai/product-guides/security-profiles), read 2026-11-19). bex reaches the same observable result, a `gh` that works, through the gateway, keeping its no-token invariant. Reads come first because they are what the evidence needs. A write path for `gh` would be a second pull-request path next to the `bex` MCP tools, so it is a separate follow-up, not part of this milestone.
- **Goal linkage:** ADR008 pillar 5, cloud coding agents ([ADR047](../../../docs/ADR047-cloud-coding-agent-sessions.md)). An agent that cannot run a repository's checks pushes work only CI verifies, and cannot reproduce a red CI run.
- **Expected outcome:** automation and session reports stop saying "relying on CI" for Python 3.12, `uv` and secret-scan checks. A routine that reads CI history (`rt-flaky-test-fixer`) can use `gh`. A workspace whose repository needs another runtime version installs it in a setup step.
- **Why now:** every bex-co/beancount-io automation run from 2026-11-16 to 2026-11-18 skipped at least one of these checks, and 17 scheduled automations run in the `bex` workspace.
- **Render parity included (reference product: Devin):** sessions are a tenant-facing surface. t007 compares the tools a bex session has with Devin's base image and blueprint model, and `gh` with Devin's token, and files any gap (for example Java or Rust) as follow-up work.
