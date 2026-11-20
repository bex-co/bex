# Grafana Alloy v1.11.3 with two carried patches, w4/m199 and w8/m525.
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
# Upstream stage.limit refills its token buckets by the wall clock when the
# file tailer hands the pipeline a batch, so during a flood history keeps a
# different set of lines than bex-api's live tail, which counts each line at
# its own timestamp: on production 37.7% of a tail's lines were missing from
# history although the counts matched (w8/341). patches/limit-entry-timestamp.patch
# adds an opt-in `refill_by_entry_timestamp` that refills each bucket by the
# entry's own (CRI write) timestamp, in whole tokens on a grid fixed to the
# Unix epoch, the rule lego/backend/internal/logs/linecap.go runs. Its test
# reads the table linecap_test.go reads, copied in from the `linecap` build
# context; unset, the stage is upstream's (sandbox_pods keeps it so).
#
# Mirrors upstream's Dockerfile (same build image, UI build, GO_TAGS and
# release flags) and swaps only /bin/alloy inside the official v1.11.3 image,
# so the runtime filesystem, user and entrypoint are unchanged.
#
# Build (CI: log-shipper-image.yml), from the repository root:
#   docker buildx build --platform linux/amd64,linux/arm64 \
#     --build-context linecap=lego/backend/internal/logs/testdata \
#     -t ghcr.io/bex-co/bex-alloy:<tag> deploy/log-shipper

# Replaced by the `linecap` build context above; left empty, the COPY of the
# shared table below fails the build.
FROM scratch AS linecap

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.21@sha256:36b1e621f32f46aa1de58843b885f0885faabe7b29468462d36a60bea1245e33 AS source
# Pin the exact upstream commit the patches apply to (tag v1.11.3; the CRI
# patch was authored against v1.11.2 and applies unchanged); bump them together.
ARG ALLOY_COMMIT=091a20eb3eb50a968f129d6094455962e9dd328e
WORKDIR /src
RUN git init -q alloy \
    && cd alloy \
    && git remote add origin https://github.com/grafana/alloy \
    && git fetch -q --depth 1 origin "${ALLOY_COMMIT}" \
    && git checkout -q FETCH_HEAD
COPY patches/cri-first-chunk-timestamp.patch patches/limit-entry-timestamp.patch /tmp/
RUN cd alloy && git apply --stat --apply /tmp/cri-first-chunk-timestamp.patch \
    && git apply --stat --apply /tmp/limit-entry-timestamp.patch
COPY --from=linecap linecap-shared.json /src/alloy/internal/component/loki/process/stages/testdata/

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.21@sha256:36b1e621f32f46aa1de58843b885f0885faabe7b29468462d36a60bea1245e33 AS ui-build
COPY --from=source /src/alloy/internal/web/ui /ui
WORKDIR /ui
RUN yarn --frozen-lockfile --network-timeout=1200000 && yarn run build

FROM --platform=$BUILDPLATFORM grafana/alloy-build-image:v0.1.21@sha256:36b1e621f32f46aa1de58843b885f0885faabe7b29468462d36a60bea1245e33 AS build
COPY --from=source /src/alloy /src/alloy
COPY --from=ui-build /ui/build /src/alloy/internal/web/ui/build
WORKDIR /src/alloy
# The carried behavior must hold before a binary is produced, and upstream's
# own limit tests with the new argument unset.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go test ./internal/component/loki/process/stages/ -run 'TestCRI|TestNewCri|TestLimit' -count=1
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    RELEASE_BUILD=1 VERSION=v1.11.3-bex-cri-first-ts-limit-entry-ts \
    GO_TAGS="netgo builtinassets promtail_journal_enabled" \
    make alloy

FROM grafana/alloy:v1.11.3@sha256:8c7256f412feb9f5f48f9f6f9394dc97ca887f63dea9304f347970ecc1787669
COPY --from=build --chown=473:473 /src/alloy/build/alloy /bin/alloy
