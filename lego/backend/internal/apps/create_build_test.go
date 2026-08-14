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
	"strings"
	"testing"
)

// TestResolveBuildStrategy pins the runtime x builder build-strategy matrix:
// explicit builder (or auto) without a runtime, dockerfile for runtime docker,
// auto for a prebuilt image, native for a Blueprint language runtime, and the
// exact rejection for every contradictory combination.
func TestResolveBuildStrategy(t *testing.T) {
	repoReq := CreateRequest{Repo: "https://github.com/acme/web", BuildCommand: "make", StartCommand: "./web"}
	cases := []struct {
		name        string
		req         CreateRequest
		wantBuilder string
		wantRuntime string
		wantErr     string
	}{
		{name: "empty defaults", req: CreateRequest{}, wantBuilder: "", wantRuntime: ""},
		{name: "builder auto", req: CreateRequest{Builder: "auto"}, wantBuilder: "auto"},
		{name: "builder buildpack", req: CreateRequest{Builder: "buildpack"}, wantBuilder: "buildpack"},
		{name: "builder dockerfile", req: CreateRequest{Builder: "dockerfile"}, wantBuilder: "dockerfile"},
		{name: "unknown builder", req: CreateRequest{Builder: "bazel"}, wantErr: "builder must be auto, buildpack, or dockerfile"},
		{name: "runtime docker selects dockerfile", req: CreateRequest{Runtime: "docker"}, wantBuilder: "dockerfile", wantRuntime: "docker"},
		{name: "runtime is case and space folded", req: CreateRequest{Runtime: "  Docker "}, wantBuilder: "dockerfile", wantRuntime: "docker"},
		{name: "runtime with auto builder allowed", req: CreateRequest{Runtime: "docker", Builder: "auto"}, wantBuilder: "dockerfile", wantRuntime: "docker"},
		{name: "runtime and builder both selecting", req: CreateRequest{Runtime: "docker", Builder: "buildpack"}, wantErr: "runtime and builder cannot both select a build strategy"},
		{name: "runtime image", req: CreateRequest{Runtime: "image", Image: "nginx:1"}, wantBuilder: "auto", wantRuntime: "image"},
		{name: "runtime image without image", req: CreateRequest{Runtime: "image"}, wantErr: "runtime image requires image and no repo"},
		{name: "runtime image with repo", req: CreateRequest{Runtime: "image", Image: "nginx:1", Repo: "https://github.com/acme/web"}, wantErr: "runtime image requires image and no repo"},
		{name: "native runtime", req: CreateRequest{Runtime: "node", Repo: repoReq.Repo, BuildCommand: repoReq.BuildCommand, StartCommand: repoReq.StartCommand}, wantBuilder: "native", wantRuntime: "node"},
		{name: "native runtime without repo", req: CreateRequest{Runtime: "go", BuildCommand: "make", StartCommand: "./x"}, wantErr: "native runtime go requires repo"},
		{name: "native runtime without commands", req: CreateRequest{Runtime: "rust", Repo: repoReq.Repo}, wantErr: "native runtime rust requires buildCommand and startCommand"},
		{name: "unsupported runtime", req: CreateRequest{Runtime: "cobol"}, wantErr: `unsupported runtime "cobol"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime, builder, err := resolveBuildStrategy(tc.req)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveBuildStrategy: %v", err)
			}
			if builder != tc.wantBuilder || runtime != tc.wantRuntime {
				t.Fatalf("builder/runtime = %q/%q, want %q/%q", builder, runtime, tc.wantBuilder, tc.wantRuntime)
			}
		})
	}
}
