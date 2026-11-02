#!/bin/sh
# One approved canonical Node-compute OCI build. Outer V4 lifecycle owns bounds.
set -eu
phase() { printf '%s\n' "$1" > "$PILOT_EVIDENCE/phase"; }
phase alias-preflight
docker version --format '{{.Server.Os}}/{{.Server.Arch}}' > "$PILOT_EVIDENCE/socket-alias-preflight.txt"
grep -x linux/arm64 "$PILOT_EVIDENCE/socket-alias-preflight.txt" >/dev/null
docker buildx version > "$PILOT_EVIDENCE/linux-buildx-version.txt"
cat /proc/meminfo > "$PILOT_EVIDENCE/linux-meminfo-before.txt"
# The separate builder may allocate4GiB; runner512MiB plus128MiB operating margin.
awk '$1=="MemAvailable:" {found=1; if($2 < 4849664) exit 1} END {if(!found) exit 2}' /proc/meminfo
for name in memory.max memory.current; do
 if [ -r "/sys/fs/cgroup/$name" ]; then cat "/sys/fs/cgroup/$name" > "$PILOT_EVIDENCE/linux-$name-before.txt"; fi
done
phase pull
if docker pull --platform linux/arm64 --quiet "$PILOT_BUILDKIT" > "$PILOT_EVIDENCE/buildkit-pull.stdout" 2> "$PILOT_EVIDENCE/buildkit-pull.stderr"; then
 printf '0\n' > "$PILOT_EVIDENCE/buildkit-pull.exit"
else code=$?;printf '%s\n' "$code" > "$PILOT_EVIDENCE/buildkit-pull.exit";exit "$code";fi
phase bootstrap
docker buildx create --name "$PILOT_OWNER" --node "$PILOT_NODE" --driver docker-container \
 --platform linux/arm64 --driver-opt "image=$PILOT_BUILDKIT" --driver-opt memory=4g \
 --driver-opt cpu-period=100000 --driver-opt cpu-quota=200000 --driver-opt restart-policy=no \
 "$DOCKER_HOST" > "$PILOT_EVIDENCE/builder-create.stdout" 2> "$PILOT_EVIDENCE/builder-create.stderr"
docker buildx inspect --bootstrap "$PILOT_OWNER" > "$PILOT_EVIDENCE/builder-inspect.txt" 2>&1
docker inspect --format '{{.State.Running}} {{.HostConfig.Memory}} {{.HostConfig.CpuQuota}} {{.HostConfig.CpuPeriod}}' "$PILOT_BUILDER_CONTAINER" > "$PILOT_EVIDENCE/builder-running.txt"
grep -x 'true 4294967296 200000 100000' "$PILOT_EVIDENCE/builder-running.txt" >/dev/null
phase case
exec docker buildx build --builder "$PILOT_OWNER" --platform linux/arm64 --network none --progress plain \
 --label "co.bex.native-qualification=$PILOT_OWNER" \
 --secret "id=render-env,src=$BUILD_RENDER_ENV" --metadata-file "$PILOT_EVIDENCE/build-metadata.json" \
 --file "$BUILD_DOCKERFILE" --output "type=oci,dest=$PILOT_EVIDENCE/child.oci" "$BUILD_CONTEXT"
