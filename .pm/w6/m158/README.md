# w6 · m158 — Platform build in minutes on the existing runners: native compile, cached and parallel image builds

**Worker:** worker6 **Goal:** the `build` job of `deploy.yml` finishes in ≤ 30 min (from 60–187 min) on the existing self-hosted runners, so a green main reaches production in about an hour. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Cross-compile the Go/Node build stages from `$BUILDPLATFORM`](t001.md) | 45m | — |
| t002 | [Registry layer cache and cache mounts for every platform image](t002.md) | 60m | t001 |
| t003 | [Build the five images, and sign + SBOM them, in parallel](t003.md) | 90m | t002 |
| t004 | [Start the build with the tests; gate only the pin on them](t004.md) | 45m | t003 |
| t005 | [Measure five main deploys against the baseline](t005.md) | 30m | t004 |
| t006 | [Simplify](t006.md) | 15m | t005 |
| t007 | [Test coverage](t007.md) | 30m | t006 |
| t008 | [Closeout](t008.md) | 10m | t007 |

## Definition of done

- No compiler (Go, Node/TypeScript) runs under amd64 emulation in `deploy.yml`'s `build` job; only target-platform runtime stages (`apt-get`, copying binaries) still do.
- Over 5 consecutive successful main deploys, `build` takes ≤ 30 min each (median and max reported from `gh run view --json jobs`), against the 2026-11-03/04 baseline of 61.6, 136.0 and 186.9 min (runs 41705171927, 41870722101, 41816119204).
- A deploy whose image inputs did not change rebuilds those images from cache (cache hits visible in the build log); only main can write the cache; `scripts/github-actions-validate.sh` passes.

## Source + Goal linkage

- **Source:** user discussion 2026-11-04 ("讨论一下怎么给 bex platform build 提速"). Baseline step timings: operator image 13–16 min, agent sandbox image 9–93 min, sign + SBOM 12–25 min, all serial, on ARM64 runners (this Mac Studio's Docker Desktop) emulating `linux/amd64`, with no `cache-from`/`cache-to` and a fresh buildx builder per run.
- **Decision 2026-11-04 (user):** keep all CI on the local Mac Studio runners — cheaper than moving to Hetzner. A native amd64 Hetzner runner (`bex-build-amd64-1`, cx43) was provisioned and deleted the same day, unused. Moving CI to the cloud (~€150/month for 2× cax41 + cx43/cx53 + cax31) is not planned; reopen only on a user decision.
- **Goal linkage:** ADR008 reliable delivery — finished work reaches production promptly, which every live-verification DoD depends on.
- **Expected outcome:** a deploy run takes about an hour instead of 1.5–4 h, and fewer runs are superseded while they build (w6/m156).
- **Why now:** w6/m156's replay showed six fully built runs discarded in four 11–25 h gaps because a run takes 1–3 h; shortening the run shrinks that window directly, alongside m156's allowlist (D8 = (a)).
- **Render parity:** omitted. CI/deploy infrastructure, no REST/GraphQL/MCP/UI surface.
