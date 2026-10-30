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
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
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
	// erlang is the Erlang/OTP release an Elixir service pinned more exactly
	// than its major (ERLANG_VERSION=28.3, w8/197); the resolve phase builds on
	// an image carrying it, or fails by name.
	erlang string
	// otp is the Erlang/OTP release the chosen image installs, once the
	// resolve phase has pinned an exact Erlang; the log names it.
	otp string
	// elixirImage is the image an exact Erlang's Elixir is copied from.
	elixirImage string
	// note is an extra build-log line (an exact version that fell back to the
	// line's reviewed patch).
	note string
}

// Narration is the build-log line that names the choice and its source.
func (c NativeChoice) Narration() string {
	if c.declined != "" {
		return fmt.Sprintf("==> Not adding %s: %s", c.Label, c.declined)
	}
	from := "default"
	if c.Source != "" {
		from = "from " + c.Source
	}
	version := c.Line
	if c.otp != "" {
		version += " on Erlang/OTP " + c.otp
	}
	line := fmt.Sprintf("==> Using %s %s (%s)", c.Label, version, from)
	if c.note != "" {
		line += "\n" + c.note
	}
	return line
}

// UnsupportedVersionError is a version request no reviewed line satisfies. It
// is tenant input: the build fails on it by name instead of silently building
// on the default line.
type UnsupportedVersionError struct{ msg string }

func (e *UnsupportedVersionError) Error() string { return e.msg }

// ResolveNativeVersion picks a runtime's toolchain line from the service's own
// version signals, with Render's precedence per runtime (docs/ADR060 D7
// addendum, w8/m51):
//
//	python  PYTHON_VERSION → .python-version → default
//	node    NODE_VERSION → .node-version → .nvmrc → package.json engines.node → default
//	ruby    Gemfile.lock / gems.locked RUBY VERSION → .ruby-version → .tool-versions → Gemfile `ruby` → default
//	elixir  ELIXIR_VERSION (→ line), ERLANG_VERSION (must match that line's OTP major; a minor or patch is installed exactly, w8/197)
//	go, rust  default (Render offers no Go selection; rustup applies Rust's)
//
// A plain version maps onto its line by major.minor (major for Node) — a
// requested patch is not reproduced, a recorded divergence from Render. A
// range (Node engines, Ruby requirements) resolves to the highest line whose
// pinned toolchain satisfies it. env is the build's effective environment;
// readFile reads a file relative to the service's root directory.
func ResolveNativeVersion(runtime string, env map[string]string, readFile func(string) ([]byte, bool)) (NativeChoice, error) {
	return ResolveNativeVersionAt(time.Time{}, runtime, env, readFile)
}

// pythonCreationCutoff is when Python's default line moved to 3.14 for new
// services (w8/174). Render picks a service's default Python by its creation
// date and gives every service created since 2026-02-11 a 3.14 release; bex
// applies it to services created after this change, so a running service's
// toolchain never moves under an operator upgrade (the WEB_CONCURRENCY cutoff
// pattern, w5/m147).
var pythonCreationCutoff = time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC)

// pythonDefaultLine is the line a Python service created at created builds on
// without a version signal. A zero created (unknown) keeps the legacy default.
func pythonDefaultLine(created time.Time) string {
	if !created.IsZero() && !created.Before(pythonCreationCutoff) {
		return "3.14"
	}
	return nativeToolchains[nativePythonRuntime].defaultLine
}

// rubyDefaultCutoff and rubyDefaultVersion: Render's default Ruby for services
// created since 2025-06-12 is exactly 3.4.4, not the newest 3.4 patch (w8/198).
// bex applies it to services created after this change, so a running service's
// toolchain never moves under an operator upgrade (the pythonCreationCutoff
// pattern).
var rubyDefaultCutoff = time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC)

const rubyDefaultVersion = "3.4.4"

// pythonDefaultVersion: Render's default Python for services created since
// 2026-10-12 is exactly 3.14.8, not the newest 3.14 patch (w8/217). It applies
// from pythonCreationCutoff, when bex's default line moved to 3.14 (w8/174);
// an explicit PYTHON_VERSION=3.14 keeps the line's reviewed patch. The official
// python:3.14.8 image is no longer rebuilt, so it predates the 3.14 line's
// base-layer refresh (the Brotli fix, w1/136); w8/222 asks whether to keep the
// reviewed patch instead.
const pythonDefaultVersion = "3.14.8"

// ResolveNativeVersionAt is ResolveNativeVersion for a service created at
// created, whose creation date picks the default line where Render's does.
func ResolveNativeVersionAt(created time.Time, runtime string, env map[string]string, readFile func(string) ([]byte, bool)) (NativeChoice, error) {
	choice, err := resolveNativeLine(created, runtime, env, readFile)
	if err == nil {
		choice.Exact = exactRequest(runtime, choice, readFile)
		if choice.Exact == "" {
			choice.Exact = defaultExact(created, runtime, choice)
		}
	}
	return choice, err
}

// exactDefaults are the runtimes whose Render default for new services is a
// specific patch rather than the newest patch of the default line.
var exactDefaults = map[string]struct {
	cutoff  time.Time
	version string
}{
	nativeRubyRuntime:   {rubyDefaultCutoff, rubyDefaultVersion},
	nativePythonRuntime: {pythonCreationCutoff, pythonDefaultVersion},
}

// defaultExact is the exact version a service with no version signal builds on
// where Render's default is a specific patch: Ruby's 3.4.4 and Python's 3.14.8
// for services created since the cutoff. The resolve phase swaps in that
// version's official image and falls back to the line's reviewed patch, saying
// so, when it cannot. A default equal to the reviewed patch needs no swap.
func defaultExact(created time.Time, runtime string, choice NativeChoice) string {
	d, ok := exactDefaults[runtime]
	if !ok || choice.Source != "" || created.IsZero() || created.Before(d.cutoff) ||
		!strings.HasPrefix(d.version, choice.Line+".") {
		return ""
	}
	if line, _ := nativeToolchains[runtime].find(choice.Line); line.version == d.version {
		return ""
	}
	return d.version
}

func resolveNativeLine(created time.Time, runtime string, env map[string]string, readFile func(string) ([]byte, bool)) (NativeChoice, error) {
	t, ok := nativeToolchains[runtime]
	if !ok {
		return NativeChoice{}, fmt.Errorf("unsupported native runtime %q", runtime)
	}
	if runtime == nativePythonRuntime {
		t.defaultLine = pythonDefaultLine(created)
	}
	r := resolver{t: t, env: env, readFile: readFile}
	switch runtime {
	case nativePythonRuntime:
		return r.first(r.fromEnv("PYTHON_VERSION", r.minorLine), r.fromFile(".python-version", r.minorLine))
	case nativeNodeRuntime:
		return r.first(r.fromEnv("NODE_VERSION", r.nodeRequest), r.fromFile(".node-version", r.nodeRequest),
			r.fromFile(".nvmrc", r.nodeRequest), r.fromPackageEngines)
	case nativeRubyRuntime:
		return r.first(r.fromLockedRuby, r.fromFile(".ruby-version", r.rubyFileLine), r.fromToolVersions("ruby", r.rubyFileLine), r.fromGemfile)
	case "elixir":
		return r.elixir()
	default:
		return r.choice(t.defaultLine, "")
	}
}

// ResolveNativeBun picks the Bun companion a native build adds, with Render's
// precedence: BUN_VERSION → .bun-version → a bun.lock or bun.lockb lockfile
// (the default line). found is false when the service does not opt in, which
// leaves the build exactly as before (w8/m71). A version maps onto its
// major.minor line, as the primary toolchains' do.
func ResolveNativeBun(env map[string]string, readFile func(string) ([]byte, bool)) (choice NativeChoice, found bool, err error) {
	r := resolver{t: nativeBun, env: env, readFile: readFile}
	return r.firstFound(r.fromEnv("BUN_VERSION", r.bunRequest), r.fromFile(".bun-version", r.bunRequest), r.fromLockfile(nativeBunLockfiles...))
}

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
// Compatibility with the chosen Python is Poetry's own: an incompatible pair
// fails the install step by name in the build log.
func ResolveNativePoetry(env map[string]string) (NativeChoice, error) {
	request := strings.TrimSpace(env["POETRY_VERSION"])
	if request == "" {
		return NativeChoice{Label: "Poetry", Line: defaultPoetryVersion}, nil
	}
	version := strings.TrimPrefix(strings.ToLower(request), "v")
	if !poetryRelease.MatchString(version) {
		return NativeChoice{}, &UnsupportedVersionError{msg: fmt.Sprintf(
			"%s is not a supported Poetry version; set an exact Poetry 1.x or 2.x release such as %s", boundedRequest("POETRY_VERSION="+request), defaultPoetryVersion)}
	}
	return NativeChoice{Label: "Poetry", Line: version, Source: "POETRY_VERSION=" + request}, nil
}

// defaultUVVersion is the uv a native Python uv project gets when the service
// names none — Render's current default for new services (its uv version
// contract, w8/m81).
const defaultUVVersion = "0.12.10"

// uvRelease is an exact uv release, the form Render's UV_VERSION takes ("any
// version number"); ranges are not resolved.
var uvRelease = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// ResolveNativeUV picks the uv a native Python build adds. As on Render, a
// uv.lock in the service's root directory opts in, and UV_VERSION then picks
// the release (else the default); without the lockfile the build is exactly
// as before and UV_VERSION is not read (w8/m81). Like Poetry it is an exact
// PyPI release, so Line is the version itself, and an incompatible Python
// fails the install step by name.
func ResolveNativeUV(env map[string]string, readFile func(string) ([]byte, bool)) (choice NativeChoice, found bool, err error) {
	if _, locked := readFile("uv.lock"); !locked {
		return NativeChoice{}, false, nil
	}
	request := strings.TrimSpace(env["UV_VERSION"])
	if request == "" {
		return NativeChoice{Label: "uv", Line: defaultUVVersion, Source: "uv.lock"}, true, nil
	}
	version := strings.TrimPrefix(strings.ToLower(request), "v")
	if !uvRelease.MatchString(version) {
		return NativeChoice{}, false, &UnsupportedVersionError{msg: fmt.Sprintf(
			"%s is not a supported uv version; set an exact uv release such as %s", boundedRequest("UV_VERSION="+request), defaultUVVersion)}
	}
	return NativeChoice{Label: "uv", Line: version, Source: "UV_VERSION=" + request}, true, nil
}

// boundedRequest bounds a tenant's version request for an error message.
func boundedRequest(what string) string {
	if len(what) > 120 {
		return strings.ToValidUTF8(what[:120], "") + "…"
	}
	return what
}

// bunRequest maps a Bun version request: `latest`, a plain version onto its
// major.minor line (a bare major onto that major's newest line), otherwise a
// semver range.
func (r resolver) bunRequest(request string) (string, bool) {
	lower := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(request)), "bun-"), "v")
	switch {
	case lower == "latest":
		return r.t.lines[len(r.t.lines)-1].line, true
	case plainVersion.MatchString(lower) && !strings.Contains(lower, "."):
		return r.highestSatisfying(lower + ".x")
	case plainVersion.MatchString(lower):
		return r.minorLine(lower)
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
	l, ok := r.t.find(line)
	if !ok {
		return NativeChoice{}, fmt.Errorf("no %s line %q", r.t.label, line)
	}
	return NativeChoice{Label: r.t.label, Line: l.line, Image: l.image, Source: source, upstream: l.upstream}, nil
}

// unsupported names the request the way the tenant wrote it: what is the env
// assignment (`PYTHON_VERSION=2.7.18`) or the file and its quoted content.
func (r resolver) unsupported(what string) error {
	what = boundedRequest(what)
	lines := make([]string, len(r.t.lines))
	for i, l := range r.t.lines {
		lines[i] = l.line
	}
	return &UnsupportedVersionError{msg: fmt.Sprintf("%s is not a supported %s version; supported %s lines are %s",
		what, r.t.label, r.t.label, strings.Join(lines, ", "))}
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
	for line := range strings.SplitSeq(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// minorLine maps "3.11", "3.11.9" (or pyenv/rbenv-style "python-3.11.9") onto
// the "3.11" line.
func (r resolver) minorLine(request string) (string, bool) {
	request = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(request), "python-"), "ruby-")
	v, err := semver.NewVersion(strings.TrimPrefix(request, "v"))
	if err != nil || strings.Count(strings.TrimPrefix(request, "v"), ".") < 1 {
		return "", false
	}
	line := fmt.Sprintf("%d.%d", v.Major(), v.Minor())
	_, ok := r.t.find(line)
	return line, ok
}

// nodeLTSCodenames are the release codenames nvm-style `lts/<name>` names.
var nodeLTSCodenames = map[string]string{"fermium": "14", "gallium": "16", "hydrogen": "18", "iron": "20", "jod": "22", "krypton": "24"}

// nodeLTSLine is the current active LTS line (24 until 26 enters LTS).
const nodeLTSLine = "24"

var plainVersion = regexp.MustCompile(`^v?\d+(\.\d+){0,2}$`)

// nodeRequest maps NODE_VERSION / .node-version / .nvmrc values: a plain
// version onto its major line; `lts`, `lts/*`, `lts/<codename>`, `node`,
// `latest`, `current`; otherwise a semver range (highest satisfying line).
func (r resolver) nodeRequest(request string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(request))
	switch {
	case lower == "lts" || lower == "lts/*":
		return nodeLTSLine, true
	case strings.HasPrefix(lower, "lts/"):
		line, ok := nodeLTSCodenames[strings.TrimPrefix(lower, "lts/")]
		return line, ok
	case lower == "node" || lower == "latest" || lower == "current" || lower == "stable":
		return r.t.lines[len(r.t.lines)-1].line, true
	case plainVersion.MatchString(lower):
		major, _, _ := strings.Cut(strings.TrimPrefix(lower, "v"), ".")
		_, ok := r.t.find(major)
		return major, ok
	}
	return r.highestSatisfying(lower)
}

// highestSatisfying resolves a semver range against each line's pinned
// toolchain version, preferring the newest line. A range the pinned patch
// misses but another release of the line meets (`~16.13.0` against 16.20.2)
// still selects that line: the pinned patch stands in for the requested one, as
// it does for a plain version (w8/173).
func (r resolver) highestSatisfying(rng string) (string, bool) {
	c, err := semver.NewConstraint(rng)
	if err != nil {
		return "", false
	}
	for _, l := range slices.Backward(r.t.lines) {
		if v, err := semver.NewVersion(l.version); err == nil && c.Check(v) {
			return l.line, true
		}
	}
	for _, l := range slices.Backward(r.t.lines) {
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
	raw, ok := r.readFile("package.json")
	if !ok {
		return "", "", false, nil
	}
	var pkg struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
	}
	if json.Unmarshal(raw, &pkg) != nil || strings.TrimSpace(pkg.Engines.Node) == "" {
		// An unparseable package.json is npm's error to report, not a version request.
		return "", "", false, nil
	}
	request := strings.TrimSpace(pkg.Engines.Node)
	line, ok := r.nodeRequest(request)
	if !ok {
		return "", "", false, r.unsupported(fmt.Sprintf("package.json engines.node %q", request))
	}
	return line, "package.json engines.node", true, nil
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
		m := lockedRuby.FindStringSubmatch(string(raw))
		if m == nil {
			continue
		}
		version := rubyPatchlevel.ReplaceAllString(m[1], "")
		if line, ok := r.minorLine(version); ok && plainVersion.MatchString(version) {
			return line, name, true, nil
		}
		return "", "", false, nil
	}
	return "", "", false, nil
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
		for line := range strings.SplitSeq(string(raw), "\n") {
			if comment := strings.IndexByte(line, '#'); comment >= 0 {
				line = line[:comment]
			}
			fields := strings.Fields(strings.TrimPrefix(line, "\ufeff"))
			if len(fields) < 2 || fields[0] != tool {
				continue
			}
			if fields[1] == "system" {
				return "", "", false, nil // asdf manages no version here
			}
			version, ok := match(fields[1])
			if !ok {
				return "", "", false, r.unsupported(fmt.Sprintf(".tool-versions %s %q", tool, fields[1]))
			}
			return version, ".tool-versions", true, nil
		}
		return "", "", false, nil
	}
}

// fromGemfile honors a Gemfile `ruby "3.3.0"` / `ruby "~> 3.3"` directive.
// `ruby file: ".ruby-version"` has no literal and falls through to the file.
func (r resolver) fromGemfile() (string, string, bool, error) {
	raw, ok := r.readFile("Gemfile")
	if !ok {
		return "", "", false, nil
	}
	m := gemfileRuby.FindStringSubmatch(string(raw))
	if m == nil {
		return "", "", false, nil
	}
	reqs := []string{m[2]}
	for _, extra := range regexp.MustCompile(`['"]([^'"]+)['"]`).FindAllStringSubmatch(m[3], -1) {
		reqs = append(reqs, extra[1])
	}
	request := strings.Join(reqs, ", ")
	line, ok := r.rubyRequirement(reqs)
	if !ok {
		return "", "", false, r.unsupported(fmt.Sprintf("Gemfile ruby %q", request))
	}
	return line, "Gemfile", true, nil
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

// elixir honors ELIXIR_VERSION (line by major.minor) and checks ERLANG_VERSION
// against the chosen line's OTP major — an Erlang the line's image does not
// carry is refused by name rather than silently ignored. A release more exact
// than the major is carried in Erlang: the line's image has whatever OTP it
// was built with, so the resolve phase finds an image with the requested one
// (w8/197).
func (r resolver) elixir() (NativeChoice, error) {
	choice, err := r.first(r.fromEnv("ELIXIR_VERSION", r.minorLine))
	if err != nil {
		return NativeChoice{}, err
	}
	erlang := strings.TrimSpace(r.env["ERLANG_VERSION"])
	if erlang == "" {
		return choice, nil
	}
	release := strings.TrimPrefix(erlang, "OTP-")
	if !otpRelease.MatchString(release) {
		return NativeChoice{}, &UnsupportedVersionError{msg: fmt.Sprintf(
			"%s is not an Erlang/OTP release; set ERLANG_VERSION to a release such as 28 or 28.3", boundedRequest("ERLANG_VERSION="+erlang))}
	}
	line, _ := r.t.find(choice.Line)
	if major, _, _ := strings.Cut(release, "."); major != line.otp {
		return NativeChoice{}, &UnsupportedVersionError{msg: fmt.Sprintf(
			"ERLANG_VERSION=%q is not available with Elixir %s, which runs on Erlang/OTP %s; set ERLANG_VERSION to an OTP %s release or choose another ELIXIR_VERSION",
			erlang, line.line, line.otp, line.otp)}
	}
	if strings.Contains(release, ".") {
		choice.erlang = release
	}
	choice.Source = strings.TrimPrefix(choice.Source+", ERLANG_VERSION="+erlang, ", ")
	return choice, nil
}

var fullVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// exactRequest returns the fully qualified version a service pinned within the
// chosen line when it differs from the line's reviewed patch, else "". Only a
// plain version signal counts (an env var or a single-version file); a range,
// an alias or a lockfile keeps resolving to the line's reviewed patch.
func exactRequest(runtime string, choice NativeChoice, readFile func(string) ([]byte, bool)) string {
	signal, _, _ := strings.Cut(choice.Source, ", ")
	var raw string
	if key, value, isEnv := strings.Cut(signal, "="); isEnv {
		if !strings.HasSuffix(key, "_VERSION") || key == "ERLANG_VERSION" {
			return ""
		}
		raw = value
	} else if strings.HasPrefix(signal, ".") && !strings.Contains(signal, "/") {
		raw = firstLine(readFile, signal)
	}
	version := strings.ToLower(strings.TrimSpace(raw))
	for _, prefix := range []string{"python-", "ruby-", "v"} {
		version = strings.TrimPrefix(version, prefix)
	}
	line, ok := nativeToolchains[runtime].find(choice.Line)
	if !ok || !fullVersion.MatchString(version) || version == line.version || !strings.HasPrefix(version, choice.Line+".") {
		return ""
	}
	return version
}

// exactImage names the official tag for an exact version: the line's tag with
// its line replaced by the version (python:3.11-bookworm → python:3.11.9-bookworm).
func exactImage(choice NativeChoice) (repo, tag string, ok bool) {
	image := choice.Image
	if choice.upstream != "" {
		image = choice.upstream
	}
	ref, _, _ := strings.Cut(image, "@")
	repo, lineTag, found := strings.Cut(ref, ":")
	if !found || choice.Exact == "" || !strings.HasPrefix(lineTag, choice.Line) {
		return "", "", false
	}
	return repo, choice.Exact + strings.TrimPrefix(lineTag, choice.Line), true
}
