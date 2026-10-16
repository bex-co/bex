# Native Python media conversion cannot find FFmpeg

**Major · operator native toolchain assembly.** Two fresh unchanged native-client requests are accepted, but their first Python3.12 builds fail an ordinary Pydub WAV→MP3 conversion with FileNotFoundError naming ffmpeg and exit1. Each build already proves exact public source, imports Pydub and reads the valid WAV. Two later WAV-only controls are Live/HTTP200 with healthy Python/pip/Poetry and FFmpeg/ffprobe ENOENT during build and runtime. [Full reproduction and evidence](evidence/cli-sweep351/README.md).

The expectation is Render's documented common native-runtime FFmpeg availability, not a claim that Pydub installs a system dependency. Its primary README correctly requires FFmpeg/Libav for MP3 and permits WAV without it. No authenticated Render comparison or exact vendor tool version was measured. No unrelated utility or codec failure is claimed.

## Failing boundary and concrete correction

The pinned service-create builder sends the original Python/Free/master/source/env/commands unchanged: both actual captured POSTs return201/native0. API-only REST/GraphQL/MCP reads preserve all six source/config comparisons. The missing executable arises later in the generated native image; changing launcher flags or wrapping each app would not supply it.

`lego/operator/internal/build/native.go:95–104` selects the official reviewed Python image. `nativeDockerfile:243–283` copies source, runs the authored build and retains the exact start command on that image. `nativeresolve.go:50–99` selects the primary toolchain and adds Poetry, JavaScript and opted-in Bun suppliers; none supplies FFmpeg or its dynamic libraries. The current actual generated Python image agrees with both hosted failures: WAV-only build0, original MP3 runtime command1/missing ffmpeg, both executable paths absent.

Supply a reviewed FFmpeg/ffprobe package with its required runtime libraries through the shared native image assembly before tenant source/build, retained for build, pre-deploy and start. Audit the reviewed Bookworm bases and decide explicitly which share that supplier; Python is the demonstrated failure, while other base execution remains unverified. Reuse a digest-reviewed base/tool layer or reproducible package provenance consistent with ADR060, rather than an unpinned user installer. Copying only a dynamic binary is insufficient without its library closure. Keep supplier/package updates auditable in freshness inventory/narration and cache inputs. Verify ordinary MP3 encoding/decoding with the actual supplied codecs and ffprobe, not just which/version output.

Preserve primary Python/Poetry isolation, Node/npm/Yarn/pnpm and Bun companions, exact literal env/PATH, selected roots, command quoting and same-input cache stability. Do not mutate user PATH, tell users to install a tool already promised by native hosting, or turn this into a broad unrelated utility bundle. Native-image size/provenance and supported-architecture execution are part of reviewing this specific supplier.

## Shared paths and blast radius

Production census: `build.go:1126` calls nativeBuildPreparer once; `native.go:330` supplies nativeDockerfile once; `cmd/native-resolve/main.go:46` is ResolveNativeBuild's sole production caller. Runtime/root inputs are already propagated through `nativeresolve.go:234–238`. Web/private/worker/cron native Python and any supported explicitly Python static build converge on this mechanism; only Free web is live exercised. Static default runtime remains Node. Other native runtimes converge on the same generator but their actual FFmpeg availability must be audited, not assumed from Python. Dockerfile/image sources and managed datastores are outside this path.

REST/GraphQL/MCP/dashboard and Blueprint inputs carry the same App runtime/build/start settings. There is no new API field, client fork or dashboard workaround. Six API-only comparisons pass; browser and other resource-kind runtime acceptance remain untested.

## Dedupe and acceptance

All open/blocked/done board searches for ffmpeg/ffprobe, source/history, DO_NOT_DO and freshly fetched main find no existing supplier or scheduled repair. Research/fetched source is bd0ffac525f2ff1dfb4d7c7add9f81000fed8036; serving SHA remains unobserved, with no newer source fix to call deploy lag. Completed [m72](../done/m72/README.md) and [m77](../done/m77/README.md) supplied Poetry/JavaScript within their accepted scopes. [m81](../m81/README.md) concerns uv.lock, [w4/321](../../w4/done/321.md) Go JavaScript and [w4/320](../../w4/320.md) Node26 Yarn; none certifies FFmpeg. Preserve those completion boundaries.

Six tasks total175m. Acceptance requires fresh original MP3-requiring first builds through both native clients, real build/runtime MP3 processing and ffprobe, exact source/WAV/output markers and coherent shared settings. Add maintained fail-before/pass-after real-image tests, review the shared supplier and meaningful neighboring paths, then clean all own fixtures. Current WAV-only controls bypass the failed conversion; nothing is implemented or completed by this filing.
