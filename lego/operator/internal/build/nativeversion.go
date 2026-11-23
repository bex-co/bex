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
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"

	"github.com/bex-co/bex/lego/types/renderlog"
	"github.com/bex-co/bex/lego/types/toolchains"
)

// NativeChoice is the toolchain line a native build runs on and why (w8/m51).
type NativeChoice struct {
	Label  string // "Python"
	Line   string // "3.11"
	Image  string // the pinned image of that line
	Source string // "PYTHON_VERSION=3.11.9", ".nvmrc", or "" for the default
	// declined, when set, says why an optional companion was left out; the
	// choice then carries no line or image.
	declined string
	// Exact is the fully qualified version the service pinned within this line
	// when it differs from the line's reviewed patch (w8/175); the resolve phase
	// swaps in that version's official image, or says why it could not.
	Exact    string
	upstream string
	// unreviewed: Exact is a release on no reviewed line (an anyRelease
	// toolchain's), so the resolve phase installs it or refuses it by name;
	// it never builds on another release (w8/372).
	unreviewed bool
	// releaseLine is the released line no reviewed image carries (Node 21)
	// whose newest release inside rangeRequest the resolve phase installs,
	// when that line's final release falls outside the range (w8/398).
	releaseLine string
	// erlang is the Erlang/OTP release an Elixir service pinned more exactly
	// than its major (ERLANG_VERSION=28.3, w8/197); the resolve phase builds on
	// an image carrying it, or fails by name.
	erlang string
	// otp is the Erlang/OTP release the chosen image installs, once the
	// resolve phase has pinned an exact Erlang; the log names it.
	otp string
	// elixirImage is the image an exact Erlang's Elixir is copied from.
	elixirImage string
	// Common assembly may take the same runtime from a compatible donor or
	// cached source stage. Image remains the selected upstream identity.
	primaryImage  string
	primaryStage  string
	elixirVersion string
	commonOTP     string
	legacySource  bool
	runtimeCheck  string
	// rangeRequest is the semver range a Node service asked for when its
	// line's reviewed patch falls outside it (`~16.13.0` against 16.20.2):
	// the resolve phase pins the newest official release inside it, as Render
	// installs one (w8/307).
	rangeRequest string
	// nodeDonor is the official image of the exact Node release in an older
	// Debian variant, when the line's own variant has none (16.13.2 has
	// bullseye images, not bookworm ones): its node binary and global
	// node_modules are copied onto the line's image (w8/309).
	nodeDonor string
	// note is an extra build-log line (an exact version that fell back to the
	// line's reviewed patch).
	note string
	// imageEnv preserves the selected primary's image environment when its
	// installation is copied onto the common base instead of inheriting its FROM.
	imageEnv []string
	// narratedVersion is the version the log names when it is not the line: the
	// Rust toolchain rustup applies inside the line's one image (w8/327).
	narratedVersion string
	// lineVersion is the chosen line's reviewed patch: what the build runs
	// when no exact version replaces it, so the log names it (w4/668).
	lineVersion string
	// requested is the Node range or alias the service asked for, as it wrote
	// it, which Render's log echoes before the version line (w9/141).
	requested string
}

// Narration is NarrationIn for a service whose source sits at NativeWorkdir,
// one with no root directory.
func (c NativeChoice) Narration() string { return c.NarrationIn(NativeWorkdir) }

// NarrationIn is the build-log lines that name the choice and its source, in
// Render's order: the Node request, the version line, the runtime's docs line,
// then bex's note. dir is the service's source directory in the image, which
// Render's line names a version file under (NativeSourceDir).
func (c NativeChoice) NarrationIn(dir string) string {
	if c.declined != "" {
		return fmt.Sprintf("==> Not adding %s: %s", c.Label, c.declined)
	}
	version := c.Line
	if c.narratedVersion != "" {
		version = c.narratedVersion
	} else if strings.HasPrefix(c.lineVersion, c.Line+".") {
		// The line's reviewed patch, not the bare line: Render's log names
		// the exact interpreter (w4/668).
		version = c.lineVersion
	}
	line, ok := c.renderNarration(version, dir)
	if !ok {
		line = c.bexNarration(version)
	}
	var lines []string
	if c.requested != "" {
		lines = append(lines, "==> Requesting Node.js version "+c.requested)
	}
	lines = append(lines, line)
	if docs := renderDocs[c.Label]; docs != "" {
		lines = append(lines, renderFormOf(c.Label).Line("Docs on specifying "+docs))
	}
	if c.note != "" {
		lines = append(lines, c.note)
	}
	return strings.Join(lines, "\n")
}

// renderDocs is the page Render's build log points to after each runtime's
// version line, `==> Docs on specifying a Ruby version:
// https://render.com/docs/ruby-version` (w9/141; the logs are quoted in
// docs/render-artifacts/native-version-narration.md). The line follows the
// runtime's version line whatever the signal, so it follows bex's own lines
// too. Render's Go line has none, and no Rust line is on record.
var renderDocs = map[string]string{
	"Node":   "a Node.js version: https://render.com/docs/node-version",
	"Python": "a Python version: https://render.com/docs/python-version",
	"Ruby":   "a Ruby version: https://render.com/docs/ruby-version",
	"Elixir": "Elixir and Erlang versions: https://render.com/docs/elixir-erlang-versions",
	"Bun":    "a Bun version: https://render.com/docs/bun-version",
	"Poetry": "a Poetry version: https://render.com/docs/poetry-version",
	"uv":     "a uv version: https://render.com/docs/uv-version",
}

// renderSignalFrom is how Render's build log says where a version came from.
type renderSignalFrom int

const (
	renderFromDefault renderSignalFrom = iota // `(default)`
	renderFromEnv                             // `via environment variable NODE_VERSION`
	renderFromFile                            // `via /opt/render/project/src/.nvmrc`
)

// renderSignals are the signals, per toolchain label, whose line Render's own
// build logs show (w9/139; the logs are quoted in
// docs/render-artifacts/native-version-narration.md). A signal is "" for no
// signal, an environment variable's name, or a version file's name. A
// lockfile that only opts a companion in names no version, so Render's line
// says its default. A signal missing here has no Render log on record and
// keeps bex's own line (bexNarration).
var renderSignals = map[string]map[string]renderSignalFrom{
	"Node": {"": renderFromDefault, "NODE_VERSION": renderFromEnv,
		".node-version": renderFromFile, ".nvmrc": renderFromFile, "package.json": renderFromFile},
	"Python": {"": renderFromDefault, "PYTHON_VERSION": renderFromEnv, ".python-version": renderFromFile},
	"Ruby":   {"": renderFromDefault, "Gemfile.lock": renderFromFile, ".ruby-version": renderFromFile, "Gemfile": renderFromFile},
	"Elixir": {"": renderFromDefault},
	"Bun":    {"bun.lock": renderFromDefault, "bun.lockb": renderFromDefault},
	"Poetry": {"": renderFromDefault},
	"uv":     {"uv.lock": renderFromDefault},
}

// renderNames are Render's names for the toolchains whose label differs.
var renderNames = map[string]string{"Node": "Node.js"}

// renderFormOf is the form Render's version and docs lines for a toolchain
// label take (w32/m576): Node.js's are in Render's blue runtime form, as
// ivanseibel/tretta's capture of Render's log API shows `==> Using Node.js
// version 22.23.1 via environment variable NODE_VERSION` and its docs line.
// No capture shows another runtime's with its codes, so those stay plain
// until w32/085's does, as do the Node request line and bex's own lines.
func renderFormOf(label string) renderlog.Form {
	if label == nativeToolchains[nativeNodeRuntime].label {
		return renderlog.BuildRuntime
	}
	return renderlog.Plain
}

// renderNarration is the choice's line as Render's build log words it, in
// its form (renderFormOf): `==> Using Ruby version 3.4.1 via
// /opt/render/project/src/Gemfile.lock`. ok is false for a signal Render's
// logs do not show. Render names Erlang on a line of its own, before
// Elixir's.
func (c NativeChoice) renderNarration(version, dir string) (line string, ok bool) {
	signal, engines := strings.CutSuffix(c.Source, enginesNodeSuffix) // "backend/package.json engines.node"
	if !engines {
		signal, _, _ = strings.Cut(signal, "=")
	}
	// A package.json below the root (w8/399) keys its signal by file name, and
	// the log names its path.
	from, ok := renderSignals[c.Label][signal[strings.LastIndex(signal, "/")+1:]]
	if !ok {
		return "", false
	}
	how := "(default)"
	switch from {
	case renderFromEnv:
		how = "via environment variable " + signal
	case renderFromFile:
		how = "via " + path.Join(dir, signal)
	}
	f := renderFormOf(c.Label)
	line = f.Line(fmt.Sprintf("Using %s version %s %s", cmp.Or(renderNames[c.Label], c.Label), version, how))
	if c.otp != "" {
		line = f.Line(fmt.Sprintf("Using Erlang version %s %s", c.otp, how)) + "\n" + line
	}
	return line, true
}

// bexNarration is bex's own line for a signal no Render log shows:
// `==> Using Bun 1.3.14 (from BUN_VERSION=1.3)`.
func (c NativeChoice) bexNarration(version string) string {
	from := "default"
	if c.Source != "" {
		from = "from " + c.Source
	}
	if c.otp != "" {
		version += " on Erlang/OTP " + c.otp
	}
	return fmt.Sprintf("==> Using %s %s (%s)", c.Label, version, from)
}

// UnsupportedVersionError is a version request no reviewed line satisfies. It
// is tenant input: the build fails on it by name instead of silently building
// on the default line.
type UnsupportedVersionError = toolchains.UnsupportedVersionError

// ResolveNativeVersion picks a runtime's toolchain line from the service's own
// version signals, with Render's precedence per runtime (docs/ADR060 D7
// addendum, w8/m51):
//
//	python  PYTHON_VERSION → .python-version → default (toolchains.ResolvePython, which agent sessions share)
//	node    NODE_VERSION → .node-version → .nvmrc → package.json engines.node (nodePackageJSON's, w8/399) → default
//	ruby    Gemfile.lock / gems.locked RUBY VERSION → .ruby-version → .tool-versions → Gemfile `ruby` → default
//	elixir  ELIXIR_VERSION (→ line, or any release from 1.12.0 on no line, w8/373), ERLANG_VERSION (any Erlang/OTP from 24.3.4 Elixir's table pairs with it; one the line's image does not carry is installed exactly, w8/197, w8/373); with neither, Render's dated default Elixir on its default OTP (w8/333)
//	go      default (Render offers no Go selection)
//	rust    the one line; RUSTUP_TOOLCHAIN → rust-toolchain → rust-toolchain.toml name the toolchain rustup applies (w8/327)
//
// A plain version maps onto its line by major.minor (major for Node), and a
// fully qualified one is then installed exactly (exactRequest, w8/175). A
// range (Node engines, Ruby requirements) resolves to the highest line whose
// pinned toolchain satisfies it. A Node request no reviewed line serves (an
// odd line's, a line's before 14, or a release no line carries) resolves to
// the release Render's node-version-alias picks, which the resolve phase
// installs or refuses by name (w8/398, w8/401). env is the build's effective environment;
// readFile reads a file relative to the service's root directory, whose
// package.json, if any, is Node's.
func ResolveNativeVersion(runtime string, env map[string]string, readFile func(string) ([]byte, bool)) (NativeChoice, error) {
	return ResolveNativeVersionAt(time.Time{}, runtime, env, readFile, packageJSON)
}

// rubyDefaultCutoff: Render's default Ruby for services created from 2025-06-12
// to 2026-11-03 is exactly 3.4.4, not the newest 3.4 patch (w8/198). bex
// applied it to services created after its own change, so a running service's
// toolchain did not move under that operator upgrade (the
// toolchains.PythonCreationCutoff pattern).
var rubyDefaultCutoff = time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC)

// rubyDefaultVersion is the default from bex's own cutoff until Render's
// 2026-11-03 move (toolchains.RenderDefaults20261103).
const rubyDefaultVersion = "3.4.4"

// renderRubyDefault20261119 is when Render moved its default Ruby to 3.4.13
// for new services (render.com/docs/ruby-version, w9/138), past the 3.4 line's
// reviewed 3.4.12: the resolve phase swaps in the official 3.4.13 image.
var renderRubyDefault20261119 = time.Date(2026, time.November, 19, 0, 0, 0, 0, time.UTC)

// renderNodeDefault20261021 and renderElixirDefault20250612 are when Render's
// current Node (24.22.1, render.com/docs/node-version) and Elixir (1.18.4,
// render.com/docs/elixir-erlang-versions) defaults took effect.
var (
	renderNodeDefault20261021   = time.Date(2026, time.October, 21, 0, 0, 0, 0, time.UTC)
	renderElixirDefault20250612 = time.Date(2025, time.June, 12, 0, 0, 0, 0, time.UTC)
)

// ResolveNativeVersionAt is ResolveNativeVersion for a service created at
// created, whose creation date picks the default line where Render's does, and
// whose Node reads engines.node from nodePackage (nodePackageJSON).
func ResolveNativeVersionAt(created time.Time, runtime string, env map[string]string, readFile func(string) ([]byte, bool), nodePackage string) (NativeChoice, error) {
	choice, err := resolveNativeLine(created, runtime, env, readFile, nodePackage)
	if err != nil {
		return choice, err
	}
	if line, ok := nativeToolchains[runtime].find(choice.Line); ok {
		choice.lineVersion = line.version
	}
	if choice.Exact == "" && !choice.unreviewed { // a release no line carries is already exact, or a range's to list
		choice.Exact = exactRequest(nativeToolchains[runtime], choice, readFile)
	}
	if choice.Exact == "" {
		choice.Exact = defaultExact(created, runtime, choice)
	}
	if runtime == nativeNodeRuntime {
		request := nodeRequestText(choice.Source, readFile)
		if choice.Exact == "" {
			// A request left on an odd line (choice.releaseLine) is always
			// one: no line's newest release met it, the default line's
			// standing in for that line included (w8/398).
			choice.rangeRequest = excludedNodeRange(choice, request)
		}
		choice.requested = renderNodeRequest(request)
	}
	if runtime == nativeElixirRuntime {
		return pairElixirOTP(created, choice, env)
	}
	return choice, nil
}

// erlangDefaultCutoff: Render's default Erlang/OTP for services created since
// 2025-06-12 is exactly 28.0.2, under its default Elixir 1.18.4
// (render.com/docs/elixir-erlang-versions), not the OTP the official elixir
// image was built on (w8/333). bex applies it to services created from this
// change on, so a running service's OTP never moves under the operator upgrade
// (the toolchains.PythonCreationCutoff pattern).
var erlangDefaultCutoff = time.Date(2026, time.November, 16, 0, 0, 0, 0, time.UTC)

// erlangDefaults are Render's default Erlang/OTP releases by creation date,
// oldest first, as exactDefaults are for the runtimes. Each is written in the
// four-part form that names exactly one release (otpTag), so a later
// maintenance patch (28.0.2.1) never moves a default.
var erlangDefaults = []toolchains.DatedDefault{{From: erlangDefaultCutoff, Version: "28.0.2.0"}}

// defaultErlang is the exact Erlang/OTP an Elixir service with neither
// ELIXIR_VERSION nor ERLANG_VERSION builds on: the resolve phase installs it as
// it does an ERLANG_VERSION of that release (w8/197). A release outside the
// chosen line's OTP major is never applied.
func defaultErlang(created time.Time, choice NativeChoice) string {
	release := toolchains.DatedDefaultAt(erlangDefaults, created)
	line, _ := nativeToolchains[nativeElixirRuntime].find(choice.Line)
	if release == "" || !strings.HasPrefix(release, line.otp+".") {
		return ""
	}
	return release
}

// exactDefaults are Render's default patches by creation date, oldest first
// (w8/198, w8/217, w4/620, w6/m291, w9/138). The versions stay exact even
// where a step equals the line's reviewed patch, so a later line-pin bump never
// moves a default (the w8/217 regression class). Python's are
// toolchains.PythonDefaults, which toolchains.ResolvePython applies.
var exactDefaults = map[string][]toolchains.DatedDefault{
	nativeNodeRuntime:   {{From: renderNodeDefault20261021, Version: "24.22.1"}},
	nativeElixirRuntime: {{From: renderElixirDefault20250612, Version: "1.18.4"}},
	nativeRubyRuntime: {
		{From: rubyDefaultCutoff, Version: rubyDefaultVersion},
		{From: toolchains.RenderDefaults20261103, Version: "3.4.12"},
		{From: renderRubyDefault20261119, Version: "3.4.13"},
	},
}

// defaultExact is the exact version a service with no version signal builds on:
// the last step of its runtime's exactDefaults dated at or before its creation.
// The resolve phase swaps in that version's official image and falls back to
// the line's reviewed patch, saying so, when it cannot. A default equal to the
// reviewed patch needs no swap.
func defaultExact(created time.Time, runtime string, choice NativeChoice) string {
	// ERLANG_VERSION alone names no Elixir: Render installs its default Elixir
	// on the requested OTP.
	if choice.Source != "" && !strings.HasPrefix(choice.Source, "ERLANG_VERSION=") {
		return ""
	}
	version := toolchains.DatedDefaultAt(exactDefaults[runtime], created)
	if version == "" || !strings.HasPrefix(version, choice.Line+".") {
		return ""
	}
	if line, _ := nativeToolchains[runtime].find(choice.Line); line.version == version {
		return ""
	}
	return version
}

func resolveNativeLine(created time.Time, runtime string, env map[string]string, readFile func(string) ([]byte, bool), nodePackage string) (NativeChoice, error) {
	t, ok := nativeToolchains[runtime]
	if !ok {
		return NativeChoice{}, fmt.Errorf("unsupported native runtime %q", runtime)
	}
	r := resolver{t: t, env: env, readFile: readFile, nodePackage: nodePackage}
	switch runtime {
	case nativePythonRuntime:
		python, err := toolchains.ResolvePython(created, env, readFile)
		if err != nil {
			return NativeChoice{}, err
		}
		choice, err := r.choice(python.Line, python.Source)
		choice.Exact = python.Exact
		return choice, err
	case nativeNodeRuntime:
		return r.first(r.fromEnv("NODE_VERSION", r.nodeRequest), r.fromFile(".node-version", r.nodeRequest),
			r.fromFile(".nvmrc", r.nodeRequest), r.fromPackageEngines)
	case nativeRubyRuntime:
		return r.first(r.fromLockedRuby, r.fromFile(".ruby-version", r.rubyFileLine), r.fromToolVersions("ruby", r.rubyFileLine), r.fromGemfile)
	case "elixir":
		return r.elixir()
	case nativeRustRuntime:
		return r.rustToolchain()
	default:
		return r.choice(t.defaultLine, "")
	}
}

// rustToolchain keeps Rust's one image and line and names the toolchain rustup
// applies inside it, in Render's precedence (render.com/docs/rust-toolchain):
// RUSTUP_TOOLCHAIN, else the root toolchain file rustup reads, rust-toolchain
// before rust-toolchain.toml. A request is named as written; rustup resolves
// or refuses it. With no signal the log names the pinned release (w8/327).
func (r resolver) rustToolchain() (NativeChoice, error) {
	choice, err := r.choice(r.t.defaultLine, "")
	if err != nil {
		return choice, err
	}
	line, _ := r.t.find(choice.Line)
	choice.narratedVersion = line.version
	if request := strings.TrimSpace(r.env["RUSTUP_TOOLCHAIN"]); request != "" {
		request = toolchains.BoundedRequest(request)
		choice.Source, choice.narratedVersion = "RUSTUP_TOOLCHAIN="+request, request
		return choice, nil
	}
	for _, name := range [...]string{"rust-toolchain", "rust-toolchain.toml"} {
		if raw, ok := r.readFile(name); ok {
			if channel := rustToolchainChannel(raw); channel != "" {
				choice.Source, choice.narratedVersion = name, toolchains.BoundedRequest(channel)
			}
			break // rustup reads only the first file present
		}
	}
	return choice, nil
}

// rustToolchainTOML is a toolchain file's `channel = "…"` setting, in whatever
// section it appears.
var rustToolchainTOML = regexp.MustCompile(`(?m)^\s*channel\s*=\s*["']([^"']+)["']`)

// rustToolchainChannel is the channel a rust-toolchain file names: its first
// line in the legacy one-line form, or the TOML form's channel setting.
func rustToolchainChannel(raw []byte) string {
	first := toolchains.FirstMeaningfulLine(raw)
	if !strings.HasPrefix(first, "[") {
		return first
	}
	if m := rustToolchainTOML.FindSubmatch(raw); m != nil {
		return strings.TrimSpace(string(m[1]))
	}
	return ""
}

// ResolveNativeBun picks the Bun companion a native build adds, with Render's
// precedence: BUN_VERSION → .bun-version → a bun.lock or bun.lockb lockfile.
// found is false when the service does not opt in, which leaves the build
// exactly as before (w8/m71). A version maps onto its major.minor line, as the
// primary toolchains' do, and a fully qualified one is installed exactly, as
// Render does (w8/315), on a reviewed line or not (w8/372). A lockfile alone
// names no version, so a service created at created gets Render's default for
// that date (w8/371).
func ResolveNativeBun(created time.Time, env map[string]string, readFile func(string) ([]byte, bool)) (choice NativeChoice, found bool, err error) {
	r := resolver{t: nativeBun, env: env, readFile: readFile}
	choice, found, err = r.firstFound(r.fromEnv("BUN_VERSION", r.bunRequest), r.fromFile(".bun-version", r.bunRequest), r.fromLockfile(nativeBunLockfiles...))
	if !found {
		return choice, found, err
	}
	if slices.Contains(nativeBunLockfiles, choice.Source) {
		choice = r.datedBunDefault(created, choice)
	} else if choice.Exact == "" {
		choice.Exact = exactRequest(nativeBun, choice, readFile)
	}
	if line, ok := nativeBun.find(choice.Line); ok {
		choice.lineVersion = line.version
	}
	return choice, found, nil
}

// bunDefaults are Render's default Bun releases by service creation date,
// oldest first (render.com/docs/bun-version, read 2026-11-21): what a service
// that opts in by its lockfile alone builds on (w8/371). The first step is
// Render's "Before 2025-08-18", which no reviewed line carries: it is
// installed exactly from its own image (w8/372).
var bunDefaults = []toolchains.DatedDefault{
	{From: time.Unix(0, 0).UTC(), Version: "1.1.0"},
	{From: time.Date(2025, time.August, 18, 0, 0, 0, 0, time.UTC), Version: "1.2.20"},
	{From: time.Date(2025, time.December, 8, 0, 0, 0, 0, time.UTC), Version: "1.3.4"},
	{From: time.Date(2026, time.July, 16, 0, 0, 0, 0, time.UTC), Version: "1.3.14"},
	{From: time.Date(2026, time.September, 9, 0, 0, 0, 0, time.UTC), Version: "1.4.2"},
	{From: time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC), Version: "1.4.3"},
	{From: time.Date(2026, time.October, 22, 0, 0, 0, 0, time.UTC), Version: "1.4.4"},
	{From: time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC), Version: "1.4.5"},
}

// datedBunDefault moves a lockfile-only Bun choice onto Render's default for a
// service created at created: that release's line, installed exactly when it
// is not the line's reviewed patch. A default on no reviewed line (1.1.0) is
// installed exactly on the default line's image, which it falls back to, as
// every default does, when its own cannot be had. A service of unknown age
// keeps the default line's reviewed patch.
func (r resolver) datedBunDefault(created time.Time, choice NativeChoice) NativeChoice {
	version := toolchains.DatedDefaultAt(bunDefaults, created)
	if version == "" {
		return choice
	}
	line, ok := r.minorLine(version)
	if !ok {
		choice.Exact = version
		return choice
	}
	dated, _ := r.choice(line, choice.Source) // minorLine found the line
	if reviewed, _ := r.t.find(line); reviewed.version != version {
		dated.Exact = version
	}
	return dated
}

// nativeBunCanary is Render's BUN_VERSION / .bun-version value for Bun's
// canary build (Render changelog 2026-10-16).
const nativeBunCanary = "canary"

// nativeBunLockfiles are Bun's lockfiles, which opt a build into Bun and mark
// a project as Bun's rather than npm's (w8/m86).
var nativeBunLockfiles = []string{"bun.lock", "bun.lockb"}

// defaultPoetryVersion is the Poetry a native Python build carries when the
// service names none — Render's current default for new services (its
// Poetry version contract, w8/m72).
const defaultPoetryVersion = "2.1.3"

// poetryRelease is an exact Poetry 1.x/2.x release, the form Render's
// POETRY_VERSION takes ("any version number"); ranges are not resolved.
var poetryRelease = regexp.MustCompile(`^[12]\.\d+\.\d+$`)

// ResolveNativePoetry picks the Poetry every native Python build carries:
// POETRY_VERSION when set, else the default. Unlike the primary toolchains it
// is an exact release installed from PyPI rather than a pinned image line —
// Poetry publishes no reviewed image — so Line is the version itself.
// Explicit versions retain Poetry's compatibility requirement: an incompatible
// pair fails the install step by name. Common assembly bootstraps the modern
// default independently when an old application Python cannot install it.
func ResolveNativePoetry(env map[string]string) (NativeChoice, error) {
	request := strings.TrimSpace(env["POETRY_VERSION"])
	if request == "" {
		return NativeChoice{Label: "Poetry", Line: defaultPoetryVersion}, nil
	}
	version := strings.TrimPrefix(strings.ToLower(request), "v")
	if !poetryRelease.MatchString(version) {
		return NativeChoice{}, &UnsupportedVersionError{Msg: fmt.Sprintf(
			"%s is not a supported Poetry version; set an exact Poetry 1.x or 2.x release such as %s", toolchains.BoundedRequest("POETRY_VERSION="+request), defaultPoetryVersion)}
	}
	return NativeChoice{Label: "Poetry", Line: version, Source: "POETRY_VERSION=" + request}, nil
}

// defaultUVVersion is Render's default uv for services created since
// 2026-11-03 (render.com/docs/uv-version, w8/328); legacyUVVersion is its
// default when w8/m81 shipped.
const (
	defaultUVVersion = "0.13.7"
	legacyUVVersion  = "0.12.10"
)

// uvDefaultCutoff: bex applies defaultUVVersion to services created after this
// change, not from Render's 2026-11-03, so a running service's toolchain never
// moves under an operator upgrade (the toolchains.PythonCreationCutoff pattern).
var uvDefaultCutoff = time.Date(2026, time.November, 14, 0, 0, 0, 0, time.UTC)

// uvDefault is the uv a service created at created gets without UV_VERSION. A
// zero created (unknown) keeps the legacy default.
func uvDefault(created time.Time) string {
	if !created.IsZero() && !created.Before(uvDefaultCutoff) {
		return defaultUVVersion
	}
	return legacyUVVersion
}

// uvRelease is an exact uv release, the form Render's UV_VERSION takes ("any
// version number"); ranges are not resolved.
var uvRelease = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// ResolveNativeUV picks the uv a native Python build adds. As on Render, a
// uv.lock in the service's root directory opts in, and UV_VERSION then picks
// the release (else the default for a service created at created); without the
// lockfile the build is exactly as before and UV_VERSION is not read (w8/m81).
// Like Poetry it is an exact PyPI release, so Line is the version itself, and
// an incompatible Python fails the install step by name.
func ResolveNativeUV(created time.Time, env map[string]string, readFile func(string) ([]byte, bool)) (choice NativeChoice, found bool, err error) {
	if _, locked := readFile("uv.lock"); !locked {
		return NativeChoice{}, false, nil
	}
	request := strings.TrimSpace(env["UV_VERSION"])
	if request == "" {
		return NativeChoice{Label: "uv", Line: uvDefault(created), Source: "uv.lock"}, true, nil
	}
	version := strings.TrimPrefix(strings.ToLower(request), "v")
	if !uvRelease.MatchString(version) {
		return NativeChoice{}, false, &UnsupportedVersionError{Msg: fmt.Sprintf(
			"%s is not a supported uv version; set an exact uv release such as %s", toolchains.BoundedRequest("UV_VERSION="+request), defaultUVVersion)}
	}
	return NativeChoice{Label: "uv", Line: version, Source: "UV_VERSION=" + request}, true, nil
}

// bunRequest maps a Bun version request: `canary`, `latest`, a plain version onto its
// major.minor line (a bare major onto that major's newest line), otherwise a
// semver range. A fully qualified release on no reviewed line is returned as
// itself: Render installs any exact Bun release (w8/372), and r.choice starts
// it on the default line's image until the resolve phase pins its own.
func (r resolver) bunRequest(request string) (string, bool) {
	lower := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(request)), "bun-"), "v")
	switch {
	case lower == r.t.floating:
		return lower, true
	case lower == "latest":
		return r.t.lines[len(r.t.lines)-1].line, true
	case plainVersion.MatchString(lower) && !strings.Contains(lower, "."):
		return r.highestSatisfying(lower + ".x")
	case plainVersion.MatchString(lower):
		if line, ok := r.minorLine(lower); ok || !fullVersion.MatchString(lower) {
			return line, ok
		}
		return lower, true
	}
	return r.highestSatisfying(lower)
}

// fromLockfile opts in on the first lockfile present, on the default line.
func (r resolver) fromLockfile(names ...string) signal {
	return func() (string, string, bool, error) {
		for _, name := range names {
			if _, ok := r.readFile(name); ok {
				return r.t.defaultLine, name, true, nil
			}
		}
		return "", "", false, nil
	}
}

type resolver struct {
	t        nativeToolchain
	env      map[string]string
	readFile func(string) ([]byte, bool)
	// nodePackage is the package.json Node reads engines.node from, relative
	// to the service's root directory, or "" for none (nodePackageJSON).
	nodePackage string
}

// signal returns (line, source, found, err): found=false means "no signal here,
// try the next one".
type signal func() (line, source string, found bool, err error)

func (r resolver) first(signals ...signal) (NativeChoice, error) {
	if choice, found, err := r.firstFound(signals...); found || err != nil {
		return choice, err
	}
	return r.choice(r.t.defaultLine, "")
}

// firstFound is the first signal that is present; found is false when none is.
func (r resolver) firstFound(signals ...signal) (NativeChoice, bool, error) {
	for _, s := range signals {
		line, source, found, err := s()
		if err != nil {
			return NativeChoice{}, false, err
		}
		if found {
			choice, err := r.choice(line, source)
			return choice, err == nil, err
		}
	}
	return NativeChoice{}, false, nil
}

func (r resolver) choice(line, source string) (NativeChoice, error) {
	if line != "" && line == r.t.floating {
		c, err := r.choice(r.t.defaultLine, source)
		c.Exact = line
		return c, err
	}
	l, ok := r.t.find(line)
	if !ok && r.t.anyRelease != "" && fullVersion.MatchString(line) {
		c, err := r.choice(r.t.defaultLine, source)
		c.Exact, c.unreviewed = line, true
		return c, err
	}
	if _, released := r.t.findReleased(line); !ok && released {
		c, err := r.choice(r.t.defaultLine, source)
		c.releaseLine, c.unreviewed = line, true
		return c, err
	}
	if !ok {
		return NativeChoice{}, fmt.Errorf("no %s line %q", r.t.label, line)
	}
	return NativeChoice{Label: r.t.label, Line: l.line, Image: l.image, Source: source, upstream: l.upstream}, nil
}

// unsupported names the request the way the tenant wrote it: what is the env
// assignment (`PYTHON_VERSION=2.7.18`) or the file and its quoted content.
func (r resolver) unsupported(what string) error {
	supported := r.t.supportedLines()
	if r.t.anyRelease != "" {
		supported += ", and " + r.t.anyRelease
	}
	return toolchains.Unsupported(what, r.t.label, supported)
}

// supportedLines lists the reviewed lines and the floating tag for a refusal.
func (t nativeToolchain) supportedLines() string {
	lines := make([]string, len(t.lines))
	for i, l := range t.lines {
		lines[i] = l.line
	}
	supported := strings.Join(lines, ", ")
	if t.floating != "" {
		supported += " or " + t.floating
	}
	return supported
}

// fromEnv reads one env var; source names it with its value, as Render's docs do.
func (r resolver) fromEnv(key string, match func(string) (string, bool)) signal {
	return func() (string, string, bool, error) {
		request := strings.TrimSpace(r.env[key])
		if request == "" {
			return "", "", false, nil
		}
		line, ok := match(request)
		if !ok {
			return "", "", false, r.unsupported(key + "=" + request)
		}
		return line, key + "=" + request, true, nil
	}
}

// fromFile reads a single-version file's first meaningful line.
func (r resolver) fromFile(name string, match func(string) (string, bool)) signal {
	return func() (string, string, bool, error) {
		request := firstLine(r.readFile, name)
		if request == "" {
			return "", "", false, nil
		}
		line, ok := match(request)
		if !ok {
			return "", "", false, r.unsupported(fmt.Sprintf("%s %q", name, request))
		}
		return line, name, true, nil
	}
}

func firstLine(readFile func(string) ([]byte, bool), name string) string {
	raw, ok := readFile(name)
	if !ok {
		return ""
	}
	return toolchains.FirstMeaningfulLine(raw)
}

// minorLine maps "3.11", "3.11.9" (or pyenv/rbenv-style "ruby-3.3.6") onto
// the "3.11" line.
func (r resolver) minorLine(request string) (string, bool) {
	line, ok := toolchains.RequestLine(request)
	if !ok {
		return "", false
	}
	_, ok = r.t.find(line)
	return line, ok
}

// nodeLTSRelease is one Node line's long-term-support entry: its codename and
// the day its first LTS release shipped.
type nodeLTSRelease struct {
	codename, line string
	from           time.Time
}

// nodeLTS is Node's LTS schedule for the reviewed lines, oldest first: the
// codename and date nodejs.org/dist/index.json gives each line's first LTS
// release (14.15.0 Fermium … 26.13.0 Lithium). Every reviewed Node line has an
// entry (TestNodeLTSCoversTheReviewedLines): a new line takes its planned LTS
// start from github.com/nodejs/Release's schedule.json, with no codename until
// Node names it, so `lts` moves on that date without another edit (w8/332).
var nodeLTS = []nodeLTSRelease{
	{"fermium", "14", time.Date(2020, time.October, 27, 0, 0, 0, 0, time.UTC)},
	{"gallium", "16", time.Date(2021, time.October, 26, 0, 0, 0, 0, time.UTC)},
	{"hydrogen", "18", time.Date(2022, time.October, 25, 0, 0, 0, 0, time.UTC)},
	{"iron", "20", time.Date(2023, time.October, 24, 0, 0, 0, 0, time.UTC)},
	{"jod", "22", time.Date(2024, time.October, 29, 0, 0, 0, 0, time.UTC)},
	{"krypton", "24", time.Date(2025, time.October, 28, 0, 0, 0, 0, time.UTC)},
	{"lithium", "26", time.Date(2026, time.October, 28, 0, 0, 0, 0, time.UTC)},
}

// nodeLTSLineAt is the line `lts` and `lts/*` name for a build running at now:
// the newest one whose LTS has begun, as nvm, n and fnm resolve the alias.
func nodeLTSLineAt(now time.Time) string {
	line := nodeLTS[0].line
	for _, r := range nodeLTS {
		if !now.Before(r.from) {
			line = r.line
		}
	}
	return line
}

var plainVersion = regexp.MustCompile(`^v?\d+(\.\d+){0,2}$`)

// nodeRequest maps NODE_VERSION / .node-version / .nvmrc values: a plain
// version onto its major line; `lts`, `lts/*`, `lts/<codename>`, `node`,
// `latest`, `current`; otherwise a semver range (highest satisfying line).
// Render resolves a request to the newest Node release that satisfies it,
// whatever its line (node-version-alias), so a request no reviewed line
// serves is returned as the release to install (w8/398, w8/401): a fully
// qualified version as itself, and one that reaches a released line (an odd
// line, or one before 14) as that line's final release when it allows it
// (`>=20 <22` → 21.7.3, `12` → 12.22.12), else as the line, whose newest
// release inside the request the resolve phase lists. r.choice starts either
// on the default line's image until then.
func (r resolver) nodeRequest(request string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(request))
	switch {
	case lower == "lts" || lower == "lts/*":
		return nodeLTSLineAt(time.Now()), true
	case strings.HasPrefix(lower, "lts/"):
		if i := slices.IndexFunc(nodeLTS, func(r nodeLTSRelease) bool { return r.codename != "" && "lts/"+r.codename == lower }); i >= 0 {
			return nodeLTS[i].line, true
		}
		// An LTS line before 14 has ended: its codename names the line's
		// final release, as node-version-alias resolves it (w8/401).
		if line, ok := nodeReleasedLTS[strings.TrimPrefix(lower, "lts/")]; ok {
			return r.releasedRequest(line, line)
		}
		return "", false
	case lower == "node" || lower == "latest" || lower == "current" || lower == "stable":
		return r.t.lines[len(r.t.lines)-1].line, true
	case plainVersion.MatchString(lower):
		version := strings.TrimPrefix(lower, "v")
		line := nodeLine(version)
		if _, ok := r.t.find(line); ok {
			return line, true
		}
		if fullVersion.MatchString(version) {
			return version, true // the resolve phase installs it or refuses it by name
		}
		return r.releasedRequest(line, version)
	}
	line, ok := r.highestSatisfying(lower)
	if !ok {
		return "", false
	}
	return r.releasedRequest(line, lower)
}

// nodeReleasedLTS are the codenames of the LTS lines before 14, each the line
// its `lts/<codename>` alias names (nodejs.org/dist/index.json, w8/401).
var nodeReleasedLTS = map[string]string{"argon": "4", "boron": "6", "carbon": "8", "dubnium": "10", "erbium": "12"}

// nodeLine is the Node line a version names: its major, or, before Node 4,
// its 0.x minor (0.10, 0.12), which were Node's lines then (w8/401).
func nodeLine(version string) string {
	major, rest, _ := strings.Cut(version, ".")
	if minor, _, _ := strings.Cut(rest, "."); major == "0" && minor != "" {
		return "0." + minor
	}
	return major
}

// releasedRequest is a request that reached line, as nodeRequest returns it:
// a reviewed line itself, a released line's final release when the request
// allows it, else that released line; ok is false for no line at all.
func (r resolver) releasedRequest(line, request string) (string, bool) {
	if _, ok := r.t.find(line); ok {
		return line, true
	}
	released, ok := r.t.findReleased(line)
	if !ok {
		return "", false
	}
	if c, err := semver.NewConstraint(request); err == nil && c.Check(semver.MustParse(released.version)) {
		return released.version, true
	}
	return line, true
}

// highestSatisfying resolves a semver range against each line's pinned
// toolchain version, preferring the newest line, released lines no reviewed
// image carries included (w8/398). A range the pinned patch misses but
// another release of the line meets (`~16.13.0` against 16.20.2) still
// selects that line: the pinned patch stands in for the requested one, as it
// does for a plain version (w8/173).
func (r resolver) highestSatisfying(rng string) (string, bool) {
	c, err := semver.NewConstraint(rng)
	if err != nil {
		return "", false
	}
	lines := r.t.rangeLines()
	for _, l := range slices.Backward(lines) {
		if v, err := semver.NewVersion(l.version); err == nil && c.Check(v) {
			return l.line, true
		}
	}
	for _, l := range slices.Backward(lines) {
		if lineHasRelease(l, c) {
			return l.line, true
		}
	}
	return "", false
}

// lineHasRelease reports whether some release up to the line's pinned version,
// within the line's major (or major.minor) series, meets c. Releases are
// probed by version number, so it answers for patch levels the line certainly
// published, none newer than the pin.
func lineHasRelease(l nativeLine, c *semver.Constraints) bool {
	pin, err := semver.NewVersion(l.version)
	if err != nil {
		return false
	}
	for minor := range pin.Minor() + 1 {
		if strings.Count(l.line, ".") == 1 && minor != pin.Minor() {
			continue
		}
		maxPatch := uint64(60)
		if minor == pin.Minor() {
			maxPatch = pin.Patch()
		}
		for patch := range maxPatch + 1 {
			if c.Check(semver.New(pin.Major(), minor, patch, "", "")) {
				return true
			}
		}
	}
	return false
}

func (r resolver) fromPackageEngines() (string, string, bool, error) {
	request := strings.TrimSpace(packageEnginesNode(r.readFile, r.nodePackage))
	if request == "" {
		return "", "", false, nil
	}
	source := r.nodePackage + enginesNodeSuffix
	line, ok := r.nodeRequest(request)
	if !ok {
		return "", "", false, r.unsupported(fmt.Sprintf("%s %q", source, request))
	}
	return line, source, true, nil
}

var gemfileRuby = regexp.MustCompile(`(?m)^\s*ruby\s*\(?\s*(['"])([^'"]+)['"]((?:\s*,\s*['"][^'"]+['"])*)`)

// lockedRuby is a Bundler lockfile's RUBY VERSION block: `ruby 3.4.7p58` (the
// patchlevel written by Bundler before 4.0; another engine adds its own version
// in parentheses after it).
var lockedRuby = regexp.MustCompile(`(?m)^RUBY VERSION[ \t]*\r?\n[ \t]+ruby[ \t]+([^\s(]+)`)

// rubyPatchlevel is the `p58` a Bundler lockfile appends to an MRI version.
var rubyPatchlevel = regexp.MustCompile(`p\d+$`)

// fromLockedRuby honors the Ruby a Bundler lockfile records, Render's first
// Ruby signal (w8/m85): gems.locked, else Gemfile.lock (Bundler's own order),
// whichever first has a RUBY VERSION block. The block records what Bundler
// last ran, which it rewrites only once the Gemfile stops allowing it, so a
// recorded version no reviewed line carries is a stale record rather than a
// requirement: the next signal decides, as it did before lockfiles were read.
func (r resolver) fromLockedRuby() (string, string, bool, error) {
	for _, name := range []string{"gems.locked", "Gemfile.lock"} {
		raw, ok := r.readFile(name)
		if !ok {
			continue
		}
		version, recorded := lockedRubyVersion(raw)
		if !recorded {
			continue
		}
		if line, ok := r.minorLine(version); ok && plainVersion.MatchString(version) {
			return line, name, true, nil
		}
		return "", "", false, nil
	}
	return "", "", false, nil
}

// lockedRubyVersion is the Ruby a lockfile's RUBY VERSION block records,
// without the patchlevel; recorded is false when it has no such block.
func lockedRubyVersion(raw []byte) (version string, recorded bool) {
	m := lockedRuby.FindSubmatch(raw)
	if m == nil {
		return "", false
	}
	return rubyPatchlevel.ReplaceAllString(string(m[1]), ""), true
}

// fromToolVersions honors a tool's line in an asdf/mise .tool-versions file
// (`ruby 3.3.6`, the first listed version preferred), Render's third Ruby
// signal (w8/m85). A file without that tool says nothing.
func (r resolver) fromToolVersions(tool string, match func(string) (string, bool)) signal {
	return func() (string, string, bool, error) {
		raw, ok := r.readFile(".tool-versions")
		if !ok {
			return "", "", false, nil
		}
		request := toolVersion(raw, tool)
		if request == "" || request == "system" {
			return "", "", false, nil // no entry, or asdf manages no version here
		}
		version, ok := match(request)
		if !ok {
			return "", "", false, r.unsupported(fmt.Sprintf(".tool-versions %s %q", tool, request))
		}
		return version, ".tool-versions", true, nil
	}
}

// toolVersion is the first version a .tool-versions file lists for tool, or
// "" when the file has no line for it.
func toolVersion(raw []byte, tool string) string {
	for line := range strings.SplitSeq(string(raw), "\n") {
		if comment := strings.IndexByte(line, '#'); comment >= 0 {
			line = line[:comment]
		}
		if fields := strings.Fields(strings.TrimPrefix(line, "\ufeff")); len(fields) >= 2 && fields[0] == tool {
			return fields[1]
		}
	}
	return ""
}

// fromGemfile honors a Gemfile `ruby "3.3.0"` / `ruby "~> 3.3"` directive.
// `ruby file: ".ruby-version"` has no literal and falls through to the file.
func (r resolver) fromGemfile() (string, string, bool, error) {
	raw, ok := r.readFile("Gemfile")
	if !ok {
		return "", "", false, nil
	}
	reqs := gemfileRubyRequirements(raw)
	if reqs == nil {
		return "", "", false, nil
	}
	line, ok := r.rubyRequirement(reqs)
	if !ok {
		return "", "", false, r.unsupported(fmt.Sprintf("Gemfile ruby %q", strings.Join(reqs, ", ")))
	}
	return line, "Gemfile", true, nil
}

var quotedRequirement = regexp.MustCompile(`['"]([^'"]+)['"]`)

// gemfileRubyRequirements are the requirements of a Gemfile's `ruby`
// directive, or nil when it has none with a literal.
func gemfileRubyRequirements(raw []byte) []string {
	m := gemfileRuby.FindStringSubmatch(string(raw))
	if m == nil {
		return nil
	}
	reqs := []string{m[2]}
	for _, extra := range quotedRequirement.FindAllStringSubmatch(m[3], -1) {
		reqs = append(reqs, extra[1])
	}
	return reqs
}

func (r resolver) rubyFileLine(request string) (string, bool) {
	return r.rubyRequirement([]string{request})
}

// rubyRequirement maps RubyGems requirements onto a line: a plain version by
// major.minor; `~> 3.3` (>= 3.3, < 4) and `~> 3.3.1` (>= 3.3.1, < 3.4) and
// comparison operators as ranges against the pinned versions.
func (r resolver) rubyRequirement(reqs []string) (string, bool) {
	if len(reqs) == 1 && plainVersion.MatchString(strings.TrimPrefix(strings.TrimSpace(reqs[0]), "ruby-")) {
		return r.minorLine(strings.TrimSpace(reqs[0]))
	}
	parts := make([]string, 0, len(reqs))
	for _, req := range reqs {
		req = strings.TrimSpace(req)
		if v, ok := strings.CutPrefix(req, "~>"); ok {
			v = strings.TrimSpace(v)
			if strings.Count(v, ".") >= 2 {
				parts = append(parts, "~"+v)
			} else {
				parts = append(parts, "^"+v)
			}
			continue
		}
		parts = append(parts, req)
	}
	return r.highestSatisfying(strings.Join(parts, ", "))
}

// otpRelease is an Erlang/OTP release as ERLANG_VERSION names it: a major
// (28), a minor (28.3) or a patch (28.3.3, 28.5.0.6).
var otpRelease = regexp.MustCompile(`^\d{1,3}(\.\d{1,4}){0,3}$`)

// elixir honors ELIXIR_VERSION (a line by major.minor, or a release no line
// carries, elixirRequest) and records an ERLANG_VERSION release for
// pairElixirOTP, which checks the pair once the exact Elixir is known.
func (r resolver) elixir() (NativeChoice, error) {
	choice, err := r.first(r.fromEnv("ELIXIR_VERSION", r.elixirRequest))
	if err != nil {
		return NativeChoice{}, err
	}
	erlang := strings.TrimSpace(r.env["ERLANG_VERSION"])
	if erlang == "" {
		return choice, nil
	}
	release := strings.TrimPrefix(erlang, "OTP-")
	if !otpRelease.MatchString(release) {
		return NativeChoice{}, &UnsupportedVersionError{Msg: fmt.Sprintf(
			"%s is not an Erlang/OTP release; set ERLANG_VERSION to a release such as 28 or 28.3", toolchains.BoundedRequest("ERLANG_VERSION="+erlang))}
	}
	choice.erlang = release
	choice.Source = strings.TrimPrefix(choice.Source+", ERLANG_VERSION="+erlang, ", ")
	return choice, nil
}

// Render's minimum Elixir and Erlang/OTP: "Render services cannot use
// versions of certain languages earlier than those listed below"
// (render.com/docs/language-support and
// render.com/docs/elixir-erlang-versions, read 2026-11-21).
const (
	elixirMinimum = "1.12.0"
	erlangMinimum = "24.3.4"
)

// nativeElixirAnyRelease is what an Elixir refusal adds about the releases no
// line carries.
var nativeElixirAnyRelease = "any other Elixir release from " + elixirMinimum + " (Render's minimum) to " +
	elixirOTP[len(elixirOTP)-1].minor + " installs on a compatible Erlang/OTP"

// elixirOTP is Elixir's compatibility table: the Erlang/OTP majors lo to hi
// each Elixir minor supports, and hi+1 from the patch named in next
// (hexdocs.pm/elixir/1.18.4/compatibility-and-deprecations.html; 1.19 and
// 1.20 from the 1.20.5 page, read 2026-11-21). Render builds any compatible
// pair from its minimums on, and an incompatible one fails: "Your Elixir
// version must be compatible with your Erlang/OTP version!"
// (render.com/docs/elixir-erlang-versions). final is the last release of a
// minor no line carries, which a bare minor (ELIXIR_VERSION=1.16) installs:
// each of those minors has stopped receiving releases.
var elixirOTP = []struct {
	minor, next, final string
	lo, hi             int
}{
	{"1.12", "", "1.12.3", 22, 24},
	{"1.13", "1.13.4", "1.13.4", 22, 24},
	{"1.14", "1.14.5", "1.14.5", 23, 25},
	{"1.15", "", "1.15.8", 24, 26},
	{"1.16", "", "1.16.3", 24, 26},
	{"1.17", "", "", 25, 27},
	{"1.18", "1.18.4", "", 25, 27},
	{"1.19", "", "", 26, 28},
	{"1.20", "", "", 27, 29},
}

// elixirOTPMajors is the Erlang/OTP majors an Elixir release supports by
// Elixir's table; ok is false for a version the table does not cover.
func elixirOTPMajors(version string) (lo, hi int, ok bool) {
	v, err := semver.NewVersion(version)
	if err != nil || !fullVersion.MatchString(version) {
		return 0, 0, false
	}
	minor := fmt.Sprintf("%d.%d", v.Major(), v.Minor())
	for _, row := range elixirOTP {
		if row.minor != minor {
			continue
		}
		if row.next != "" && !v.LessThan(semver.MustParse(row.next)) {
			return row.lo, row.hi + 1, true
		}
		return row.lo, row.hi, true
	}
	return 0, 0, false
}

// elixirRequest maps ELIXIR_VERSION onto its line by major.minor. A release of
// a minor no line carries, from Render's minimum on, is returned as itself (a
// bare minor as its final release): r.choice starts it on the default line's
// image, and the resolve phase installs hexpm's build of it on a compatible
// Erlang/OTP (w8/373).
func (r resolver) elixirRequest(request string) (string, bool) {
	if line, ok := r.minorLine(request); ok {
		return line, true
	}
	version := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(request)), "v")
	for _, row := range elixirOTP {
		if version == row.minor && row.final != "" {
			version = row.final
		}
	}
	if _, _, ok := elixirOTPMajors(version); !ok {
		return "", false // the table starts at Render's minimum
	}
	return version, true
}

// pairElixirOTP checks the Erlang/OTP an Elixir choice runs on against Elixir's
// table and Render's minimum, and records the OTP the resolve phase installs:
// a requested release the line's image does not carry (anything finer than a
// major, w8/197, or another major, w8/373); for a release no line carries,
// the newest major it supports; with no signal, Render's default (w8/333).
// An incompatible pair is refused by name, as Render's build fails on it.
func pairElixirOTP(created time.Time, choice NativeChoice, env map[string]string) (NativeChoice, error) {
	elixir := cmp.Or(choice.Exact, choice.lineVersion)
	lo, hi, _ := elixirOTPMajors(elixir) // every line's and accepted release's minor has a row (TestElixirOTPTable)
	if choice.erlang == "" {
		switch {
		case choice.unreviewed:
			choice.erlang = strconv.Itoa(hi)
		case choice.Source == "":
			choice.erlang = defaultErlang(created, choice)
		}
		return choice, nil
	}
	request := strings.TrimSpace(env["ERLANG_VERSION"])
	parts := otpParts(choice.erlang)
	n := strings.Count(choice.erlang, ".") + 1
	if slices.Compare(parts[:n], otpParts(erlangMinimum)[:n]) < 0 {
		return NativeChoice{}, &UnsupportedVersionError{Msg: fmt.Sprintf(
			"ERLANG_VERSION=%q is earlier than Erlang/OTP %s, the oldest Render supports; set ERLANG_VERSION to %s or a later release",
			request, erlangMinimum, erlangMinimum)}
	}
	if parts[0] < lo || parts[0] > hi {
		with := "Elixir " + elixir
		if !strings.Contains(choice.Source, "ELIXIR_VERSION=") {
			with += " (the default)"
		}
		return NativeChoice{}, &UnsupportedVersionError{Msg: fmt.Sprintf(
			"ERLANG_VERSION=%q is not compatible with %s, which supports Erlang/OTP %d to %d; set ERLANG_VERSION to a compatible release or choose another ELIXIR_VERSION",
			request, with, lo, hi)}
	}
	if line, _ := nativeToolchains[nativeElixirRuntime].find(choice.Line); !choice.unreviewed && choice.erlang == line.otp {
		choice.erlang = "" // the line's own image carries that major
	}
	return choice, nil
}

var fullVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// excludedNodeRange is the Node request behind choice when it is a semver
// range its line's reviewed patch does not satisfy, else "". A partial version
// (`18.17`) is a range too, as node-version-alias reads it; a fully qualified
// one is exactRequest's, and an alias (lts/*, a codename) names no range.
func excludedNodeRange(choice NativeChoice, raw string) string {
	request := strings.TrimPrefix(strings.ToLower(raw), "v")
	if request == "" || fullVersion.MatchString(request) {
		return ""
	}
	constraint, err := semver.NewConstraint(request)
	if err != nil {
		return ""
	}
	line, ok := nativeToolchains[nativeNodeRuntime].find(choice.Line)
	if !ok {
		return ""
	}
	if pin, err := semver.NewVersion(line.version); err != nil || constraint.Check(pin) {
		return ""
	}
	return request
}

// nodeRequestText is the Node request behind a choice's signal, as the
// service wrote it, or "" for the default.
func nodeRequestText(source string, readFile func(string) ([]byte, bool)) string {
	switch {
	case strings.HasPrefix(source, "NODE_VERSION="):
		return strings.TrimPrefix(source, "NODE_VERSION=")
	case source == ".node-version" || source == ".nvmrc":
		return firstLine(readFile, source)
	case strings.HasSuffix(source, enginesNodeSuffix):
		return strings.TrimSpace(packageEnginesNode(readFile, strings.TrimSuffix(source, enginesNodeSuffix)))
	}
	return ""
}

// renderNodeRequest is the request Render's log echoes before a Node version
// line, `==> Requesting Node.js version >= 20.4.0`: a range or an alias, as the
// service wrote it (w9/141). Render echoes none for an exact version. Whether
// it echoes a partial one (`22`) is unmeasured, so bex echoes no plain version.
func renderNodeRequest(request string) string {
	if request == "" || plainVersion.MatchString(strings.ToLower(request)) {
		return ""
	}
	return toolchains.BoundedRequest(request)
}

// packageJSON is npm's manifest, and the root's is Node's whenever the root
// has one (nodePackageJSON).
const packageJSON = "package.json"

// enginesNodeSuffix follows the package.json path in a Node choice's Source:
// `backend/package.json engines.node`.
const enginesNodeSuffix = " engines.node"

// packageEnginesNode is the engines.node of the package.json at name, or ""
// when there is none. An unparseable package.json is npm's error to report,
// not a version request.
func packageEnginesNode(readFile func(string) ([]byte, bool), name string) string {
	raw, ok := readFile(name)
	if !ok {
		return ""
	}
	var pkg struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if json.Unmarshal(raw, &pkg) != nil {
		return ""
	}
	return pkg.Engines.Node
}

// exactRequest returns the fully qualified version a service pinned within the
// chosen line when it differs from the line's reviewed patch, else "". A plain
// version counts wherever the signal holds one: an env var, a single-version
// file, and Ruby's lockfile RUBY VERSION, .tool-versions entry and Gemfile
// `ruby "3.2.0"`, which Bundler refuses any other patch for (w9/133). A range
// or an alias keeps resolving to the line's reviewed patch.
func exactRequest(t nativeToolchain, choice NativeChoice, readFile func(string) ([]byte, bool)) string {
	line, ok := t.find(choice.Line)
	if !ok {
		return ""
	}
	return toolchains.ExactVersion(requestedVersion(choice.Source, readFile), choice.Line, line.version)
}

// requestedVersion is the version text behind a choice's first signal, as the
// service wrote it, or "" when that signal names none.
func requestedVersion(source string, readFile func(string) ([]byte, bool)) string {
	signal, _, _ := strings.Cut(source, ", ")
	if key, value, isEnv := strings.Cut(signal, "="); isEnv {
		if !strings.HasSuffix(key, "_VERSION") || key == "ERLANG_VERSION" {
			return ""
		}
		return value
	}
	switch signal {
	case "Gemfile.lock", "gems.locked":
		raw, _ := readFile(signal)
		version, _ := lockedRubyVersion(raw)
		return version
	case ".tool-versions":
		raw, _ := readFile(signal)
		return toolVersion(raw, "ruby")
	case "Gemfile":
		raw, _ := readFile(signal)
		if reqs := gemfileRubyRequirements(raw); len(reqs) == 1 {
			return reqs[0]
		}
		return ""
	}
	if strings.HasPrefix(signal, ".") && !strings.Contains(signal, "/") {
		return firstLine(readFile, signal)
	}
	return ""
}

// exactImage names the official tag for an exact version: the line's tag with
// its line replaced by the version (python:3.11-bookworm → python:3.11.9-bookworm).
func exactImage(choice NativeChoice) (repo, tag string, ok bool) {
	repo, suffix, ok := lineImageVariant(choice)
	if !ok || choice.Exact == "" {
		return "", "", false
	}
	return repo, choice.Exact + suffix, true
}

// lineImageVariant splits the line's official image (python:3.11-bookworm)
// into its repository and the variant suffix after the line (-bookworm).
func lineImageVariant(choice NativeChoice) (repo, suffix string, ok bool) {
	return toolchains.ImageVariant(cmp.Or(choice.upstream, choice.Image), choice.Line)
}
