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

// Package toolchains holds the reviewed toolchain lines and Render's version
// rules that more than one bex module applies: a native build picks its
// toolchain from them (lego/operator/internal/build), and an agent session
// picks the Python it runs from the same table (lego/backend/cmd/agent-toolchain,
// w4/m544), so the two can never disagree. Python is the first runtime here;
// the others still live in the operator.
package toolchains

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// Line is one reviewed toolchain line: its name ("3.13"), the exact
// toolchain its pinned image carries ("3.13.18") and that image, which keeps
// a readable tag and pins its multi-arch manifest by digest.
type Line struct {
	Line, Version, Image string
}

// Toolchain is one runtime's reviewed lines, ascending.
type Toolchain struct {
	Label       string
	DefaultLine string
	Lines       []Line
}

// Find is the reviewed line named line.
func (t Toolchain) Find(line string) (Line, bool) {
	for _, l := range t.Lines {
		if l.Line == line {
			return l, true
		}
	}
	return Line{}, false
}

// lineNames lists the reviewed lines, ascending, as an error names them.
func (t Toolchain) lineNames() string {
	names := make([]string, len(t.Lines))
	for i, l := range t.Lines {
		names[i] = l.Line
	}
	return strings.Join(names, ", ")
}

// UnsupportedVersionError is a version request no reviewed line satisfies. It
// is tenant input: the build or session fails on it by name instead of
// silently running on another line.
type UnsupportedVersionError struct{ Msg string }

func (e *UnsupportedVersionError) Error() string { return e.Msg }

// Unsupported names the request the way the tenant wrote it, what (the env
// assignment `PYTHON_VERSION=2.7.18`, or the file and its quoted content),
// and the lines that are supported.
func Unsupported(what, label, supported string) error {
	return &UnsupportedVersionError{Msg: fmt.Sprintf("%s is not a supported %s version; supported %s lines are %s",
		BoundedRequest(what), label, label, supported)}
}

// BoundedRequest bounds a tenant's version request for an error message.
func BoundedRequest(what string) string {
	if len(what) > 120 {
		return strings.ToValidUTF8(what[:120], "") + "…"
	}
	return what
}

// MaxVersionFileBytes bounds what a version file may make a resolver read;
// real ones are a line, package.json/Gemfile a few KiB, and a Bundler
// lockfile (whose RUBY VERSION block sits near its end) well under it even
// for thousands of gems.
const MaxVersionFileBytes = 1 << 20

// ReadVersionFile reads name from dir, the root directory a version signal
// lives in. Only a regular file is read: a symlink is ignored, because a
// refusal quotes what it read and a link could point anywhere, such as at a
// build's env bundle beside the checkout.
func ReadVersionFile(dir, name string) ([]byte, bool) {
	path := filepath.Join(dir, name)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, false
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, MaxVersionFileBytes))
	return raw, err == nil
}

// FirstMeaningfulLine is raw's first line that is neither blank nor a #
// comment: what a single-version file (.python-version) asks for.
func FirstMeaningfulLine(raw []byte) string {
	for line := range strings.SplitSeq(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return ""
}

// RequestLine maps a plain version request onto its major.minor line:
// "3.11", "3.11.9", "v3.11" and the pyenv/rbenv-style "python-3.11.9" are all
// "3.11". A bare major or anything that is not a version is not a line.
func RequestLine(request string) (string, bool) {
	request = strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(request), "python-"), "ruby-")
	request = strings.TrimPrefix(request, "v")
	v, err := semver.NewVersion(request)
	if err != nil || strings.Count(request, ".") < 1 {
		return "", false
	}
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor()), true
}

var fullVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// ExactVersion is the fully qualified version request names within line when
// it is not patch, the line's reviewed patch, else "": a fully qualified
// request is run exactly, as Render installs it (w8/175), while a line or a
// range keeps the reviewed patch.
func ExactVersion(request, line, patch string) string {
	version := strings.ToLower(strings.TrimSpace(request))
	for _, prefix := range []string{"python-", "ruby-", "bun-", "v"} {
		version = strings.TrimPrefix(version, prefix)
	}
	if !fullVersion.MatchString(version) || version == patch || !strings.HasPrefix(version, line+".") {
		return ""
	}
	return version
}

// DatedDefault is one step of a runtime's default by creation date: services
// created from From on run Version, until the next step.
type DatedDefault struct {
	From    time.Time
	Version string
}

// DatedDefaultAt is the version of the last step dated at or before created,
// or "" for something created before the first step or of unknown age.
func DatedDefaultAt(steps []DatedDefault, created time.Time) string {
	version := ""
	if created.IsZero() {
		return version
	}
	for _, step := range steps {
		if !created.Before(step.From) {
			version = step.Version
		}
	}
	return version
}

// RenderDefaults20261103 is when Render moved its default Python to 3.14.10
// and Ruby to 3.4.12 for new services (w4/620): the 3.14 and 3.4 lines'
// reviewed patches. bex follows Render's own date, so a service created since
// then builds on what Render gives it.
var RenderDefaults20261103 = time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC)

// ImageVariant splits a line's official image (python:3.11-bookworm@sha256:…)
// into its repository and the variant suffix after the line (-bookworm). An
// exact version's official image is the repository and that version with the
// same suffix (python:3.11.9-bookworm).
func ImageVariant(image, line string) (repo, suffix string, ok bool) {
	ref, _, _ := strings.Cut(image, "@")
	repo, lineTag, found := strings.Cut(ref, ":")
	if !found || !strings.HasPrefix(lineTag, line) {
		return "", "", false
	}
	return repo, strings.TrimPrefix(lineTag, line), true
}
