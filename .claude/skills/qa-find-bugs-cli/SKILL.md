---
name: qa-find-bugs-cli
description: >-
  Hunt live hosting bugs through the bex CLI against the pinned Render CLI contract, trace failures into server compatibility or the Bex launcher, and file researched, non-duplicate fixes through /pm. Use when asked to QA the CLI or find hosting bugs from terminal journeys. Ships the filing only when explicitly authorized.
---

# Live CLI QA → contract diagnosis → researched PM filing

Adapt the customer-journey discipline of `../qa-find-bugs/SKILL.md` to the real `bex` executable against production (`https://api.bex.co/v1/`). This skill is self-contained; do not execute the dashboard hunt as a prerequisite. Find and research bugs, then schedule fixes; implementing fixes is separate work.

## Contract and scope

**The pinned Render CLI is the command and wire-contract oracle.** Bex imports it; command names, flags, defaults, validation, request construction, output schemas, and exit semantics stay upstream. Bex owns its branding, endpoints, isolated configuration, and documented native additions. A valid upstream request failing against Bex is a server-compatibility candidate first, not a reason to fork the CLI, rewrite requests, or teach the launcher a workaround. Server ownership is a hypothesis to prove, not a conclusion to force.

Read these before probing:

- `lego/cli/UPSTREAM_RENDER_CLI.md` and `lego/cli/go.mod`: exact release, commit, and dependency pin. Never substitute latest upstream for the tested pin.
- `docs/bex-cli.md`: Bex configuration, OAuth, native additions, and branding boundaries.
- `docs/cli-compatibility-checklist.md`: supported commands, partial support, upstream defects, and deliberate non-goals. Historical checkmarks are leads to re-test, not proof of current production behavior.
- `.pm/DO_NOT_DO.md` and `docs/ADR018-render-parity.md`: excluded work.

Preserve `render.yaml`, upstream schema/enum names, truthful compatibility version, and protocol identifiers. Help examples, executable identity, docs destinations, and Bex-owned messages should use Bex branding. Do not blindly replace every `render` string. Documented runtime-copy, User-Agent, and upstream skills-path residuals are not new bugs without a changed requirement or regression. Coding-provider launchers, installation, self-update, and upstream skills management are outside the default hosting sweep.

Parse arguments:

- `wN`: target workstream, default `w6`.
- Surface names such as `auth`, `services`, `deploys`, `env`, `logs`, `projects`, `postgres`, `keyvalue`, `shell`, `blueprints`, `sandbox`, `branding`: restrict coverage; default is the supported hosting sweep below.
- `DRY_RUN=1`: hunt and report, no board writes or ship. This is **not** a production-mutation dry run; normal disposable-resource rules still apply.
- Honor explicit read-only or audit-only requests: skip creation/mutations and report the resulting coverage gaps.

## Autonomous execution and repeated sweeps

Choose the next useful journey from the supported help tree, coverage gaps, recent changes, and prior findings. Resolve routine setup, workspace selection, fixture choice, and cleanup yourself; do not ask for confirmation when a safe choice is available within the authorized QA account. A blocked surface is a reason to exercise another safe surface, not to end the hunt.

For a requested loop, repeat hunt → reproduce/research → dedupe/file → cleanup with a fresh fixture nonce each sweep. Preserve the requested workstream across sweeps. Keep a sanitized coverage/ownership checkpoint so interruption or context compaction does not lose cleanup obligations. After covering the safe surfaces, rotate meaningful variants and re-test changed behavior at human pace; do not repeatedly create identical fixtures or file duplicate findings just to keep busy. Pause between exhausted sweeps and remain interruptible. Continue until the user stops the loop or no safe progress remains because of unavailable authentication, infrastructure, or unresolved cleanup. Never run an unbounded background mutation script. A request to edit or ship this skill alone does not start a hunt; a separate instruction to continue the loop does.

## 0 — Preflight and isolation

Record branch, HEAD, initial `git status --porcelain`, executable path, Bex release/build identity, upstream pin, API origin, and date. Record deployed revision only if observable; do not assume it equals HEAD. A hunt need not switch branches. If shipping is authorized later, follow the ship skill's branch rules then.

Use the installed customer binary first when available. If building from `lego/cli`, put the binary in a private temporary directory and label all evidence as a checkout build. A release-only failure must be compared with HEAD before filing. Do not install over the user's binary.

Create a private temporary directory (0700) and set `BEX_CLI_CONFIG_PATH` to a new config inside it; never borrow or overwrite personal CLI state. Use a per-process environment with inherited `RENDER_*` overrides and unrelated Bex auth/host/workspace/config overrides removed. Explicit `RENDER_*` inputs take precedence over Bex mappings and can silently select the wrong host or identity. Do not repurpose `HOME` or print the environment. Set the production `BEX_HOST` explicitly and disable update notices with `BEX_NO_UPDATE_NOTIFIER=1`. Track background processes and temporary files for cleanup on failure as well as success.

Inspect root and relevant subcommand `--help` before constructing commands. Derive flags and selector forms from this binary; do not invent dashboard-equivalent commands. Use bounded execution for streaming, interactive, and async journeys. After a mutation times out, inspect resulting state before retrying: the server may already have applied it.

## 1 — Authenticate as the QA user

Prefer **human device login**, because client-credentials tokens do not exercise the same granular capability checks as human tokens. A machine-token success cannot certify the human login path.

1. Read `scripts/qa-login.sh` and the login instructions in `../qa-find-bugs/SKILL.md`. Use `bash scripts/qa-login.sh --serve` and its one-shot loopback cookie handoff to sign the Playwright browser in without reading `.env` or placing passwords/cookies in tool code. Browser tools are needed only for device approval and optional cross-surface diagnosis.
2. Run the isolated `bex login`, retain the process while it waits, and visit its **actual Bex device-verification URL** in that authenticated browser. Approve only this run's device request. Keep login captures private; do not publish device codes, token responses, cookies, or config contents.
3. Confirm login terminates successfully and creates an owner-only config. Run `bex whoami`, `bex workspaces -o json`, and the supported workspace selection/current commands. Establish the QA identity, then select the workspace automatically using the rule below. Multiple accessible workspaces alone are not a reason to ask the user.
4. Select an accessible workspace in this order: an explicit user choice from this session; a workspace documented for this QA identity; an existing clearly named QA/test workspace; the authenticated QA dashboard's active workspace; otherwise an accessible workspace with verified free capacity and permission to create and delete fixtures (break equal candidates by workspace ID). Use read-only metadata to establish eligibility, never billing changes or a test purchase. Announce the selected name/ID and reason, then proceed without confirmation. If a candidate lacks safe capacity, select the next eligible candidate before creating anything; if none qualifies, continue read-only/offline coverage and report the limitation. Do not create a workspace, elevate access, or borrow another identity to bypass constraints.
5. Snapshot existing resource IDs in the selected workspace and record plan/quota constraints before creating fixtures. Set explicit `BEX_WORKSPACE` for every subsequent command and verify `workspace set`/`current` only in the isolated CLI config. Keep each sweep in that workspace; never switch tenants for authorization probes. Existing resources may be inspected through safe metadata reads, but never become mutation fixtures.

If browser access is unavailable, an already-authorized QA OAuth credential can be passed privately via `BEX_ACCESS_TOKEN`; report device login/refresh/logout as untested. An API-key client secret is **not** a bearer token: follow the exchange contract in `docs/bex-cli.md`. Do not mint new keys or use admin credentials merely to bypass a failed human flow. If no safe auth route exists, complete offline help/contract inspection and report live testing blocked. Missing QA credentials belong in `.env`, never chat.

`scripts/bex-cli-auth-e2e.sh` is useful implementation precedent, **not a drop-in production QA login helper**: it has local defaults, admin dependencies, identity-creation options, and its own session cleanup. Read it before any reuse; do not run production identity creation or admin flows for this hunt.

## 2 — Exercise whole CLI journeys

Create only in the automatically selected or user-specified workspace, with verified free capacity and an available deletion path. Use `qa-<yyyymmdd>-<random-run-nonce>-<fixture>` names. Before each create, persist its intent (workspace, exact name/nonce, type, time, dependency IDs); immediately record the returned ID and creation evidence before issuing another mutation. Track server-created children as well as top-level fixtures. The private ledger must survive process failure; keep a sanitized checkpoint containing IDs and cleanup state, never credentials.

**Pre-existing resources are immutable for this hunt.** Never update, restart, redeploy, suspend, delete, rename, move, attach configuration to, execute commands in, or write data to them. This includes old `qa-` resources, shared projects/environments, credentials, domains, billing settings, local CLI state, and browser sessions. Do not temporarily alter them with a promise to restore them. Use an isolated browser context when available; otherwise preserve pre-existing browser state and remove only this run's additions. Never perform account-wide logout/revocation or clear all cookies from a shared context. Safe metadata discovery is allowed; avoid reads that wake workloads or reveal secrets. Never buy a plan, add a payment method, create paid resources/add-ons, exhaust shared quotas, or attach domains you do not control. Skip journeys without a safe free fixture.

Before mutating or deleting an existing object, require the exact workspace/ID in this run's ledger with creation proof and absent from the baseline. A name prefix, name match, or absence from the baseline alone is insufficient. For timed-out creates, reconcile the pending intent against same-workspace state using the unique nonce, creation time, request/response evidence, and baseline before retrying; if ownership cannot be proved, do not delete the ambiguous object or repeat the create. Report the uncertainty and stop new creates until resolved. Name-selector tests may target only a uniquely resolved ledger-owned ID. Before deleting a parent, enumerate its dependents and refuse any cascade that could touch a non-owned resource. Cleanup is mandatory after each journey, including failures.

Build the sweep from the current help tree and compatibility ledger. For each surface, test valid success plus a meaningful supported failure, with real stdout, stderr, exit code, elapsed time, and observable server state. Use the upstream noninteractive flags only where supported; use a bounded PTY for genuinely interactive commands and record external prerequisites (`ssh`, `psql`, etc.). A non-TTY guard is not a server defect.

| Journey | What to prove |
| --- | --- |
| Auth and workspace | Human login → whoami → select/current → read/write/sensitive operation where safe; refresh on this run's session when feasible; logout last. Wrong credentials fail without appearing authenticated. |
| Discovery and selectors | Lists, supported filters/pagination, ID/name selectors, project/environment membership; distinguish empty results from failed requests. Use nonexistent synthetic IDs for negative cases, not other tenants' IDs. |
| Services | Create supported free web/static/cron/worker/private fixtures where available → inspect/list → safe update → observe deployment and real runtime behavior. A 2xx or printed deploy ID is not proof of a working service. |
| Deploys and jobs | Supported create/list/detail/cancel/restart/rollback flows on owned fixtures; wait for terminal state and compare it with live HTTP behavior and logs. Don't invent flags or force unsupported actions. |
| Env and configuration | Use supported create/update flags for env vars, secret files, health/build/start/cron settings; read back safely and prove a harmless marker reaches the process. Never capture real secrets. |
| Logs | Supported ranges, resource filters, and live streaming; generate a unique harmless marker, verify filtering, bound the stream, and check interrupt behavior. |
| Postgres and Key Value | Free create → inspect → connect through supported CLI entrypoints → harmless query/PING → delete. Verify ID/name resolution and actual connections without printing connection credentials. |
| SSH | Supported service/name/instance selection reaches the intended owned running instance; preserve passthrough semantics; exclude documented ephemeral non-goals. |
| Blueprints | Validate valid and invalid `render.yaml` fixtures; inspect output modes, source locations, and exit status. Validation is not apply/deploy evidence. Test apply only if the pinned CLI exposes it. |
| Sandbox | Exercise supported create/list/exec/stop only if free and safely disposable; verify output/exit status and eventual deletion. |
| Branding and automation | Bex root/nested help, examples, docs destination, version identity and completion; parse supported JSON output and separate diagnostic streams according to upstream semantics. Preserve contract filenames and known residuals. |
| Cleanup | Delete by recorded ID in dependency order; poll for absence from detail/list and dependent runtime state, not just successful delete acknowledgment. |

Mark unsupported surfaces (for example a dashboard-only action) as uncovered, not failed. REST/GraphQL/browser probes can diagnose a CLI failure, but do not count as CLI journey coverage. A second upstream mutation must use a fresh equivalent fixture, not replay a destructive request blindly.

## 3 — Reproduce and locate the failing boundary

Reproduce each candidate in a fresh CLI process. Retry throttled reads at human pace after 30–60 seconds; do not turn sweep-induced 429s into product findings. Check host, workspace, auth grant/scopes, client version, missing external tools, non-TTY restrictions, and async convergence first. Use the actual CLI's User-Agent for HTTP diagnosis; a Python-urllib Cloudflare block is not evidence that the CLI fails. Conversely, a real supported CLI blocked at the edge is a legitimate infrastructure candidate.

Trace **command → pinned upstream request builder → HTTP route → backend handler/service → response decoder → output/exit**. Locate the installed dependency with `go list -m -f '{{.Dir}}' github.com/render-oss/cli` from `lego/cli`; read its actual code and schemas. Consult official upstream sources at that exact commit when local source is unavailable. Current online documentation must not silently override the pin.

Where useful, build/run the **unmodified same-pin Render CLI against Bex**, in separate temporary config with equivalent QA authentication and workspace. Never point it at Render production or reuse personal Render credentials. Inspect `scripts/cli-compat.sh` before reusing any part: do not run a broad harness whose credentials, target, costs, and cleanup are unknown.

- Same valid wire request fails in both binaries against Bex: investigate server/edge compatibility; prove the response violates the pinned consumer's requirement.
- Only Bex fails: compare bridge configuration and branding/native glue before blaming the backend.
- Pinned client fails before sending a request: inspect upstream validation/runtime or local prerequisites. Reproduce upstream defects separately; do not file a server fix for them.
- Server reports success but runtime fails: trace operator/build/network/data-plane behavior, retaining the CLI reproduction.

Preserve the exact method, path, query, non-secret body, status, response headers relevant to the claim, and full redacted response. Replay the same request without “repairing” payloads first; a corrected payload succeeding does not make the upstream request invalid. Prefer existing safe diagnostics or a private credential-reading probe; never enable unredacted HTTP tracing, put Authorization values in command arguments, or capture tokens in a general transcript. Mark every redaction and leave non-secret types/nulls/envelopes intact.

## 4 — Research a concrete fix and durable evidence

Read applicable cascading guides and ADRs. Start with `lego/backend/internal/` for REST adapters/auth/serialization and shared domain services, `lego/operator/` for reconciliation/runtime, and `lego/cli/internal/bridge` or `branding` for proven launcher issues. Use `docs/CLAUDE.md` to locate the governing ADR.

For every finding, read producer **and consumer**, generated types/serializers and pinned library paths. Specify the exact target status/body/behavior that satisfies the consumer, including forbidden/unauthenticated/not-found/timeout neighbors without introducing resource-existence leaks. Search all callers and aliases of shared code and enumerate affected resource types. Trace similar symptoms separately. Reconcile evidence that contradicts the proposed cause or deployed revision; label an unverified cause rather than inventing a file:line diagnosis.

Write one record per reproducible bug:

```text
### <symptom>
- Severity: blocker | major | minor
- Versions: Bex path/release or build SHA; upstream pin; deployed revision or unknown
- Context: date, API origin, QA workspace, human vs machine auth, TTY/output mode
- Repro: exact commands with credential placeholders; fixture and preconditions
- Expected / actual: target behavior from the pinned contract; stdout, stderr, exit, duration
- Wire evidence: exact redacted request and complete redacted response, or not captured
- Runtime evidence: externally observed state and bounded wait
- Attribution: server | infrastructure/runtime | Bex launcher | upstream | unverified
- Root cause: file:line and traced mechanism, including pinned client consumer
- Fix: exact target shape/behavior; REST/GraphQL/MCP/UI implications
- Blast radius: counted callers, aliases, sibling resource types, global vs allowlisted fix
- Regression checks: failing CLI journey plus valid control and affected existing callers
- Unverified: everything inferred but not exercised
- Dedupe / non-goals / deploy lag: searches and matching board/history references
- Estimate: tens of minutes
```

Blocker prevents a core journey; major misleads, loses data, or materially breaks behavior; minor is polish. Do not inflate known upstream residuals into Bex defects.

Evidence must survive handoff: include sanitized commands and responses in the board record, not merely a gitignored transcript path. Keep raw captures private, sanitize **before** returning tool output or filing, and exclude credentials, secret env values/files, connection URLs, and device codes. Check every cited artifact exists and supports that particular claim. If secret material is essential to a reproduction, provide placeholders and safe acquisition steps rather than publishing it.

## 5 — Dedupe, file, and optionally ship

Search `.pm/` open **and done** with `rg` for distinctive symptoms, endpoints, and symbols; inspect open milestones across all workstreams. Re-read `.pm/DO_NOT_DO.md` and the CLI compatibility ledger. Check targeted `git log -S` and recent CLI/backend/operator history for fixes already on main but not deployed. Report deploy lag without filing a new implementation bug. Existing open coverage gets an update through `/pm`; a regression cites the original milestone and rechecks its complete DoD.

Unless `DRY_RUN=1` or audit-only, use `../pm/SKILL.md` to file researched, non-duplicate findings (default `w6`). Only `/pm` writes `.pm/`. Use an inbox note for ≤ ~1 hour; a milestone requires > ~1 hour across multiple tasks. Supply source/goal linkage, one task per bug, estimates/dependencies, explicit observable CLI acceptance commands and target results, and unverified areas. Shared-code blast-radius verification gets its own task. Let `/pm` allocate numbering and standing closing tasks, including parity when applicable. Verify status/frontmatter consistency and required Markdown formatting.

**Commit/push only with explicit user `/ship` or `$ship` authorization.** Invoking this QA skill alone does not override the repository's commit rule. When authorized, read `../ship/SKILL.md` and ship only this hunt's board filing; exclude pre-existing changes and secret/raw artifacts. Otherwise leave the filing reviewable and uncommitted. Creating or editing this skill does not authorize running a live hunt or shipping it.

## 6 — Cleanup and report

Cleanup is a completion gate after every journey/sweep and on cancellation, failure, or interruption. Register cleanup with `try/finally` or shell traps where practical, and reconcile the persisted ledger when resuming; traps alone do not survive a killed process. Delete owned children before parents by exact recorded IDs, then poll detail/list absence and dependent runtime teardown with bounded waits. Check that baseline resources remain present; never revert unrelated concurrent changes.

If CLI cleanup fails, inspect resulting state before retrying and use the same-workspace API with the same QA authority as an authorized fallback for proven-owned IDs only. Never use admin access, bulk prefix deletion, force deletion, or changes to shared infrastructure. Resolve routine errors and finish cleanup before creating more fixtures. If cleanup remains blocked after bounded retries, stop new mutations, prominently report surviving IDs/workspace and the exact blocker, and retain a sanitized recovery ledger; never claim cleanup succeeded or discard the only ownership proof. Resume cleanup first when access recovers.

After resource cleanup, log out only this run's isolated device session, terminate owned processes/loopback servers, and remove owned temporary credentials/config/captures and browser state. Never remove pre-existing files, sessions, cookies, or credentials. Environment-token logout does not revoke that token; report any necessary remaining credential cleanup accurately. Cleanup instructions remain in force even when the user stops the loop.

Report journeys exercised/skipped with reasons, findings by severity with root cause or explicit uncertainty, proposed server/launcher/upstream ownership, exact filing locations, duplicates/non-goals/deploy lag, and cleanup status. List any surviving resource IDs/workspace and why deletion failed prominently. State whether the filing is uncommitted or give the shipped HEAD; distinguish scheduling from implemented fixes.
