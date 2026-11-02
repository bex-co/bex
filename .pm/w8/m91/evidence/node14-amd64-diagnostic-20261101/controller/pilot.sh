#!/bin/sh
# One existing donor case only. The outer controller owns every timeout/cleanup.
set -eu
phase() { printf '%s\n' "$1" > "$PILOT_EVIDENCE/phase"; }
phase alias-preflight
# VM /var/run/docker.sock is mounted at this shared endpoint path. This is a
# real read-only API preflight, before pulling or creating a builder.
docker version --format '{{.Server.Os}}/{{.Server.Arch}}' > "$PILOT_EVIDENCE/socket-alias-preflight.txt"
grep -x 'linux/arm64' "$PILOT_EVIDENCE/socket-alias-preflight.txt" > /dev/null
phase pull
if docker pull --platform linux/arm64 --quiet "$PILOT_BUILDKIT" > "$PILOT_EVIDENCE/buildkit-pull.stdout" 2> "$PILOT_EVIDENCE/buildkit-pull.stderr"; then
  printf '0\n' > "$PILOT_EVIDENCE/buildkit-pull.exit"
else code=$?; printf '%s\n' "$code" > "$PILOT_EVIDENCE/buildkit-pull.exit"; exit "$code"; fi
phase bootstrap
docker buildx create --name "$BEX_NATIVE_TEST_OWNER" --node "$PILOT_NODE" --driver docker-container \
  --platform linux/amd64 --driver-opt "image=$PILOT_BUILDKIT" --driver-opt memory=2g \
  --driver-opt cpu-period=100000 --driver-opt cpu-quota=200000 --driver-opt restart-policy=no \
  "$DOCKER_HOST" > "$PILOT_EVIDENCE/builder-create.stdout" 2> "$PILOT_EVIDENCE/builder-create.stderr"
# v0.14 inspect is plaintext and can print a node error with exit0. Check the
# actual pinned builder container/worker before starting the unchanged test.
docker buildx inspect --bootstrap "$BEX_NATIVE_TEST_OWNER" > "$PILOT_EVIDENCE/builder-inspect.txt" 2>&1
docker inspect --format '{{.State.Running}}' "$PILOT_BUILDER_CONTAINER" > "$PILOT_EVIDENCE/builder-running.txt"
grep -x true "$PILOT_EVIDENCE/builder-running.txt" > /dev/null
cat /proc/meminfo > "$PILOT_EVIDENCE/linux-meminfo-before.txt"
for name in memory.max memory.current; do
  if [ -r "/sys/fs/cgroup/$name" ]; then cat "/sys/fs/cgroup/$name" > "$PILOT_EVIDENCE/linux-$name-before.txt"; fi
done
phase case
exec /bex-build.test '-test.run=^TestCommonNodeGeneratedImages$/^node\-14$' -test.timeout=90m -test.count=1 -test.v
