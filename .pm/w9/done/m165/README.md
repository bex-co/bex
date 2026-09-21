# w9 · m165 — A Docker cron job's command is dropped on create and invisible on read

**Worker:** worker9 **Goal:** the command a user gives a Docker-runtime cron job is the command it runs, and every surface reads back the value that is actually stored **Status:** done

## Tasks (in order)

| id   | title                                                                  | est | depends_on |
| ---- | ---------------------------------------------------------------------- | --- | ---------- |
| t001 | Create: stop the docker branch from discarding the cron's startCommand  | 45m | —          | — **DONE** |
| t002 | Read-back: project a cron's command into `envSpecificDetails`           | 45m | —          | — **DONE** |
| t003 | Decide and document the cron command's canonical wire spelling          | 30m | t001, t002 | — **DONE** |
| t004 | Close m56's hole: cover the **docker** cron command in the verifier     | 60m | t003       | — **DONE** |
| t005 | Render parity across the touched surfaces                               | 45m | t003, t004 | — **DONE** |
| t006 | Simplify the code this milestone changed                                | 30m | t005       | — **DONE** |
| t007 | Test coverage for the shipped behavior                                  | 45m | t005       | — **DONE** |
| t008 | Closeout                                                                | 30m | t007       | — **DONE** |

## Definition of done

- `bex services create --type cron_job --runtime docker --cron-schedule '<sched>' --cron-command '<cmd>'` produces a cron whose stored command is `<cmd>` — not empty, and not the image's default entrypoint.
- Reading that service back through REST (`GET /v1/services/{id}`), GraphQL and MCP reports `<cmd>` in the place a Render client looks for it, so `bex services get` and `bex services create --from <that cron>` both see it. Today `--from` clones an empty command because `startCommandFromEnvDetails` reads the empty projection.
- `bex services update --cron-command '<new>'` changes the stored command **and** shows the new value in its own output. Today the write lands but the printed `envSpecificDetails.dockerCommand` stays `""`, so the CLI reports a no-op it did not perform.
- A native-runtime cron keeps working exactly as it does today (the control that is already green).
- `scripts/cli-services-parity-verify.sh` (the `w9/done/m56` verifier) fails if either half regresses. m56's DoD claims every supported create/update flag "either round-trips exactly or fails explicitly — never as a silent no-op"; that claim currently has a docker-cron-shaped hole.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs-cli` sweep 2, 2026-09-21 ~21:00–21:10 UTC. Workspace `bex-canary` (`tea-daif693dqjvc73e7as3g`); fixtures `qa-20260921-4763a134-{static,cron,dweb,ncron}` plus two raw-API probe services — all created and deleted in-sweep, baseline `hello-go` untouched and verified still present. Installed `bex v0.2.1`, upstream pin Render CLI v2.27.0 (`a764810a7682`), human device-flow OAuth.
- **Goal linkage:** bex is the open-source Render alternative (ADR008) and the pinned Render CLI is the wire-contract oracle. A cron that silently ignores the command you gave it is worse than one that refuses: the service exists, deploys, and runs the wrong thing on a schedule. Governing docs: ADR006 (bex-api surfaces), ADR018 (parity ledger — its create row explicitly claims "Docker honors context, Dockerfile path, and command").
- **Expected outcome:** Docker cron jobs run the command they were configured with, and the CLI stops reporting successful updates that its own read-back contradicts.
- **Why now:** `w9/done/m56` shipped a verifier whose stated purpose is that no supported flag is ever a silent no-op, and it covered only the **native** `--cron-command` path — so this regressed class sits directly inside an invariant the project already paid to guarantee. `w9/done/m44` t004 also worked around the symptom ("Render CLI requires `--cron-command` even for an image-entrypoint cron … Added `--cron-command "/cron-demo"`") without noticing the value never took effect, because a cron that runs its image entrypoint still produces a run in the logs.

## Evidence (live, 2026-09-21, production)

**Create drops it.** Raw `POST /v1/services` with the exact body the pinned CLI builds for `--type cron_job --runtime docker --cron-command 'echo qa-cli-shape'`:

```json
{"type":"cron_job","name":"…","serviceDetails":{"runtime":"docker","plan":"free",
 "schedule":"*/10 * * * *",
 "envSpecificDetails":{"buildCommand":"","startCommand":"echo qa-cli-shape"}}}
```

→ `201`, and the created service has **no `command` at all**:

```text
serviceDetails.command : None
envSpecificDetails     : {"dockerCommand": "", "dockerContext": "examples/hello-go", "dockerfilePath": ""}
```

The identical journey with `--runtime go` (native) works:

```text
command     : "echo qa-native-cron-4763a134"
envSpecific : {"buildCommand": "…", "preDeployCommand": "", "startCommand": "echo qa-native-cron-4763a134"}
```

**Read-back hides it.** `PATCH /v1/services/{id}` with `{"serviceDetails":{"envSpecificDetails":{"dockerCommand":"echo qa-probe"}}}` → `200`, and the command **is** stored — but it is invisible where it was written:

```text
serviceDetails.command                     : 'echo qa-root-level'
serviceDetails.envSpecificDetails.dockerCommand : ""
```

A `web_service` on the same runtime does **not** have this problem — the same PATCH round-trips (`"" → "echo qa-probe"`), which is the control that localizes the bug to the cron path rather than to docker services generally.

## Mechanism (traced)

- **Create** — `lego/backend/internal/apps/rest.go:476-486`: when the runtime is docker, `startCommand` is *overwritten* from `EnvSpecificDetails.DockerCommand`, discarding whatever arrived in `StartCommand`. Three lines later, `rest.go:498-500` does the cron bridge — `if command == "" && r.Type == TypeCronJob { command = startCommand }` — whose own comment reads "official CLI encodes cron command in envSpecificDetails.startCommand". That is exactly right, and the docker branch above it defeats it: the CLI's cron builder (`pkg/service/create.go:315-330 buildCronEnvSpecificDetails`) emits the **native** shape for every runtime, with no docker branch, so a docker cron's command arrives in `StartCommand` and is thrown away.
- **Read-back** — `lego/backend/internal/apps/render.go:495`: the docker projection is `"dockerCommand": a.StartCommand`. A cron's command is stored in `a.Command`, not `a.StartCommand`, so a docker cron with a perfectly good command always projects `dockerCommand: ""`.
- **Update** — `rest.go:853-856` decodes `dockerCommand` into `f.startCommand` correctly and the write reaches storage; but the response is rendered through the same `render.go:495` projection, so the CLI prints a value that contradicts what it just stored.

## Blast radius

- `bex services create --from <docker cron>` clones an empty command: the pinned client reads it via `pkg/service/clone.go:143` `startCommandFromEnvDetails(details.EnvSpecificDetails)`, which sees the empty projection.
- Any consumer that round-trips a service definition through the read shape — blueprint generation, the dashboard's cron settings, MCP `update_service` — is reading a field that does not reflect storage. `blueprint_generate.go:283,297` already has its own `dockerCommand`/`Command` handling; check whether it agrees with the REST projection or has a second, divergent answer.
- The write half is not lost data — the command is stored — so no existing cron needs repair beyond re-reading. Confirm that before closing: a cron created before this fix has an empty command and **will** need its command re-set.

## Unverified

- Whether the scheduled run actually executes the image entrypoint when `command` is empty. The next fire was 5 minutes out and the fixture was deleted first; the claim here is about configuration round-trip, which is proven at the wire, not about the executed process.
- GraphQL and MCP were not probed for the same projection — t005 covers them.
- Whether Render itself accepts a native-shaped `envSpecificDetails` on a docker cron (i.e. whether upstream's no-runtime-branch cron builder is an upstream defect or relies on server-side leniency). t003 should answer it, because it decides whether bex should be liberal in what it accepts or upstream should be reported.

## Closeout 2026-09-21 (`/loopx w9`)

**Done, and verified live — not only in tests.** Three halves, the third of
which only appeared once the first two were exercised end to end:

1. **Create** (`rest.go`) — the docker branch no longer discards the cron's
   `startCommand`; either spelling is accepted for a cron, `dockerCommand` wins
   when both are sent, and every non-cron docker service keeps the prior rule.
2. **Read-back** (`render.go`) — a docker cron's `dockerCommand` is folded from
   `Spec.Command`, the same fold the native branch and blueprint generation
   already did.
3. **Clone** — with 1 and 2 shipped, every bex-side read looked right and
   `create --from` *still* cloned an empty command: the pinned client resolves a
   cron's command only through `AsNativeEnvironmentDetails().StartCommand`
   (`pkg/service/clone.go:143,328-334`), mirroring its create path having no
   docker branch. A docker cron now emits the command in **both** spellings.

**Live evidence (dev-9, pinned CLI, 2026-09-21):**

```text
1. after create                 command: './job docker-create'  dockerCommand: './job docker-create'  startCommand: './job docker-create'
2. after update --cron-command  command: './job docker-update'  dockerCommand: './job docker-update'  startCommand: './job docker-update'
3. create --from (clone)        command: './job docker-update'  dockerCommand: './job docker-update'  startCommand: './job docker-update'
```

Before the fix the same journey produced `command: None` / `dockerCommand: ''`
on create, and a clone with an empty command.

**Guards:** `internal/apps/cron_docker_command_test.go` (mutation-checked — each
revert fails a distinct test) plus three new verifier legs
(`docker-cron-create`, `docker-cron-update`, `docker-cron-clone`) registered in
the leg census, closing the docker-shaped hole in `w9/done/m56`'s "never a
silent no-op" claim. Full backend suite + `make lint-backend` (0 issues) green.

**Decision recorded** (t003): a cron's command is runtime-independent on the
wire — `docs/ADR006-bex-api.md` and the `--cron-command` row of
`docs/cli-compatibility-checklist.md`.

**Not claimed:** whether a scheduled run executes the image entrypoint when the
command is empty (the note's own open question) — this milestone is about the
configuration round trip, which is proven at the wire. Pre-existing docker crons
created before this fix still have an empty stored command and need it re-set;
the fix does not backfill them.
