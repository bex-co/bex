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

package agentsession

import (
	"errors"
	"testing"
)

func boundLabels(t *testing.T, workspace, session, repository, branch string) map[string]string {
	t.Helper()
	labels, err := BindingLabels(session, repository, branch)
	if err != nil {
		t.Fatal(err)
	}
	labels[LabelWorkspace] = workspace
	labels[LabelRegime] = RegimeSandbox
	return labels
}

func TestAuthorizePodBindsEverySessionTarget(t *testing.T) {
	labels := boundLabels(t, "tea-a", "ags-one", "Octo/Repo.git", "bex-agent/task-1")
	request := MintRequest{SessionID: "ags-one", Repository: "octo/repo", Branch: "bex-agent/task-1"}
	got, err := AuthorizePod("tea-a-sandbox", "sandbox-one", "uid-one", labels, request)
	if err != nil {
		t.Fatal(err)
	}
	if got.Workspace != "tea-a" || got.PodName != "sandbox-one" || got.PodUID != "uid-one" || got.Repository != "octo/repo" {
		t.Fatalf("verified request = %+v", got)
	}

	cases := []struct {
		name      string
		namespace string
		labels    map[string]string
		request   MintRequest
	}{
		{"other workspace namespace", "tea-b-sandbox", labels, request},
		{"other session", "tea-a-sandbox", labels, MintRequest{SessionID: "ags-two", Repository: "octo/repo", Branch: "bex-agent/task-1"}},
		{"other repo", "tea-a-sandbox", labels, MintRequest{SessionID: "ags-one", Repository: "octo/other", Branch: "bex-agent/task-1"}},
		{"other branch", "tea-a-sandbox", labels, MintRequest{SessionID: "ags-one", Repository: "octo/repo", Branch: "bex-agent/task-2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := AuthorizePod(tc.namespace, "sandbox-one", "uid-one", tc.labels, tc.request); !errors.Is(err, ErrForbidden) {
				t.Fatalf("AuthorizePod error = %v, want forbidden", err)
			}
		})
	}
}

func TestValidateBranchConfinesMint(t *testing.T) {
	for _, branch := range []string{"main", "feature/x", "bex-agent/", "bex-agent/a..b", "bex-agent/a b", "bex-agent/a~b"} {
		if err := ValidateBranch(branch); !errors.Is(err, ErrForbidden) {
			t.Errorf("ValidateBranch(%q) = %v, want forbidden", branch, err)
		}
	}
	if err := ValidateBranch("bex-agent/session-123/fix"); err != nil {
		t.Fatalf("valid session branch: %v", err)
	}
}
