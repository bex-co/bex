# w32 · m462 — Passenger Standalone starts on a native Ruby service: native runtimes run `render-v1`, Render's measured capability set

**Worker:** worker32 **Goal:** every native runtime service (web, private, worker and cron, with their pre-deploy and one-off jobs) runs under `render-v1`, the capability set ADR089 measured on Render, as Docker and image services already do. `bundle exec passenger start`, and any other server that lowers its privileges while running as root, then starts on bex as it does on Render. **Status:** in progress (t001–t007 done; t008 waits for a production pin)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | New native runtime services get `render-v1` on every creation path — **DONE** | 45m | — |
| [t002](done/t002.md) | Native workloads under `render-v1`: the operator projection, and the Passenger image under the pod's real security context — **DONE** | 45m | — |
| [t003](done/t003.md) | The operator-run migration moves existing native services, one at a time or all at once — **DONE** | 45m | t001 |
| [t004](done/t004.md) | ADR089, ADR022 and ADR018 bring native runtimes into `render-v1`'s scope — **DONE** | 25m | t001, t002, t003 |
| [t005](done/t005.md) | Render parity — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Simplify — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Test coverage — **DONE** | 40m | t005 |
| [t008](t008.md) | Closeout (production: migrate existing native services, walk the Passenger fixture, unblock w32/m424) | 60m | t006, t007 |

## Definition of done

On production, in `bex-canary`, with owned free fixtures deleted afterwards (by-id `GET` `404`):

1. **Passenger starts.** A new free native Ruby web service with w32/m424's Passenger fixture (passenger 6.1.0, rack 3.1.8, `bundle exec passenger start`, no port configured) goes `live` on its first rollout, listens on 10000 and answers `200`. With its own `PASSENGER_PORT=9000` it listens on 9000, goes `live` and answers `200`. The App reads `containerPolicy: render-v1`, and the pod reads `CapEff 00000000000400cb` (exactly the six) with the Localhost seccomp profile `profiles/bex-render-v1.json`.
2. **Every creation path.** A native web, private, worker and cron service created through REST, GraphQL, MCP, the dashboard and a Blueprint each gets `render-v1`. A static site stays strict. A native service's port mode is unchanged (never an image port mode).
3. **Existing services.** The operator-run migration planned, then applied, every existing native service on production. Each one rolled once as a tracked `settings_change` deploy and kept its status. The number migrated and every refusal (suspended, in flight, no control-plane row) are recorded in t008.
4. **No regression.** A native Python Gunicorn service (w32/m424 DoD 2's fixture) and a native Node service go `live` under `render-v1`. A native cron run and a native pre-deploy command succeed. A native service with a disk writes a file and reads it back after a restart, under the pod user namespace's idmapped mount.
5. **Docs.** ADR089 (policy table row and § Privilege slice decisions item 6), ADR022 and ADR018 put native runtimes in `render-v1`'s scope and give the basis. ADR089 records that Render's native process identity is unmeasured, and a follow-up item to measure it is filed.
6. **w32/m424 unblocked.** DoD 1's evidence is recorded on w32/m424 through `/pm unblock w32/m424`, so its t007 can close.

## Source + Goal linkage

- **Source:** inbox note w32/038 (promoted here 2026-11-17), filed by w32/m424 t007's production walk on 2026-11-17, pin `6984548a30f8`, in `bex-canary`. Severity **major**: every Passenger Standalone app fails, with no workaround short of switching servers.
- **Evidence (from w32/038):**
  - **Production.** The fixture `qa-20261117-m424-passenger` (`srv-dbu2uuhc204s73bi6v8g`, deleted afterwards) was a free native Ruby 3.4.12 web service. Its build command wrote a `Gemfile` with `passenger` 6.1.0 and `rack` 3.1.8 and a `config.ru`, then ran `bundle lock`, `bundle install` and `passenger-config install-standalone-runtime --auto`. Its start command was `bundle exec passenger start`. The build passed, and `PASSENGER_ENGINE=builtin` and `PASSENGER_PORT=10000` reached the build and the pod. The container then printed `Could not start the server engine: Daemon 'PassengerAgent watchdog' failed to start.` and exited 1. The deploy closed `update_failed` ("container exited shortly after start and is restarting repeatedly (last exit code 1)"). The pod ran as uid 0 with `CapEff: 0000000000000000` (`capabilities.drop: [ALL]`, RuntimeDefault seccomp), and the App's `containerPolicy` was empty.
  - **Local repro** on `ruby:3.4` with the same Gemfile, Passenger's environment variables and start command. With Docker's default capabilities it answered `200` on 10000. With `--cap-drop ALL` it exited 1, and Passenger's log read `ERROR: Unable to lower Passenger watchdog's privilege to that of user 'root' and group 'root': cannot set supplementary groups: Operation not permitted (errno=1)`. With `--cap-drop ALL --cap-add SETGID` and Docker's default seccomp it answered `200` on 10000.
  - **Cause.** Passenger Standalone always starts the watchdog with `--no-user-switching --default-user <current user>` (`standalone/start_command/builtin_engine.rb:103-131` in passenger 6.1.0). Running as euid 0 with a user set, the watchdog's `lowerPrivilege()` calls `initgroups`, `setgid` and `setuid` (`src/agent/Watchdog/WatchdogMain.cpp:981-1011`), even when the target is root itself. `setgroups` needs `CAP_SETGID`.
  - **bex's side.** `runsContainerPolicy` (`lego/backend/internal/apps/service.go:2933-2936`) gives the widened policy to Docker and image services only. `ContainerPolicyCapabilities` (`lego/types/v1alpha1/container_policy.go:35-44`) adds nothing for strict. [ADR089](../../../docs/ADR089-render-container-compatibility.md)'s policy table says "Static sites and native runtimes are out of scope" for `render-v1`, whose measured Render set includes `SETGID`.
- **Route, and why it needs no user sign-off.** The m424 worker asked whether widening native runtimes needs the user, because ADR089 says not to widen the allowlist silently. It does not:
  - ADR089 (§ D2's security paragraph) already reads: "**Superseded 2026-10-20:** under Render parity first the allowlist widens to Render's measured set".
  - ADR089 § Privilege slice decisions item 1 sets the criterion: bex "may be more permissive where Render is unmeasured, but it is never stricter on a measured axis". Render's measured set includes `SETGID`, so a strict native runtime is stricter than Render on a measured axis.
  - AGENTS.md (Render parity first) says to amend the older ADR rather than keep the divergence. So this milestone ships `render-v1` for native runtimes and amends ADR089's scope row.
  - **Not decided here: Render's native process identity.** If Render runs native services as a non-root user, Passenger there never calls `initgroups`. Render's docs do not say (render.com/docs/native-runtimes, /docs/troubleshooting-deploys and /docs/ssh, checked 2026-11-17), and this checkout has no Render credential to open a shell. That is a stricter variant, not a gate: t004 files it as its own item, and `render-v1` is parity-sufficient either way.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). An app that starts on Render must start on bex unchanged, in every workspace. Passenger is a documented Render Ruby setup (`PASSENGER_ENGINE=builtin` and `PASSENGER_PORT=10000` are Render's own defaults, w5/162 and w32/m424).
- **Expected outcome:** a Passenger Standalone app moved from Render deploys and serves on its first rollout. Native services keep the same seccomp, `NoNewPrivs` and user-namespace posture as `render-v1` Docker services.
- **Why now:** it gates w32/m424's last DoD (the milestone is parked on it). It also breaks every Passenger app today, with no workaround.
- **Render parity task included:** the creation default must be the same on REST, GraphQL, MCP, the dashboard and Blueprints, and the runtime behaviour is tenant-facing.
- **Dedupe:** `rg -i 'passenger|SETGID|initgroups' .pm docs` finds no other open item. w5/162 (done) added Passenger's environment variables only. w6/m149 (done) set `render-v1`'s scope to Docker and image services and left native runtimes out.
