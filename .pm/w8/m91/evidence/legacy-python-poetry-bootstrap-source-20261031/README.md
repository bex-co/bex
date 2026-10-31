# Common-only default Poetry bootstrap source repair — 2026-10-31

Frozen four-file source delta from37ec75d713305fe8471c42ae63b8417653e60b56. This implements the root-authorized narrow repair; no PM, original28 fixture, root integration/testdata, production pin, Docker, CI or resource changes were made by this worker. The prior29-file caller audit remains unchanged and anchored to37ec75d71.

`nativeresolve.go:251` now gives only common-base Python3.7/3.8 and default Poetry2.1.3 a private tool venv bootstrapped by absolute `/usr/bin/python3`. Selected app Python/version/ENV/PATH and exact Poetry version are unchanged. An explicit `POETRY_VERSION`, including an explicitly incompatible2.1.3 pair, keeps the existing compatible-version contract and selected-app bootstrap behavior. Compatible explicit1.x releases, modern Python, historical supplied bases, uv, companions and authored command bytes retain their existing paths. The known release condition prevents a future default bump from silently inheriting this compatibility decision.

The common factory explicitly installs `python3-venv` at Dockerfile:28. Its final offline command seeds and checks a throwaway system-Python venv and pip, removes it, then performs the existing common checks. This factory smoke command is shipped source: it was syntax-checked here, not executed in an actual image.

Executed source controls:

- Focused13 resolver recipe cases pass: reviewed/exact3.7/3.8, blank default, static, uv, compatible explicit1.x, explicit incompatible-version boundary, modern3.9/3.13 and historical route. They verify selected upstream identity/ENV, private installer interpreter and exact tool version, retained Node/Yarn/pnpm/Bun/media, uv's unchanged interpreter, and decoded build/start command bytes. No source or lockfile is needed to expose the default pre-command bug.
- The new tests overlaid against the pre-fix resolver fail exactly the seven affected common legacy-default cases. The live resolver was never replaced. `negative-control.json` names them. This is a source regression control, not an actual pip/Poetry failure/pass.
- `go test -p 1 ./internal/build ./internal/registry ./cmd/native-resolve -count=1` passes with Docker integration opt-in unset. Common6, diff whitespace check and factory-command shell syntax pass.
- Two earlier scoped lint attempts report the same foreign integration-test complexity34 issue. The attempted Go overlay did not hide it from gocyclo. Root then extracted cohesive helpers in that owned integration test; the final pinned scoped lint on LIVE sources passes with no overlay. All attempts and their actual exit codes are retained in checks.json.

Source and limits:

[Render's Poetry contract](https://render.com/docs/poetry-version) automatically includes Poetry and documents default2.1.3 for new services, with compatible explicit version selection. [Poetry2.1.3 metadata](https://pypi.org/pypi/poetry/2.1.3/json) requires Python>=3.9. [Upstream installation](https://github.com/python-poetry/poetry/blob/2.1.3/docs/_index.md) supports an isolated tool venv separate from the project environment. Render's private bootstrap interpreter and actual default-Poetry outcome on old Python remain undocumented/unexecuted here.

No actual common factory build, system venv supply, old app/tool interpreter, Poetry project/dependency creation, generated image, architecture, pre-deploy/runtime or hosted acceptance ran in this worker turn. In particular this does not establish that Poetry2.1.3 can create every Python3.7/3.8 project environment. Explicit-compatible version semantics are preserved; tool/project compatibility remains qualification. All three production common/supplied/artifact catalogs remain empty. No native task is marked done.

Suggested ADR060 amendment for root's review: after the existing private-Poetry-venv description, state that the qualified common Bookworm route can bootstrap its known default2.1.3 using the separate system interpreter when selected app Python3.7/3.8 cannot install that tool; app interpreter selection is unchanged, explicit Poetry releases retain compatibility requirements, and actual old-Python tool/project behavior is qualification evidence rather than a claim from recipe controls. Do not replace the original28 criteria or describe all27 reviewed recipes as runtime-qualified.
