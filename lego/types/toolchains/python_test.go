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
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func files(contents map[string]string) func(string) ([]byte, bool) {
	return func(name string) ([]byte, bool) {
		content, ok := contents[name]
		return []byte(content), ok
	}
}

// The cases Render's python-version page documents, resolved the way a
// native build and an agent session both resolve them (w4/m544).
func TestResolvePython(t *testing.T) {
	created := time.Date(2026, time.November, 20, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name          string
		created       time.Time
		env, files    map[string]string
		line, version string
		source, exact string
	}{
		{name: "PYTHON_VERSION pins the patch exactly", created: created, env: map[string]string{"PYTHON_VERSION": "3.11.9"}, line: "3.11", version: "3.11.9", source: "PYTHON_VERSION=3.11.9", exact: "3.11.9"},
		{name: "PYTHON_VERSION beats .python-version", created: created, env: map[string]string{"PYTHON_VERSION": "3.11.9"}, files: map[string]string{".python-version": "3.13\n"}, line: "3.11", version: "3.11.9", source: "PYTHON_VERSION=3.11.9", exact: "3.11.9"},
		{name: "PYTHON_VERSION naming the reviewed patch runs it", created: created, env: map[string]string{"PYTHON_VERSION": "3.12.16"}, line: "3.12", version: "3.12.16", source: "PYTHON_VERSION=3.12.16"},
		{name: "a major.minor PYTHON_VERSION runs the line's patch", created: created, env: map[string]string{"PYTHON_VERSION": " 3.12 "}, line: "3.12", version: "3.12.16", source: "PYTHON_VERSION=3.12"},
		{name: ".python-version with a line gets the line's patch", created: created, files: map[string]string{".python-version": "3.13\n"}, line: "3.13", version: "3.13.18", source: ".python-version"},
		{name: ".python-version skips comments and blank lines", created: created, files: map[string]string{".python-version": "# pyenv\n\n  python-3.10.4  \n3.12\n"}, line: "3.10", version: "3.10.4", source: ".python-version", exact: "3.10.4"},
		{name: "an empty .python-version is no signal", created: created, files: map[string]string{".python-version": "\n# none\n"}, line: "3.14", version: "3.14.10"},
		{name: "no signal gets Render's dated default", created: created, line: "3.14", version: "3.14.10"},
		// w3/m636: every row of Render's table from bex's first service on,
		// at its boundary and inside it.
		{name: "created exactly at 2026-11-03 gets 3.14.10", created: RenderDefaults20261103, line: "3.14", version: "3.14.10"},
		{name: "created 2026-11-05 gets 3.14.10", created: day(2026, time.November, 5), line: "3.14", version: "3.14.10"},
		{name: "a service from before Render's 2026-11-03 move keeps 3.14.8", created: RenderDefaults20261103.Add(-time.Hour), line: "3.14", version: "3.14.8", exact: "3.14.8"},
		{name: "created 2026-10-15 gets 3.14.8", created: day(2026, time.October, 15), line: "3.14", version: "3.14.8", exact: "3.14.8"},
		{name: "created exactly at 2026-10-12 gets 3.14.8", created: RenderDefaults20261012, line: "3.14", version: "3.14.8", exact: "3.14.8"},
		{name: "created the hour before 2026-10-12 gets 3.14.3", created: RenderDefaults20261012.Add(-time.Hour), line: "3.14", version: "3.14.3", exact: "3.14.3"},
		{name: "created 2026-05-01 gets 3.14.3", created: day(2026, time.May, 1), line: "3.14", version: "3.14.3", exact: "3.14.3"},
		{name: "created when bex's first service could be gets 3.14.3", created: day(2026, time.June, 5), line: "3.14", version: "3.14.3", exact: "3.14.3"},
		{name: "created exactly at 2026-02-11 gets 3.14.3", created: RenderDefaults20260211, line: "3.14", version: "3.14.3", exact: "3.14.3"},
		{name: "created before the table's first row keeps the legacy line", created: RenderDefaults20260211.Add(-time.Hour), line: "3.13", version: "3.13.18"},
		{name: "an unknown creation time keeps the legacy line", line: "3.13", version: "3.13.18"},
		{name: "a signal wins over the dated default", created: created, files: map[string]string{".python-version": "3.12"}, line: "3.12", version: "3.12.16", source: ".python-version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolvePython(tc.created, tc.env, files(tc.files))
			if err != nil {
				t.Fatal(err)
			}
			if got.Line != tc.line || got.Version() != tc.version || got.Source != tc.source || got.Exact != tc.exact {
				t.Fatalf("got %+v (version %s), want line %s version %s source %q exact %q", got, got.Version(), tc.line, tc.version, tc.source, tc.exact)
			}
			if line, _ := Python.Find(got.Line); got.Image != line.Image || got.Patch != line.Version {
				t.Fatalf("choice %+v does not carry its line's image and patch %+v", got, line)
			}
		})
	}
}

func TestResolvePythonRefusesUnsupportedByName(t *testing.T) {
	for _, tc := range []struct {
		env, files map[string]string
		want       string
	}{
		{env: map[string]string{"PYTHON_VERSION": "2.7.18"}, want: "PYTHON_VERSION=2.7.18 is not a supported Python version; supported Python lines are 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14, 3.15"},
		{env: map[string]string{"PYTHON_VERSION": "3"}, want: "PYTHON_VERSION=3 is not a supported Python version"},
		{env: map[string]string{"PYTHON_VERSION": "latest"}, want: "PYTHON_VERSION=latest is not"},
		{files: map[string]string{".python-version": "pypy3.10\n"}, want: `.python-version "pypy3.10" is not a supported Python version`},
		{files: map[string]string{".python-version": "3.16.0"}, want: `.python-version "3.16.0" is not`},
		{env: map[string]string{"PYTHON_VERSION": strings.Repeat("9", 200)}, want: "…"},
	} {
		_, err := ResolvePython(time.Now(), tc.env, files(tc.files))
		var unsupported *UnsupportedVersionError
		if !errors.As(err, &unsupported) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("env %v files %v: err = %v, want an UnsupportedVersionError containing %q", tc.env, tc.files, err, tc.want)
		}
	}
}

func TestPythonChoiceExactImage(t *testing.T) {
	exact, err := ResolvePython(time.Time{}, map[string]string{"PYTHON_VERSION": "3.11.9"}, files(nil))
	if err != nil || exact.ExactImage() != "python:3.11.9-bookworm" {
		t.Fatalf("exact image = %q, %v", exact.ExactImage(), err)
	}
	legacy, _ := ResolvePython(time.Time{}, map[string]string{"PYTHON_VERSION": "3.7.3"}, files(nil))
	if legacy.ExactImage() != "python:3.7.3-bullseye" {
		t.Fatalf("a bullseye line's exact image = %q", legacy.ExactImage())
	}
	reviewed, _ := ResolvePython(time.Time{}, nil, files(nil))
	if reviewed.ExactImage() != "" {
		t.Fatalf("the reviewed patch needs no exact image, got %q", reviewed.ExactImage())
	}
}

// Every line pins an image of its own line by digest, ascending, and the
// legacy default and every dated default are lines of the table, the dated
// ones in date order: the reviewed table is the only input either caller
// reads.
func TestPythonTableIsWellFormed(t *testing.T) {
	previous := ""
	for _, line := range Python.Lines {
		repo, suffix, ok := ImageVariant(line.Image, line.Line)
		if !ok || repo != "python" || (suffix != "-bookworm" && suffix != "-bullseye") || !strings.Contains(line.Image, "@sha256:") {
			t.Errorf("line %s image %s is not a digest-pinned official python image of its line", line.Line, line.Image)
		}
		if !strings.HasPrefix(line.Version, line.Line+".") || fullVersion.FindString(line.Version) == "" {
			t.Errorf("line %s patch %s is not a patch of the line", line.Line, line.Version)
		}
		if previous != "" && !lineLess(previous, line.Line) {
			t.Errorf("lines are not ascending: %s before %s", previous, line.Line)
		}
		previous = line.Line
	}
	if _, ok := Python.Find(Python.DefaultLine); !ok {
		t.Errorf("legacy default %s is not a line", Python.DefaultLine)
	}
	for i, step := range PythonDefaults {
		line, ok := RequestLine(step.Version)
		if _, found := Python.Find(line); !ok || !found {
			t.Errorf("dated default %s is on no line", step.Version)
		}
		if i > 0 && !PythonDefaults[i-1].From.Before(step.From) {
			t.Errorf("dated default %s (%s) is not after %s (%s)", step.Version, step.From, PythonDefaults[i-1].Version, PythonDefaults[i-1].From)
		}
	}
}

// lineLess orders two major.minor lines numerically.
func lineLess(a, b string) bool {
	var am, an, bm, bn int
	_, _ = fmt.Sscanf(a, "%d.%d", &am, &an)
	_, _ = fmt.Sscanf(b, "%d.%d", &bm, &bn)
	return am < bm || (am == bm && an < bn)
}

// w4/620, w3/m636: a later bump of a line's reviewed patch must not move a
// dated default: something created in any step still asks for exactly that
// step's patch. Only something of unknown age follows the legacy line's pin.
func TestDatedDefaultSurvivesALinePinBump(t *testing.T) {
	saved := Python
	t.Cleanup(func() { Python = saved })
	Python.Lines = append([]Line(nil), saved.Lines...)
	for i, bump := range Python.Lines {
		switch bump.Line {
		case "3.13":
			Python.Lines[i].Version = "3.13.19"
		case "3.14":
			Python.Lines[i].Version = "3.14.11"
		}
	}
	for created, want := range map[time.Time]string{
		day(2026, time.May, 1):      "3.14.3",
		day(2026, time.October, 15): "3.14.8",
		day(2026, time.November, 5): "3.14.10",
	} {
		got, err := ResolvePython(created, nil, files(nil))
		if err != nil || got.Exact != want || got.Version() != want {
			t.Errorf("default for %s after the 3.13.19 and 3.14.11 line bumps = %+v, %v; want exactly %s", created.Format(time.DateOnly), got, err, want)
		}
	}
	if got, _ := ResolvePython(time.Time{}, nil, files(nil)); got.Line != "3.13" || got.Exact != "" || got.Version() != "3.13.19" {
		t.Errorf("an unknown-age default = %+v; want the bumped 3.13 line's 3.13.19", got)
	}
}

func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}
