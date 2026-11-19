/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package build

import (
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

// $XDG_CACHE_HOME across native builds (w9/m493, docs/ADR060 D3).
//
// Render gives every native build XDG_CACHE_HOME=/opt/render/.cache, keeps
// what a build leaves there for the service's next build, empties it on
// "Clear build cache & deploy", stops keeping it once it passes 7 GB, and
// never ships it with the running service. A native image is one stage, so
// the build command's RUN mounts the directory as a BuildKit cache mount
// (nativeXDGCacheMount): nothing written there reaches a layer.
//
// A cache mount lives as long as its daemon, which is the build pod's own. So
// with BEX_BUILD_CACHE=s3 the buildkit phase runs two more solves against that
// daemon: before the image, one that seeds the mount from the archive
// cache-restore fetched; after it, one that archives the mount for cache-save
// to store. They are solves of their own rather than steps of the image's,
// because any input of the build command's step is part of its cache key: an
// archive that changes every build would cost every build its "Reusing the
// cached result" (w4/582). A cache mount is no part of that key.
//
// The archive is made and unpacked only inside BuildKit's sandbox, on the
// image the build itself runs on. The credentialed phases move one opaque file
// and parse nothing in it, and whatever a hostile archive could do while it is
// unpacked lands in a sandbox whose only lasting effect is the cache mount,
// which the tenant's own build command writes anyway.
const (
	// xdgCacheObject is the archive's object, beside the layer cache in the
	// App's BuildCacheRoot, so App deletion removes it with the rest.
	xdgCacheObject = "xdg-cache.tar.gz"
	// xdgInMount holds the archive cache-restore fetched, and xdgOutMount the
	// one BuildKit made for cache-save: separate volumes for the reason
	// cacheInMount and cacheOutMount are.
	xdgInMount   = "/xdg-in"
	xdgOutMount  = "/xdg-out"
	xdgInVolume  = "xdg-in"
	xdgOutVolume = "xdg-out"
	// xdgDir is where the buildkit phase writes its buildctl wrapper and the
	// seed and export solves' Dockerfile.
	xdgDir = "/tmp/bex-xdg"
	// xdgCacheKind labels the archive's outcomes, beside the layer cache's.
	xdgCacheKind = "xdg"
	// xdgBuildkitdFlags keeps the daemon's garbage collector off: between the
	// image's solve and the export, the cache mount is in use by no solve,
	// which is exactly what GC prunes first. The daemon dies with the pod.
	xdgBuildkitdFlags = "--oci-worker-gc=false"
)

// xdgCacheMaxBytes is the largest $XDG_CACHE_HOME kept for the next build:
// Render's 7 GB build cache bound. It bounds the directory, the uncompressed
// archive stream and the stored object alike, so a stored archive always fits
// the 8G volumes that carry it.
const xdgCacheMaxBytes int64 = 7_000_000_000

// xdgCache reports whether this build keeps $XDG_CACHE_HOME between builds:
// a native build whose layer cache is in the S3 store.
func (o Options) xdgCache() bool {
	return o.Builder == BuilderNative && o.cacheBackend() == cacheS3
}

// xdgCacheURI is the s3:// URI of this App lifetime's archive.
func (o Options) xdgCacheURI() string {
	return o.CacheStore.uri(o.buildCacheRoot() + xdgCacheObject)
}

// withXDGCache gives a native build's cache phase the archive's URI and bound
// and its volume, and leaves any other build's unchanged.
func withXDGCache(c corev1.Container, o Options, mounts ...corev1.VolumeMount) corev1.Container {
	if !o.xdgCache() {
		return c
	}
	c.Env = append(c.Env,
		corev1.EnvVar{Name: "BEX_XDG_URI", Value: o.xdgCacheURI()},
		corev1.EnvVar{Name: "BEX_XDG_MAX_BYTES", Value: strconv.FormatInt(xdgCacheMaxBytes, 10)},
	)
	c.VolumeMounts = append(c.VolumeMounts, mounts...)
	return c
}

// s3CacheScript is one S3 cache phase's script: its layer-cache body, and on a
// native build the archive's transfer started in the background beside it.
// The body runs in a subshell, which confines its early exits to its own half,
// and the script waits for both. Both halves share the phase's one deadline.
func s3CacheScript(o Options, layer, xdg string) string {
	if !o.xdgCache() {
		return s3CachePrelude + layer
	}
	return s3CachePrelude + xdg + "(\n" + layer + ")\nwait\n"
}

// xdgCacheOutcome reports the archive's outcome as its own line of the phase's
// termination message: "xdg <outcome> <bytes> <seconds>", the seconds its own,
// since the layer cache's line takes the phase's (cachePhaseSignals).
const xdgCacheOutcome = `bex_xdg_t0=$(date +%s)
bex_xdg() { bex_outcome ` + xdgCacheKind + ` "$1" "${2:-0}" "$(( $(date +%s) - bex_xdg_t0 ))"; }
`

// xdgCacheRestore fetches the archive into /xdg-in, in the background. A
// listing comes first: it tells a first build (miss) from an unreachable
// store (error), and an archive over the bound is never downloaded. The
// transfer's size and seconds go beside it for the build log's Render line.
// aws-cli exits 1 for a listing that matches nothing.
const xdgCacheRestore = xdgCacheOutcome + `bex_s3_ls() {
  bex_left=$(( bex_deadline - $(date +%s) ))
  [ "$bex_left" -gt 0 ] || return 124
  timeout "$bex_left" aws --endpoint-url "$BEX_CACHE_ENDPOINT" s3 ls "$1" 2>/dev/null
}
(
bex_listing=$(bex_s3_ls "$BEX_XDG_URI")
bex_rc=$?
bex_bytes=$(printf '%s\n' "$bex_listing" | while read -r bex_d bex_t bex_n bex_name; do
  [ "$bex_name" = ` + xdgCacheObject + ` ] && echo "$bex_n"
done | head -n 1)
case $bex_bytes in ''|*[!0-9]*) bex_bytes= ;; esac
if [ "$bex_rc" -gt 1 ]; then
  bex_xdg ` + CacheError + `
elif [ -z "$bex_bytes" ]; then
  bex_xdg ` + CacheMiss + `
elif [ "$bex_bytes" -gt "$BEX_XDG_MAX_BYTES" ]; then
  bex_xdg ` + CacheSkipped + ` "$bex_bytes"
else
  echo "==> Downloading cache..." >&2
  if bex_s3 cp "$BEX_XDG_URI" ` + xdgInMount + `/` + xdgCacheObject + ` --no-progress &&
    [ "$(wc -c < ` + xdgInMount + `/` + xdgCacheObject + ` | tr -d ' ')" = "$bex_bytes" ]; then
    printf '%s %s\n' "$bex_bytes" "$(( $(date +%s) - bex_xdg_t0 ))" > ` + xdgInMount + `/transfer
    bex_xdg ` + CacheHit + ` "$bex_bytes"
  else
    rm -f ` + xdgInMount + `/` + xdgCacheObject + `
    bex_xdg ` + CacheError + `
  fi
fi
) &
`

// xdgCachePurge deletes the archive before a clear-cache rebuild, which
// neither fetches nor seeds it, in the background. A missing object deletes
// cleanly.
const xdgCachePurge = xdgCacheOutcome + `(
if bex_s3 rm "$BEX_XDG_URI"; then
  bex_xdg ` + CachePurged + `
else
  bex_xdg ` + CacheError + `
fi
) &
`

// xdgCacheSave stores the archive BuildKit made, in the background. Like the
// layout, /xdg-out was written by the container that runs tenant code: only a
// regular file is stored, and only within the bound. A "skipped" marker means
// the build left $XDG_CACHE_HOME empty or over the bound, so the next build
// starts without one, as on Render. Neither file means the export did not run
// to the end: the stored archive stays.
const xdgCacheSave = xdgCacheOutcome + `(
bex_archive=` + xdgOutMount + `/` + xdgCacheObject + `
if [ -f "$bex_archive" ] && [ ! -L "$bex_archive" ]; then
  bex_bytes=$(wc -c < "$bex_archive" | tr -d ' ')
  if [ "$bex_bytes" -gt "$BEX_XDG_MAX_BYTES" ]; then
    bex_s3 rm "$BEX_XDG_URI"
    bex_xdg ` + CacheSkipped + ` "$bex_bytes"
  elif bex_s3 cp "$bex_archive" "$BEX_XDG_URI" --no-progress; then
    bex_xdg ` + CacheSaved + ` "$bex_bytes"
  else
    bex_xdg ` + CacheError + `
  fi
elif [ -f ` + xdgOutMount + `/skipped ] && [ ! -L ` + xdgOutMount + `/skipped ]; then
  bex_bytes=$(head -c 20 ` + xdgOutMount + `/skipped | tr -cd 0-9)
  if bex_s3 rm "$BEX_XDG_URI"; then
    bex_xdg ` + CacheSkipped + ` "${bex_bytes:-0}"
  else
    bex_xdg ` + CacheError + `
  fi
else
  bex_xdg ` + CacheSkipped + `
fi
) &
`

// xdgMaxBytes is the bound as the scripts inside BuildKit spell it.
var xdgMaxBytes = strconv.FormatInt(xdgCacheMaxBytes, 10)

// xdgDockerfile holds the two solves around the image's. Both run on the
// image the build's own final stage runs on (BEX_XDG_BASE), which that solve
// pulls anyway, and mount the build command's cache mount by its id. Neither
// step can be served from a cache: the daemon is the pod's own, and neither
// solve imports one.
//
// The seed unpacks no more than the bound and keeps nothing of an archive
// that does not unpack cleanly, gzip's own check included. The save archives
// the directory only within the bound, removing each file as it is archived so
// the cache never sits on BuildKit's disk twice. It names the directory's
// entries rather than the directory, which is the cache mount's mountpoint and
// so cannot be removed. An empty directory or one
// over the bound leaves the "skipped" marker instead. Its export holds just
// that one file.
var xdgDockerfile = `ARG BEX_XDG_BASE
FROM $BEX_XDG_BASE AS bex-xdg-seed
RUN --mount=type=bind,target=/run/bex-xdg-in --mount=type=cache,id=` + nativeXDGCacheID + `,target=` + nativeXDGCacheHome + `,sharing=locked <<'BEX_XDG_SEED'
bex_t0=$(date +%s)
if { { gzip -dc /run/bex-xdg-in/` + xdgCacheObject + `; echo $? > /tmp/bex-gzip; } | head -c ` + strconv.FormatInt(xdgCacheMaxBytes+1, 10) + ` | tar -xf - -C ` + nativeXDGCacheHome + ` --no-same-owner; } 2>/dev/null &&
  [ "$(cat /tmp/bex-gzip)" = 0 ]; then
  echo "bex-xdg-seed: restored in $(( $(date +%s) - bex_t0 ))s"
else
  find ` + nativeXDGCacheHome + ` -mindepth 1 -delete
  echo "bex-xdg-seed: unusable"
fi
BEX_XDG_SEED

FROM $BEX_XDG_BASE AS bex-xdg-save
RUN --mount=type=cache,id=` + nativeXDGCacheID + `,target=` + nativeXDGCacheHome + `,sharing=locked <<'BEX_XDG_SAVE'
mkdir /bex-xdg-out && cd /bex-xdg-out || exit 1
if [ -z "$(ls -A ` + nativeXDGCacheHome + `)" ]; then
  echo 0 > skipped
  exit 0
fi
bex_bytes=$(du -sb ` + nativeXDGCacheHome + ` | cut -f1)
if [ "$bex_bytes" -gt ` + xdgMaxBytes + ` ]; then
  echo "$bex_bytes" > skipped
  exit 0
fi
{ find ` + nativeXDGCacheHome + ` -mindepth 1 -maxdepth 1 -printf '%P\0' |
  tar --sparse --numeric-owner --totals --remove-files -C ` + nativeXDGCacheHome + ` --null -T - -cf - 2> /tmp/bex-tar.log; echo $? > /tmp/bex-tar; } |
  gzip -1 > ` + xdgCacheObject + ` || exit 1
bex_stream=$(sed -n 's/^Total bytes written: \([0-9]*\).*/\1/p' /tmp/bex-tar.log)
[ "$(cat /tmp/bex-tar)" = 0 ] && [ -n "$bex_stream" ] || exit 1
if [ "$bex_stream" -gt ` + xdgMaxBytes + ` ] || [ "$(wc -c < ` + xdgCacheObject + `)" -gt ` + xdgMaxBytes + ` ]; then
  rm ` + xdgCacheObject + `
  echo "$bex_stream" > skipped
fi
BEX_XDG_SAVE

FROM scratch AS bex-xdg-export
COPY --from=bex-xdg-save /bex-xdg-out/ /
`

// xdgBuildctl is the buildctl buildctl-daemonless.sh runs for a native build
// that keeps $XDG_CACHE_HOME ($BUILDCTL, which the script reads for exactly
// this): `$BUILDCTL --addr=<socket> <command> ...`. Anything but a build is
// buildctl's. A build seeds the cache mount when cache-restore fetched an
// archive, runs the image's solve exactly as asked, and then archives the
// mount into /xdg-out. Seed and export are best-effort, quiet and bounded:
// their output never reaches the build log, which shows Render's restore line
// instead, and only the image's solve decides the phase's exit status.
//
// The final stage's base is the last FROM naming an image alone: the
// resolver may put a stage ahead of it, and every such stage is named. Once
// seeded, the fetched archive and BuildKit's copy of it are deleted, so the
// build has the disk they held.
var xdgBuildctl = `bex_addr=$1
shift
[ "${1:-}" = build ] || exec buildctl "$bex_addr" "$@"
bex_base=$(sed -n 's/^FROM \([^ ]*\)$/\1/p' /native/Dockerfile | tail -n 1)
bex_xdg() {
  bex_target=$1
  shift
  timeout ` + strconv.Itoa(cacheBudgetSeconds) + ` buildctl "$bex_addr" build --progress=plain --frontend dockerfile.v0 \
    --local dockerfile=` + xdgDir + ` --opt target="$bex_target" --opt build-arg:BEX_XDG_BASE="$bex_base" "$@" 2>&1
}
bex_number() { case $1 in ''|*[!0-9]*) return 1 ;; esac; }
bex_size() {
  awk -v b="$1" 'BEGIN { split("B KB MB GB TB", u, " "); i = 1
    while (b >= 1000 && i < 5) { b /= 1000; i++ }
    if (i == 1) printf "%dB", b; else if (b < 10) printf "%.1f%s", b, u[i]; else printf "%d%s", b, u[i] }'
}
if [ -n "$bex_base" ] && [ -f ` + xdgInMount + `/` + xdgCacheObject + ` ]; then
  bex_took=$(bex_xdg bex-xdg-seed --local context=` + xdgInMount + ` | sed -n 's/.* bex-xdg-seed: restored in \([0-9][0-9]*\)s$/\1/p' | tail -n 1)
  bex_bytes= bex_secs=
  [ -f ` + xdgInMount + `/transfer ] && read -r bex_bytes bex_secs < ` + xdgInMount + `/transfer
  rm -f ` + xdgInMount + `/` + xdgCacheObject + `
  buildctl "$bex_addr" prune --filter type==source.local >/dev/null 2>&1 || :
  if bex_number "$bex_took" && bex_number "$bex_bytes" && bex_number "$bex_secs"; then
    echo "==> Transferred $(bex_size "$bex_bytes") in ${bex_secs}s. Extraction took ${bex_took}s."
  else
    echo "bex: no usable build cache; building clean"
  fi
fi
buildctl "$bex_addr" "$@" || exit
if [ -n "$bex_base" ]; then
  bex_xdg bex-xdg-export --local context=` + xdgDir + ` --output type=local,dest=` + xdgOutMount + ` >/dev/null ||
    rm -rf ` + xdgOutMount + `/* ` + xdgOutMount + `/.[!.]*
fi
`

// xdgBuildkitSetup writes the wrapper and its Dockerfile where $BUILDCTL
// finds them, before the phase runs BuildKit. Both are fixed platform text.
var xdgBuildkitSetup = "mkdir -p " + xdgDir + "\n" +
	"cat > " + xdgDir + "/Dockerfile <<'BEX_XDG_DOCKERFILE'\n" + xdgDockerfile + "BEX_XDG_DOCKERFILE\n" +
	"cat > " + xdgDir + "/buildctl <<'BEX_XDG_BUILDCTL'\n" + xdgBuildctl + "BEX_XDG_BUILDCTL\n"

// withXDGBuildkit gives a native build's buildkit phase what the seed and
// export solves need, and returns volumes with the two archive volumes added:
// buildctl-daemonless.sh pointed at the wrapper, xdgBuildkitdFlags among the
// daemon's flags, and both volumes writable, since the seed deletes the
// fetched archive once it is unpacked and the export writes the new one. Any
// other build's phase and volumes are left as they are.
func withXDGBuildkit(o Options, buildkit *corev1.Container, volumes []corev1.Volume) []corev1.Volume {
	if !o.xdgCache() {
		return volumes
	}
	flags := xdgBuildkitdFlags
	buildkit.Env = slices.DeleteFunc(buildkit.Env, func(e corev1.EnvVar) bool {
		if e.Name != "BUILDKITD_FLAGS" {
			return false
		}
		flags = e.Value + " " + flags
		return true
	})
	buildkit.Env = append([]corev1.EnvVar{
		{Name: "BUILDCTL", Value: "sh " + xdgDir + "/buildctl"},
		{Name: "BUILDKITD_FLAGS", Value: flags},
	}, buildkit.Env...)
	volumes = append(volumes, boundedEmptyDirVolume(xdgOutVolume, cacheEmptyDirSizeLimit))
	if !o.SkipCacheImport {
		volumes = append(volumes, boundedEmptyDirVolume(xdgInVolume, cacheEmptyDirSizeLimit))
		buildkit.VolumeMounts = append(buildkit.VolumeMounts, corev1.VolumeMount{Name: xdgInVolume, MountPath: xdgInMount})
	}
	buildkit.VolumeMounts = append(buildkit.VolumeMounts, corev1.VolumeMount{Name: xdgOutVolume, MountPath: xdgOutMount})
	return volumes
}
