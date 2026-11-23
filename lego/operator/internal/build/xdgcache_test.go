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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/operator/internal/execution/executiontest"
)

// xdgOpts is a native source build with BEX_BUILD_CACHE=s3: the one shape
// that keeps $XDG_CACHE_HOME between builds (w9/m493).
func xdgOpts() Options {
	o := s3CacheOpts()
	o.Builder, o.Runtime, o.BuildCommand, o.StartCommand = BuilderNative, "node", "npm ci && npm run build", "npm start"
	return o
}

// --- the recipe: every native build, whatever the cache backend ---

func TestNativeRecipeGivesTheBuildXDGCacheHomeAndKeepsItOutOfTheImage(t *testing.T) {
	static := nativeOptions()
	static.StaticSite, static.Runtime, static.StartCommand = true, "", ""
	cases := map[string]Options{"static": static}
	for runtime := range nativeToolchains {
		o := nativeOptions()
		o.Runtime = runtime
		cases[runtime] = o
	}
	for name, o := range cases {
		df := nativeDockerfile(o)
		env, source := strings.Index(df, "\nENV XDG_CACHE_HOME="+nativeXDGCacheHome+"\n"), strings.Index(df, "\nCOPY . .\n")
		build := strings.Index(df, "\n"+nativeEnvRunMount)
		if env < 0 || env > source {
			t.Errorf("%s: $XDG_CACHE_HOME is not image env ahead of the source:\n%s", name, df)
		}
		if !strings.Contains(df, "\nRUN mkdir -p "+nativeXDGCacheHome+"\n") {
			t.Errorf("%s: the running service would find no $XDG_CACHE_HOME directory:\n%s", name, df)
		}
		// uv copies out of a cache on its own mount; telling it so keeps its
		// hardlink warning out of the log, for the build alone.
		if arg := strings.Index(df, "\nARG UV_LINK_MODE=copy\n"); arg < 0 || arg > build || strings.Contains(df, "ENV UV_LINK_MODE") {
			t.Errorf("%s: uv is not told to copy from its cache, for the build alone:\n%s", name, df)
		}
		// The directory's contents ride a cache mount on the build command's
		// RUN, and only there: nothing written into it reaches a layer.
		runLine := df[build+1:]
		runLine = runLine[:strings.Index(runLine, "\n")]
		if !strings.Contains(runLine, nativeXDGCacheMount+" [") || strings.Count(df, "type=cache") != 1 {
			t.Errorf("%s: the build command does not write $XDG_CACHE_HOME into its own cache mount:\n%s", name, df)
		}
	}
	// The seed and export solves must name the very mounts the build writes:
	// the same id and sharing, or they seed and archive other ones. A cache
	// mount is its id; where a solve mounts it is the solve's own choice.
	for _, stage := range []string{"bex-xdg-seed", "bex-xdg-save"} {
		at := strings.Index(xdgDockerfile, " AS "+stage+"\n")
		for _, id := range []string{nativeXDGCacheID, nativeDepsCacheID} {
			if at < 0 || !regexp.MustCompile(`--mount=type=cache,id=`+id+`,target=\S+,sharing=locked`).MatchString(xdgDockerfile[at:]) {
				t.Errorf("%s does not mount the build command's cache mount %s", stage, id)
			}
		}
	}
	if !strings.Contains(nativeXDGCacheMount, ",sharing=locked") || !strings.Contains(nativeDepsCacheMount, ",sharing=locked") {
		t.Error("the build command's cache mounts are not the locked ones the seed and export solves mount")
	}
}

// --- the pod: only a native build on the S3 store carries the archive ---

func TestXDGCacheRidesOnlyANativeBuildsS3Cache(t *testing.T) {
	registry := xdgOpts()
	registry.CacheStore = CacheStore{}
	gateOff := xdgOpts()
	gateOff.BuildCache = false
	for label, o := range map[string]Options{"dockerfile on S3": s3CacheOpts(), "native on the registry cache": registry, "native with the cache off": gateOff} {
		raw, err := json.Marshal(BuildJob(o, o.ImageRef()))
		if err != nil {
			t.Fatal(err)
		}
		for _, token := range []string{xdgInMount, xdgOutMount, "BEX_XDG_", xdgDir, xdgBuildkitdFlags} {
			if strings.Contains(string(raw), token) {
				t.Errorf("%s carries %q", label, token)
			}
		}
	}

	spec := s3Spec(xdgOpts())
	bk := cachePhaseIn(&spec, "buildkit")
	if got := envValue(bk.Env, "BUILDCTL"); got != "sh "+xdgDir+"/buildctl" {
		t.Errorf("buildctl-daemonless.sh is not pointed at the wrapper: BUILDCTL=%q", got)
	}
	if got := envValue(bk.Env, "BUILDKITD_FLAGS"); got != xdgBuildkitdFlags {
		t.Errorf("BUILDKITD_FLAGS = %q, want the GC kept off between solves", got)
	}
	script := scriptOf(t, bk)
	if !strings.Contains(script, xdgBuildkitSetup) || strings.Index(script, xdgBuildkitSetup) > strings.Index(script, "bex_run buildctl-daemonless.sh") {
		t.Error("the wrapper is not written before BuildKit runs")
	}
	for _, m := range []string{xdgInVolume, xdgOutVolume} {
		if mount := mountByName(bk.VolumeMounts, m); mount == nil || mount.ReadOnly {
			t.Errorf("buildkit mounts %s %+v, want it writable", m, mount)
		}
	}
	for _, v := range []string{xdgInVolume, xdgOutVolume} {
		vol := volByName(spec.Volumes, v)
		if vol == nil || vol.EmptyDir == nil || vol.EmptyDir.SizeLimit == nil || vol.EmptyDir.SizeLimit.Value() < xdgCacheMaxBytes {
			t.Errorf("volume %s = %+v, want an emptyDir bounded above the archive's bound", v, vol)
		}
	}
	restore, save := cachePhaseIn(&spec, cacheRestorePhase), cachePhaseIn(&spec, cacheSavePhase)
	if m := mountByName(restore.VolumeMounts, xdgInVolume); m == nil || m.ReadOnly {
		t.Errorf("cache-restore cannot write the archive it fetches: %+v", m)
	}
	if m := mountByName(save.VolumeMounts, xdgOutVolume); m == nil || !m.ReadOnly {
		t.Errorf("cache-save must read the archive read-only: %+v", m)
	}
	for _, c := range []*corev1.Container{restore, save} {
		if envValue(c.Env, "BEX_XDG_MAX_BYTES") != strconv.FormatInt(xdgCacheMaxBytes, 10) {
			t.Errorf("%s is not bounded by the 7 GB archive bound", c.Name)
		}
	}

	// A clear-cache rebuild neither fetches nor seeds the archive: it deletes
	// it, then saves the one this build leaves.
	clear := xdgOpts()
	clear.SkipCacheImport = true
	spec = s3Spec(clear)
	if volByName(spec.Volumes, xdgInVolume) != nil || mountByName(cachePhaseIn(&spec, "buildkit").VolumeMounts, xdgInVolume) != nil {
		t.Error("a clear-cache rebuild carries a fetched archive")
	}
	purge := cachePhaseIn(&spec, cachePurgePhase)
	if envValue(purge.Env, "BEX_XDG_URI") == "" || !strings.Contains(scriptOf(t, purge), xdgCachePurge) {
		t.Error("a clear-cache rebuild does not delete the archive")
	}
	if mountByName(cachePhaseIn(&spec, cacheSavePhase).VolumeMounts, xdgOutVolume) == nil {
		t.Error("a clear-cache rebuild does not save the archive its build leaves")
	}
}

func TestXDGCacheArchiveIsThisAppLifetimesOwn(t *testing.T) {
	o := xdgOpts()
	const want = "s3://bex-build-cache/tea-w1/uid-hello/xdg-cache.tar.gz"
	spec := s3Spec(o)
	for _, name := range []string{cacheRestorePhase, cacheSavePhase} {
		if got := envValue(cachePhaseIn(&spec, name).Env, "BEX_XDG_URI"); got != want {
			t.Errorf("%s BEX_XDG_URI = %q, want %q", name, got, want)
		}
		if strings.Contains(scriptOf(t, cachePhaseIn(&spec, name)), "tea-w1") {
			t.Errorf("%s splices the archive's key into its script", name)
		}
	}
	// App deletion's purge removes the App's whole root, archive included.
	job, err := CachePurgeJob(o.Name, o.AppUID, o.Workspace, "", o.CacheStore, "bex-build", "cache-purge-hello")
	if err != nil {
		t.Fatal(err)
	}
	if root := envValue(job.Spec.Template.Spec.Containers[0].Env, "BEX_CACHE_URI"); !strings.HasPrefix(want, root) {
		t.Errorf("App deletion purges %q, which leaves the archive %q", root, want)
	}
	other, recreated := xdgOpts(), xdgOpts()
	other.Workspace, recreated.AppUID = "tea-w2", "uid-hello-2"
	for label, alt := range map[string]Options{"another workspace": other, "a recreated App": recreated} {
		if alt.xdgCacheURI() == o.xdgCacheURI() {
			t.Errorf("%s shares the archive %q", label, o.xdgCacheURI())
		}
	}
}

func TestXDGCacheKeepsEveryCredentialOutOfBuildKit(t *testing.T) {
	for _, skip := range []bool{false, true} {
		o := xdgOpts()
		o.SkipCacheImport = skip
		bk := cachePhaseIn(new(s3Spec(o)), "buildkit")
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
	}
}

func TestXDGCacheScriptsSurviveKubeletExpansion(t *testing.T) {
	// The wrapper, its Dockerfile and the phase scripts are container
	// commands, which the kubelet expands $(NAME) and $$ in: they must reach
	// the shell byte for byte.
	for _, skip := range []bool{false, true} {
		o := xdgOpts()
		o.SkipCacheImport = skip
		spec := s3Spec(o)
		for _, c := range slices.Concat(spec.InitContainers, spec.Containers) {
			for _, part := range c.Command {
				if got := executiontest.Expand(part, expandedBuildValues(c.Env)); got != part {
					t.Errorf("skipImport=%v: %s's command changes through the kubelet", skip, c.Name)
				}
			}
		}
	}
}

// --- the credentialed phases, against the fake store ---

func xdgPhases(t *testing.T, skip bool) (restoreOrPurge, save *corev1.Container) {
	t.Helper()
	o := xdgOpts()
	o.SkipCacheImport = skip
	spec := s3Spec(o)
	first := cacheRestorePhase
	if skip {
		first = cachePurgePhase
	}
	return cachePhaseIn(&spec, first), cachePhaseIn(&spec, cacheSavePhase)
}

func (r *cacheScriptRig) xdgStored() string {
	return filepath.Join(r.store, "bex-build-cache", "tea-w1", "uid-hello", xdgCacheObject)
}

// xdgOutcome is the archive's outcome line without its seconds, which vary.
func (r *cacheScriptRig) xdgOutcome() string {
	r.t.Helper()
	for line := range strings.SplitSeq(r.outcomeLine(), "\n") {
		if fields := strings.Fields(line); len(fields) == 4 && fields[0] == xdgCacheKind {
			return strings.Join(fields[1:3], " ")
		}
	}
	return ""
}

func (r *cacheScriptRig) layerOutcome() string {
	for line := range strings.SplitSeq(r.outcomeLine(), "\n") {
		if !strings.HasPrefix(line, xdgCacheKind+" ") {
			return line
		}
	}
	return ""
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestXDGCacheArchiveRoundTripsBesideTheLayout(t *testing.T) {
	r := newCacheScriptRig(t)
	restore, save := xdgPhases(t, false)
	r.writeLayout(r.out)
	writeFile(t, filepath.Join(r.xout, xdgCacheObject), "archive bytes")
	if out, err := r.run(save); err != nil {
		t.Fatalf("save: %v\n%s", err, out)
	}
	if got := r.xdgOutcome(); got != "saved 13" {
		t.Errorf("save reported %q, want saved 13", got)
	}
	if !strings.HasPrefix(r.layerOutcome(), CacheSaved) {
		t.Errorf("the layout beside the archive was not saved: %q", r.outcomeLine())
	}
	stored, err := os.ReadFile(r.xdgStored())
	if err != nil || string(stored) != "archive bytes" {
		t.Fatalf("stored archive = %q, %v", stored, err)
	}

	out, err := r.run(restore)
	if err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	if got := r.xdgOutcome(); got != "hit 13" {
		t.Errorf("restore reported %q, want hit 13", got)
	}
	if !strings.HasPrefix(r.layerOutcome(), CacheHit) {
		t.Errorf("the layout beside the archive was not restored: %q", r.outcomeLine())
	}
	if got, _ := os.ReadFile(filepath.Join(r.xin, xdgCacheObject)); string(got) != "archive bytes" {
		t.Errorf("fetched archive = %q", got)
	}
	// Both caches' sizes and the restore's seconds, for the build log's Render
	// line, which the buildkit phase prints once the seed has run.
	transfer, _ := os.ReadFile(filepath.Join(r.xin, "transfer"))
	if fields := strings.Fields(string(transfer)); len(fields) != 3 || fields[0] != strconv.FormatInt(dirBytes(t, r.in), 10) || fields[1] != "13" {
		t.Errorf("transfer stats = %q, want the layout's bytes, the archive's 13 and the seconds", transfer)
	}
	if strings.Count(out, rendersShellLine("Downloading cache...")) != 1 || strings.Contains(shown(out), "==> Transferred") {
		t.Errorf("restore did not say once that it is downloading the cache, as Render does:\n%s", out)
	}
}

func TestXDGCacheRestoreLinesCoverTheLayoutAndTheArchiveTogether(t *testing.T) {
	// Render's one build cache gets one pair of restore lines. A native build
	// restores two things into it, the layout and the archive, so the pair
	// counts both: "Downloading" once, and one size for what arrived.
	for _, tc := range []struct {
		name    string
		archive bool
		seeded  string // the wrapper's extraction, the seed's own seconds
	}{
		{name: "layout and archive", archive: true, seeded: "4"},
		// A build that left $XDG_CACHE_HOME empty stores no archive; its
		// layout is still a cache restored, and nothing is extracted.
		{name: "layout alone", seeded: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newCacheScriptRig(t)
			restore, save := xdgPhases(t, false)
			r.writeLayout(r.out)
			if tc.archive {
				writeFile(t, filepath.Join(r.xout, xdgCacheObject), "archive bytes")
			} else {
				writeFile(t, filepath.Join(r.xout, "skipped"), "0\n")
			}
			if out, err := r.run(save); err != nil {
				t.Fatalf("save: %v\n%s", err, out)
			}
			out, err := r.run(restore)
			if err != nil || out != rendersShellLine("Downloading cache...")+"\n" {
				t.Fatalf("restore: err=%v, want Render's first line alone:\n%s", err, out)
			}

			w := newWrapperRig(t, "FROM "+wrapperBase+"\n")
			for _, f := range []string{"transfer", xdgCacheObject} {
				if b, err := os.ReadFile(filepath.Join(r.xin, f)); err == nil {
					writeFile(t, filepath.Join(w.xin, f), string(b))
				}
			}
			out, err = w.run(wrapperBuildArgs)
			if err != nil {
				t.Fatalf("wrapper: %v\n%s", err, out)
			}
			line, _, _ := strings.Cut(out, "\n")
			total := dirBytes(t, r.in)
			if tc.archive {
				total += int64(len("archive bytes"))
			}
			if !renderTransferred.MatchString(line) || !strings.HasPrefix(shown(line), "==> Transferred "+strconv.FormatInt(total, 10)+"B in ") ||
				!strings.HasSuffix(shown(line), " Extraction took "+tc.seeded+"s.") {
				t.Errorf("the build log opens with %q, want Render's line for %d bytes:\n%s", line, total, out)
			}
			if seeded := slices.ContainsFunc(w.calls(), func(c string) bool { return callTarget(c) == "bex-xdg-seed" }); seeded != tc.archive {
				t.Errorf("seed solve ran: %v, want %v", seeded, tc.archive)
			}
		})
	}
	// A first build says nothing, in either phase.
	r := newCacheScriptRig(t)
	restore, _ := xdgPhases(t, false)
	if out, err := r.run(restore); err != nil || out != "" {
		t.Errorf("first build's restore: err=%v\n%s", err, out)
	}
	if entries, _ := os.ReadDir(r.xin); len(entries) != 0 {
		t.Errorf("a first build left the wrapper %v", entries)
	}
}

func TestXDGCacheRestoreFetchesNothingItCannotUse(t *testing.T) {
	restore, _ := xdgPhases(t, false)
	for _, tc := range []struct {
		name    string
		store   string // the stored archive, "" for none
		env     []string
		outcome string
		// tried: the restore leaves the wrapper a record of nothing usable, so
		// the build log says it builds clean; a first build leaves none.
		tried bool
	}{
		{name: "first build", outcome: "miss 0"},
		{name: "store unreachable", store: "archive", env: []string{"FAKE_S3_FAIL=ls", "FAKE_S3_FAIL_RC=255"}, outcome: "error 0", tried: true},
		{name: "download fails", store: "archive", env: []string{"FAKE_S3_FAIL=cp"}, outcome: "error 0", tried: true},
		{name: "over the bound", store: "an archive over the bound", env: []string{"BEX_XDG_MAX_BYTES=10"}, outcome: "skipped 25"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newCacheScriptRig(t)
			if tc.store != "" {
				writeFile(t, r.xdgStored(), tc.store)
			}
			if out, err := r.run(restore, tc.env...); err != nil {
				t.Fatalf("a restore failed the build: %v\n%s", err, out)
			}
			if got := r.xdgOutcome(); got != tc.outcome {
				t.Errorf("outcome = %q, want %q", got, tc.outcome)
			}
			if _, err := os.Stat(filepath.Join(r.xin, xdgCacheObject)); err == nil {
				t.Error("an unusable archive was left for the seed")
			}
			transfer, err := os.ReadFile(filepath.Join(r.xin, "transfer"))
			if fields := strings.Fields(string(transfer)); tc.tried != (err == nil) || tc.tried && (len(fields) != 3 || fields[0] != "0" || fields[1] != "0") {
				t.Errorf("transfer record = %q (%v), want one of nothing usable: %v", transfer, err, tc.tried)
			}
		})
	}
}

func TestXDGCacheSaveStoresOnlyWhatBuildKitArchived(t *testing.T) {
	_, save := xdgPhases(t, false)
	for _, tc := range []struct {
		name    string
		setup   func(r *cacheScriptRig)
		env     []string
		outcome string
		stored  string // the stored archive afterwards, "" for none
	}{
		{
			name:    "the build left it empty",
			setup:   func(r *cacheScriptRig) { writeFile(t, filepath.Join(r.xout, "skipped"), "0\n") },
			outcome: "skipped 0",
		},
		{
			name:    "the build left it over the bound",
			setup:   func(r *cacheScriptRig) { writeFile(t, filepath.Join(r.xout, "skipped"), "7000000001\n") },
			outcome: "skipped 7000000001",
		},
		{
			name:    "an archive over the bound",
			setup:   func(r *cacheScriptRig) { writeFile(t, filepath.Join(r.xout, xdgCacheObject), "new archive") },
			env:     []string{"BEX_XDG_MAX_BYTES=5"},
			outcome: "skipped 11",
		},
		{
			// The export did not run to the end: the stored archive stays.
			name:    "nothing exported",
			setup:   func(*cacheScriptRig) {},
			outcome: "skipped 0",
			stored:  "old archive",
		},
		{
			name: "a link where the archive should be",
			setup: func(r *cacheScriptRig) {
				secret := filepath.Join(r.dir, "credential")
				writeFile(t, secret, "AWS_SECRET_ACCESS_KEY=leaked")
				if err := os.Symlink(secret, filepath.Join(r.xout, xdgCacheObject)); err != nil {
					t.Fatal(err)
				}
			},
			outcome: "skipped 0",
			stored:  "old archive",
		},
		{
			name:    "the store refuses",
			setup:   func(r *cacheScriptRig) { writeFile(t, filepath.Join(r.xout, xdgCacheObject), "new archive") },
			env:     []string{"FAKE_S3_FAIL=cp"},
			outcome: "error 0",
			stored:  "old archive",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newCacheScriptRig(t)
			writeFile(t, r.xdgStored(), "old archive")
			tc.setup(r)
			if out, err := r.run(save, tc.env...); err != nil {
				t.Fatalf("a save failed the build: %v\n%s", err, out)
			}
			if got := r.xdgOutcome(); got != tc.outcome {
				t.Errorf("outcome = %q, want %q", got, tc.outcome)
			}
			got, err := os.ReadFile(r.xdgStored())
			if tc.stored == "" && err == nil {
				t.Errorf("the stored archive %q outlived a build that left none", got)
			}
			if tc.stored != "" && string(got) != tc.stored {
				t.Errorf("stored archive = %q, want %q", got, tc.stored)
			}
		})
	}
}

func TestXDGCachePurgeDeletesTheArchiveBesideTheLayout(t *testing.T) {
	r := newCacheScriptRig(t)
	purge, _ := xdgPhases(t, true)
	writeFile(t, r.xdgStored(), "archive")
	r.writeLayout(filepath.Join(r.store, "bex-build-cache", "tea-w1", "uid-hello", "layer-cache"))
	if out, err := r.run(purge); err != nil {
		t.Fatalf("purge: %v\n%s", err, out)
	}
	if _, err := os.Stat(r.xdgStored()); err == nil {
		t.Error("a clear-cache rebuild left the archive")
	}
	if r.stored("index.json") {
		t.Error("a clear-cache rebuild left the layout")
	}
	if got := r.xdgOutcome(); got != "purged 0" {
		t.Errorf("purge reported %q", got)
	}
	if out, err := r.run(purge, "FAKE_S3_FAIL=rm"); err != nil || r.xdgOutcome() != "error 0" {
		t.Errorf("a refused purge: err=%v outcome=%q\n%s", err, r.xdgOutcome(), out)
	}
}

// --- the buildkit phase's wrapper, against a fake buildctl ---

// fakeBuildctl logs each call and answers the wrapper's three solves:
// FAKE_SEED=unusable makes the seed report an archive it could not unpack,
// FAKE_BUILD_RC fails the image's own solve, FAKE_EXPORT_FAIL leaves a
// partial export behind a failure, and FAKE_EXPORT_SLEEP holds the export
// for that many seconds.
const fakeBuildctl = `#!/bin/sh
printf '%s\n' "$*" >> "$FAKE_BUILDCTL_LOG"
case " $* " in
*" target=bex-xdg-seed "*)
  echo "#5 [bex-xdg-seed 2/2] RUN --mount=type=bind,target=/run/bex-xdg-in"
  if [ "$FAKE_SEED" = unusable ]; then echo "#5 0.120 bex-xdg-seed: unusable"; else echo "#5 0.120 bex-xdg-seed: restored in 4s"; fi
  ;;
*" target=bex-xdg-export "*)
  [ -z "$FAKE_EXPORT_SLEEP" ] || sleep "$FAKE_EXPORT_SLEEP"
  echo partial > "$FAKE_XDG_OUT/` + xdgCacheObject + `"
  [ -z "$FAKE_EXPORT_FAIL" ] || exit 1
  ;;
*" prune "*) ;;
*" build "*)
  echo "#9 the image's solve"
  exit "${FAKE_BUILD_RC:-0}"
  ;;
esac
`

type wrapperRig struct {
	t                   *testing.T
	dir, xin, xout, log string
	bin                 string
}

func newWrapperRig(t *testing.T, dockerfile string) *wrapperRig {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh unavailable")
	}
	dir := t.TempDir()
	r := &wrapperRig{t: t, dir: dir, xin: filepath.Join(dir, "xdg-in"), xout: filepath.Join(dir, "xdg-out"),
		log: filepath.Join(dir, "calls"), bin: filepath.Join(dir, "bin")}
	for _, d := range []string{r.xin, r.xout, r.bin} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(dir, "native", "Dockerfile"), dockerfile)
	for name, body := range map[string]string{"buildctl": fakeBuildctl, "timeout": "#!/bin/sh\nshift\nexec \"$@\"\n"} {
		if err := os.WriteFile(filepath.Join(r.bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return r
}

var wrapperBuildArgs = []string{"build", "--frontend", "dockerfile.v0", "--local", "context=/source", "--output", "type=oci,dest=/output/image.tar"}

// run executes the wrapper as buildctl-daemonless.sh does, with its paths
// retargeted at the rig.
func (r *wrapperRig) run(args []string, env ...string) (string, error) {
	r.t.Helper()
	script := strings.NewReplacer("/native/Dockerfile", filepath.Join(r.dir, "native", "Dockerfile"),
		xdgInMount, r.xin, xdgOutMount, r.xout).Replace(xdgBuildctl)
	_ = os.Remove(r.log)
	cmd := exec.Command("sh", append([]string{"-c", script, "bex-buildctl", "--addr=unix:///run/buildkit/buildkitd.sock"}, args...)...)
	cmd.Env = append([]string{"PATH=" + r.bin + ":/usr/bin:/bin:/usr/sbin:/sbin", "FAKE_BUILDCTL_LOG=" + r.log, "FAKE_XDG_OUT=" + r.xout}, env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *wrapperRig) calls() []string {
	b, _ := os.ReadFile(r.log)
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

func callTarget(call string) string {
	for _, target := range []string{"bex-xdg-seed", "bex-xdg-export"} {
		if strings.Contains(call, " --opt target="+target+" ") {
			return target
		}
	}
	if strings.Contains(call, " prune ") {
		return "prune"
	}
	return "image"
}

const wrapperBase = "golang:1.27-bookworm@sha256:4fa7c968ba93f1635670522dbb01a2c78150443e1385746924566079f36570ea"

func TestXDGBuildctlSeedsAroundTheImagesOwnSolve(t *testing.T) {
	r := newWrapperRig(t, "# syntax=docker/dockerfile:1.7\nFROM "+wrapperBase+"\nRUN true\n")
	writeFile(t, filepath.Join(r.xin, xdgCacheObject), "archive")
	writeFile(t, filepath.Join(r.xin, "transfer"), "1000000 34000000 8\n")
	out, err := r.run(wrapperBuildArgs)
	if err != nil {
		t.Fatalf("wrapper: %v\n%s", err, out)
	}
	calls := r.calls()
	order := make([]string, 0, len(calls))
	for _, c := range calls {
		order = append(order, callTarget(c))
	}
	if !slices.Equal(order, []string{"bex-xdg-seed", "prune", "image", "bex-xdg-export"}) {
		t.Fatalf("solves ran in order %v:\n%s", order, strings.Join(calls, "\n"))
	}
	// The image's solve is exactly the one buildctl-daemonless.sh asked for.
	if want := "--addr=unix:///run/buildkit/buildkitd.sock " + strings.Join(wrapperBuildArgs, " "); calls[2] != want {
		t.Errorf("image solve = %q, want %q", calls[2], want)
	}
	for _, i := range []int{0, 3} {
		if !strings.Contains(calls[i], "build-arg:BEX_XDG_BASE="+wrapperBase+" ") {
			t.Errorf("%s does not run on the build's own base: %s", order[i], calls[i])
		}
	}
	if !strings.Contains(calls[0], "--local context="+r.xin) || !strings.Contains(calls[3], "--output type=local,dest="+r.xout) {
		t.Errorf("seed or export carries the wrong directory:\n%s", strings.Join(calls, "\n"))
	}
	if !strings.Contains(calls[1], "--filter type==source.local") {
		t.Errorf("the seed's copy of the archive is not pruned: %s", calls[1])
	}
	// Render's restore line, the layout and the archive together, and nothing
	// of the seed's own transcript.
	if !strings.Contains(out, rendersShellLine("Transferred 35MB in 8s. Extraction took 4s.")+"\n") || strings.Contains(out, "bex-xdg-seed") {
		t.Errorf("build log:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(r.xin, xdgCacheObject)); err == nil {
		t.Error("the fetched archive outlived its seed")
	}
	if strings.Index(out, "==> Transferred") > strings.Index(out, "the image's solve") {
		t.Errorf("the restore line follows the build's own output:\n%s", out)
	}
}

func TestXDGBuildctlNamesRendersSizes(t *testing.T) {
	for size, want := range map[string]string{
		"512": "512B", "2048": "2.0KB", "5300": "5.3KB", "114000000": "114MB", "580123456": "580MB", "7000000000": "7.0GB",
	} {
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		writeFile(t, filepath.Join(r.xin, xdgCacheObject), "archive")
		writeFile(t, filepath.Join(r.xin, "transfer"), "0 "+size+" 7\n")
		out, err := r.run(wrapperBuildArgs)
		if err != nil || !strings.Contains(out, rendersShellLine("Transferred "+want+" in 7s. Extraction took 4s.")) {
			t.Errorf("%s bytes: %v\n%s", size, err, out)
		}
	}
}

func TestXDGBuildctlFindsTheFinalStagesBase(t *testing.T) {
	// The resolver may put named stages ahead of the final one; the base is
	// the final stage's, an image reference rather than a stage name.
	df := "# syntax=docker/dockerfile:1.7\nFROM node:14-bullseye@sha256:aa AS bex-node-build\nRUN make\n" +
		"FROM scratch AS bex-node-bookworm\nCOPY --link --from=bex-node-build /usr/local/ /usr/local/\n" +
		"FROM " + nativeCommonBase + "\nCOPY --link --from=bex-node-bookworm /usr/local/ /usr/local/\n"
	r := newWrapperRig(t, df)
	if out, err := r.run(wrapperBuildArgs); err != nil {
		t.Fatalf("wrapper: %v\n%s", err, out)
	}
	export := r.calls()[len(r.calls())-1]
	if callTarget(export) != "bex-xdg-export" || !strings.Contains(export, "build-arg:BEX_XDG_BASE="+nativeCommonBase+" ") {
		t.Errorf("export runs on the wrong base: %s", export)
	}
}

func TestXDGBuildctlNeverLetsTheCacheDecideTheBuild(t *testing.T) {
	t.Run("an archive that does not unpack", func(t *testing.T) {
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		writeFile(t, filepath.Join(r.xin, xdgCacheObject), "archive")
		writeFile(t, filepath.Join(r.xin, "transfer"), "0 7 1\n")
		out, err := r.run(wrapperBuildArgs, "FAKE_SEED=unusable")
		if err != nil || !strings.Contains(out, "bex: no usable build cache; building clean") || strings.Contains(out, "==> Transferred") {
			t.Errorf("err=%v\n%s", err, out)
		}
	})
	t.Run("an archive that does not unpack beside a layout", func(t *testing.T) {
		// The layout still serves the build: the line counts it alone.
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		writeFile(t, filepath.Join(r.xin, xdgCacheObject), "archive")
		writeFile(t, filepath.Join(r.xin, "transfer"), "2700000 7 3\n")
		out, err := r.run(wrapperBuildArgs, "FAKE_SEED=unusable")
		if err != nil || !strings.Contains(out, rendersShellLine("Transferred 2.7MB in 3s. Extraction took 0s.")+"\n") || strings.Contains(out, "no usable") {
			t.Errorf("err=%v\n%s", err, out)
		}
	})
	t.Run("a restore that came to nothing", func(t *testing.T) {
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		writeFile(t, filepath.Join(r.xin, "transfer"), "0 0 2\n")
		out, err := r.run(wrapperBuildArgs)
		if err != nil || !strings.Contains(out, "bex: no usable build cache; building clean") || strings.Contains(out, "==> Transferred") {
			t.Errorf("err=%v\n%s", err, out)
		}
	})
	t.Run("a first build", func(t *testing.T) {
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		out, err := r.run(wrapperBuildArgs)
		order := make([]string, 0, len(r.calls()))
		for _, c := range r.calls() {
			order = append(order, callTarget(c))
		}
		if err != nil || !slices.Equal(order, []string{"image", "bex-xdg-export"}) || strings.Contains(out, "==>") {
			t.Errorf("err=%v order=%v\n%s", err, order, out)
		}
	})
	t.Run("a failed build", func(t *testing.T) {
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		_, err := r.run(wrapperBuildArgs, "FAKE_BUILD_RC=7")
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 7 {
			t.Errorf("the image solve's status was not the wrapper's: %v", err)
		}
		if order := r.calls(); len(order) != 1 {
			t.Errorf("a failed build still archived its cache: %v", order)
		}
	})
	t.Run("an export that fails", func(t *testing.T) {
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		if out, err := r.run(wrapperBuildArgs, "FAKE_EXPORT_FAIL=1"); err != nil {
			t.Fatalf("a failed export failed the build: %v\n%s", err, out)
		}
		if entries, _ := os.ReadDir(r.xout); len(entries) != 0 {
			t.Errorf("a partial export was left for cache-save: %v", entries)
		}
	})
	t.Run("anything but a build", func(t *testing.T) {
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		if _, err := r.run([]string{"debug", "workers"}); err != nil || len(r.calls()) != 1 || r.calls()[0] != "--addr=unix:///run/buildkit/buildkitd.sock debug workers" {
			t.Errorf("err=%v calls=%v", err, r.calls())
		}
	})
}

// w33/m571: after the image's solve, the wrapper reports how long its
// archive's solve took, on a line of its own, whether the archive came out
// or not. A build that failed, and anything but a build, report nothing.
func TestXDGBuildctlReportsTheSecondsItsArchiveTook(t *testing.T) {
	line := regexp.MustCompile(`(?m)^` + xdgExportLine + ` ([0-9]+)$`)
	for name, env := range map[string][]string{
		"an archive":           {"FAKE_EXPORT_SLEEP=1"},
		"an export that fails": {"FAKE_EXPORT_SLEEP=1", "FAKE_EXPORT_FAIL=1"},
	} {
		r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
		out, err := r.run(wrapperBuildArgs, env...)
		m := line.FindAllStringSubmatch(out, -1)
		if err != nil || len(m) != 1 {
			t.Fatalf("%s: err=%v, want one seconds line:\n%s", name, err, out)
		}
		if n, _ := strconv.Atoi(m[0][1]); n < 1 || n > 3 {
			t.Errorf("%s: the archive took %ds, want the 1 s its solve was held", name, n)
		}
		if strings.Index(out, xdgExportLine) < strings.Index(out, "the image's solve") {
			t.Errorf("%s: the seconds precede the image's solve:\n%s", name, out)
		}
	}
	r := newWrapperRig(t, "FROM "+wrapperBase+"\n")
	if out, _ := r.run(wrapperBuildArgs, "FAKE_BUILD_RC=7"); strings.Contains(out, xdgExportLine) {
		t.Errorf("a failed build reported an archive:\n%s", out)
	}
	if out, _ := r.run([]string{"debug", "workers"}); strings.Contains(out, xdgExportLine) {
		t.Errorf("a command other than a build reported an archive:\n%s", out)
	}
}

// --- the seed and save steps BuildKit runs, on GNU userland ---

// xdgStep is one of xdgDockerfile's RUN heredocs, its paths retargeted at dir
// and its bound replaced by bound. dir/cache stands for the archive root,
// dir/cache/xdg and dir/cache/deps for the two cache mounts under it.
func xdgStep(t *testing.T, delim, dir string, bound int64) string {
	t.Helper()
	start := strings.Index(xdgDockerfile, "<<'"+delim+"'\n")
	if start < 0 {
		t.Fatalf("no %s step", delim)
	}
	body := xdgDockerfile[start+len("<<'"+delim+"'\n"):]
	body = body[:strings.Index(body, "\n"+delim+"\n")+1]
	return strings.NewReplacer(
		xdgArchiveRoot, filepath.Join(dir, "cache"),
		"/run/bex-xdg-in", filepath.Join(dir, "in"),
		"/bex-xdg-out", filepath.Join(dir, "out"),
		"/tmp/bex-", filepath.Join(dir, "tmp-"),
		strconv.FormatInt(xdgCacheMaxBytes+1, 10), strconv.FormatInt(bound+1, 10),
		xdgMaxBytes, strconv.FormatInt(bound, 10),
	).Replace(body)
}

// gnuUserland skips where tar and find are not GNU's, as on a developer's
// BSD userland; every toolchain image and CI's runner have both.
func gnuUserland(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"tar", "find"} {
		out, err := exec.Command(tool, "--version").Output()
		if err != nil || !bytes.Contains(out, []byte("GNU")) {
			t.Skipf("%s is not GNU's", tool)
		}
	}
}

// runStep runs a step as BuildKit would, with env as the stage's ARGs.
func runStep(t *testing.T, script string, env ...string) string {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("step failed: %v\n%s", err, out)
	}
	return string(out)
}

func tarGz(t *testing.T, entries []tar.Header, data map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		h.Size = int64(len(data[h.Name]))
		if h.Mode == 0 {
			h.Mode = 0o644
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(data[h.Name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// archiveMounts makes dir/cache/xdg and dir/cache/deps, the two cache mounts
// the seed and save steps find under the archive root, and returns the first.
func archiveMounts(t *testing.T, dir string) string {
	t.Helper()
	for _, m := range []string{xdgArchiveXDG, xdgArchiveDeps} {
		if err := os.MkdirAll(filepath.Join(dir, "cache", m), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "cache", xdgArchiveXDG)
}

func TestXDGSaveAndSeedRoundTripTheDirectory(t *testing.T) {
	gnuUserland(t)
	dir := t.TempDir()
	cache := archiveMounts(t, dir)
	writeFile(t, filepath.Join(cache, "pip", "http", "wheel"), "wheel bytes")
	writeFile(t, filepath.Join(cache, ".hidden"), "dotfile")
	if err := os.Symlink("/etc/passwd", filepath.Join(cache, "pip", "link")); err != nil {
		t.Fatal(err)
	}
	runStep(t, xdgStep(t, "BEX_XDG_SAVE", dir, xdgCacheMaxBytes))
	if entries, _ := os.ReadDir(cache); len(entries) != 0 {
		t.Errorf("the archived files still sit on BuildKit's disk: %v", entries)
	}
	if err := os.Rename(filepath.Join(dir, "out"), filepath.Join(dir, "in")); err != nil {
		t.Fatalf("save left no archive: %v", err)
	}
	out := runStep(t, xdgStep(t, "BEX_XDG_SEED", dir, xdgCacheMaxBytes))
	if !strings.Contains(out, "bex-xdg-seed: restored in ") {
		t.Fatalf("seed: %s", out)
	}
	if got, _ := os.ReadFile(filepath.Join(cache, "pip", "http", "wheel")); string(got) != "wheel bytes" {
		t.Errorf("restored wheel = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(cache, ".hidden")); string(got) != "dotfile" {
		t.Errorf("restored dotfile = %q", got)
	}
	// A link stays a link: neither step reads through it.
	if target, err := os.Readlink(filepath.Join(cache, "pip", "link")); err != nil || target != "/etc/passwd" {
		t.Errorf("restored link -> %q, %v", target, err)
	}
}

func TestXDGSaveKeepsNothingEmptyOrOverTheBound(t *testing.T) {
	gnuUserland(t)
	for name, tc := range map[string]struct {
		files map[string]string
		bound int64
		want  string
	}{
		"empty":          {files: nil, bound: xdgCacheMaxBytes, want: "0"},
		"over the bound": {files: map[string]string{"big": strings.Repeat("x", 4096)}, bound: 1024},
		// Within the bound as two directories (du), over it as a tar stream.
		"archive over its bound": {files: map[string]string{"a": "x", "b": "y"}, bound: 9000},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			cache := archiveMounts(t, dir)
			for f, body := range tc.files {
				writeFile(t, filepath.Join(cache, f), body)
			}
			runStep(t, xdgStep(t, "BEX_XDG_SAVE", dir, tc.bound))
			if _, err := os.Stat(filepath.Join(dir, "out", xdgCacheObject)); err == nil {
				t.Error("an archive was kept")
			}
			marker, err := os.ReadFile(filepath.Join(dir, "out", "skipped"))
			if err != nil {
				t.Fatalf("no skipped marker: %v", err)
			}
			if tc.want != "" && strings.TrimSpace(string(marker)) != tc.want {
				t.Errorf("marker = %q, want %q", marker, tc.want)
			}
		})
	}
}

func TestXDGSeedKeepsNothingOfAnArchiveThatDoesNotUnpackCleanly(t *testing.T) {
	gnuUserland(t)
	const key = "k-x86_64"
	good := []tar.Header{{Name: "xdg/ok", Typeflag: tar.TypeReg}, {Name: "deps/ok", Typeflag: tar.TypeReg}, {Name: xdgArchiveDepsKey, Typeflag: tar.TypeReg}}
	fine := map[string]string{"xdg/ok": "fine", "deps/ok": "fine", xdgArchiveDepsKey: key + "\n"}
	valid := tarGz(t, good, fine)
	corruptCRC := slices.Clone(valid)
	corruptCRC[len(corruptCRC)-6] ^= 0xff // gzip's CRC-32 trailer
	with := func(extra []tar.Header, data map[string]string) []byte {
		maps.Copy(data, fine)
		return tarGz(t, append(slices.Clone(good), extra...), data)
	}
	for name, archive := range map[string][]byte{
		"not an archive":      []byte("garbage that is no gzip stream"),
		"gzip's check fails":  corruptCRC,
		"a member climbs out": with([]tar.Header{{Name: "../escape", Typeflag: tar.TypeReg}}, map[string]string{"../escape": "pwn"}),
		"a link is written through": with([]tar.Header{{Name: "xdg/link", Typeflag: tar.TypeSymlink, Linkname: "../../outside"},
			{Name: "xdg/link/pwned", Typeflag: tar.TypeReg}}, map[string]string{"xdg/link/pwned": "pwn"}),
		"more than the bound unpacks": with([]tar.Header{{Name: "deps/bomb", Typeflag: tar.TypeReg}},
			map[string]string{"deps/bomb": strings.Repeat("\x00", 64<<10)}),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, "in", xdgCacheObject), string(archive))
			archiveMounts(t, dir)
			out := runStep(t, xdgStep(t, "BEX_XDG_SEED", dir, 32<<10), "BEX_DEPS_KEY="+key)
			if !strings.Contains(out, "bex-xdg-seed: unusable") {
				t.Errorf("seed accepted it: %s", out)
			}
			for _, m := range []string{xdgArchiveXDG, xdgArchiveDeps} {
				if entries, _ := os.ReadDir(filepath.Join(dir, "cache", m)); len(entries) != 0 {
					t.Errorf("an unusable archive left %v behind in %s", entries, m)
				}
			}
			for _, outside := range []string{"escape", filepath.Join("outside", "pwned")} {
				if _, err := os.Stat(filepath.Join(dir, outside)); err == nil {
					t.Errorf("a member was written outside the cache: %s", outside)
				}
			}
		})
	}
	// The same archive, whole, within the bound, is restored, both halves.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "in", xdgCacheObject), string(valid))
	archiveMounts(t, dir)
	if out := runStep(t, xdgStep(t, "BEX_XDG_SEED", dir, 32<<10), "BEX_DEPS_KEY="+key); !strings.Contains(out, "restored in") {
		t.Errorf("a valid archive was refused: %s", out)
	}
	for _, m := range []string{xdgArchiveXDG, xdgArchiveDeps} {
		if got, _ := os.ReadFile(filepath.Join(dir, "cache", m, "ok")); string(got) != "fine" {
			t.Errorf("%s/ok = %q", m, got)
		}
	}
}
