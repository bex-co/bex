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

	"github.com/bex-co/bex/lego/types/renderlog"
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
// The same archive carries the runtime's dependency directories (w9/m494,
// depscache.go) beside the directory, under a top-level name of their own,
// as Render keeps both in its one build cache.
//
// The archive is made and unpacked only inside BuildKit's sandbox, on the
// image the build itself runs on. The credentialed phases move one opaque file
// and parse nothing in it, and whatever a hostile archive could do while it is
// unpacked lands in a sandbox whose only lasting effects are the cache mounts,
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

// s3CacheScript is the purge or save phase's script: its layer-cache body, and
// on a native build the archive's transfer started in the background beside
// it. The body runs in a subshell, which confines its early exits to its own
// half, and the script waits for both. Both halves share the phase's one
// deadline. The restore's is s3CacheRestoreScript.
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

// xdgCacheList asks the store for the archive before cache-restore says or
// fetches anything, beside the layout's listing (s3CacheListLayer). The
// listing tells a first build (miss) from an unreachable store (error), and
// an archive over the bound is never fetched. One within it is named by its
// size in bex_xdg_get, which xdgCacheFetch fetches.
const xdgCacheList = xdgCacheOutcome + `bex_listing=$(bex_s3_ls "$BEX_XDG_URI")
bex_rc=$?
bex_bytes=$(printf '%s\n' "$bex_listing" | while read -r bex_d bex_t bex_n bex_name; do
  [ "$bex_name" = ` + xdgCacheObject + ` ] && echo "$bex_n"
done | head -n 1)
case $bex_bytes in ''|*[!0-9]*) bex_bytes= ;; esac
bex_xdg_get=
if [ "$bex_rc" -gt 1 ]; then
  bex_xdg ` + CacheError + `
elif [ -z "$bex_bytes" ]; then
  bex_xdg ` + CacheMiss + `
elif [ "$bex_bytes" -gt "$BEX_XDG_MAX_BYTES" ]; then
  bex_xdg ` + CacheSkipped + ` "$bex_bytes"
else
  bex_xdg_get=$bex_bytes
fi
`

// xdgCacheFetch fetches the listed archive into /xdg-in, in the background
// beside the layout, and keeps it only when it arrived at its listed size.
const xdgCacheFetch = `if [ -n "$bex_xdg_get" ]; then
  (
  if bex_s3 cp "$BEX_XDG_URI" ` + xdgInMount + `/` + xdgCacheObject + ` --no-progress &&
    [ "$(wc -c < ` + xdgInMount + `/` + xdgCacheObject + ` | tr -d ' ')" = "$bex_xdg_get" ]; then
    bex_xdg ` + CacheHit + ` "$bex_xdg_get"
  else
    rm -f ` + xdgInMount + `/` + xdgCacheObject + `
    bex_xdg ` + CacheError + `
  fi
  ) &
fi
`

// xdgCacheRestored leaves the buildkit phase what Render's second line
// needs, once the layout and the archive are both in:
// "<layout bytes> <archive bytes> <seconds>" in /xdg-in/transfer, a size 0
// for either that did not arrive whole, the seconds the whole restore took.
// The wrapper adds the seed's own seconds as the line's extraction
// (xdgBuildctl). It is written whenever the store held a cache to fetch or
// could not say, so a restore that came to nothing still says it builds
// clean, and a first build writes nothing.
const xdgCacheRestored = `bex_layout=0 bex_archive=0
[ ! -s ` + cacheInMount + `/index.json ] || bex_layout=$(du -sb ` + cacheInMount + ` | cut -f1)
[ ! -f ` + xdgInMount + `/` + xdgCacheObject + ` ] || bex_archive=$bex_xdg_get
if [ "$bex_layer_rc" != 1 ] || [ -n "$bex_xdg_get" ]; then
  printf '%s %s %s\n' "$bex_layout" "$bex_archive" "$(( $(date +%s) - bex_t0 ))" > ` + xdgInMount + `/transfer
fi
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

// xdgArchiveRoot is where the seed and export solves mount both caches the
// archive carries, each under its own top-level name: $XDG_CACHE_HOME's as
// xdgArchiveXDG, the dependency directories' (depscache.go) as
// xdgArchiveDeps, beside xdgArchiveDepsKey, the toolchain they were built
// with. A cache mount is its id, wherever a solve mounts it.
const (
	xdgArchiveRoot    = "/bex-cache"
	xdgArchiveXDG     = "xdg"
	xdgArchiveDeps    = "deps"
	xdgArchiveDepsKey = "deps.key"
)

// xdgArchiveMounts is the seed and export steps' mounts of both caches.
const xdgArchiveMounts = " --mount=type=cache,id=" + nativeXDGCacheID + ",target=" + xdgArchiveRoot + "/" + xdgArchiveXDG + ",sharing=locked" +
	" --mount=type=cache,id=" + nativeDepsCacheID + ",target=" + xdgArchiveRoot + "/" + xdgArchiveDeps + ",sharing=locked"

// xdgDockerfile holds the two solves around the image's. Both run on the
// image the build's own final stage runs on (BEX_XDG_BASE), which that solve
// pulls anyway, and mount the build command's cache mounts by their ids.
// Neither step can be served from a cache: the daemon is the pod's own, and
// neither solve imports one.
//
// The seed unpacks no more than the bound and keeps nothing of an archive
// that does not unpack cleanly, gzip's own check included. It keeps the
// dependency directories only when the archive names this build's toolchain
// (BEX_DEPS_KEY): a gem's extension, a NIF or a target directory built
// against another Ruby, OTP, Rust or system library is never restored. A
// member outside both caches lands in the seed's own discarded filesystem.
//
// The save archives both caches only within the bound together, Render's one
// 7 GB build cache, removing each file as it is archived so the cache never
// sits on BuildKit's disk twice. It names each cache's entries rather than
// the cache, which is a mountpoint and so cannot be removed. Two empty caches,
// or a total over the bound, leave the "skipped" marker instead. Its export
// holds just that one file.
var xdgDockerfile = `ARG BEX_XDG_BASE
FROM $BEX_XDG_BASE AS bex-xdg-seed
ARG BEX_DEPS_KEY
RUN --mount=type=bind,target=/run/bex-xdg-in` + xdgArchiveMounts + ` <<'BEX_XDG_SEED'
bex_t0=$(date +%s)
if { { gzip -dc /run/bex-xdg-in/` + xdgCacheObject + `; echo $? > /tmp/bex-gzip; } | head -c ` + strconv.FormatInt(xdgCacheMaxBytes+1, 10) + ` | tar -xf - -C ` + xdgArchiveRoot + ` --no-same-owner; } 2>/dev/null &&
  [ "$(cat /tmp/bex-gzip)" = 0 ]; then
  [ -n "$BEX_DEPS_KEY" ] && [ "$(cat ` + xdgArchiveRoot + `/` + xdgArchiveDepsKey + ` 2>/dev/null)" = "$BEX_DEPS_KEY" ] ||
    find ` + xdgArchiveRoot + `/` + xdgArchiveDeps + ` -mindepth 1 -delete
  echo "bex-xdg-seed: restored in $(( $(date +%s) - bex_t0 ))s"
else
  find ` + xdgArchiveRoot + `/` + xdgArchiveXDG + ` ` + xdgArchiveRoot + `/` + xdgArchiveDeps + ` -mindepth 1 -delete
  echo "bex-xdg-seed: unusable"
fi
BEX_XDG_SEED

FROM $BEX_XDG_BASE AS bex-xdg-save
ARG BEX_DEPS_KEY
RUN` + xdgArchiveMounts + ` <<'BEX_XDG_SAVE'
mkdir /bex-xdg-out && cd ` + xdgArchiveRoot + ` || exit 1
if [ -n "$(ls -A ` + xdgArchiveDeps + `)" ]; then
  [ -n "$BEX_DEPS_KEY" ] && printf '%s\n' "$BEX_DEPS_KEY" > ` + xdgArchiveDepsKey + ` || find ` + xdgArchiveDeps + ` -mindepth 1 -delete
fi
if [ -z "$(ls -A ` + xdgArchiveXDG + `)" ] && [ -z "$(ls -A ` + xdgArchiveDeps + `)" ]; then
  echo 0 > /bex-xdg-out/skipped
  exit 0
fi
bex_bytes=$(du -sbc ` + xdgArchiveXDG + ` ` + xdgArchiveDeps + ` | tail -n 1 | cut -f1)
if [ "$bex_bytes" -gt ` + xdgMaxBytes + ` ]; then
  echo "$bex_bytes" > /bex-xdg-out/skipped
  exit 0
fi
{ { find ` + xdgArchiveXDG + ` ` + xdgArchiveDeps + ` -mindepth 1 -maxdepth 1 -print0; [ ! -f ` + xdgArchiveDepsKey + ` ] || printf '` + xdgArchiveDepsKey + `\0'; } |
  tar --sparse --numeric-owner --totals --remove-files --null -T - -cf - 2> /tmp/bex-tar.log; echo $? > /tmp/bex-tar; } |
  gzip -1 > /bex-xdg-out/` + xdgCacheObject + ` || exit 1
bex_stream=$(sed -n 's/^Total bytes written: \([0-9]*\).*/\1/p' /tmp/bex-tar.log)
[ "$(cat /tmp/bex-tar)" = 0 ] && [ -n "$bex_stream" ] || exit 1
if [ "$bex_stream" -gt ` + xdgMaxBytes + ` ] || [ "$(wc -c < /bex-xdg-out/` + xdgCacheObject + `)" -gt ` + xdgMaxBytes + ` ]; then
  rm /bex-xdg-out/` + xdgCacheObject + `
  echo "$bex_stream" > /bex-xdg-out/skipped
fi
BEX_XDG_SAVE

FROM scratch AS bex-xdg-export
COPY --from=bex-xdg-save /bex-xdg-out/ /
`

// xdgBuildctl is the buildctl buildctl-daemonless.sh runs for a native build
// that keeps $XDG_CACHE_HOME ($BUILDCTL, which the script reads for exactly
// this): `$BUILDCTL --addr=<socket> <command> ...`. Anything but a build is
// buildctl's. A build seeds the cache mounts when cache-restore fetched an
// archive, runs the image's solve exactly as asked, and then archives the
// mounts into /xdg-out. Seed and export are best-effort, quiet and bounded:
// their output never reaches the build log, and only the image's solve
// decides the phase's exit status. The export's seconds follow it on a line
// of their own (xdgExportLine), which nativeLogDisplay keeps out of the log.
//
// Before the image's solve it prints Render's second restore line from the
// totals cache-restore left (xdgCacheRestored): the layout and the archive
// together, the restore's seconds, and the seed's own as the extraction. An
// archive the seed could not unpack counts for nothing; when nothing that
// arrived is usable, the line is "bex: no usable build cache; building
// clean" instead.
//
// The final stage's base is the last FROM naming an image alone: the
// resolver may put a stage ahead of it, and every such stage is named. Once
// seeded, the fetched archive and BuildKit's copy of it are deleted, so the
// build has the disk they held.
//
// The dependency directories' key (BEX_DEPS_KEY) is the toolchain they are
// built with: the generated Dockerfile up to its source copy, which pins every
// image the build runs on by digest (the final base with its system
// libraries, the selected runtime copied onto it, every companion) and sets
// the runtime's defaults, plus the machine. The source, the service's
// variables and its build command come after that line, so a source-only
// commit keeps the key; a new runtime version, a refreshed image or another
// architecture changes it. A Dockerfile that cannot be hashed gives no key,
// and no dependency directories cross into or out of the build.
var xdgBuildctl = `bex_addr=$1
shift
[ "${1:-}" = build ] || exec buildctl "$bex_addr" "$@"
bex_base=$(sed -n 's/^FROM \([^ ]*\)$/\1/p' /native/Dockerfile | tail -n 1)
bex_deps_key=$(sed '/^COPY \. \.$/q' /native/Dockerfile | sha256sum | cut -c1-64)
case $bex_deps_key in *[!0-9a-f]*) bex_deps_key= ;; esac
[ "${#bex_deps_key}" = 64 ] && bex_deps_key=$bex_deps_key-$(uname -m) || bex_deps_key=
bex_xdg() {
  bex_target=$1
  shift
  timeout ` + strconv.Itoa(cacheBudgetSeconds) + ` buildctl "$bex_addr" build --progress=plain --frontend dockerfile.v0 \
    --local dockerfile=` + xdgDir + ` --opt target="$bex_target" --opt build-arg:BEX_XDG_BASE="$bex_base" \
    --opt build-arg:BEX_DEPS_KEY="$bex_deps_key" "$@" 2>&1
}
bex_number() { case $1 in ''|*[!0-9]*) return 1 ;; esac; }
` + cacheSizeFunc + `if [ -f ` + xdgInMount + `/transfer ]; then
  bex_took=0
  read -r bex_layout bex_archive bex_secs < ` + xdgInMount + `/transfer
  if [ -f ` + xdgInMount + `/` + xdgCacheObject + ` ]; then
    bex_took=
    if [ -n "$bex_base" ]; then
      bex_took=$(bex_xdg bex-xdg-seed --local context=` + xdgInMount + ` | sed -n 's/.* bex-xdg-seed: restored in \([0-9][0-9]*\)s$/\1/p' | tail -n 1)
      buildctl "$bex_addr" prune --filter type==source.local >/dev/null 2>&1 || :
    fi
    rm -f ` + xdgInMount + `/` + xdgCacheObject + `
  fi
  bex_number "$bex_took" || bex_archive=0 bex_took=0
  if bex_number "$bex_layout" && bex_number "$bex_archive" && bex_number "$bex_secs" &&
    [ $(( bex_layout + bex_archive )) -gt 0 ]; then
    ` + shellLine(renderlog.BuildShell, "Transferred $(bex_size $(( bex_layout + bex_archive ))) in ${bex_secs}s. Extraction took ${bex_took}s.") + `
  else
    echo "bex: no usable build cache; building clean"
  fi
fi
buildctl "$bex_addr" "$@" || exit
if [ -n "$bex_base" ]; then
  bex_t0=$(date +%s)
  bex_xdg bex-xdg-export --local context=` + xdgDir + ` --output type=local,dest=` + xdgOutMount + ` >/dev/null ||
    rm -rf ` + xdgOutMount + `/* ` + xdgOutMount + `/.[!.]*
  echo "` + xdgExportLine + ` $(( $(date +%s) - bex_t0 ))"
fi
`

// xdgExportLine heads the line on which xdgBuildctl reports the seconds its
// archive's solve took, for nativeLogDisplay to keep out of the log and
// count among the export's parts (w33/m571).
const xdgExportLine = "bex-export-xdg"

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
