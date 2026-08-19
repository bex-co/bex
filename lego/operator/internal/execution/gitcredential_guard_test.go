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

package execution_test

import (
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/publish"
)

// cloneScript returns the full command+args text of the Job's "clone"
// container — the phase that holds GIT_AUTH_TOKEN and therefore must carry the
// host-bound credential helper.
func cloneScript(t *testing.T, job *batchv1.Job) string {
	t.Helper()
	all := append([]corev1.Container{}, job.Spec.Template.Spec.InitContainers...)
	all = append(all, job.Spec.Template.Spec.Containers...)
	for _, c := range all {
		if c.Name != "clone" {
			continue
		}
		return strings.Join(append(append([]string{}, c.Command...), c.Args...), "\n")
	}
	t.Fatalf("job %s has no clone container", job.Name)
	return ""
}

// TestClonePhasesEmbedTheGuardedCredentialHelper is w1/m80 t001's guard: the
// host-bound credential helper — the only thing keeping the tenant GitHub
// token off non-GitHub hosts — existed as two verbatim copies (build clone
// initContainer, publish direct-clone container) with its SECURITY rationale
// on only one. Both Job constructors must embed the ONE shared constant; if
// either stops (say, by re-inlining an edited copy), this fails.
func TestClonePhasesEmbedTheGuardedCredentialHelper(t *testing.T) {
	buildJob := build.BuildJob(build.Options{
		Repo:        "https://github.com/acme/web",
		Name:        "web",
		AppUID:      "uid-build",
		Registry:    "zot.bex-registry.svc:5000",
		Revision:    "gen-1",
		Namespace:   "bex-build",
		CloneSecret: "clone-web",
	}, "zot.bex-registry.svc:5000/web:gen-1")
	if !strings.Contains(cloneScript(t, buildJob), execution.GitHubCredentialHelper) {
		t.Fatal("build.BuildJob clone container no longer embeds execution.GitHubCredentialHelper")
	}

	publishJob := publish.PublishJob(publish.Options{
		Repo:        "https://github.com/acme/site",
		PublishPath: "dist",
		AppID:       "site",
		AppUID:      "uid-publish",
		Revision:    "rev-1",
		Store:       publish.Store{Bucket: "bex-static", Endpoint: "https://s3.example", Secret: "s3-creds"},
		Namespace:   "bex-build",
		CloneSecret: "clone-site",
	})
	if !strings.Contains(cloneScript(t, publishJob), execution.GitHubCredentialHelper) {
		t.Fatal("publish.PublishJob clone container no longer embeds execution.GitHubCredentialHelper")
	}
}

// TestGitHubCredentialHelperStaysHostBound pins the load-bearing pieces of the
// helper itself: it must gate on the credential protocol's host line equaling
// github.com before ever echoing the token.
func TestGitHubCredentialHelperStaysHostBound(t *testing.T) {
	for _, needle := range []string{
		`[ "$h" = github.com ] || exit 0`,
		`echo "password=$GIT_AUTH_TOKEN"`,
		`credential.helper='`,
	} {
		if !strings.Contains(execution.GitHubCredentialHelper, needle) {
			t.Fatalf("GitHubCredentialHelper lost %q — the host binding is what keeps the token off non-GitHub hosts", needle)
		}
	}
}
