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

package github

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

func TestDetectRuntime(t *testing.T) {
	tests := []struct {
		name    string
		entries []RepoTreeEntry
		want    RuntimeDetection
	}{
		{"docker", []RepoTreeEntry{{Name: "Dockerfile", Type: "file"}}, RuntimeDetection{"docker", "Dockerfile"}},
		{"go", []RepoTreeEntry{{Name: "go.mod", Type: "file"}}, RuntimeDetection{"go", "go.mod"}},
		{"node", []RepoTreeEntry{{Name: "package.json", Type: "file"}}, RuntimeDetection{"node", "package.json"}},
		{"python requirements", []RepoTreeEntry{{Name: "requirements.txt", Type: "file"}}, RuntimeDetection{"python", "requirements.txt"}},
		{"python pyproject", []RepoTreeEntry{{Name: "pyproject.toml", Type: "file"}}, RuntimeDetection{"python", "pyproject.toml"}},
		{"ruby", []RepoTreeEntry{{Name: "Gemfile", Type: "file"}}, RuntimeDetection{"ruby", "Gemfile"}},
		{"elixir", []RepoTreeEntry{{Name: "mix.exs", Type: "file"}}, RuntimeDetection{"elixir", "mix.exs"}},
		{"rust", []RepoTreeEntry{{Name: "Cargo.toml", Type: "file"}}, RuntimeDetection{"rust", "Cargo.toml"}},
		{"docker wins over native", []RepoTreeEntry{{Name: "package.json", Type: "file"}, {Name: "Dockerfile", Type: "file"}}, RuntimeDetection{"docker", "Dockerfile"}},
		{"two python manifests agree", []RepoTreeEntry{{Name: "pyproject.toml", Type: "file"}, {Name: "requirements.txt", Type: "file"}}, RuntimeDetection{"python", "requirements.txt"}},
		{"conflicting native manifests", []RepoTreeEntry{{Name: "go.mod", Type: "file"}, {Name: "package.json", Type: "file"}}, RuntimeDetection{}},
		{"manifest-named directory ignored", []RepoTreeEntry{{Name: "Cargo.toml", Type: "dir"}}, RuntimeDetection{}},
		{"unknown", []RepoTreeEntry{{Name: "README.md", Type: "file"}}, RuntimeDetection{}},
		{"empty", nil, RuntimeDetection{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectRuntime(tt.entries); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("DetectRuntime() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func runtimeProbeService(client *fakeClient) *Service {
	st := newFakeStore()
	st.conns = append(st.conns, store.GitConnection{
		WorkspaceID: core.DefaultTenant, InstallationID: 42, AccountLogin: "acme",
	})
	return &Service{
		Base:   &core.Base{Namespace: "default"},
		GitHub: client,
		Store:  st,
	}
}

func TestProbeRepoTreeNestedRootFiltersDirectories(t *testing.T) {
	client := &fakeClient{
		token: "ghs-tree",
		tree: []RepoTreeEntry{
			{Name: "go.mod", Type: "file"},
			{Name: "package.json", Type: "dir"},
		},
	}
	svc := runtimeProbeService(client)
	probe, err := svc.ProbeRepoTree(context.Background(), "", "https://github.com/acme/mono", "main", "services/api")
	if err != nil {
		t.Fatal(err)
	}
	if probe.Unknown || !reflect.DeepEqual(probe.Entries, []RepoTreeEntry{{Name: "go.mod", Type: "file"}}) {
		t.Fatalf("probe = %+v", probe)
	}
	wantCall := []string{"ghs-tree", "acme", "mono", "services/api", "main"}
	if !reflect.DeepEqual(client.gotTree, wantCall) {
		t.Fatalf("ListRepoTree call = %v, want %v", client.gotTree, wantCall)
	}
}

func TestProbeRepoTreeExpectedFailuresAreTypedUnknown(t *testing.T) {
	tests := []struct {
		name   string
		client *fakeClient
	}{
		{"empty directory", &fakeClient{token: "ghs"}},
		{"missing directory", &fakeClient{token: "ghs", treeErr: &APIError{Status: 404}}},
		{"rate limited", &fakeClient{token: "ghs", treeErr: &APIError{Status: 429}}},
		{"forbidden grant", &fakeClient{token: "ghs", treeErr: &APIError{Status: 403}}},
		{"network failure", &fakeClient{token: "ghs", treeErr: errors.New("network down")}},
		{"token failure", &fakeClient{tokenErr: errors.New("token unavailable")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			probe, err := runtimeProbeService(tt.client).ProbeRepoTree(context.Background(), "", "https://github.com/acme/mono", "main", "")
			if err != nil || !probe.Unknown || len(probe.Entries) != 0 {
				t.Fatalf("probe = %+v, err=%v; want typed unknown", probe, err)
			}
		})
	}
}

func TestDetectRepoRuntimeAndInputGuards(t *testing.T) {
	client := &fakeClient{token: "ghs", tree: []RepoTreeEntry{{Name: "Cargo.toml", Type: "file"}}}
	svc := runtimeProbeService(client)
	detection, err := svc.DetectRepoRuntime(context.Background(), "", "https://github.com/acme/mono", "main", "crates/api")
	if err != nil || detection.Runtime != "rust" || detection.MatchedManifest != "Cargo.toml" {
		t.Fatalf("detection = %+v, err=%v", detection, err)
	}
	client.treeErr = errors.New("must not be called after a positive detection cache hit")
	cached, err := svc.DetectRepoRuntime(context.Background(), "", "https://github.com/acme/mono", "main", "crates/api")
	if err != nil || cached != detection {
		t.Fatalf("cached detection = %+v, err=%v; want %+v", cached, err, detection)
	}

	if _, err := svc.DetectRepoRuntime(context.Background(), "", "https://github.com/acme/mono", "main", "../secret"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("invalid rootDir err = %v, want ErrBadRequest", err)
	}
	unknown, err := svc.DetectRepoRuntime(context.Background(), "", "https://gitlab.com/acme/mono", "main", "")
	if err != nil || unknown.Runtime != "" {
		t.Fatalf("non-GitHub detection = %+v, err=%v", unknown, err)
	}
}

func TestDetectRepoRuntimeDeniesUnauthorizedCaller(t *testing.T) {
	svc := runtimeProbeService(&fakeClient{})
	svc.Base.Authz = allowChecker{}
	_, err := svc.DetectRepoRuntime(withIdentity(context.Background()), "", "https://github.com/acme/mono", "main", "")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}
