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

package core

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w2/m365: a service re-created under a just-deleted name runs as its own
// App (CRName-xxxx) while the old one, still holding CRName, tears down.
// By name, every resolver answers the live successor — never the dying
// predecessor, and never a 409 SERVICE_NAME_AMBIGUOUS between the two.

func tearingDown(a *appv1alpha1.App) *appv1alpha1.App {
	now := metav1.Now()
	a.DeletionTimestamp = &now
	a.Finalizers = []string{"app.bex.co/finalizer"}
	return a
}

func TestByNameAJustDeletedServiceYieldsToItsSuccessor(t *testing.T) {
	old := tearingDown(workspaceApp("tea-a", "web", "srv-old"))
	successor := workspaceApp("tea-a", "web", "srv-new")
	successor.Name = CRName("tea-a", "web") + "-ab12"
	bob := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	cases := map[string]struct {
		ws  multiWorkspace
		ctx context.Context
	}{
		"default workspace (direct candidate)": {multiWorkspace{"bob": {"tea-a"}}, bob},
		"named workspace":                      {multiWorkspace{"bob": {"tea-c", "tea-a"}}, WithWorkspace(bob, "tea-a")},
		"another default (label sweep)":        {multiWorkspace{"bob": {"tea-c", "tea-a"}}, bob},
	}
	for _, displayName := range []string{"", "Web API"} {
		// A display name other than the service name takes the by-display-
		// name tier out of play, leaving the object-name candidates.
		old.Spec.DisplayName, successor.Spec.DisplayName = displayName, displayName
		for name, tc := range cases {
			t.Run(name+" displayName="+displayName, func(t *testing.T) {
				base := &Base{Client: fakeAppClient(old, successor), Namespace: "default", Workspace: tc.ws, Authz: &fakeAllowChecker{}}
				for verb, resolve := range map[string]func() (*appv1alpha1.App, error){
					"AuthorizeApp": func() (*appv1alpha1.App, error) { return base.AuthorizeApp(tc.ctx, RelCanOperate, "web") },
					"GetApp":       func() (*appv1alpha1.App, error) { return base.GetApp(tc.ctx, RelCanView, "web") },
				} {
					got, err := resolve()
					if err != nil || got.Labels[LabelAppID] != "srv-new" {
						t.Errorf("%s: got (%v, %v), want srv-new", verb, got, err)
					}
				}
				// The predecessor's own id still reaches it, for a repeat delete.
				if got, err := base.GetApp(tc.ctx, RelCanView, "srv-old"); err != nil || got.Labels[LabelAppID] != "srv-old" {
					t.Errorf("GetApp srv-old: got (%v, %v)", got, err)
				}
			})
		}
	}
}

// Alone, a service being deleted still resolves by name: a repeat DELETE
// answers as before, and the dashboard shows it until it is gone.
func TestByNameAServiceBeingDeletedAloneStillResolves(t *testing.T) {
	old := tearingDown(workspaceApp("tea-a", "web", "srv-old"))
	base := &Base{Client: fakeAppClient(old), Namespace: "default", Workspace: multiWorkspace{"bob": {"tea-a"}}, Authz: &fakeAllowChecker{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	if got, err := base.GetApp(ctx, RelCanView, "web"); err != nil || got.Labels[LabelAppID] != "srv-old" {
		t.Fatalf("GetApp web: got (%v, %v), want srv-old", got, err)
	}
}

// Without a workspace (bex-api alone), the successor answers to the public
// name through its LabelServiceName; a workspace's App of that name does not.
func TestByNameUnboundSuccessorAnswers(t *testing.T) {
	old := tearingDown(sampleApp("web", ""))
	successor := sampleApp("web-ab12", "")
	successor.Labels = map[string]string{LabelServiceName: "web"}
	other := workspaceApp("tea-a", "web", "srv-a")
	other.Namespace = "default"
	base := &Base{Client: fakeAppClient(old, successor, other), Namespace: "default", Authz: &fakeAllowChecker{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	if got, err := base.GetApp(ctx, RelCanView, "web"); err != nil || got.Name != "web-ab12" {
		t.Fatalf("GetApp web: got (%v, %v), want web-ab12", got, err)
	}
}

func TestPreferLive(t *testing.T) {
	dying := *tearingDown(sampleApp("a", ""))
	live := *sampleApp("b", "")
	if got := preferLive([]appv1alpha1.App{dying, live}); len(got) != 1 || got[0].Name != "b" {
		t.Fatalf("preferLive(dying, live) = %v, want only the live one", got)
	}
	if got := preferLive([]appv1alpha1.App{dying}); len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("preferLive(dying) = %v, want it kept: nothing else answers", got)
	}
}
