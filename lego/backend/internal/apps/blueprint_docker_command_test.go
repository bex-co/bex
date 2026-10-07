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

package apps

import (
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestBlueprintDockerCommandCompilerAndParseAgree (w5/102): a Blueprint
// dockerCommand is accepted only where the build runs a container command,
// runtime image or runtime docker that is not the schema's spelling of a
// buildpack build, and the compiler and the parse agree on every service.
// Before, a buildpack build's dockerCommand became its start command, while
// a create ignored it. A static site's is refused by Render's schema itself.
func TestBlueprintDockerCommandCompilerAndParseAgree(t *testing.T) {
	for _, tc := range []struct {
		name, service string
		accepted      bool
	}{
		{"a Dockerfile build", `{type: web, name: api, runtime: docker, repo: https://github.com/bex-co/api, dockerCommand: bin/server}`, true},
		{"a Dockerfile build with the auto builder", `{type: web, name: api, runtime: docker, repo: https://github.com/bex-co/api, x-bex: {builder: auto}, dockerCommand: bin/server}`, true},
		{"a prebuilt image", `{type: web, name: api, runtime: image, image: {url: "nginx:1.27"}, dockerCommand: nginx}`, true},
		{"a prebuilt image's cron", `{type: cron, name: nightly, runtime: image, image: {url: "busybox:1.36"}, schedule: "0 2 * * *", dockerCommand: echo hi}`, true},
		{"a buildpack build", `{type: web, name: api, runtime: docker, repo: https://github.com/bex-co/api, x-bex: {builder: buildpack}, dockerCommand: npm start}`, false},
		{"a buildpack build's cron", `{type: cron, name: nightly, runtime: docker, repo: https://github.com/bex-co/api, x-bex: {builder: buildpack}, schedule: "0 2 * * *", dockerCommand: echo hi}`, false},
		{"a native runtime", `{type: web, name: api, runtime: node, repo: https://github.com/bex-co/api, dockerCommand: npm start}`, false},
		{"a static site", `{type: web, name: site, runtime: static, repo: https://github.com/bex-co/site, staticPublishPath: dist, dockerCommand: serve}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, ir, problems := CompileBlueprintIR("services:\n  - " + tc.service + "\n")
			compilerRefused := false
			for _, problem := range problems {
				if problem.Path != "#/services/0/dockerCommand" {
					t.Fatalf("an unrelated problem: %+v", problem)
				}
				compilerRefused = true
			}
			_, err := parseCompiledStack(blueprintParseOverrides{}, source, ir)
			parseRefused := err != nil && errors.Is(err, core.ErrBadRequest) && strings.Contains(err.Error(), "dockerCommand")
			if err != nil && !parseRefused {
				t.Fatalf("the parse failed for another reason: %v", err)
			}
			if compilerRefused == tc.accepted || parseRefused == tc.accepted {
				t.Fatalf("accepted %v, but the compiler refused %v (problems %+v) and the parse refused %v (%v)",
					tc.accepted, compilerRefused, problems, parseRefused, err)
			}
		})
	}
}
