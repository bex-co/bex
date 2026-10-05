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

	"github.com/Masterminds/semver/v3"
)

// NativeChoice is the toolchain line a native build runs on and why (w8/m51).
type NativeChoice struct {
	Label  string // "Python"
	Line   string // "3.11"
	Image  string // the pinned image of that line
	Source string // "PYTHON_VERSION=3.11.9", ".nvmrc", or "" for the default
}

// Narration is the build-log line that names the choice and its source.
func (c NativeChoice) Narration() string {
	from := "default"
	if c.Source != "" {
		from = "from " + c.Source
	}
	return fmt.Sprintf("==> Using %s %s (%s)", c.Label, c.Line, from)
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
//	ruby    Gemfile `ruby` → .ruby-version → default
//	elixir  ELIXIR_VERSION (→ line), ERLANG_VERSION (must match that line's OTP major)
//	go, rust  default (Render offers no Go selection; rustup applies Rust's)
//
// A plain version maps onto its line by major.minor (major for Node) — a
// requested patch is not reproduced, a recorded divergence from Render. A
// range (Node engines, Ruby requirements) resolves to the highest line whose
// pinned toolchain satisfies it. env is the build's effective environment;
// readFile reads a file relative to the service's root directory.
func ResolveNativeVersion(runtime string, env map[string]string, readFile func(string) ([]byte, bool)) (NativeChoice, error) {
	t, ok := nativeToolchains[runtime]
	if !ok {
		return NativeChoice{}, fmt.Errorf("unsupported native runtime %q", runtime)
	}
	r := resolver{t: t, env: env, readFile: readFile}
	switch runtime {
	case "python":
		return r.first(r.fromEnv("PYTHON_VERSION", r.minorLine), r.fromFile(".python-version", r.minorLine))
	case nativeNodeRuntime:
		return r.first(r.fromEnv("NODE_VERSION", r.nodeRequest), r.fromFile(".node-version", r.nodeRequest),
			r.fromFile(".nvmrc", r.nodeRequest), r.fromPackageEngines)
	case "ruby":
		return r.first(r.fromGemfile, r.fromFile(".ruby-version", r.rubyFileLine))
	case "elixir":
		return r.elixir()
	default:
		return r.choice(t.defaultLine, "")
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
	for _, s := range signals {
		line, source, found, err := s()
		if err != nil {
			return NativeChoice{}, err
		}
		if found {
			return r.choice(line, source)
		}
	}
	return r.choice(r.t.defaultLine, "")
}

func (r resolver) choice(line, source string) (NativeChoice, error) {
	l, ok := r.t.find(line)
	if !ok {
		return NativeChoice{}, fmt.Errorf("no %s line %q", r.t.label, line)
	}
	return NativeChoice{Label: r.t.label, Line: l.line, Image: l.image, Source: source}, nil
}

// unsupported names the request the way the tenant wrote it: what is the env
// assignment (`PYTHON_VERSION=2.7.18`) or the file and its quoted content.
func (r resolver) unsupported(what string) error {
	if len(what) > 120 {
		what = strings.ToValidUTF8(what[:120], "") + "…"
	}
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
var nodeLTSCodenames = map[string]string{"jod": "22", "krypton": "24"}

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
// toolchain version, preferring the newest line.
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
	return "", false
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

// elixir honors ELIXIR_VERSION (line by major.minor) and checks ERLANG_VERSION
// against the chosen line's OTP major — an Erlang the line's image does not
// carry is refused by name rather than silently ignored.
func (r resolver) elixir() (NativeChoice, error) {
	choice, err := r.first(r.fromEnv("ELIXIR_VERSION", r.minorLine))
	if err != nil {
		return NativeChoice{}, err
	}
	erlang := strings.TrimSpace(r.env["ERLANG_VERSION"])
	if erlang == "" {
		return choice, nil
	}
	line, _ := r.t.find(choice.Line)
	if major, _, _ := strings.Cut(strings.TrimPrefix(erlang, "OTP-"), "."); major != line.otp {
		return NativeChoice{}, &UnsupportedVersionError{msg: fmt.Sprintf(
			"ERLANG_VERSION=%q is not available with Elixir %s, which runs on Erlang/OTP %s; set ERLANG_VERSION to an OTP %s release or choose another ELIXIR_VERSION",
			erlang, line.line, line.otp, line.otp)}
	}
	choice.Source = strings.TrimPrefix(choice.Source+", ERLANG_VERSION="+erlang, ", ")
	return choice, nil
}
