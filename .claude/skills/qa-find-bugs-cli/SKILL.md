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

## 0 — Preflight and isolation

Record branch, HEAD, initial `git status --porcelain`, executable path, Bex release/build identity, upstream pin, API origin, and date. Record deployed revision only if observable; do not assume it equals HEAD. A hunt need not switch branches. If shipping is authorized later, follow the ship skill's branch rules then.

Use the installed customer binary first when available. If building from `lego/cli`, put the binary in a private temporary directory and label all evidence as a checkout build. A release-only failure must be compared with HEAD before filing. Do not install over the user's binary.

Create a private temporary directory (0700) and set `BEX_CLI_CONFIG_PATH` to a new config inside it; never borrow or overwrite personal CLI state. Use a per-process environment with inherited `RENDER_*` overrides and unrelated Bex auth/host/workspace/config overrides removed. Explicit `RENDER_*` inputs take precedence over Bex mappings and can silently select the wrong host or identity. Do not repurpose `HOME` or print the environment. Set the production `BEX_HOST` explicitly and disable update notices with `BEX_NO_UPDATE_NOTIFIER=1`. Track background processes and temporary files for cleanup on failure as well as success.

Inspect root and relevant subcommand `--help` before constructing commands. Derive flags and selector forms from this binary; do not invent dashboard-equivalent commands. Use bounded execution for streaming, interactive, and async journeys. After a mutation times out, inspect resulting state before retrying: the server may already have applied it.

## 1 — Authenticate as the QA user

Prefer **human device login**, because client-credentials tokens do not exercise the same granular capability checks as human tokens. A machine-token success cannot certify the human login path.

1. Read `scripts/qa-login.sh` and the login instructions in `../qa-find-bugs/SKILL.md`. Use `bash scripts/qa-login.sh --serve` and its one-shot loopback cookie handoff to sign the Playwright browser in without reading `.env` or placing passwords/cookies in tool code. Browser tools are needed only for device approval and optional cross-surface diagnosis.
2. Run the isolated `bex login`, retain the process while it waits, and visit its **actual Bex device-verification URL** in that authenticated browser. Approve only this run's device request. Keep login captures private; do not publish device codes, token responses, cookies, or config contents.
3. Confirm login terminates successfully and creates an owner-only config. Run `bex whoami`, `bex workspaces -o json`, and the supported workspace selection/current commands. Establish the QA identity and intended QA workspace before any write. If the workspace is ambiguous, ask for its identity; never pick the first workspace merely because it is listed.
4. Record existing resource IDs and plan constraints. Use explicit `BEX_WORKSPACE` thereafter. Do not switch into unrelated workspaces for isolation probes.

If browser access is unavailable, an already-authorized QA OAuth credential can be passed privately via `BEX_ACCESS_TOKEN`; report device login/refresh/logout as untested. An API-key client secret is **not** a bearer token: follow the exchange contract in `docs/bex-cli.md`. Do not mint new keys or use admin credentials merely to bypass a failed human flow. If no safe auth route exists, complete offline help/contract inspection and report live testing blocked. Missing QA credentials belong in `.env`, never chat.

`scripts/bex-cli-auth-e2e.sh` is useful implementation precedent, **not a drop-in production QA login helper**: it has local defaults, admin dependencies, identity-creation options, and its own session cleanup. Read it before any reuse; do not run production identity creation or admin flows for this hunt.

## 2 — Exercise whole CLI journeys

Create only in the confirmed QA workspace. Name every new resource `qa-<yyyymmdd>-<unique-suffix>` and maintain an ID-based creation ledger. Mutate/delete only resources created in this run; a matching prefix alone is not ownership proof. Never buy a plan, add a payment method, create paid resources/add-ons, alter existing resources, or attach domains you do not control. Skip journeys without a safe free fixture. Clean up even when a probe fails.

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

Perform cleanup before ending, including failed/partial journeys. Verify every ledger resource is gone; if CLI deletion fails, a same-workspace API deletion of this run's resource is a permitted cleanup fallback and must be disclosed. Do not bypass authorization with admin access. Log out the isolated device session, terminate owned processes, remove temporary credentials/config/captures, and unset environment credentials. Environment-token logout does not revoke that token; report any necessary remaining credential cleanup accurately.

Report journeys exercised/skipped with reasons, findings by severity with root cause or explicit uncertainty, proposed server/launcher/upstream ownership, exact filing locations, duplicates/non-goals/deploy lag, and cleanup status. List any surviving resource IDs/workspace and why deletion failed prominently. State whether the filing is uncommitted or give the shipped HEAD; distinguish scheduling from implemented fixes.
