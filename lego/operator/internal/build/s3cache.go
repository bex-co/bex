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
	"cmp"
	"fmt"
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/identity"
	"github.com/bex-co/bex/lego/operator/internal/publish"
)

// CacheStore is the object store BEX_BUILD_CACHE=s3 keeps each App's layer
// cache in (w9/m492, docs/ADR060 D3 S3 amendment), off the Zot registry whose
// single PVC the registry backend's mode=max caches outgrew (w7/047).
//
// The cache travels exactly as the registry backend's does: BuildKit exports
// it to a local OCI layout and imports it from one, and separate phases carry
// it in and out. Here those phases run aws-cli with this store's credential,
// and the BuildKit container, which executes tenant RUN steps, still holds no
// credential of any kind.
type CacheStore struct {
	// Endpoint is the S3-compatible endpoint URL.
	Endpoint string
	// Bucket is the bucket dedicated to build caches (bex-build-cache).
	Bucket string
	// Region is the bucket's region, passed as AWS_DEFAULT_REGION when set.
	Region string
	// Secret names a Secret in the build namespace holding AWS_ACCESS_KEY_ID
	// and AWS_SECRET_ACCESS_KEY, scoped to Bucket alone.
	Secret string
	// Image overrides the aws-cli image (tests / air-gapped).
	Image string
}

// Configured reports whether every field the cache phases need is set.
func (s CacheStore) Configured() bool {
	return s.Endpoint != "" && s.Bucket != "" && s.Secret != ""
}

// uri is the s3:// URI of prefix in the store.
func (s CacheStore) uri(prefix string) string {
	return "s3://" + s.Bucket + "/" + prefix
}

func (s CacheStore) image() string { return cmp.Or(s.Image, publish.DefaultAWSCLIImage) }

// cacheBackend is the store a build's layer cache travels through.
type cacheBackend int

const (
	cacheOff cacheBackend = iota
	cacheRegistry
	cacheS3
)

// cacheBackend resolves the gate and the store into one answer. An S3 cache
// is keyed by the App's lifetime, so a build with no App UID has no prefix of
// its own to restore from or save to and runs uncached rather than falling
// back to the registry the S3 store exists to keep caches out of.
func (o Options) cacheBackend() cacheBackend {
	switch {
	case !o.BuildCache:
		return cacheOff
	case !o.CacheStore.Configured():
		return cacheRegistry
	case o.AppUID == "":
		return cacheOff
	default:
		return cacheS3
	}
}

// buildCacheRoot is this App lifetime's root in the store,
// "<workspace>/<app-uid>/", which App deletion removes whole; "" without an
// App UID.
func (o Options) buildCacheRoot() string {
	return identity.ForApp(o.Name, o.Workspace).BuildCacheRoot(o.AppUID)
}

// layerCachePrefix is the object-key prefix this App lifetime's BuildKit layer
// cache occupies: "<workspace>/<app-uid>/layer-cache/".
func (o Options) layerCachePrefix() string {
	root := o.buildCacheRoot()
	if root == "" {
		return ""
	}
	return root + "layer-cache/"
}

// cacheMaxBytes is the largest layout cache-save stores: the cache volumes'
// own bound (cacheEmptyDirSizeLimit), so a store never holds a cache a build
// could not restore into its 8G volume.
var cacheMaxBytes = mustSizeLimit(cacheEmptyDirSizeLimit).Value()

// cacheBudgetSeconds is the phase-wide deadline every S3 transfer of one phase
// shares: the registry phases' per-command bound, cacheCommandTimeout.
var cacheBudgetSeconds = func() int {
	d, err := time.ParseDuration(cacheCommandTimeout)
	if err != nil {
		panic(err)
	}
	return int(d.Seconds())
}()

// terminationLog is where each cache phase records its outcome for the
// operator's metrics (CachePhaseSignal). The phases always exit 0, so the file
// is the only channel that says whether a restore hit, missed or failed.
const terminationLog = "/dev/termination-log"

// Cache phase outcomes, a CLOSED set because each is a Prometheus label.
const (
	CacheHit        = "hit"        // restore: a complete layout was restored
	CacheMiss       = "miss"       // restore: the store holds no cache for this App yet
	CacheIncomplete = "incomplete" // restore: the layout names a blob the store no longer holds
	CacheSaved      = "saved"      // save: the exported layout was stored
	CacheSkipped    = "skipped"    // save: BuildKit exported no layout, or one over the size bound
	CachePurged     = "purged"     // purge: the App's cache was deleted
	CacheError      = "error"      // any phase: the store could not be reached or refused the request
)

// s3CachePrelude defines the helpers every S3 cache phase shares. bex_s3 runs
// one aws-cli transfer against the store with all of its output discarded: a
// third-party CLI's error text is the wrong thing to stream into a tenant's
// build log, and nothing it prints is needed to decide the outcome. bex_s3_ls
// lists one key, printing the listing: aws-cli exits 1 for a listing that
// matches nothing, a first build, and anything above 1 is a store that could
// not be asked. Every call shares one phase-wide deadline, so the phase as a
// whole is bounded no matter how many transfers it makes
// (cacheCommandTimeout). bex_outcome appends one line per cache kind the phase
// carries: the layer cache's "<outcome> [bytes]", and on a native build an
// xdg line beside it (xdgCacheOutcome).
const s3CachePrelude = `bex_deadline=$(( $(date +%s) + $BEX_CACHE_BUDGET_SECONDS ))
bex_s3() {
  bex_left=$(( bex_deadline - $(date +%s) ))
  [ "$bex_left" -gt 0 ] || return 124
  timeout "$bex_left" aws --endpoint-url "$BEX_CACHE_ENDPOINT" s3 "$@" --only-show-errors >/dev/null 2>&1
}
bex_s3_ls() {
  bex_left=$(( bex_deadline - $(date +%s) ))
  [ "$bex_left" -gt 0 ] || return 124
  timeout "$bex_left" aws --endpoint-url "$BEX_CACHE_ENDPOINT" s3 ls "$1" 2>/dev/null
}
bex_outcome() { printf '%s\n' "$*" >> ` + terminationLog + `; }
`

// s3CacheListLayer asks the store whether it holds this App lifetime's
// layout before cache-restore says or fetches anything. index.json is the
// key to ask for: cache-save stores it after the blobs it names.
const s3CacheListLayer = `bex_t0=$(date +%s)
bex_s3_ls "${BEX_CACHE_URI}index.json" >/dev/null
bex_layer_rc=$?
`

// s3CacheDownloading is the first of Render's two restore lines, said once,
// before anything is fetched, and only when the store holds a cache to fetch:
// the layout, or on a native build the archive (xdgCacheList). A first build
// says nothing, as on Render.
const s3CacheDownloading = `if [ "$bex_layer_rc" = 0 ] || [ -n "${bex_xdg_get:-}" ]; then
  echo "==> Downloading cache..." >&2
fi
`

// s3CacheRestoreLayer downloads the App's layout into /cache-in, when the
// listing found one. The layout arrives exactly as BuildKit exported it,
// index.json included, so its org.opencontainers.image.ref.name=latest
// annotation survives the round trip without the re-tagging the registry
// backend needs (cacheLayoutTag).
//
// A layout is offered to BuildKit only when every blob it names arrived: the
// bucket's 30-day expiry removes an unchanged blob that cache-save never
// re-uploads while index.json, rewritten every build, stays, and a save that
// died part-way leaves an index ahead of its blobs. Either would otherwise
// pass the buildkit step's "is there an index?" test. Two levels cover the
// layout BuildKit writes: index.json names the cache manifest, which names the
// cache config and every layer. Only "digest" fields count: each layer also
// carries its uncompressed diff ID, a sha256 that is never a blob.
//
// It runs in a subshell of its own, so its early exits end only its half, and
// it says nothing: what the restore came to is said once both halves are in
// (s3CacheRestored, xdgCacheRestored).
const s3CacheRestoreLayer = `bex_cold() {
  rm -rf ` + cacheInMount + `/* ` + cacheInMount + `/.[!.]*
  bex_outcome "$1"
  exit 0
}
[ "$bex_layer_rc" -le 1 ] || bex_cold ` + CacheError + `
[ "$bex_layer_rc" = 0 ] || bex_cold ` + CacheMiss + `
bex_s3 sync "$BEX_CACHE_URI" ` + cacheInMount + ` --no-progress || bex_cold ` + CacheError + `
[ -s ` + cacheInMount + `/index.json ] || bex_cold ` + CacheMiss + `
bex_digests() { grep -ao '"digest": *"sha256:[0-9a-f]\{64\}"' "$1" | grep -o '[0-9a-f]\{64\}'; }
for bex_ref in $(bex_digests ` + cacheInMount + `/index.json); do
  [ -f ` + cacheInMount + `/blobs/sha256/$bex_ref ] || bex_cold ` + CacheIncomplete + `
  for bex_dep in $(bex_digests ` + cacheInMount + `/blobs/sha256/$bex_ref); do
    [ -f ` + cacheInMount + `/blobs/sha256/$bex_dep ] || bex_cold ` + CacheIncomplete + `
  done
done
bex_outcome ` + CacheHit + ` "$(du -sb ` + cacheInMount + ` | cut -f1)"
`

// cacheSizeFunc defines bex_size, which spells a byte count as Render's
// restore line does: bytes, then KB, MB, GB or TB in steps of 1000, with one
// decimal below 10 ("512B", "2.7MB", "149MB"). It is shell arithmetic alone,
// so the aws-cli image that prints a Dockerfile build's line and the BuildKit
// image that prints a native build's run it alike.
const cacheSizeFunc = `bex_size() {
  bex_v=$1 bex_d=0 bex_u=B
  for bex_n in KB MB GB TB; do
    [ "$bex_v" -ge 1000 ] || break
    bex_d=$(( bex_v % 1000 / 100 ))
    bex_v=$(( bex_v / 1000 ))
    bex_u=$bex_n
  done
  if [ "$bex_u" = B ] || [ "$bex_v" -ge 10 ]; then echo "$bex_v$bex_u"; else echo "$bex_v.$bex_d$bex_u"; fi
}
`

// s3CacheRestored ends a Dockerfile build's restore with Render's second
// line, before BuildKit's first step: the layout's size and the seconds the
// restore took. BuildKit reads the layout where it lies, so nothing is
// extracted, and Render's own Docker builds read "Extraction took 0s." too.
// A listed layout that did not arrive whole, or a store that could not be
// listed, says the build runs clean instead.
const s3CacheRestored = cacheSizeFunc + `if [ -s ` + cacheInMount + `/index.json ]; then
  echo "==> Transferred $(bex_size "$(du -sb ` + cacheInMount + ` | cut -f1)") in $(( $(date +%s) - bex_t0 ))s. Extraction took 0s." >&2
elif [ "$bex_layer_rc" != 1 ]; then
  echo "bex: no usable build cache; building clean" >&2
fi
`

// s3CacheRestoreScript is cache-restore's script. It lists what the store
// holds for this App lifetime, says Render's "==> Downloading cache..." when
// that is anything, and fetches the layout and, on a native build, the
// $XDG_CACHE_HOME archive side by side under the phase's one deadline. A
// Dockerfile build's restore then prints Render's second line itself; a
// native build's buildkit phase prints it once the seed solve has unpacked
// the archive, from the totals this phase leaves it (xdgCacheRestored).
func s3CacheRestoreScript(o Options) string {
	if !o.xdgCache() {
		return s3CachePrelude + s3CacheListLayer + s3CacheDownloading + "(\n" + s3CacheRestoreLayer + ")\n" + s3CacheRestored
	}
	return s3CachePrelude + s3CacheListLayer + xdgCacheList + s3CacheDownloading + xdgCacheFetch +
		"(\n" + s3CacheRestoreLayer + ")\nwait\n" + xdgCacheRestored
}

// s3CachePurgeLayer deletes the App's layer cache before a clear-cache rebuild,
// which then neither restores nor imports (w7/m88). Best-effort for the same
// reason the registry purge is: the rebuild already skips the import, and its
// own cache-save replaces whatever a failed purge left.
const s3CachePurgeLayer = `if bex_s3 rm "$BEX_CACHE_URI" --recursive; then
  bex_outcome ` + CachePurged + `
else
  bex_outcome ` + CacheError + `
  echo "bex: build cache purge skipped; continuing (clear rebuild does not import)" >&2
fi
`

// s3CacheSaveLayer stores the layout BuildKit exported to /cache-out.
//
// Everything in /cache-out was written by the container that runs tenant
// code, so nothing in it is trusted to be what it claims. --no-follow-symlinks
// keeps a link to /proc/self/environ or any other path local to this
// credentialed container from becoming an object (the static publish rule),
// and the index and layout marker are stored only when they are regular files.
//
// Blobs go first and are compared by size alone: each is named for its
// digest, so a name already in the store is that content, and an unchanged
// dependency layer is never uploaded again. The index follows, so a reader
// never sees an index ahead of its blobs, and blobs no index names any more are
// pruned last, when a failure costs only dead weight.
const s3CacheSaveLayer = `if [ ! -f ` + cacheOutMount + `/index.json ] || [ -L ` + cacheOutMount + `/index.json ] || [ -L ` + cacheOutMount + `/blobs ] || [ -L ` + cacheOutMount + `/blobs/sha256 ]; then
  bex_outcome ` + CacheSkipped + `
  echo "bex: the build exported no cache; nothing saved" >&2
  exit 0
fi
bex_bytes=$(du -sb ` + cacheOutMount + ` | cut -f1)
if [ "$bex_bytes" -gt "$BEX_CACHE_MAX_BYTES" ]; then
  bex_outcome ` + CacheSkipped + ` "$bex_bytes"
  echo "bex: the build cache is larger than its limit; not saved" >&2
  exit 0
fi
bex_blobs() { bex_s3 sync ` + cacheOutMount + ` "$BEX_CACHE_URI" --exclude '*' --include 'blobs/sha256/*' --size-only --no-follow-symlinks --no-progress "$@"; }
if ! bex_blobs ||
  { [ -f ` + cacheOutMount + `/oci-layout ] && [ ! -L ` + cacheOutMount + `/oci-layout ] && ! bex_s3 cp ` + cacheOutMount + `/oci-layout "${BEX_CACHE_URI}oci-layout" --no-progress; } ||
  ! bex_s3 cp ` + cacheOutMount + `/index.json "${BEX_CACHE_URI}index.json" --no-progress; then
  bex_outcome ` + CacheError + `
  echo "bex: build cache step failed; continuing (the cache affects speed, never results)" >&2
  exit 0
fi
bex_blobs --delete || true
bex_outcome ` + CacheSaved + ` "$bex_bytes"
`

// s3CacheRestoreContainer is the S3 twin of cacheRestoreContainer. A native
// build's also fetches its $XDG_CACHE_HOME archive, beside the layout.
func s3CacheRestoreContainer(o Options) corev1.Container {
	c := s3CachePhase(o, cacheRestorePhase, s3CacheRestoreScript(o),
		corev1.VolumeMount{Name: cacheInVolume, MountPath: cacheInMount})
	return withXDGCache(c, o, corev1.VolumeMount{Name: xdgInVolume, MountPath: xdgInMount})
}

// s3CachePurgeContainer is the S3 twin of cachePurgeContainer. A native
// build's also deletes its $XDG_CACHE_HOME archive.
func s3CachePurgeContainer(o Options) corev1.Container {
	return withXDGCache(s3CachePhase(o, cachePurgePhase, s3CacheScript(o, s3CachePurgeLayer, xdgCachePurge)), o)
}

// s3CacheSaveContainer is the S3 twin of cacheSaveContainer. A native build's
// also stores its $XDG_CACHE_HOME archive, beside the layout.
func s3CacheSaveContainer(o Options) corev1.Container {
	c := s3CachePhase(o, cacheSavePhase, s3CacheScript(o, s3CacheSaveLayer, xdgCacheSave),
		corev1.VolumeMount{Name: cacheOutVolume, MountPath: cacheOutMount, ReadOnly: true})
	c.Env = append(c.Env, corev1.EnvVar{Name: "BEX_CACHE_MAX_BYTES", Value: strconv.FormatInt(cacheMaxBytes, 10)})
	return withXDGCache(c, o, corev1.VolumeMount{Name: xdgOutVolume, MountPath: xdgOutMount, ReadOnly: true})
}

// s3CachePhase is the shape every S3 cache phase shares: aws-cli holding the
// cache store's credential and nothing else, every value passed as env rather
// than spliced into the script, and the registry phases' resource bounds.
//
// The credential is an OPTIONAL envFrom. A Secret missing from the build
// namespace must cost a build its cache, never the build: a required envFrom
// would leave the pod in CreateContainerConfigError until the build deadline.
// Without the credential every transfer fails and the phase records an error.
func s3CachePhase(o Options, name, script string, mounts ...corev1.VolumeMount) corev1.Container {
	env := []corev1.EnvVar{
		{Name: "BEX_CACHE_URI", Value: o.CacheStore.uri(o.layerCachePrefix())},
		{Name: "BEX_CACHE_ENDPOINT", Value: o.CacheStore.Endpoint},
		{Name: "BEX_CACHE_BUDGET_SECONDS", Value: strconv.Itoa(cacheBudgetSeconds)},
		// No instance-metadata fallback: a missing credential fails at once
		// instead of spending the budget probing 169.254.169.254, which the
		// build namespace's egress policy denies anyway.
		{Name: "AWS_EC2_METADATA_DISABLED", Value: "true"},
	}
	if o.CacheStore.Region != "" {
		env = append(env, corev1.EnvVar{Name: "AWS_DEFAULT_REGION", Value: o.CacheStore.Region})
	}
	c := cachePhaseShape(name, o.CacheStore.image(), script, mounts)
	c.Env = env
	c.EnvFrom = []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{
		LocalObjectReference: corev1.LocalObjectReference{Name: o.CacheStore.Secret},
		Optional:             new(true),
	}}}
	return c
}

// cachePhaseShape is what a cache phase is regardless of its store: a
// best-effort script under the cache phases' own resource bounds.
func cachePhaseShape(name, image, script string, mounts []corev1.VolumeMount) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           image,
		Command:         []string{"sh", "-c", script, name},
		VolumeMounts:    mounts,
		SecurityContext: execution.RestrictedContainerSecurityContext(),
		// NOT pushPhaseResources: see cacheEphemeralLimit. The pod's ceiling is
		// the sum of its regular containers' limits, so reusing the push phase's
		// 16G here would double the disk a build can fill before eviction.
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("100m"),
				corev1.ResourceMemory:           resource.MustParse("128Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse(cacheEphemeralRequest),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:              resource.MustParse("1"),
				corev1.ResourceMemory:           resource.MustParse("512Mi"),
				corev1.ResourceEphemeralStorage: resource.MustParse(cacheEphemeralLimit),
			},
		},
	}
}

// CachePurgeComponent labels the App finalizer's build-cache purge Job. It is
// deliberately not one of the components ReclaimAppArtifacts sweeps: the Job
// is the finalizer's own durable state and is torn down only after it ran.
const CachePurgeComponent = "cache-purge"

// CachePurgeJob is the App finalizer's cleanup Job for one App lifetime's S3
// build caches: it deletes the App's whole BuildCacheRoot, the layer cache and
// any other cache kept beside it. Unlike the build's own purge phase it is not
// best-effort: a failure fails the Job, and the finalizer retries until the
// prefix is gone (an absent prefix is success). It refuses an empty root,
// which would name the whole bucket.
func CachePurgeJob(name, appUID, workspace, appNamespace string, store CacheStore, namespace, jobName string) (*batchv1.Job, error) {
	root := identity.ForApp(name, workspace).BuildCacheRoot(appUID)
	if root == "" {
		return nil, fmt.Errorf("build cache purge for %s: empty App UID", name)
	}
	deadline := int64((15 * time.Minute) / time.Second)
	backoff := int32(3)
	env := []corev1.EnvVar{
		{Name: "BEX_CACHE_URI", Value: store.uri(root)},
		{Name: "BEX_CACHE_ENDPOINT", Value: store.Endpoint},
		{Name: "AWS_EC2_METADATA_DISABLED", Value: "true"},
	}
	if store.Region != "" {
		env = append(env, corev1.EnvVar{Name: "AWS_DEFAULT_REGION", Value: store.Region})
	}
	if appNamespace == namespace {
		appNamespace = ""
	}
	labels := execution.PodLabels(name, appUID, CachePurgeComponent, workspace, appNamespace, false)
	spec := corev1.PodSpec{
		// No legacy Docker-link env vars (w4/m123) — a platform maintenance pod.
		EnableServiceLinks: new(false),
		RestartPolicy:      corev1.RestartPolicyNever,
		// bex-build's bex-build-job-shape admission requires at least one
		// emptyDir/Secret volume; this one gives the CLI a writable HOME.
		Volumes: []corev1.Volume{{Name: "aws-home", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
		Containers: []corev1.Container{{
			Name:    "purge",
			Image:   store.image(),
			Command: []string{"sh", "-c", `aws --endpoint-url "$BEX_CACHE_ENDPOINT" s3 rm "$BEX_CACHE_URI" --recursive --only-show-errors`},
			Env:     env,
			EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: store.Secret},
			}}},
			VolumeMounts:    []corev1.VolumeMount{{Name: "aws-home", MountPath: "/root/.aws"}},
			SecurityContext: execution.RestrictedContainerSecurityContext(),
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
		}},
	}
	execution.HardenPod(&spec)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: jobName, Namespace: namespace, Labels: labels},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoff,
			ActiveDeadlineSeconds: &deadline,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec:       spec,
			},
		},
	}, nil
}
