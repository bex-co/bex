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
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// cacheOpts is a source build with the D3 registry cache enabled.
func cacheOpts() Options {
	o := opts()
	o.Workspace = "tea-w1"
	o.PushSecret = "reg-pull-tea-w1-hello"
	o.BuildCache = true
	return o
}

// scriptOf is the shell text a phase runs — the word after "-c". Phases differ
// in whether they pass "-eu", so the index is found rather than assumed.
func scriptIndex(t *testing.T, c *corev1.Container) int {
	t.Helper()
	for i, a := range c.Command {
		if a == "-c" && i+1 < len(c.Command) {
			return i + 1
		}
	}
	t.Fatalf("phase %q runs no -c script: %v", c.Name, c.Command)
	return 0
}

func scriptOf(t *testing.T, c *corev1.Container) string {
	t.Helper()
	return c.Command[scriptIndex(t, c)]
}

// cachePhaseIn finds a phase in either container list. The build package's
// containerByName searches one slice; a cache phase can be in either, and which
// one it lands in is itself under test.
func cachePhaseIn(spec *corev1.PodSpec, name string) *corev1.Container {
	if c := containerByName(spec.InitContainers, name); c != nil {
		return c
	}
	return containerByName(spec.Containers, name)
}

func TestBuildCacheGateOffLeavesTheBuildJobUntouched(t *testing.T) {
	// The gate-off path must be the pre-m86 build, not merely "a build without
	// obvious cache bits". Every seam the feature adds is checked here, because
	// a stray volume or an extra arg on the default path would change every
	// tenant's build without anyone asking for it.
	o := cacheOpts()
	o.BuildCache = false
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		if cachePhaseIn(&spec, name) != nil {
			t.Errorf("gate off still creates the %q phase", name)
		}
	}
	for _, v := range []string{cacheInVolume, cacheOutVolume} {
		if volByName(spec.Volumes, v) != nil {
			t.Errorf("gate off still mounts the %q volume", v)
		}
	}
	bk := cachePhaseIn(&spec, "buildkit")
	if bk == nil {
		t.Fatal("buildkit phase missing")
	}
	// The exact pre-feature invocation. Written out rather than derived, so a
	// change to the cached form cannot silently redefine what "unchanged" means.
	wantScript := classifyPrelude + `bex_run buildctl-daemonless.sh "$@"`
	if got := scriptOf(t, bk); got != wantScript {
		t.Errorf("buildkit script changed with the gate off:\n got %q\nwant %q", got, wantScript)
	}
	for _, a := range bk.Args {
		if strings.Contains(a, "export-cache") || strings.Contains(a, "import-cache") {
			t.Errorf("gate off still passes a cache arg to buildctl: %q", a)
		}
	}
	for _, m := range bk.VolumeMounts {
		if m.MountPath == cacheInMount || m.MountPath == cacheOutMount {
			t.Errorf("gate off still mounts %q into buildkit", m.MountPath)
		}
	}
	// Whole-spec sweep: catches a cache seam added somewhere none of the
	// targeted assertions above happen to look.
	raw, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, token := range []string{cacheInMount, cacheOutMount, "-cache", "cache-restore", "cache-save"} {
		if strings.Contains(string(raw), token) {
			t.Errorf("gate-off Job spec mentions %q", token)
		}
	}
}

func TestBuildCacheAddsOnlyWhatItClaimsToAdd(t *testing.T) {
	// The strong form of "gate off is byte-identical". Strip the cache seams out
	// of the enabled spec and it must equal the disabled spec EXACTLY — so the
	// feature cannot quietly move a resource request, reorder a phase, or change
	// a security context along the way. Unlike a pinned digest this stays true
	// through legitimate future changes to the build Job, and unlike a token
	// sweep it compares every field.
	//
	// The gate-off spec was verified byte-identical to the pre-milestone code
	// when the feature landed; this test is what keeps it so.
	for _, tc := range []struct {
		name string
		o    Options
	}{
		{"dockerfile", cacheOpts()},
		{"native+signing", func() Options {
			o := cacheOpts()
			o.Builder, o.Runtime, o.BuildCommand, o.StartCommand = BuilderNative, "node", "npm ci", "npm start"
			o.RuntimeEnvSecret, o.SignKeySecret = "env", "signing"
			return o
		}()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			off := tc.o
			off.BuildCache = false
			// The WHOLE Job, not just its pod spec: backoffLimit,
			// activeDeadlineSeconds, the failure policy and the labels are all
			// things this feature must leave alone, and an earlier version of
			// this test compared only the pod template and missed them.
			want := BuildJob(off, off.ImageRef())
			got := BuildJob(tc.o, tc.o.ImageRef())

			spec := &got.Spec.Template.Spec
			spec.InitContainers = withoutCachePhases(spec.InitContainers)
			spec.Containers = withoutCachePhases(spec.Containers)
			spec.Volumes = slices.DeleteFunc(spec.Volumes, func(v corev1.Volume) bool {
				return v.Name == cacheInVolume || v.Name == cacheOutVolume
			})
			for i := range spec.InitContainers {
				c := &spec.InitContainers[i]
				if c.Name != "buildkit" {
					continue
				}
				c.VolumeMounts = slices.DeleteFunc(c.VolumeMounts, func(m corev1.VolumeMount) bool {
					return m.MountPath == cacheInMount || m.MountPath == cacheOutMount
				})
				c.Command[scriptIndex(t, c)] = classifyPrelude + `bex_run buildctl-daemonless.sh "$@"`
				if j := slices.Index(c.Args, "--export-cache"); j >= 0 {
					c.Args = slices.Delete(c.Args, j, j+2)
				}
			}
			if !reflect.DeepEqual(want, got) {
				wantJSON, _ := json.MarshalIndent(want, "", "  ")
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				t.Errorf("enabling the cache changed something outside the cache seams\ngate off:\n%s\n\ngate on, cache seams stripped:\n%s", wantJSON, gotJSON)
			}
		})
	}
}

func withoutCachePhases(cs []corev1.Container) []corev1.Container {
	return slices.DeleteFunc(cs, func(c corev1.Container) bool {
		return c.Name == cacheRestorePhase || c.Name == cacheSavePhase
	})
}

func TestBuildCacheRefComesFromTheAppIdentity(t *testing.T) {
	// The cache must land in the SAME workspace column as the image. Deriving it
	// from identity rather than string-building it is what makes that true by
	// construction — docs/ADR034 §6 isolation rests on it.
	o := cacheOpts()
	if got, want := o.CacheRef(), "zot.bex-registry.svc:5000/tea-w1/hello-cache:cache"; got != want {
		t.Errorf("CacheRef = %q, want %q", got, want)
	}
	// Legacy (unlabeled) Apps keep the flat column, exactly as their image does.
	legacy := cacheOpts()
	legacy.Workspace = ""
	if got, want := legacy.CacheRef(), "zot.bex-registry.svc:5000/hello-cache:cache"; got != want {
		t.Errorf("legacy CacheRef = %q, want %q", got, want)
	}
	// Two workspaces owning the same App name must not share a cache. This is
	// the isolation property; a collision here would be a confidentiality bug,
	// since cache layers carry source-derived content.
	other := cacheOpts()
	other.Workspace = "tea-w2"
	if o.CacheRef() == other.CacheRef() {
		t.Errorf("same-name Apps in different workspaces share a cache ref: %q", o.CacheRef())
	}
}

func TestBuildCacheKeepsRegistryCredentialsOutOfBuildKit(t *testing.T) {
	// The reason the cache travels through skopeo at all. BuildKit runs
	// tenant-authored RUN steps; handing it a credential that can write to the
	// platform registry would invert the invariant the whole pipeline is built
	// on, and --export-cache type=registry would have done exactly that.
	o := cacheOpts()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	bk := cachePhaseIn(&spec, "buildkit")
	if bk == nil {
		t.Fatal("buildkit phase missing")
	}
	if mountByName(bk.VolumeMounts, "push-registry-cred") != nil {
		t.Fatal("buildkit mounts the push credential; the cache export must stay local")
	}
	for _, a := range bk.Args {
		if strings.Contains(a, "type=registry") {
			t.Fatalf("buildkit exports/imports the cache directly to the registry: %q", a)
		}
	}
	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		c := cachePhaseIn(&spec, name)
		if c == nil {
			t.Fatalf("%s phase missing", name)
		}
		if mountByName(c.VolumeMounts, "push-registry-cred") == nil {
			t.Errorf("%s cannot authenticate: no push credential mounted", name)
		}
	}
}

func TestBuildCachePhasesCannotFailTheBuild(t *testing.T) {
	// ADR060 D3's invariant: cache loss changes speed, never results. Both
	// directions fail for ordinary reasons — a first build has no cache to
	// restore — so neither may propagate a non-zero status.
	o := cacheOpts()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		c := cachePhaseIn(&spec, name)
		if c == nil {
			t.Fatalf("%s phase missing", name)
		}
		// The ref reaches skopeo as argv, never as script text — the same rule
		// classifyPrelude enforces for tenant values.
		if strings.Contains(scriptOf(t, c), o.CacheRef()) {
			t.Errorf("%s interpolates the cache ref into the script text", name)
		}
		if !strings.Contains(strings.Join(c.Args, " "), o.CacheRef()) {
			t.Errorf("%s never names the cache ref in Args: %v", name, c.Args)
		}
	}
}

func TestBuildCacheShellIsValidAndSurvivesAFailedStep(t *testing.T) {
	// The best-effort wrapper and the conditional import are both hand-written
	// shell. Prove they parse, and that a failing skopeo really does exit 0 —
	// asserting on the string alone would not catch a misplaced `||`.
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	o := cacheOpts()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	for _, name := range []string{cacheRestorePhase, cacheSavePhase, "buildkit"} {
		c := cachePhaseIn(&spec, name)
		if c == nil {
			t.Fatalf("%s phase missing", name)
		}
		if out, err := exec.Command("sh", "-n", "-c", scriptOf(t, c)).CombinedOutput(); err != nil {
			t.Errorf("%s script is not valid sh: %v\n%s", name, err, out)
		}
	}

	// A failing cache step exits 0. `skopeo` is not on PATH here, so the command
	// genuinely fails and the wrapper is what supplies the exit status.
	//
	// The restore script's failure branch is retargeted at a temp directory
	// first: it contains a real `rm -rf` against the container's absolute mount
	// path, and a test must never run that against the developer's own root.
	// Retargeting also lets the cleanup itself be checked.
	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		dir := t.TempDir()
		c := cachePhaseIn(&spec, name)
		script := strings.ReplaceAll(scriptOf(t, c), cacheInMount, dir)
		layout := filepath.Join(dir, "index.json")
		if err := os.WriteFile(layout, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("sh", "-c", script, name)
		cmd.Args = append(cmd.Args, c.Args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("a failed %s propagated a failure: %v\n%s", name, err, out)
		}
		if name != cacheRestorePhase {
			continue
		}
		// A half-written layout must not survive a failed restore: the import
		// guard only asks whether an index exists, so leaving one behind would
		// offer BuildKit a cache whose blobs never arrived.
		if _, err := os.Stat(layout); !os.IsNotExist(err) {
			t.Errorf("a failed restore left its layout in place (stat err = %v)", err)
		}
	}

	// The import is added only when a restored layout is actually present.
	bk := cachePhaseIn(&spec, "buildkit")
	bkScript := scriptOf(t, bk)
	probe := "set -- a b\n" + bkScript[strings.Index(bkScript, "if [ -s "):]
	probe = strings.Replace(probe, `bex_run buildctl-daemonless.sh "$@"`, `echo "$@"`, 1)
	out, err := exec.Command("sh", "-eu", "-c", probe).CombinedOutput()
	if err != nil {
		t.Fatalf("import guard did not run: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "a b" {
		t.Errorf("import-cache was added with no restored layout present: %q", got)
	}
}

func TestBuildCacheExportIsMaxModeAndNeverFailsTheBuild(t *testing.T) {
	o := cacheOpts()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	bk := cachePhaseIn(&spec, "buildkit")
	if bk == nil {
		t.Fatal("buildkit phase missing")
	}
	var export string
	for i, a := range bk.Args {
		if a == "--export-cache" && i+1 < len(bk.Args) {
			export = bk.Args[i+1]
		}
	}
	if export == "" {
		t.Fatal("no --export-cache arg")
	}
	// Each of these was measured against a real BuildKit + Zot before it was
	// written down. mode=max keeps the intermediate stages bex builds actually
	// have; image-manifest=true is what lets skopeo carry the artifact and what
	// an OCI-strict registry accepts; ignore-error=true is what keeps a failed
	// export from failing a build that already produced its image.
	for _, want := range []string{
		"type=local", "dest=" + cacheOutMount, "mode=max",
		"image-manifest=true", "compression=zstd", "ignore-error=true",
	} {
		if !strings.Contains(export, want) {
			t.Errorf("--export-cache %q is missing %q", export, want)
		}
	}
}

func TestBuildCacheRestoreRetagsTheLayoutForBuildKit(t *testing.T) {
	// The single non-obvious step in the whole feature. BuildKit's local cache
	// importer selects the manifest annotated org.opencontainers.image.ref.name,
	// and a registry round-trip drops that annotation — so a restore that does
	// not re-tag yields a cache BuildKit silently ignores. Measured: without the
	// tag every step rebuilt; with it every step was CACHED. There is no error
	// and no log line to catch this, which is why it gets its own test.
	o := cacheOpts()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	restore := cachePhaseIn(&spec, cacheRestorePhase)
	if restore == nil {
		t.Fatal("cache-restore phase missing")
	}
	want := "oci:" + cacheInMount + ":" + cacheLayoutTag
	if got := restore.Args[len(restore.Args)-1]; got != want {
		t.Errorf("restore destination = %q, want %q (an untagged layout imports nothing)", got, want)
	}
}

func TestBuildCacheSaveRunsBesideThePushRatherThanBeforeIt(t *testing.T) {
	// A milestone about build speed must not add serial work to every build.
	// Storing the cache shares nothing with the image push, so it belongs beside
	// it; restoring must precede BuildKit, and follow the clone so a build that
	// fails on tenant input does not first pull a cache it cannot use.
	o := cacheOpts()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	order := map[string]int{}
	for i, c := range spec.InitContainers {
		order[c.Name] = i
	}
	if _, ok := order[cacheRestorePhase]; !ok {
		t.Fatal("cache-restore is not an init container")
	}
	if order[cacheRestorePhase] < order["clone"] {
		t.Error("cache-restore runs before the clone")
	}
	if order[cacheRestorePhase] > order["buildkit"] {
		t.Error("cache-restore runs after buildkit, so the import can never hit")
	}
	if _, ok := order[cacheSavePhase]; ok {
		t.Error("cache-save is an init container; it would delay the image push on every build")
	}
	if cachePhaseIn(&spec, cacheSavePhase) == nil {
		t.Error("cache-save phase missing")
	}
	if len(spec.Containers) != 2 {
		t.Errorf("want push and cache-save running in parallel, got %v", contNames(spec.Containers))
	}
}

func TestBuildCacheWithSigningKeepsSigningAfterThePush(t *testing.T) {
	// The signing layout moves the push into initContainers and replaces
	// Containers wholesale. Appending cache-save before that ran would have
	// silently dropped it; appending after must not disturb cosign's ordering
	// guarantee (init → containers is what makes signing follow a good push).
	o := cacheOpts()
	o.SignKeySecret = "signing"
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	if cachePhaseIn(&spec, cacheSavePhase) == nil {
		t.Fatal("cache-save was dropped by the signing layout")
	}
	init := map[string]bool{}
	for _, c := range spec.InitContainers {
		init[c.Name] = true
	}
	if !init[pushContainer] {
		t.Error("signing layout no longer runs the push as an init container")
	}
	main := contNames(spec.Containers)
	if len(main) != 2 || main[0] != "sign" {
		t.Errorf("main containers = %v, want sign plus cache-save", main)
	}
}

func TestBuildCacheOccupiesExactlyOneTag(t *testing.T) {
	// What actually bounds cache growth. The cache is a rolling artifact under a
	// constant tag, so a repository holds one manifest no matter how many builds
	// run — superseded manifests go untagged and Zot's deleteUntagged reaps
	// them. That structural bound, not a retention policy, is what keeps cache
	// growth from ever competing with deployable images (see docs/ADR060 D3).
	o := cacheOpts()
	withRevision := cacheOpts()
	withRevision.Revision = "gen-999"
	if o.CacheRef() != withRevision.CacheRef() {
		t.Errorf("the cache ref varies with the revision (%q vs %q); every build would add a tag",
			o.CacheRef(), withRevision.CacheRef())
	}
}

func TestBuildCacheKeepsThePodDiskCeilingBounded(t *testing.T) {
	// A pod's ephemeral-storage limit is the SUM of its regular containers'
	// limits (init containers only contribute their max). Giving cache-save the
	// push phase's 16G therefore silently DOUBLED the disk a tenant build could
	// consume before eviction, quietly undoing the ceiling build.go documents —
	// a regression no existing test could see, because every test looked at
	// containers one at a time. This one adds them up the way kubelet does.
	o := cacheOpts()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec

	sum := resource.NewQuantity(0, resource.DecimalSI)
	for _, c := range spec.Containers {
		q := c.Resources.Limits[corev1.ResourceEphemeralStorage]
		if q.IsZero() {
			t.Fatalf("phase %q has no ephemeral-storage limit; the pod's ceiling becomes unbounded", c.Name)
		}
		sum.Add(q)
	}
	initMax := resource.NewQuantity(0, resource.DecimalSI)
	for _, c := range spec.InitContainers {
		q := c.Resources.Limits[corev1.ResourceEphemeralStorage]
		if q.Cmp(*initMax) > 0 {
			initMax = &q
		}
	}
	ceiling := sum
	if initMax.Cmp(*sum) > 0 {
		ceiling = initMax
	}
	want := resource.MustParse("24G") // pushEphemeralLimit 16G + cacheEphemeralLimit 8G
	if ceiling.Cmp(want) > 0 {
		t.Errorf("pod ephemeral-storage ceiling = %s, want at most %s; a tenant build can fill more node disk than intended",
			ceiling, want.String())
	}

	// The cache volumes carry their own, smaller bound for the same reason: an
	// over-limit emptyDir evicts the pod, and eviction sets DisruptionTarget,
	// which buildPodFailurePolicy IGNORES — turning a disk-filling build into a
	// retry loop that ends only at the deadline.
	for _, name := range []string{cacheInVolume, cacheOutVolume} {
		v := volByName(spec.Volumes, name)
		if v == nil {
			t.Fatalf("volume %q missing", name)
		}
		limit := v.EmptyDir.SizeLimit
		if limit == nil {
			t.Fatalf("volume %q is unbounded", name)
		}
		if want := resource.MustParse(cacheEmptyDirSizeLimit); limit.Cmp(want) != 0 {
			t.Errorf("volume %q size limit = %s, want %s", name, limit, want.String())
		}
	}
}

func TestBuildCacheTransfersCannotHangPastTheirOwnBudget(t *testing.T) {
	// A hung registry connection is not a failure the best-effort wrapper can
	// catch — it never exits. Without a bound, a stuck transfer consumes the
	// Job's whole 30-minute deadline and then fails a build whose image was
	// already pushed, which is precisely what "the cache affects speed, never
	// results" forbids.
	o := cacheOpts()
	spec := BuildJob(o, o.ImageRef()).Spec.Template.Spec
	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		c := cachePhaseIn(&spec, name)
		if c == nil {
			t.Fatalf("%s phase missing", name)
		}
		// A skopeo global flag: it has to precede the subcommand to apply.
		if len(c.Args) < 3 || c.Args[0] != "--command-timeout" || c.Args[2] != "copy" {
			t.Errorf("%s args = %v; want a --command-timeout ahead of the copy subcommand", name, c.Args)
		}
		if d, err := time.ParseDuration(c.Args[1]); err != nil {
			t.Errorf("%s timeout %q does not parse: %v", name, c.Args[1], err)
		} else if d >= BuildTimeout {
			t.Errorf("%s timeout %s does not bound anything below the %s build deadline", name, d, BuildTimeout)
		}
	}
}
