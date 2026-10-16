# w8 · m82 — Supply FFmpeg in native Python builds and runtime

**Worker:** worker8 **Goal:** ordinary native Python media builds and started services can use the supplied FFmpeg package. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Supply reviewed FFmpeg and runtime libraries | 55m | — |
| [t002](t002.md) | Audit shared native image consumers | 30m | t001 |
| [t003](t003.md) | Render parity | 20m | t002 |
| [t004](t004.md) | Simplify | 10m | t003 |
| [t005](t005.md) | Test coverage | 40m | t003 |
| [t006](t006.md) | Closeout | 20m | t004, t005 |

Six tasks,175m total.

## Definition of done

- Both native clients create fresh permitted Free Python media fixtures whose first original MP3-requiring deployment reaches Live/HTTP200 without bypassing conversion, installing FFmpeg in app code or repairing PATH.
- Real build/runtime media conversion and ffprobe checks prove exact source/input/output behavior. A reviewed supplier and required libraries work on supported architectures; cache/version inputs and supported shared consumers are audited.
- Maintained fail-before/pass-after execution tests preserve Python/Poetry/JS/Bun, env/PATH and source-root behavior. Public-surface settings agree and required checks pass with limits explicit.
- Owned parents/children and metadata are absent, original baselines remain and quota termination is0. Completed tasks/milestone move to done through /pm with all statuses synchronized and no stubs. Current WAV-only diagnostics do not meet the original media requirement.

## Source + Goal linkage

- **Source:** [functional CLI sweep351](evidence/cli-sweep351/README.md), [shared assembly diagnosis](finding.md).
- **Goal linkage:** ADR008 usable hosting, ADR018 native-runtime compatibility and ADR060 reviewed toolchain assembly.
- **Expected outcome:** a valid Python media application builds and processes MP3 without bootstrapping a promised native executable itself.
- **Why now:** both first builds pass exact source/WAV checks then fail missing ffmpeg; two Live WAV-only controls and one actual generated image confirm absent FFmpeg/ffprobe with healthy primary tooling. No newer source fix or duplicate is found.
- **Render parity included:** FFmpeg is part of the common native tools contract; this is an existing native build/runtime behavior, with no new public API or bespoke client.
