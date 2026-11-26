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

package toolchains

import (
	"cmp"
	"fmt"
	"strings"
	"time"
)

// Python is the reviewed set of Python lines (w8/m51, docs/ADR060 D7
// addendum): the official images a native build runs on, and, for every
// bookworm line, the build an agent session runs (lego/agent-image/Dockerfile
// copies each from the same digest, w4/m544). Patch upgrades are deliberate
// reviewed changes; last-reviewed resolution times live in
// lego/operator/internal/build/toolchain-freshness.json and must move with
// each digest.
var Python = Toolchain{Label: "Python", DefaultLine: "3.13", Lines: []Line{
	{Line: "3.7", Version: "3.7.17", Image: "python:3.7-bullseye@sha256:f36f6fe9ddb2622aab9803944cbe7e2bc50be75d624a081f7895e60e7c6b7f0a"},
	{Line: "3.8", Version: "3.8.20", Image: "python:3.8-bookworm@sha256:d411270700143fa2683cc8264d9fa5d3279fd3b6afff62ae81ea2f9d070e390c"},
	{Line: "3.9", Version: "3.9.25", Image: "python:3.9-bookworm@sha256:ff12c273af1e1814efcd8dfdefe16a70a4d901afe94dce721ffed8a34176f285"},
	{Line: "3.10", Version: "3.10.22", Image: "python:3.10-bookworm@sha256:f72a86806f92fe13a49f49aeca1db0b19ea5cf4a0470b842a01b0e14314945b8"},
	{Line: "3.11", Version: "3.11.18", Image: "python:3.11-bookworm@sha256:9884c17ef6d305098e3111a7d285941b247fadbd5e0292563b32f8c04bb8ebe0"},
	{Line: "3.12", Version: "3.12.16", Image: "python:3.12-bookworm@sha256:5f0cc9bc924d72614831d2b1575de5d7c4114609a0d42ca60dfc2f0abca10dd6"},
	{Line: "3.13", Version: "3.13.18", Image: "python:3.13-bookworm@sha256:ee1a30c36c5cf43df939eb664489fb167e1bbe07cd884ea694054f6aff4dd9d9"},
	{Line: "3.14", Version: "3.14.10", Image: "python:3.14-bookworm@sha256:fbaf91204f2758a70a7f7d6cf3c2871851227152a54e6a7b5b59941c4566b6b5"},
	{Line: "3.15", Version: "3.15.2", Image: "python:3.15-bookworm@sha256:2bf848386eab766326ae3fc2f061788ae858d7945aefce74967d4a861ebdad12"},
}}

// PythonDefaults are Render's default Python patches by creation date, oldest
// first, from the row in force when bex's first service could be created
// (2026-06-05; w8/174, w4/620, w3/m636). A step's line is the default line of
// something created in it. They stay exact even where a step equals the
// line's reviewed patch, so a later line-pin bump never moves a default (the
// w8/217 regression class).
var PythonDefaults = []DatedDefault{
	// The official python:3.14.3 and python:3.14.8 images are no longer
	// rebuilt, so they predate the line's Brotli fix (w8/222).
	{RenderDefaults20260211, "3.14.3"},
	{RenderDefaults20261012, "3.14.8"},
	{RenderDefaults20261103, "3.14.10"},
}

// Choice is the Python line a request resolves to, and why.
type Choice struct {
	Line  string // "3.13"
	Patch string // the line's reviewed patch, which its pinned image carries
	Image string // the line's pinned image
	// Source is the signal that chose the line: "PYTHON_VERSION=3.11.9",
	// ".python-version", or "" for the default.
	Source string
	// Exact is the fully qualified version to run instead of Patch: one the
	// request pinned (w8/175), or Render's dated default, when either differs
	// from the reviewed patch. The official image of it is ExactImage.
	Exact string
}

// Version is the Python the choice runs: Exact when set, else the patch.
func (c Choice) Version() string { return cmp.Or(c.Exact, c.Patch) }

// ExactImage is the official image tag of Exact (python:3.11.9-bookworm), or
// "" when the choice runs the line's reviewed patch.
func (c Choice) ExactImage() string {
	repo, suffix, ok := ImageVariant(c.Image, c.Line)
	if !ok || c.Exact == "" {
		return ""
	}
	return repo + ":" + c.Exact + suffix
}

// ResolvePython picks Python with Render's precedence
// (render.com/docs/python-version): PYTHON_VERSION, then the first line of a
// root .python-version, then Render's default for something created at
// created. A version maps onto its major.minor line, and a fully qualified
// one is then run exactly. env is the effective environment; readFile reads a
// file relative to the root directory. A request no reviewed line satisfies
// is an *UnsupportedVersionError naming it.
func ResolvePython(created time.Time, env map[string]string, readFile func(string) ([]byte, bool)) (Choice, error) {
	if request := strings.TrimSpace(env["PYTHON_VERSION"]); request != "" {
		return pythonRequest(request, "PYTHON_VERSION="+request, "PYTHON_VERSION="+request)
	}
	if raw, ok := readFile(".python-version"); ok {
		if request := FirstMeaningfulLine(raw); request != "" {
			return pythonRequest(request, ".python-version", fmt.Sprintf("%s %q", ".python-version", request))
		}
	}
	// A zero created (unknown age) has no dated default and keeps the legacy
	// line's reviewed patch.
	version := DatedDefaultAt(PythonDefaults, created)
	name, dated := RequestLine(version)
	if !dated {
		name = Python.DefaultLine
	}
	line, _ := Python.Find(name) // every dated default is on a line (TestPythonTableIsWellFormed)
	choice := pythonChoice(line, "")
	if dated && version != line.Version {
		choice.Exact = version
	}
	return choice, nil
}

// pythonRequest resolves one signal's request; what names it in a refusal.
func pythonRequest(request, source, what string) (Choice, error) {
	name, ok := RequestLine(request)
	line, found := Python.Find(name)
	if !ok || !found {
		return Choice{}, Unsupported(what, Python.Label, Python.lineNames())
	}
	choice := pythonChoice(line, source)
	choice.Exact = ExactVersion(request, line.Line, line.Version)
	return choice, nil
}

func pythonChoice(line Line, source string) Choice {
	return Choice{Line: line.Line, Patch: line.Version, Image: line.Image, Source: source}
}
