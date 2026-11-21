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

// Command agent-toolchain tells the agent session driver which Python a
// session runs (w4/m544): `agent-toolchain python [dir]` resolves it with
// Render's rules from the table a native build uses (lego/types/toolchains),
// reading PYTHON_VERSION and BEX_AGENT_SESSION_CREATED_AT from its
// environment and .python-version from dir (the checkout root, "." by
// default). The agent image carries it at /usr/local/bin/agent-toolchain, so
// the TypeScript driver needs no copy of the table or the rules.
//
// It prints the selection as one JSON object and exits 0. A request no agent
// session can run exits 2 with the refusal, naming the request, on stderr;
// any other failure exits 1.
package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/types/toolchains"
)

// sessionCreatedEnv is the session's (or snapshot build's) creation time,
// RFC 3339, which picks Render's dated default as a service's creation time
// does for a native build.
const sessionCreatedEnv = "BEX_AGENT_SESSION_CREATED_AT"

// sessionVariant is the Debian release the agent image is built on. Only a
// line whose official image is on it runs in a session: 3.7's is bullseye's
// and needs its OpenSSL 1.1, so it is refused by name rather than baked.
const sessionVariant = "-bookworm"

// selection is what the driver reads.
type selection struct {
	// Line is the major.minor line ("3.11"), whose reviewed patch the image
	// bakes as /usr/local/bin/python<line>.
	Line string `json:"line"`
	// Version is the Python the session runs.
	Version string `json:"version"`
	// Baked is the line's reviewed patch, which the image carries.
	Baked string `json:"baked"`
	// Image is set when Version is not Baked: the official image the driver
	// fetches it from.
	Image string `json:"image,omitempty"`
	// Source is what chose it: PYTHON_VERSION, .python-version or default.
	// A PYTHON_VERSION secret's value is not repeated: the transcript would
	// show it redacted (ADR098 D6).
	Source string `json:"source"`
	// Narration is the transcript step's line, and Fallback the step's
	// line for Baked when Image cannot be fetched.
	Narration string `json:"narration"`
	Fallback  string `json:"fallback,omitempty"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	if len(args) == 0 || len(args) > 2 || args[0] != "python" {
		_, _ = fmt.Fprintln(stderr, "usage: agent-toolchain python [dir]")
		return 64
	}
	dir := "."
	if len(args) == 2 {
		dir = args[1]
	}
	sel, err := resolvePython(sessionCreated(getenv(sessionCreatedEnv)), map[string]string{"PYTHON_VERSION": getenv("PYTHON_VERSION")}, dir)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		var unsupported *toolchains.UnsupportedVersionError
		if errors.As(err, &unsupported) {
			return 2
		}
		return 1
	}
	if err := json.NewEncoder(stdout).Encode(sel); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// sessionCreated parses the creation time; an absent or malformed one is
// unknown, which keeps the legacy default line.
func sessionCreated(raw string) time.Time {
	created, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return created
}

func resolvePython(created time.Time, env map[string]string, dir string) (selection, error) {
	choice, err := toolchains.ResolvePython(created, env, func(name string) ([]byte, bool) { return toolchains.ReadVersionFile(dir, name) })
	if err != nil {
		return selection{}, err
	}
	line, _ := toolchains.Python.Find(choice.Line)
	if variant, ok := sessionLine(line); !ok {
		return selection{}, &toolchains.UnsupportedVersionError{Msg: fmt.Sprintf(
			"%s is not a supported Python version in an agent session: Python %s's official build is for Debian %s, not the session image's %s; supported Python lines are %s",
			choice.Source, choice.Line, variant, strings.TrimPrefix(sessionVariant, "-"), sessionLines())}
	}
	// The signal without its value: PYTHON_VERSION=3.11.9 names PYTHON_VERSION.
	name, _, _ := strings.Cut(choice.Source, "=")
	source := cmp.Or(name, "default")
	from := "from " + source
	if source == "default" {
		from = source
	}
	using := func(version string) string { return fmt.Sprintf("==> Using Python %s (%s)", version, from) }
	sel := selection{
		Line: choice.Line, Version: choice.Version(), Baked: choice.Patch, Image: choice.ExactImage(),
		Source: source, Narration: using(choice.Version()),
	}
	if sel.Image != "" {
		sel.Fallback = using(choice.Patch)
	}
	return sel, nil
}

// sessionLine reports whether a session runs line, and its official image's
// Debian release.
func sessionLine(line toolchains.Line) (variant string, ok bool) {
	_, suffix, _ := toolchains.ImageVariant(line.Image, line.Line)
	return strings.TrimPrefix(suffix, "-"), suffix == sessionVariant
}

// sessionLines lists the lines a session runs.
func sessionLines() string {
	var lines []string
	for _, line := range toolchains.Python.Lines {
		if _, ok := sessionLine(line); ok {
			lines = append(lines, line.Line)
		}
	}
	return strings.Join(lines, ", ")
}
