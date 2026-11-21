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

package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/types/toolchains"
)

const agentDockerfile = "../../../agent-image/Dockerfile"

var (
	pythonVersionsArg = regexp.MustCompile(`(?m)^ARG PYTHON_VERSIONS="([^"]*)"$`)
	// Each line's stage is the official image, assembled by python-tree.sh,
	// and copied into the final stage as one linked layer.
	pythonStage = regexp.MustCompile(`(?m)^FROM (python:\S+) AS python-(\S+)\nRUN --mount=type=bind,source=agent-image/python-tree.sh,target=/python-tree.sh sh /python-tree.sh (\S+)$`)
	pythonCopy  = regexp.MustCompile(`(?m)^COPY --link --from=python-(\S+) /out/ /$`)
)

// imagePythons is what an agent image Dockerfile bakes: the patch list its
// check verifies, each python-<line> stage's image, and the lines copied.
func imagePythons(dockerfile string) (versions []string, stages map[string]string, copied []string, err error) {
	m := pythonVersionsArg.FindAllStringSubmatch(dockerfile, -1)
	if len(m) != 1 {
		return nil, nil, nil, fmt.Errorf("want one ARG PYTHON_VERSIONS with a default, found %d", len(m))
	}
	versions = strings.Fields(m[0][1])
	stages = map[string]string{}
	for _, stage := range pythonStage.FindAllStringSubmatch(dockerfile, -1) {
		if stage[2] != stage[3] {
			return nil, nil, nil, fmt.Errorf("stage python-%s assembles Python %s", stage[2], stage[3])
		}
		stages[stage[2]] = stage[1]
	}
	for _, c := range pythonCopy.FindAllStringSubmatch(dockerfile, -1) {
		copied = append(copied, c[1])
	}
	return versions, stages, copied, nil
}

// pythonDrift lists how a Dockerfile's baked Pythons differ from what
// sessions run: every bookworm line of the shared table, at its reviewed
// patch, from the digest a native build of the line runs on.
func pythonDrift(dockerfile string) []string {
	versions, stages, copied, err := imagePythons(dockerfile)
	if err != nil {
		return []string{err.Error()}
	}
	var drift []string
	var want []string
	for _, line := range toolchains.Python.Lines {
		if _, ok := sessionLine(line); !ok {
			if _, baked := stages[line.Line]; baked {
				drift = append(drift, fmt.Sprintf("Python %s has no %s build, but a python-%s stage copies one", line.Line, strings.TrimPrefix(sessionVariant, "-"), line.Line))
			}
			continue
		}
		want = append(want, line.Version)
		if image := stages[line.Line]; image != line.Image {
			drift = append(drift, fmt.Sprintf("stage python-%s is %q, want the table's %s assembled by python-tree.sh", line.Line, image, line.Image))
		}
		if !slices.Contains(copied, line.Line) {
			drift = append(drift, fmt.Sprintf("the final stage does not copy python-%s", line.Line))
		}
		delete(stages, line.Line)
	}
	if !slices.Equal(versions, want) {
		drift = append(drift, fmt.Sprintf("PYTHON_VERSIONS is %q, want %q", strings.Join(versions, " "), strings.Join(want, " ")))
	}
	for line, image := range stages {
		drift = append(drift, fmt.Sprintf("stage python-%s (%s) is no session line of the table", line, image))
	}
	if len(copied) != len(want) {
		drift = append(drift, fmt.Sprintf("the final stage copies %d lines, want %d", len(copied), len(want)))
	}
	return drift
}

// The agent image bakes exactly the table's session lines (w4/m544): a
// refreshed digest or patch in lego/types/toolchains fails here until the
// Dockerfile follows it, and a hand edit of the Dockerfile fails here too.
func TestAgentImageBakesTheTablesPythonLines(t *testing.T) {
	raw, err := os.ReadFile(agentDockerfile)
	if err != nil {
		t.Fatal(err)
	}
	if drift := pythonDrift(string(raw)); len(drift) > 0 {
		t.Fatalf("lego/agent-image/Dockerfile drifted from lego/types/toolchains/python.go:\n%s", strings.Join(drift, "\n"))
	}
	// The guard itself: one hand-edited patch, digest or stage is drift.
	dockerfile := string(raw)
	for name, edit := range map[string]func(string) string{
		"a patch": func(s string) string { return strings.Replace(s, " 3.11.18 ", " 3.11.17 ", 1) },
		"a digest": func(s string) string {
			line, _ := toolchains.Python.Find("3.13")
			return strings.Replace(s, line.Image, strings.Replace(line.Image, "sha256:", "sha256:0", 1), 1)
		},
		"a missing line": func(s string) string { return strings.Replace(s, " 3.15.2\"", "\"", 1) },
		"a copy":         func(s string) string { return strings.Replace(s, "COPY --link --from=python-3.9 /out/ /\n", "", 1) },
		"an assembly": func(s string) string {
			return strings.Replace(s, "/python-tree.sh 3.10\n", "/python-tree.sh 3.11\n", 1)
		},
	} {
		if edited := edit(dockerfile); edited == dockerfile || len(pythonDrift(edited)) == 0 {
			t.Errorf("changing %s by hand is not drift", name)
		}
	}
}

// Render's current default for a new service is a baked patch, so a session
// created today needs no download for it (the image's build check asserts
// the same). When Render's default moves off a line's reviewed patch, bake it.
func TestRendersCurrentPythonDefaultIsBaked(t *testing.T) {
	current := toolchains.PythonDefaults[len(toolchains.PythonDefaults)-1]
	choice, err := toolchains.ResolvePython(current.From, nil, func(string) ([]byte, bool) { return nil, false })
	if err != nil || choice.Exact != "" || choice.Version() != current.Version {
		t.Fatalf("a session created %s runs %+v (%v), want the baked %s", current.From, choice, err, current.Version)
	}
}
