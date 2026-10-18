# Grafana Alloy v1.11.3 with one carried patch, w4/m199.
#
# v1.11.3, not the chart-default v1.11.2 (w4/m174): v1.11.2's loki.source.file
# re-applies its readers on every target change through a temporary drain
# goroutine that it then cancels, and an entry that goroutine had taken from a
# reader but not yet forwarded is dropped. Every short-lived pod churns the
# targets: scripts/log-shipper-kind-replay.py lost 4 of 800 lines from fast
# cron runs, a different line each time, each one read from its file
# (loki_source_file_read_lines_total counts it) and never sent
# (loki_write_sent_entries_total doesn't). v1.11.3 forwards from one
# dedicated consumer and applies tasks separately, so nothing is cancelled
# mid-handoff; it lost 0 of 800. Upstream lists the change only as "Schedule
# new path targets faster in loki.source.file".
#
# Upstream stage.cri stamps a record reassembled from CRI partial ("P") chunks
# with its LAST chunk's time; the kubelet's pod-log API (bex's live tail) uses
# the FIRST chunk's. History and live then disagree by the write delay and the
# dashboard renders one record twice. patches/cri-first-chunk-timestamp.patch
# keeps the first chunk's time on both merge branches (repeated P and the
# completing F) and adds a regression test; message assembly, buffer keys and
# the partial count/size bounds are untouched. Still unfixed upstream as of
# v1.19.2 — drop the patch once a release carries the fix and the
# scripts/test_log_shipper.py replay passes on it.
#
# Mirrors upstream's Dockerfile (same build image, UI build, GO_TAGS and
# release flags) and swaps only /bin/alloy inside the official v1.11.3 image,
# so the runtime filesystem, user and entrypoint are unchanged.
#
# Build (CI: log-shipper-image.yml):
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     -t ghcr.io/bex-co/bex-alloy:<tag> deploy/log-shipper

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.21@sha256:36b1e621f32f46aa1de58843b885f0885faabe7b29468462d36a60bea1245e33 AS source
# Pin the exact upstream commit the patch applies to (tag v1.11.3; authored
# against v1.11.2, applies unchanged); bump the two together.
ARG ALLOY_COMMIT=091a20eb3eb50a968f129d6094455962e9dd328e
WORKDIR /src
RUN git init -q alloy \
    && cd alloy \
    && git remote add origin https://github.com/grafana/alloy \
    && git fetch -q --depth 1 origin "${ALLOY_COMMIT}" \
    && git checkout -q FETCH_HEAD
COPY patches/cri-first-chunk-timestamp.patch /tmp/
RUN cd alloy && git apply --stat --apply /tmp/cri-first-chunk-timestamp.patch

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.21@sha256:36b1e621f32f46aa1de58843b885f0885faabe7b29468462d36a60bea1245e33 AS ui-build
COPY --from=source /src/alloy/internal/web/ui /ui
WORKDIR /ui
RUN yarn --frozen-lockfile --network-timeout=1200000 && yarn run build

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.21@sha256:36b1e621f32f46aa1de58843b885f0885faabe7b29468462d36a60bea1245e33 AS build
COPY --from=source /src/alloy /src/alloy
COPY --from=ui-build /ui/build /src/alloy/internal/web/ui/build
WORKDIR /src/alloy
# The carried behavior must hold before a binary is produced.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go test ./internal/component/loki/process/stages/ -run 'TestCRI|TestNewCri' -count=1
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    RELEASE_BUILD=1 VERSION=v1.11.3-bex-cri-first-ts \
    GO_TAGS="netgo builtinassets promtail_journal_enabled" \
    make alloy

FROM grafana/alloy:v1.11.3@sha256:8c7256f412feb9f5f48f9f6f9394dc97ca887f63dea9304f347970ecc1787669
COPY --from=build --chown=473:473 /src/alloy/build/alloy /bin/alloy
