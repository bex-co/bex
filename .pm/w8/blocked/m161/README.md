# w8 · m161 — A Node build reuses its dependencies between builds, and `NODE_MODULES_CACHE` switches it

**Worker:** worker8 **Goal:** a second build of a native Node service whose lockfile did not change reuses the dependencies the previous build installed, as on Render; `NODE_MODULES_CACHE` reads `true` by default and `false` builds without the cache; "Clear build cache & deploy" clears it too. **Status:** blocked (2026-11-12: promoted from w8/275; the design below needs the build plane on a cluster to verify before any of it ships)

## Tasks (in order)

| id   | title                                                                                         | est | depends_on |
| ---- | --------------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Measure today's second build of an unchanged lockfile on the mock cluster                      | 30m | —          |
| t002 | Carry the dependency cache: save after a build, restore before the next (design below)         | 2h  | t001       |
| t003 | `NODE_MODULES_CACHE` default `true`; `false` and a clear-cache deploy build without it          | 30m | t002       |
| t004 | Render parity (ADR018 runtime-defaults row; ADR060 amendment for the deferred package cache)  | 15m | t003       |
| t005 | Simplify                                                                                      | 15m | t004       |
| t006 | Test coverage                                                                                 | 45m | t004       |
| t007 | Closeout (live replay on production)                                                          | 30m | t006       |

## Definition of done

- On the mock cluster and then production: a Node service built once, then rebuilt after a source-only change with the same lockfile, spends less time in its install step, and the build log says the dependencies were restored.
- With `NODE_MODULES_CACHE=false`, or a deploy with the build cache cleared, the rebuild installs from nothing.
- `NODE_MODULES_CACHE` reads `true` in a Node build and at runtime unless the service sets it.
- A first build, a build whose lockfile changed and a build whose cache artifact is missing or corrupt all succeed (the cache affects speed, never results, ADR060 D3).

## Design (proposed, to verify in t001–t002)

- **Why not a BuildKit cache mount.** `RUN --mount=type=cache` lives in the builder's local state, and bex's build pods are ephemeral; ADR060 D3 says so and deferred package caches for exactly that reason. The layer cache bex does carry (BuildKit `mode=max` exported to `/cache-out`, carried to Zot by skopeo) cannot help either: the native recipe runs the whole build command after `COPY . .`, so any source change invalidates the install.
- **What to carry.** The dependency directory the build left in the service root (`node_modules`), keyed by the lockfile (`package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`, `bun.lock[b]`) and the Node line, as one per-App OCI artifact next to the layer cache tag (`…/cache:deps`), saved by a phase after a successful build and restored by the existing cache-restore init container into a volume BuildKit reads as a named build context. The Dockerfile then copies it into the source tree before the build command when the key matches. Registry credentials stay in the skopeo containers, never in BuildKit.
- **Why `node_modules` and not the package manager's store.** Render documents only `NODE_MODULES_CACHE`, a node_modules switch, and its default build commands are `npm install` / `yarn` / `pnpm install` / `bun install`, which reuse a present `node_modules`. `npm ci` deletes it; restoring npm's own cache (`~/.npm`) as well would cover that, at the cost of a second artifact; t001 decides.

## Source + Goal linkage

- **Source:** promoted from inbox note w8/275 (`/qa-find-bugs-cli` sweep `965c` via w8/256, 2026-11-11) by `/loopx w8` on 2026-11-12: more than one task and about 3 h, touching the build Job's phases and the registry cache.
- **Goal linkage:** Render parity for native Node builds (ADR018 "Default environment variables" row, `NODE_MODULES_CACHE` listed as not set); ADR060 D3's deferred package-manager cache. Render parity is a task because the defaults row changes.
- **Expected outcome:** Node rebuilds are as fast as Render's when dependencies did not change, and the variable Render documents means the same thing on bex.
- **Why now:** every other per-runtime default Render documents is set since w8/256, w8/272 and w8/274; this is the last one.

## Gate

The build plane must be exercised on a cluster before anything here ships: the save and restore phases run in the build Job, need the registry, and a mistake there fails every Node build. The local CAPD mock cluster is down until this host's inotify limits are raised (`sudo sysctl -w fs.inotify.max_user_instances=512 fs.inotify.max_user_watches=524288`).

## From the note (w8/275)


Why: Render caches a Node service's `node_modules` between builds and sets `NODE_MODULES_CACHE=true`; a service sets it to `false` to build without the cache. bex reinstalls dependencies whenever the source changes, and the variable is unset. Builds are slower than on Render, and there is nothing for the variable to switch.

**Severity:** minor (build time; no wrong result). **Attribution:** infrastructure/runtime (native build). **Estimate:** ~3h, milestone-sized. **Source:** `/qa-find-bugs-cli` sweep `965c` via w8/256, 2026-11-11.

## Why it was not set with the others (w8/256)

The variable is a switch for a cache. Defaulting it to `true` without the cache would state something untrue, and `false` would change nothing.

## Fix

1. Decide where a dependency cache can live: the native recipe copies the whole source before the build command, so any source change invalidates the install layer. A BuildKit cache mount for the package manager's store (npm, pnpm, Yarn, Bun) is the usual answer; check it against the build cache the registry already keeps (ADR060).
2. Honor `NODE_MODULES_CACHE=false` by building without it, and default the variable to `true` (`nativeBuildDefaults`).

## Acceptance

- A second build of an unchanged `package-lock.json` after a source-only change reuses the dependency cache; with `NODE_MODULES_CACHE=false` it does not.
- `NODE_MODULES_CACHE` reads `true` in a Node build and at runtime unless the service sets it.

## Unverified

- Whether bex's layer cache already makes some second builds fast; not measured.
- What exactly Render caches (the directory, or the package manager's store).

## Dedupe

w8/256 (the other per-runtime defaults). No `.pm` item covers a dependency cache for native builds.
