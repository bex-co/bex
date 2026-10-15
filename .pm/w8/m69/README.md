# w8 · m69 — Preserve native cron startup semantics

**Worker:** worker8 **Goal:** a native cron runs its accepted startup command through the native launch path and reads it back correctly. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Keep native cron start commands in the native launch role | 45m | — |
| t002 | Keep readback and shared command roles coherent | 45m | t001 |
| t003 | Render parity | 25m | t002 |
| t004 | Simplify | 10m | t003 |
| t005 | Test coverage | 30m | t003 |
| t006 | Closeout | 15m | t004, t005 |

## Definition of done

- Fresh native Bex and unchanged Render create a Free Node cron with an inline finite Bash command; deployment is Live, a finite API-only manual run is successful, and native logs contain the exact marker without a Bash wrapper workaround. Repeat native command updates and read back the correct command.
- Initial/create/update/Blueprint/clone/generate and shared REST/GraphQL/MCP/dashboard roles stay coherent. Native baked startup remains distinct from explicit image overrides. Opaque Docker/image aliases, clearing, legacy fallback and no-op behavior remain correct, as do carrying-release/source switches.
- Producer/consumer/readback regressions and affected required checks pass. Six native runtimes and buildpack/opaque siblings have honest source/runtime coverage. The exact deployed fix and all remaining limits are identified; no claim of Render-production parity from a local fixture alone.
- Every owned parent/deploy/run/container/session created for acceptance is cleaned and baseline/quota reconciled. Complete all tasks before moving the milestone and tasks to done with synchronized statuses and no stubs.

## Source + Goal linkage

- **Source:** functional `$qa-find-bugs-cli` w8/sweep285; [finding](finding.md), [durable live/native evidence](evidence/cli-sweep285/README.md).
- **Goal linkage:** ADR008 usable hosting, ADR018 Render compatibility, ADR004 native launch and ADR049 declarative command fidelity; typed native StartCommand contract.
- **Expected outcome:** accepted native Bash commands run successfully, and readback/clone retains their real native start text.
- **Why now:** both actual native clients create Live Free services whose runs fail in /bin/sh, and the complete decoder→typed App→operator→container chain proves override promotion. A naive initial routing fix then breaks native readback, also proved, so the consumer correction belongs in the same milestone.
- **Sizing:**170m / six tasks across command routing, shared consumers, parity and hosted acceptance; exceeds an inbox note.
- **Render parity:** included for tenant-facing CLI/API/UI behavior. The client wire is already valid; Bex owns the correction. Independent Render production is unverified.

Severity major. The two private proposal fragments prove only initial routing and native projection, not a complete fix. Two Free parents/two deploys/two runs and four local containers are cleaned; baseline6/quota6/terminating0. No product edit, dependency change, upstream submission, deployment, commit or push by QA.

## Independent mutation-transition replay — sweep286

[Actual local REST and Blueprint evidence](evidence/cli-sweep286/README.md) confirms both native update handlers return200 but put the new native program in Command while leaving the old baked StartCommand. A genuine valid Blueprint command edit does the same and opens one deploy; initial and JSON-reloaded unchanged controls retain the correct role. These are actual producer/storage proofs of the previously source-established siblings, not hosted update acceptance. The excluded first Blueprint fixture lacked YAML quoting; the corrected run reaches the real mutation. Keep this within m69; no duplicate finding or product fix.
