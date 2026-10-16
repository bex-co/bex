# Native Python MP3 conversion lacks FFmpeg — sweep351

[Results](results.json) · [full native/wire/log/API/runtime/cleanup receipts](native-wire-API-runtime-cleanup.json) · [primary source reference](primary-public-source.json) · [generated-image execution](generated-image-observation.json) · [resolver events](resolver-observation-events.jsonl) · [final inventory](final-inventory.json) · [dedupe](dedupe.json).

Research source bd0ffac525f2ff1dfb4d7c7add9f81000fed8036; [fresh fetched main](fresh-main-comparison.json) is identical, with no FFmpeg supplier or scheduled matching item. Serving revision is unobserved. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682 use api.bex.co and the continuing isolated human identity in bex-canary. [Installed0.2.1 create help](installed-create-help.json) is retained; the customer binary is not overwritten.

## Actual native first deployments

Both clients create one fresh explicit Free Python3.12 web service, Frankfurt, autoDeploy off, public jiaaro/pydub/master, rootDir omitted/root, no PATH override or app-level installer. Each native POST is forwarded unchanged once by an owned loopback relay;201/native0 and exact workspace/name/Free/runtime/pre-inventory absence prove ownership. Copied configs/relays are removed. [Bex exact command](bex-initial-command.sh.txt) and [Render exact command](render-initial-command.sh.txt) retain complete authored programs.

- bex: srv-db8rq67dbg8c73dghc60; first dep-db8rq67dbg8c73dghc6g; WAV-only diagnostic dep-db8rrlndbg8c73dghc80.
- render: srv-db8rq9q9cl7s73ae4al0; first dep-db8rq9q9cl7s73ae4alg; WAV-only diagnostic dep-db8rrofdbg8c73dghc90.

An anonymous primary checkout proves master commit103e339d3bc7a9a58a9f1de65f7d8074659b5f5a and exact pydub/**init**.py39bytes and pydub/audio_segment.py49233bytes, with their hashes retained. The build writes a harmless stdlib probe, verifies both source hashes, creates an ordinary mono16-bit/8kHz/800-frame silent WAV, and uses Pydub to read it as100ms. Only then does it print nonce_SOURCE_AND_WAV_CONFIRMED and execute:

```python
from pydub import AudioSegment
audio = AudioSegment.from_wav('qa351-input.wav')
assert len(audio) == 100
audio.export('qa351-output.mp3', format='mp3')
```

Both first deployments end build_failed. Native build logs exit0 and show the standalone executed proof marker, FileNotFoundError naming ffmpeg, and build exit1. This is after successful imports/WAV decoding, not a Python-version/source/media-format error. Native create exit0 is expected async acceptance. Initial queue observations are retained and are not classified as failures; each queue progresses to the actual missing-program error. Interim HTTP404 while a diagnostic build is still running is a readiness observation, not another finding.

Pydub's own retained checkout documents pure-Python WAV handling and its external FFmpeg/Libav dependency for formats such as MP3 (README.markdown:182–186). [Render's common native tools contract](https://render.com/docs/native-runtimes#tools-and-utilities) supplies FFmpeg during build and runtime. No authenticated Render-hosting experiment or exact provider FFmpeg version is claimed.

## Narrow recovery and runtime evidence

Each native update removes only the single pydub MP3-conversion command. Full string equality after that removal, unchanged start/source/runtime/env, and exactly one new child are checked. No FFmpeg installation, PATH repair, changed Python, changed media input, Git write or platform deploy is made. Both diagnostic children reach Live/HTTP200 at the exact independent source commit.

Both cached build/start snapshots retain the own marker, exact source hashes, Python3.12 with working Python/pip/Poetry, and the identical1644-byte WAV/SHA256c726d333dd159a31423f3480dbb1c5c4a9dfcd30efe1f7e12ade390dc92e8908 with the expected silence/shape. Both FFmpeg and ffprobe report ENOENT in both phases. Six API-only REST/GraphQL/MCP get-service comparisons pass for the exact source/master/Python/Free/autoDeploy-off/build/start configuration; MCP explicitly names this workspace. WAV-only recovery bypasses the failed requirement and is not successful MP3 processing or acceptance of a fix.

## Source experiment

The [private overlay observation](resolver-observation.go.txt) executes unchanged nativeDockerfile plus ResolveNativeBuild for Python root/nested, Node root and Ruby root. Four subcases plus parent pass as observations with zero skips/failures. Choices/generated Dockerfiles are retained. These are not passing FFmpeg regression tests and do not prove Node/Ruby images were executed. The fixed test revision makes no cache-defect claim. [Inputs](resolver-inputs.json) retain the diagnostic build and original MP3 runtime command.

Only the Python-root generated image is actually built and executed. It contains the exact public source/probe, successfully performs the WAV-only build, then its default CMD attempts the original Pydub conversion and exits1 naming missing ffmpeg. An independent network-disabled runtime snapshot confirms Python3.12.15/aarch64, both absent executable paths, exact source hashes and the WAV hash. Hosted architecture is separately recorded in its probes. [Full build log](generated-image-build.log) and [bounded driver](generated-image-driver.py.txt) are retained. Only the exact owned image tag is removed and absence verified; shared caches/bases remain. No repository Go file or production code is changed.

## Cleanup and limits

Both native deletes exit0. Both service details/former URLs and four recorded children return404; owned services are absent from list. Original6 services and1 Blueprint remain, services used6/terminating0, PostgreSQL/KeyValue0, groups/projects empty. No paid service or cron is created. Isolated auth remains for the authorized continuing loop; temporary per-call configs/request files are removed. Actual credentials and baseline configuration are masked/projected before filing.

Live scope is two Free Python3.12 web services and their WAV-only controls. A supplied-FFmpeg MP3 success, ffprobe media decoding, other Python lines/native runtimes, static/private/worker/cron, pre-deploy phase, browser UI, every architecture and supplier/cache changes are implementation acceptance work. There is no inference that every other utility or native base is broken. Helpers: [finite native journey](finite-native-journey.py.txt), [probe](harmless-runtime-probe.py.txt), [capture/client isolation](isolated-native-capture-helpers.py.txt), [shared reads](shared-API-readbacks.py.txt). Uncommitted.
