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
	"errors"
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m45: with a service named "web" in two of the caller's workspaces and no
// workspace named on the request (the pinned CLI sends none on by-path verbs),
// the by-name resolver silently picked the DEFAULT workspace's service — live,
// `bex deploys list tianpan-v4-web` with bex-canary selected answered with
// tian-personal's deploys. A name visible in more than one workspace is now a
// 409 naming every candidate id.

func workspaceApp(tenant, name, id string) *appv1alpha1.App {
	a := sampleApp(CRName(tenant, name), tenant)
	a.Namespace = tenant
	a.Labels[LabelServiceName] = name
	a.Labels[LabelAppID] = id
	return a
}

func TestByNameInSeveralWorkspacesIsAmbiguous(t *testing.T) {
	a, b := workspaceApp("tea-a", "web", "srv-a"), workspaceApp("tea-b", "web", "srv-b")
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	for name, ws := range map[string]multiWorkspace{
		"default workspace has it (direct candidate)": {"bob": {"tea-a", "tea-b"}},
		"neither is the default (fallback sweep)":     {"bob": {"tea-c", "tea-a", "tea-b"}},
	} {
		t.Run(name, func(t *testing.T) {
			base := &Base{Client: fakeAppClient(a, b), Namespace: "default", Workspace: ws, Authz: &fakeAllowChecker{}}
			for verb, resolve := range map[string]func() (*appv1alpha1.App, error){
				"AuthorizeApp can_operate": func() (*appv1alpha1.App, error) { return base.AuthorizeApp(ctx, RelCanOperate, "web") },
				"GetApp can_view":          func() (*appv1alpha1.App, error) { return base.GetApp(ctx, RelCanView, "web") },
			} {
				got, err := resolve()
				var coded *CodedError
				if !errors.Is(err, ErrConflict) || !errors.As(err, &coded) || coded.Code != "SERVICE_NAME_AMBIGUOUS" ||
					!strings.Contains(err.Error(), "srv-a") || !strings.Contains(err.Error(), "srv-b") {
					t.Errorf("%s: got (%v, %v), want a 409 naming srv-a and srv-b", verb, got, err)
				}
			}
		})
	}
}

// The rule refuses only ambiguity the caller can see, and never an address
// that already pins the service: a named workspace, a typed id, or a
// same-named service in a workspace the caller does not belong to.
func TestByNameResolvesWhenUnambiguousForTheCaller(t *testing.T) {
	a, b := workspaceApp("tea-a", "web", "srv-a"), workspaceApp("tea-b", "web", "srv-b")
	bob := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	for name, tc := range map[string]struct {
		ws     multiWorkspace
		ctx    context.Context
		target string
		want   string
	}{
		"named workspace B":          {multiWorkspace{"bob": {"tea-a", "tea-b"}}, WithWorkspace(bob, "tea-b"), "web", "srv-b"},
		"named workspace A":          {multiWorkspace{"bob": {"tea-a", "tea-b"}}, WithWorkspace(bob, "tea-a"), "web", "srv-a"},
		"typed id":                   {multiWorkspace{"bob": {"tea-a", "tea-b"}}, bob, "srv-b", "srv-b"},
		"other workspace not joined": {multiWorkspace{"bob": {"tea-a"}}, bob, "web", "srv-a"},
	} {
		t.Run(name, func(t *testing.T) {
			base := &Base{Client: fakeAppClient(a, b), Namespace: "default", Workspace: tc.ws, Authz: &fakeAllowChecker{}}
			for verb, resolve := range map[string]func() (*appv1alpha1.App, error){
				"AuthorizeApp": func() (*appv1alpha1.App, error) { return base.AuthorizeApp(tc.ctx, RelCanOperate, tc.target) },
				"GetApp":       func() (*appv1alpha1.App, error) { return base.GetApp(tc.ctx, RelCanView, tc.target) },
			} {
				got, err := resolve()
				if err != nil || got.Labels[LabelAppID] != tc.want {
					t.Errorf("%s: got (%v, %v), want %s", verb, got, err, tc.want)
				}
			}
		})
	}
}

// A membership outage while probing the other workspace fails closed instead
// of resolving the default workspace's service.
func TestByNameAmbiguityProbeFailsClosed(t *testing.T) {
	a, b := workspaceApp("tea-a", "web", "srv-a"), workspaceApp("tea-b", "web", "srv-b")
	base := &Base{Client: fakeAppClient(a, b), Namespace: "default",
		Workspace: brokenWorkspace{multiWorkspace{"bob": {"tea-a", "tea-b"}}}, Authz: &fakeAllowChecker{}}
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	if got, err := base.GetApp(ctx, RelCanView, "web"); err == nil {
		t.Errorf("GetApp = %v, want the membership outage", got)
	}
}
