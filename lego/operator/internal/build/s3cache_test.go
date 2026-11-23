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
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/publish"
)

const testCacheSecret = "bex-build-cache-s3"

// s3CacheOpts is a source build with BEX_BUILD_CACHE=s3.
func s3CacheOpts() Options {
	o := cacheOpts()
	o.CacheStore = CacheStore{
		Endpoint: "https://s3.eu-central-2.wasabisys.com",
		Bucket:   "bex-build-cache",
		Region:   "eu-central-2",
		Secret:   testCacheSecret,
	}
	return o
}

func s3Spec(o Options) corev1.PodSpec {
	return BuildJob(o, o.ImageRef()).Spec.Template.Spec
}

func TestS3BuildCacheKeepsEveryCredentialOutOfBuildKit(t *testing.T) {
	// ADR060 D3's founding invariant, restated for a store whose credential is
	// bucket-wide: the container that runs tenant RUN steps must hold no part of
	// it, and the credentialed phases must hold nothing else.
	for _, skip := range []bool{false, true} {
		o := s3CacheOpts()
		o.SkipCacheImport = skip
		spec := s3Spec(o)

		bk := cachePhaseIn(&spec, "buildkit")
		if bk == nil {
			t.Fatal("buildkit phase missing")
		}
		raw, err := json.Marshal(bk)
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range []string{testCacheSecret, "AWS_", "s3://", o.CacheStore.Endpoint} {
			if strings.Contains(string(raw), token) {
				t.Errorf("skipImport=%v: the buildkit container carries %q", skip, token)
			}
		}
		if len(bk.EnvFrom) != 0 {
			t.Errorf("skipImport=%v: buildkit has envFrom %v", skip, bk.EnvFrom)
		}

		phases := []string{cacheRestorePhase, cacheSavePhase}
		if skip {
			phases = []string{cachePurgePhase, cacheSavePhase}
		}
		for _, name := range phases {
			c := cachePhaseIn(&spec, name)
			if c == nil {
				t.Fatalf("skipImport=%v: %s phase missing", skip, name)
			}
			if c.Image != publish.DefaultAWSCLIImage {
				t.Errorf("%s runs %q, want the pinned aws-cli image", name, c.Image)
			}
			if len(c.EnvFrom) != 1 || c.EnvFrom[0].SecretRef == nil || c.EnvFrom[0].SecretRef.Name != testCacheSecret {
				t.Fatalf("%s envFrom = %+v, want exactly the cache Secret", name, c.EnvFrom)
			}
			// A missing Secret must cost the cache, never the build: a required
			// envFrom would hold the pod in CreateContainerConfigError.
			if opt := c.EnvFrom[0].SecretRef.Optional; opt == nil || !*opt {
				t.Errorf("%s requires the cache Secret; a missing one would wedge the build", name)
			}
			if mountByName(c.VolumeMounts, "push-registry-cred") != nil {
				t.Errorf("%s mounts the registry push credential, which an S3 phase never needs", name)
			}
			if *c.SecurityContext.AllowPrivilegeEscalation || len(c.SecurityContext.Capabilities.Drop) == 0 {
				t.Errorf("%s is not restricted: %+v", name, c.SecurityContext)
			}
		}
	}
}

func TestS3BuildCacheTargetsOnlyThisAppLifetime(t *testing.T) {
	// The prefix is the isolation boundary inside a bucket the credential can
	// read whole: it must carry the workspace and the App's UID, so neither a
	// same-named App in another workspace nor this App's recreation can read
	// the layers.
	o := s3CacheOpts()
	const want = "s3://bex-build-cache/tea-w1/uid-hello/layer-cache/"
	spec := s3Spec(o)
	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		if got := envValue(cachePhaseIn(&spec, name).Env, "BEX_CACHE_URI"); got != want {
			t.Errorf("%s BEX_CACHE_URI = %q, want %q", name, got, want)
		}
	}
	other := s3CacheOpts()
	other.Workspace = "tea-w2"
	recreated := s3CacheOpts()
	recreated.AppUID = "uid-hello-2"
	for label, alt := range map[string]Options{"another workspace": other, "a recreated App": recreated} {
		if alt.layerCachePrefix() == o.layerCachePrefix() {
			t.Errorf("%s shares the cache prefix %q", label, o.layerCachePrefix())
		}
	}
	// The values reach the script as env, never as script text.
	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		if c := cachePhaseIn(&spec, name); strings.Contains(scriptOf(t, c), "bex-build-cache") {
			t.Errorf("%s splices the bucket into its script", name)
		}
	}
}

func TestS3BuildCacheAddsOnlyWhatItClaimsToAdd(t *testing.T) {
	// The S3 backend changes only the phases that carry the layout: with them,
	// their volumes, the buildkit mounts and the export flag stripped, the Job is
	// the gate-off Job exactly. The BuildKit half is shared with the registry
	// backend, so the same strip must work for both. A native build also
	// carries $XDG_CACHE_HOME (w9/m493): its two volumes, their buildkit
	// mounts and the env pointing buildctl-daemonless.sh at the wrapper.
	for _, tc := range []struct {
		name string
		o    Options
	}{
		{"dockerfile", s3CacheOpts()},
		{"native+signing", func() Options {
			o := s3CacheOpts()
			o.Builder, o.Runtime, o.BuildCommand, o.StartCommand = BuilderNative, "node", "npm ci", "npm start"
			o.RuntimeEnvSecret, o.SignKeySecret = "env", "signing"
			return o
		}()},
		{"clear-cache", func() Options { o := s3CacheOpts(); o.SkipCacheImport = true; return o }()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			off := tc.o
			off.BuildCache = false
			want := BuildJob(off, off.ImageRef())
			got := BuildJob(tc.o, tc.o.ImageRef())
			withoutCacheSaveRule(got)
			spec := &got.Spec.Template.Spec
			spec.InitContainers = withoutCachePhases(spec.InitContainers)
			spec.Containers = withoutCachePhases(spec.Containers)
			spec.Volumes = slices.DeleteFunc(spec.Volumes, func(v corev1.Volume) bool {
				return v.Name == cacheInVolume || v.Name == cacheOutVolume || v.Name == xdgInVolume || v.Name == xdgOutVolume
			})
			for i := range spec.InitContainers {
				c := &spec.InitContainers[i]
				if c.Name != "buildkit" {
					continue
				}
				c.VolumeMounts = slices.DeleteFunc(c.VolumeMounts, func(m corev1.VolumeMount) bool {
					return m.MountPath == cacheInMount || m.MountPath == cacheOutMount || m.MountPath == xdgInMount || m.MountPath == xdgOutMount
				})
				c.Env = slices.DeleteFunc(c.Env, func(e corev1.EnvVar) bool { return e.Name == "BUILDCTL" || e.Name == "BUILDKITD_FLAGS" })
				script := revisionStamp + `bex_run buildctl-daemonless.sh "$@"`
				if tc.o.Builder == BuilderNative {
					script = nativeLogDisplay + script + nativeExportReport
				}
				c.Command[scriptIndex(t, c)] = classifyPrelude + script
				if j := slices.Index(c.Args, "--export-cache"); j >= 0 {
					c.Args = slices.Delete(c.Args, j, j+2)
				}
			}
			for _, pod := range []*corev1.PodSpec{&want.Spec.Template.Spec, &got.Spec.Template.Spec} {
				for _, cs := range [][]corev1.Container{pod.InitContainers, pod.Containers} {
					for i := range cs {
						cs[i].Env = slices.DeleteFunc(cs[i].Env, func(e corev1.EnvVar) bool { return e.Name == "BEX_MESSAGE_BYTES" })
					}
				}
			}
			if !reflect.DeepEqual(want, got) {
				wantJSON, _ := json.MarshalIndent(want, "", "  ")
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				t.Errorf("the S3 cache changed something outside its seams\ngate off:\n%s\n\ngate on, seams stripped:\n%s", wantJSON, gotJSON)
			}
		})
	}
}

func TestS3BuildCacheShareTheRegistryBackendsBuildKitSide(t *testing.T) {
	// Only the carriers differ: BuildKit's flags, mounts and import guard are
	// the ones measured against a real BuildKit for the registry backend.
	s3, reg := s3CacheOpts(), cacheOpts()
	bkS3 := cachePhaseIn(new(s3Spec(s3)), "buildkit")
	bkReg := cachePhaseIn(new(s3Spec(reg)), "buildkit")
	if !reflect.DeepEqual(bkS3, bkReg) {
		t.Errorf("the buildkit phase differs between backends:\ns3:       %+v\nregistry: %+v", bkS3, bkReg)
	}
}

func TestS3BuildCacheOffUnlessEverythingItNeedsIsThere(t *testing.T) {
	plain := opts()
	plain.Workspace = "tea-w1"
	plain.PushSecret = "reg-pull-tea-w1-hello"
	baseline, err := json.Marshal(BuildJob(plain, plain.ImageRef()))
	if err != nil {
		t.Fatal(err)
	}
	// A configured store behind a closed gate is the gate-off Job, byte for byte.
	gateOff := s3CacheOpts()
	gateOff.BuildCache = false
	if got, _ := json.Marshal(BuildJob(gateOff, gateOff.ImageRef())); string(got) != string(baseline) {
		t.Error("a configured store with BuildCache off changed the Job")
	}
	// No App UID means no prefix of its own: uncached, and never silently on
	// the registry the S3 store exists to keep caches out of.
	noUID := s3CacheOpts()
	noUID.AppUID = ""
	plain.AppUID = ""
	want, _ := json.Marshal(BuildJob(plain, plain.ImageRef()))
	if got, _ := json.Marshal(BuildJob(noUID, noUID.ImageRef())); string(got) != string(want) {
		t.Errorf("an S3 cache with no App UID still adds cache seams:\n%s", got)
	}
	// A half-configured store is not an S3 store: the registry backend stays.
	half := s3CacheOpts()
	half.CacheStore.Secret = ""
	if c := cachePhaseIn(new(s3Spec(half)), cacheRestorePhase); c == nil || c.Image != defaultPushImage {
		t.Errorf("a store with no Secret did not leave the registry backend in place: %+v", c)
	}
}

func TestS3BuildCacheClearPurgesInsteadOfRestoring(t *testing.T) {
	o := s3CacheOpts()
	o.SkipCacheImport = true
	spec := s3Spec(o)
	if cachePhaseIn(&spec, cacheRestorePhase) != nil {
		t.Fatal("a clear-cache rebuild restores prior layers")
	}
	purge := cachePhaseIn(&spec, cachePurgePhase)
	if purge == nil || !strings.Contains(scriptOf(t, purge), `rm "$BEX_CACHE_URI" --recursive`) {
		t.Fatalf("a clear-cache rebuild does not purge the App's prefix: %+v", purge)
	}
	if cachePhaseIn(&spec, cacheSavePhase) == nil {
		t.Fatal("a clear-cache rebuild must still save a fresh cache")
	}
	if volByName(spec.Volumes, cacheInVolume) != nil {
		t.Fatal("a clear-cache rebuild mounts cache-in")
	}
	order := map[string]int{}
	for i, c := range spec.InitContainers {
		order[c.Name] = i
	}
	if order[cachePurgePhase] < order["clone"] || order[cachePurgePhase] > order["buildkit"] {
		t.Errorf("purge must run after the clone and before buildkit: %v", contNames(spec.InitContainers))
	}
}

func TestS3BuildCachePhasesAreBoundedBelowTheBuild(t *testing.T) {
	// A hung store connection never exits, so the best-effort wrapper cannot
	// catch it; the phase-wide deadline is what does.
	spec := s3Spec(s3CacheOpts())
	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		c := cachePhaseIn(&spec, name)
		secs := envValue(c.Env, "BEX_CACHE_BUDGET_SECONDS")
		d, err := time.ParseDuration(secs + "s")
		if err != nil || d <= 0 || d >= BuildTimeout {
			t.Errorf("%s budget %q does not bound the phase below the %s build", name, secs, BuildTimeout)
		}
		if !strings.Contains(scriptOf(t, c), `timeout "$bex_left" aws`) {
			t.Errorf("%s runs aws without the shared deadline", name)
		}
	}
	if got := envValue(cachePhaseIn(&spec, cacheSavePhase).Env, "BEX_CACHE_MAX_BYTES"); got != "8000000000" {
		t.Errorf("cache-save bound = %q, want the 8G volume bound", got)
	}
}

func TestCachePurgeJobDeletesTheWholeAppLifetimeAndOnlyIt(t *testing.T) {
	store := s3CacheOpts().CacheStore
	job, err := CachePurgeJob("hello", "uid-hello", "tea-w1", "tea-w1-ns", store, "bex-build", "cache-purge-hello-1234")
	if err != nil {
		t.Fatal(err)
	}
	c := job.Spec.Template.Spec.Containers[0]
	if got := envValue(c.Env, "BEX_CACHE_URI"); got != "s3://bex-build-cache/tea-w1/uid-hello/" {
		t.Errorf("purge URI = %q, want the App's whole root", got)
	}
	// Not best-effort: the finalizer must see a failure to retry it.
	if strings.Contains(c.Command[len(c.Command)-1], "||") {
		t.Errorf("the finalizer purge swallows its failure: %q", c.Command)
	}
	if ref := c.EnvFrom[0].SecretRef; ref.Name != testCacheSecret || (ref.Optional != nil && *ref.Optional) {
		t.Errorf("purge credential = %+v, want the cache Secret, required", ref)
	}
	if job.Labels[execution.LabelAppUID] != "uid-hello" || job.Labels[execution.LabelComponent] != CachePurgeComponent {
		t.Errorf("purge Job labels %v cannot be tied to its App lifetime", job.Labels)
	}
	pod := job.Spec.Template.Spec
	// bex-build-job-shape admission: pod user namespace, no token, a volume.
	if pod.HostUsers == nil || *pod.HostUsers || pod.AutomountServiceAccountToken == nil || *pod.AutomountServiceAccountToken || len(pod.Volumes) == 0 {
		t.Errorf("purge pod fails the bex-build Job shape: %+v", pod)
	}
	if _, err := CachePurgeJob("hello", "", "tea-w1", "", store, "bex-build", "x"); err == nil {
		t.Error("an empty App UID would purge the whole bucket, and was accepted")
	}
}

func TestCachePhaseSignalsReadOnlyOurOwnOutcomes(t *testing.T) {
	start := metav1.NewTime(time.Unix(1000, 0))
	term := func(name, msg string, secs int64) corev1.ContainerStatus {
		return corev1.ContainerStatus{Name: name, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Message: msg, StartedAt: start, FinishedAt: metav1.NewTime(start.Add(time.Duration(secs) * time.Second)),
		}}}
	}
	pod := &corev1.Pod{Status: corev1.PodStatus{
		InitContainerStatuses: []corev1.ContainerStatus{
			term("clone", "hit 5", 1),
			term(cacheRestorePhase, "hit 1048576\n", 4),
		},
		ContainerStatuses: []corev1.ContainerStatus{
			term(pushContainer, "", 2),
			term(cacheSavePhase, "saved 2097152", 9),
		},
	}}
	got := cachePhaseSignals(pod)
	want := []CachePhaseSignal{
		{Phase: cacheRestorePhase, Kind: CacheKindLayer, Outcome: CacheHit, Seconds: 4, Bytes: 1 << 20},
		{Phase: cacheSavePhase, Kind: CacheKindLayer, Outcome: CacheSaved, Seconds: 9, Bytes: 2 << 20},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cachePhaseSignals = %+v, want %+v", got, want)
	}
	// A native build's phase reports its $XDG_CACHE_HOME archive on a line of
	// its own, in either order, with the archive's own seconds: the layer
	// line keeps the phase's, since the two transfers run side by side.
	native := &corev1.Pod{Status: corev1.PodStatus{
		InitContainerStatuses: []corev1.ContainerStatus{
			term(cacheRestorePhase, "xdg hit 3145728 2\nmiss\n", 5),
		},
		ContainerStatuses: []corev1.ContainerStatus{
			term(cacheSavePhase, "saved 10\nxdg skipped 0 0\n", 3),
		},
	}}
	got = cachePhaseSignals(native)
	want = []CachePhaseSignal{
		{Phase: cacheRestorePhase, Kind: CacheKindXDG, Outcome: CacheHit, Seconds: 2, Bytes: 3 << 20},
		{Phase: cacheRestorePhase, Kind: CacheKindLayer, Outcome: CacheMiss, Seconds: 5},
		{Phase: cacheSavePhase, Kind: CacheKindLayer, Outcome: CacheSaved, Seconds: 3, Bytes: 10},
		{Phase: cacheSavePhase, Kind: CacheKindXDG, Outcome: CacheSkipped},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("native cachePhaseSignals = %+v, want %+v", got, want)
	}
	// Each phase may report only its own outcomes, and a registry-backend
	// phase, which writes nothing, reports none.
	for _, status := range []corev1.ContainerStatus{
		term(cacheRestorePhase, "saved 10", 1),
		term(cacheSavePhase, "hit", 1),
		term(cachePurgePhase, "purged everything now", 1),
		term(cacheRestorePhase, "", 1),
		term(cacheRestorePhase, "pwned 1", 1),
		// The archive's lines carry exactly their four fields, its own
		// outcomes (a layer restore's "incomplete" is no archive's), and a
		// whole number of seconds.
		term(cacheRestorePhase, "xdg hit 10", 1),
		term(cacheRestorePhase, "xdg incomplete 0 1", 1),
		term(cachePurgePhase, "xdg hit 0 1", 1),
		term(cacheSavePhase, "xdg saved 10 -1", 1),
		term(cacheSavePhase, "xdg saved 10 1 extra", 1),
		term(cacheSavePhase, "layer saved 10", 1),
	} {
		p := &corev1.Pod{Status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{status}}}
		if got := cachePhaseSignals(p); len(got) != 0 {
			t.Errorf("%s %q was read as %+v", status.Name, status.State.Terminated.Message, got)
		}
	}
	// One outcome per kind: a second line of a kind already read is not a
	// second observation.
	twice := &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
		term(cacheSavePhase, "saved 1\nsaved 2\nxdg saved 3 1\nxdg error 0 1", 1),
	}}}
	if got := cachePhaseSignals(twice); len(got) != 2 || got[0].Bytes != 1 || got[1].Bytes != 3 {
		t.Errorf("repeated lines read as %+v", got)
	}
}

// --- the phase scripts, run against a fake store ---

// fakeAWS stands in for aws-cli, mapping s3://<bucket>/<key> onto $FAKE_S3.
// It honours exactly the flags the phases rely on and nothing more:
// --no-follow-symlinks (without it, symlinks are followed, so a dropped flag
// leaks), the blobs include filter, and --delete. Every call is logged, and
// FAKE_S3_FAIL=<subcommand> makes that subcommand fail, with exit status
// FAKE_S3_FAIL_RC (default 1). ls answers as aws-cli does: one line per
// matching key, and exit 1 when none matches.
const fakeAWS = `#!/bin/sh
shift 2 # --endpoint-url <url>
[ "$1" = s3 ] || exit 64
shift
cmd=$1
shift
echo "$cmd $*" >> "$FAKE_S3_LOG"
[ "$FAKE_S3_FAIL" = "$cmd" ] && exit "${FAKE_S3_FAIL_RC:-1}"
local_of() { printf '%s/%s' "$FAKE_S3" "${1#s3://}"; }
case "$cmd" in
rm)
  rm -rf "$(local_of "$1")"
  ;;
ls)
  f=$(local_of "$1")
  [ -f "$f" ] || exit 1
  printf '2026-11-18 10:00:00 %10s %s\n' "$(wc -c < "$f" | tr -d ' ')" "$(basename "$f")"
  ;;
cp)
  case "$1" in
  s3://*)
    [ -f "$(local_of "$1")" ] || exit 1
    cp "$(local_of "$1")" "$2"
    ;;
  *)
    mkdir -p "$(dirname "$(local_of "$2")")"
    cp "$1" "$(local_of "$2")"
    ;;
  esac
  ;;
sync)
  src=$1 dst=$2
  case "$src" in
  s3://*)
    [ -d "$(local_of "$src")" ] || exit 0
    mkdir -p "$dst"
    cp -R "$(local_of "$src")/." "$dst/"
    ;;
  *)
    follow=-L
    case " $* " in *" --no-follow-symlinks "*) follow= ;; esac
    store=$(local_of "$dst")
    mkdir -p "$store/blobs/sha256"
    for f in $(find $follow "$src/blobs/sha256" -type f 2>/dev/null); do
      [ -f "$store/blobs/sha256/$(basename "$f")" ] || cp "$f" "$store/blobs/sha256/"
    done
    case " $* " in *" --delete "*)
      for f in "$store"/blobs/sha256/*; do
        [ -e "$f" ] || continue
        n=$(basename "$f")
        [ -f "$src/blobs/sha256/$n" ] && [ ! -L "$src/blobs/sha256/$n" ] || rm -f "$f"
      done
      ;;
    esac
    ;;
  esac
  ;;
esac
`

type cacheScriptRig struct {
	t                *testing.T
	dir, store, logf string
	in, out, outcome string
	xin, xout        string
	bin              string
}

func newCacheScriptRig(t *testing.T) *cacheScriptRig {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	dir := t.TempDir()
	r := &cacheScriptRig{t: t, dir: dir, store: filepath.Join(dir, "store"), logf: filepath.Join(dir, "calls"),
		in: filepath.Join(dir, "in"), out: filepath.Join(dir, "out"), outcome: filepath.Join(dir, "outcome"), bin: filepath.Join(dir, "bin"),
		xin: filepath.Join(dir, "xdg-in"), xout: filepath.Join(dir, "xdg-out")}
	for _, d := range []string{r.store, r.in, r.out, r.xin, r.xout, r.bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// GNU timeout and du -b are what the aws-cli image ships; shim both so the
	// scripts run the same on a developer's BSD userland.
	for name, body := range map[string]string{
		"aws":     fakeAWS,
		"timeout": "#!/bin/sh\nshift\nexec \"$@\"\n",
		"du":      "#!/bin/sh\nprintf '%s\\t%s\\n' \"$(find \"$2\" -type f -exec cat {} + | wc -c | tr -d ' ')\" \"$2\"\n",
	} {
		if err := os.WriteFile(filepath.Join(r.bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

// run executes a phase's script with the mounts and the termination log
// retargeted at the rig, returning its exit error and stderr.
func (r *cacheScriptRig) run(phase *corev1.Container, extraEnv ...string) (string, error) {
	r.t.Helper()
	script := scriptOf(r.t, phase)
	script = strings.ReplaceAll(script, cacheInMount, r.in)
	script = strings.ReplaceAll(script, cacheOutMount, r.out)
	script = strings.ReplaceAll(script, xdgInMount, r.xin)
	script = strings.ReplaceAll(script, xdgOutMount, r.xout)
	script = strings.ReplaceAll(script, terminationLog, r.outcome)
	_ = os.Remove(r.outcome)
	_ = os.Remove(r.logf)
	cmd := exec.Command("sh", "-c", script, phase.Name)
	cmd.Env = []string{"PATH=" + r.bin + ":/usr/bin:/bin", "FAKE_S3=" + r.store, "FAKE_S3_LOG=" + r.logf}
	for _, e := range phase.Env {
		cmd.Env = append(cmd.Env, e.Name+"="+e.Value)
	}
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *cacheScriptRig) outcomeLine() string {
	b, _ := os.ReadFile(r.outcome)
	return strings.TrimSpace(string(b))
}

func (r *cacheScriptRig) calls() []string {
	b, _ := os.ReadFile(r.logf)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// writeLayout writes the shape BuildKit's local exporter produces: index.json
// names a manifest, which names a config and a layer, and each layer carries
// an uncompressed diff ID that is not a blob.
func (r *cacheScriptRig) writeLayout(dir string) (layer string) {
	r.t.Helper()
	sum := func(c byte) string { return strings.Repeat(string(c), 64) }
	config, layer, manifest, diffID := sum('c'), sum('1'), sum('a'), sum('d')
	blobs := filepath.Join(dir, "blobs", "sha256")
	if err := os.MkdirAll(filepath.Join(dir, "ingest"), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.MkdirAll(blobs, 0o755); err != nil {
		r.t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(dir, "oci-layout"): `{"imageLayoutVersion":"1.0.0"}`,
		filepath.Join(dir, "index.json"): `{"schemaVersion":2,"manifests":[{"digest":"sha256:` + manifest + `","annotations":{"org.opencontainers.image.ref.name":"latest"}}]}`,
		filepath.Join(blobs, manifest): `{"config":{"digest":"sha256:` + config + `"},"layers":[{"digest":"sha256:` + layer +
			`","annotations":{"containerd.io/uncompressed":"sha256:` + diffID + `"}}]}`,
		filepath.Join(blobs, config): `{"layers":[{"blob":"sha256:` + layer + `"}]}`,
		filepath.Join(blobs, layer):  "layer bytes",
	}
	for p, body := range files {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			r.t.Fatal(err)
		}
	}
	return layer
}

func (r *cacheScriptRig) stored(rel string) bool {
	_, err := os.Stat(filepath.Join(r.store, "bex-build-cache", "tea-w1", "uid-hello", "layer-cache", rel))
	return err == nil
}

func s3Phases(t *testing.T, skip bool) (restoreOrPurge, save *corev1.Container) {
	t.Helper()
	o := s3CacheOpts()
	o.SkipCacheImport = skip
	spec := s3Spec(o)
	first := cacheRestorePhase
	if skip {
		first = cachePurgePhase
	}
	return cachePhaseIn(&spec, first), cachePhaseIn(&spec, cacheSavePhase)
}

func TestS3CacheScriptsRoundTripALayoutUnchanged(t *testing.T) {
	r := newCacheScriptRig(t)
	restore, save := s3Phases(t, false)

	// First build: nothing stored yet.
	if out, err := r.run(restore); err != nil || r.outcomeLine() != CacheMiss {
		t.Fatalf("first restore: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	if entries, _ := os.ReadDir(r.in); len(entries) != 0 {
		t.Fatalf("a miss left %d entries in cache-in", len(entries))
	}

	r.writeLayout(r.out)
	if out, err := r.run(save); err != nil || !strings.HasPrefix(r.outcomeLine(), CacheSaved+" ") {
		t.Fatalf("save: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	// Blobs before the index, and the prune last: a reader never meets an
	// index ahead of its blobs.
	calls := r.calls()
	if len(calls) != 4 || !strings.HasPrefix(calls[0], "sync") || !strings.Contains(calls[1], "oci-layout") ||
		!strings.Contains(calls[2], "index.json") || !strings.Contains(calls[3], "--delete") {
		t.Fatalf("save calls out of order: %q", calls)
	}
	for _, c := range []string{calls[0], calls[3]} {
		if !strings.Contains(c, "--no-follow-symlinks") || !strings.Contains(c, "--size-only") {
			t.Errorf("a blob sync without --no-follow-symlinks --size-only: %q", c)
		}
	}
	if r.stored("ingest") {
		t.Error("BuildKit's ingest directory was stored")
	}

	if out, err := r.run(restore); err != nil || !strings.HasPrefix(r.outcomeLine(), CacheHit+" ") {
		t.Fatalf("warm restore: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	// Byte-identical index: the ref.name annotation BuildKit's importer selects
	// on arrives without re-tagging.
	want, _ := os.ReadFile(filepath.Join(r.out, "index.json"))
	got, _ := os.ReadFile(filepath.Join(r.in, "index.json"))
	if string(got) != string(want) {
		t.Errorf("restored index.json differs:\n got %s\nwant %s", got, want)
	}
}

// renderTransferred is Render's second restore line as its build logs print
// it after "==> Downloading cache...", on native and Docker builds alike: the
// size in Render's units, then the transfer's and the extraction's seconds.
var renderTransferred = regexp.MustCompile(`^==> Transferred [0-9]+(\.[0-9])?(B|KB|MB|GB|TB) in [0-9]+s\. Extraction took [0-9]+s\.$`)

// dirBytes is what the rig's du reports for dir: its regular files' bytes.
func dirBytes(t *testing.T, dir string) int64 {
	t.Helper()
	var n int64
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err
		}
		info, err := d.Info()
		n += info.Size()
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestS3CacheRestoreSaysRendersRestoreLinesOnADockerfileBuild(t *testing.T) {
	// Render's Docker build log says when it restores a build cache, after the
	// clone and before BuildKit's first step: "==> Downloading cache..." and
	// "==> Transferred 149MB in 9s. Extraction took 0s.". cache-restore is the
	// phase between the two.
	spec := s3Spec(s3CacheOpts())
	names := make([]string, 0, len(spec.InitContainers))
	for _, c := range spec.InitContainers {
		names = append(names, c.Name)
	}
	if i := slices.Index(names, cacheRestorePhase); i < 1 || i > slices.Index(names, "buildkit") {
		t.Fatalf("cache-restore does not run between the clone and BuildKit: %v", names)
	}

	r := newCacheScriptRig(t)
	restore, save := s3Phases(t, false)

	// A first build says neither line, as on Render, and fetches nothing.
	if out, err := r.run(restore); err != nil || out != "" {
		t.Errorf("first build: err=%v\n%s", err, out)
	}
	if calls := r.calls(); len(calls) != 1 || !strings.HasPrefix(calls[0], "ls ") {
		t.Errorf("a first build did more than ask the store: %q", calls)
	}

	layer := r.writeLayout(r.out)
	if _, err := r.run(save); err != nil {
		t.Fatal(err)
	}
	out, err := r.run(restore)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	size := strconv.FormatInt(dirBytes(t, r.in), 10) + "B in "
	if len(lines) != 2 || lines[0] != "==> Downloading cache..." || !renderTransferred.MatchString(lines[1]) ||
		!strings.HasPrefix(lines[1], "==> Transferred "+size) || !strings.HasSuffix(lines[1], " Extraction took 0s.") {
		t.Errorf("a warm restore's log, want Render's two lines with the layout's size:\n%s", out)
	}

	// A layout that does not arrive whole is no cache: the log says so. Each
	// build pod's /cache-in starts empty.
	blob := filepath.Join(r.store, "bex-build-cache", "tea-w1", "uid-hello", "layer-cache", "blobs", "sha256", layer)
	if err := os.Remove(blob); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(r.in); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(r.in, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, _ := r.run(restore); out != "==> Downloading cache...\nbex: no usable build cache; building clean\n" {
		t.Errorf("an incomplete layout's log:\n%s", out)
	}
	// Nor does a store that cannot be asked offer one.
	if out, _ := r.run(restore, "FAKE_S3_FAIL=ls", "FAKE_S3_FAIL_RC=255"); out != "bex: no usable build cache; building clean\n" {
		t.Errorf("an unreachable store's log:\n%s", out)
	}
}

func TestCacheSizeSpellsRendersUnits(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	for bytes, want := range map[string]string{
		"0": "0B", "999": "999B", "1000": "1.0KB", "2048": "2.0KB", "5399": "5.3KB", "62000": "62KB",
		"2700000": "2.7MB", "149000000": "149MB", "999999999": "999MB", "7000000000": "7.0GB", "12000000000000": "12TB",
	} {
		out, err := exec.Command("sh", "-c", cacheSizeFunc+`bex_size "$1"`, "bex", bytes).CombinedOutput()
		if got := strings.TrimSpace(string(out)); err != nil || got != want {
			t.Errorf("bex_size %s = %q (%v), want %q", bytes, got, err, want)
		}
	}
}

func TestS3CacheRestoreOffersNoLayoutWithAMissingBlob(t *testing.T) {
	// The bucket's expiry removes an old, unchanged blob while the index,
	// rewritten every build, survives. Offering that layout would hand BuildKit
	// a cache naming content it does not have.
	r := newCacheScriptRig(t)
	restore, save := s3Phases(t, false)
	layer := r.writeLayout(r.out)
	if _, err := r.run(save); err != nil {
		t.Fatal(err)
	}
	blob := filepath.Join(r.store, "bex-build-cache", "tea-w1", "uid-hello", "layer-cache", "blobs", "sha256", layer)
	if err := os.Remove(blob); err != nil {
		t.Fatal(err)
	}
	if out, err := r.run(restore); err != nil || r.outcomeLine() != CacheIncomplete {
		t.Fatalf("restore: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	if entries, _ := os.ReadDir(r.in); len(entries) != 0 {
		t.Errorf("an incomplete layout was left in cache-in (%d entries)", len(entries))
	}
	// The next save heals it: the layer is uploaded again because it is missing.
	if _, err := r.run(save); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run(restore); err != nil || !strings.HasPrefix(r.outcomeLine(), CacheHit) {
		t.Errorf("after the healing save, restore = %q", r.outcomeLine())
	}
}

func TestS3CacheScriptsNeverFailTheBuild(t *testing.T) {
	r := newCacheScriptRig(t)
	restore, save := s3Phases(t, false)
	purge, _ := s3Phases(t, true)

	r.writeLayout(r.out)
	if out, err := r.run(save, "FAKE_S3_FAIL=sync"); err != nil || r.outcomeLine() != CacheError {
		t.Errorf("failed save: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	if r.stored("index.json") {
		t.Error("a save whose blobs failed still published its index")
	}

	// A stored layout whose download fails part-way: nothing of it is offered.
	if _, err := r.run(save); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(r.in, "index.json"), "{}")
	if out, err := r.run(restore, "FAKE_S3_FAIL=sync"); err != nil || r.outcomeLine() != CacheError {
		t.Errorf("failed restore: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	if entries, _ := os.ReadDir(r.in); len(entries) != 0 {
		t.Error("a failed restore left a partial layout behind")
	}

	if out, err := r.run(purge, "FAKE_S3_FAIL=rm"); err != nil || r.outcomeLine() != CacheError {
		t.Errorf("failed purge: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	// No credential at all: aws cannot even list, the phase still exits 0 and
	// fetches nothing.
	if out, err := r.run(restore, "FAKE_S3_FAIL=ls", "FAKE_S3_FAIL_RC=253"); err != nil || r.outcomeLine() != CacheError {
		t.Errorf("restore without a store: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	if calls := r.calls(); len(calls) != 1 || !strings.HasPrefix(calls[0], "ls ") {
		t.Errorf("a store that cannot list was still asked for the layout: %q", calls)
	}
}

func TestS3CacheSaveStoresNothingTheTenantPointedElsewhere(t *testing.T) {
	// /cache-out is written by the container that runs tenant code. A symlink
	// there resolves inside the credentialed save container, where
	// /proc/self/environ holds the bucket credential.
	r := newCacheScriptRig(t)
	_, save := s3Phases(t, false)
	secret := filepath.Join(r.dir, "environ")
	if err := os.WriteFile(secret, []byte("AWS_SECRET_ACCESS_KEY=leak"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.writeLayout(r.out)
	evil := strings.Repeat("e", 64)
	if err := os.Symlink(secret, filepath.Join(r.out, "blobs", "sha256", evil)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.run(save); err != nil || !strings.HasPrefix(r.outcomeLine(), CacheSaved) {
		t.Fatalf("save: %v %q", err, r.outcomeLine())
	}
	if r.stored("blobs/sha256/" + evil) {
		t.Fatal("a symlinked blob became an object")
	}

	for _, link := range []string{"index.json", "blobs"} {
		r := newCacheScriptRig(t)
		r.writeLayout(r.out)
		if err := os.RemoveAll(filepath.Join(r.out, link)); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(secret, filepath.Join(r.out, link)); err != nil {
			t.Fatal(err)
		}
		if out, err := r.run(save); err != nil || r.outcomeLine() != CacheSkipped {
			t.Errorf("symlinked %s: err=%v outcome=%q\n%s", link, err, r.outcomeLine(), out)
		}
		if b, _ := os.ReadFile(r.logf); len(b) != 0 {
			t.Errorf("symlinked %s still reached the store: %s", link, b)
		}
	}
}

func TestS3CacheSavePrunesWhatNoLayoutNames(t *testing.T) {
	r := newCacheScriptRig(t)
	_, save := s3Phases(t, false)
	stale := filepath.Join(r.store, "bex-build-cache", "tea-w1", "uid-hello", "layer-cache", "blobs", "sha256", strings.Repeat("0", 64))
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.writeLayout(r.out)
	if _, err := r.run(save); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("a blob the new layout does not name survived the save")
	}
}

func TestS3CacheSaveRefusesALayoutOverItsBound(t *testing.T) {
	r := newCacheScriptRig(t)
	_, save := s3Phases(t, false)
	r.writeLayout(r.out)
	if out, err := r.run(save, "BEX_CACHE_MAX_BYTES=10"); err != nil || !strings.HasPrefix(r.outcomeLine(), CacheSkipped+" ") {
		t.Fatalf("oversize save: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	if b, _ := os.ReadFile(r.logf); len(b) != 0 {
		t.Errorf("an oversize layout reached the store: %s", b)
	}
}

func TestS3CachePurgeEmptiesThePrefix(t *testing.T) {
	r := newCacheScriptRig(t)
	_, save := s3Phases(t, false)
	purge, _ := s3Phases(t, true)
	r.writeLayout(r.out)
	if _, err := r.run(save); err != nil {
		t.Fatal(err)
	}
	if out, err := r.run(purge); err != nil || r.outcomeLine() != CachePurged {
		t.Fatalf("purge: err=%v outcome=%q\n%s", err, r.outcomeLine(), out)
	}
	if r.stored("index.json") {
		t.Error("the purge left the App's layout in the store")
	}
}
