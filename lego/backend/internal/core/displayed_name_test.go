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

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m47: after A (created "qa-a") and B (created "qa-b") swapped names, the
// list showed B as "qa-a" while `bex deploys list qa-a` answered with A's
// deploys — resolution matched only the hidden creation name. By-name
// resolution now matches the name each service is shown as first.

func shownAs(app *appv1alpha1.App, display string) *appv1alpha1.App {
	app.Spec.DisplayName = display
	return app
}

func TestByNameResolvesTheDisplayedName(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	for name, tc := range map[string]struct {
		apps   []*appv1alpha1.App
		target string
		want   string
	}{
		"swapped names: the service shown as qa-a": {
			apps: []*appv1alpha1.App{
				shownAs(workspaceApp("tea-a", "qa-a", "srv-a"), "qa-b"),
				shownAs(workspaceApp("tea-a", "qa-b", "srv-b"), "qa-a"),
			},
			target: "qa-a", want: "srv-b",
		},
		"renamed: the new name": {
			apps:   []*appv1alpha1.App{shownAs(workspaceApp("tea-a", "old", "srv-a"), "new")},
			target: "new", want: "srv-a",
		},
		"renamed: the old name still works while nothing else shows it": {
			apps:   []*appv1alpha1.App{shownAs(workspaceApp("tea-a", "old", "srv-a"), "new")},
			target: "old", want: "srv-a",
		},
		"never renamed": {
			apps:   []*appv1alpha1.App{workspaceApp("tea-a", "web", "srv-a")},
			target: "web", want: "srv-a",
		},
	} {
		t.Run(name, func(t *testing.T) {
			cl := fakeAppClient(objectsOf(tc.apps)...)
			base := &Base{Client: cl, Namespace: "default", Workspace: multiWorkspace{"bob": {"tea-a"}}, Authz: &fakeAllowChecker{}}
			for verb, resolve := range map[string]func() (*appv1alpha1.App, error){
				"AuthorizeApp can_operate": func() (*appv1alpha1.App, error) { return base.AuthorizeApp(ctx, RelCanOperate, tc.target) },
				"GetApp can_view":          func() (*appv1alpha1.App, error) { return base.GetApp(ctx, RelCanView, tc.target) },
			} {
				got, err := resolve()
				if err != nil || got.Labels[LabelAppID] != tc.want {
					t.Errorf("%s(%q) = (%v, %v), want %s", verb, tc.target, got, err, tc.want)
				}
			}
		})
	}
}

// Duplicates that predate the rename rule are refused with both ids.
func TestByNameRefusesDuplicateDisplayedNames(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	cl := fakeAppClient(objectsOf([]*appv1alpha1.App{
		shownAs(workspaceApp("tea-a", "qa-a", "srv-a"), "same"),
		shownAs(workspaceApp("tea-a", "qa-b", "srv-b"), "same"),
	})...)
	base := &Base{Client: cl, Namespace: "default", Workspace: multiWorkspace{"bob": {"tea-a"}}, Authz: &fakeAllowChecker{}}
	for verb, resolve := range map[string]func() (*appv1alpha1.App, error){
		"AuthorizeApp": func() (*appv1alpha1.App, error) { return base.AuthorizeApp(ctx, RelCanOperate, "same") },
		"GetApp":       func() (*appv1alpha1.App, error) { return base.GetApp(ctx, RelCanView, "same") },
	} {
		var coded *CodedError
		if got, err := resolve(); !errors.As(err, &coded) || coded.Code != "SERVICE_NAME_AMBIGUOUS" {
			t.Errorf("%s = (%v, %v), want SERVICE_NAME_AMBIGUOUS", verb, got, err)
		}
	}
}

func objectsOf(apps []*appv1alpha1.App) []client.Object {
	out := make([]client.Object, len(apps))
	for i, a := range apps {
		out[i] = a
	}
	return out
}

// The account default differs from the workspace selected in the CLI. Renames
// must work in visible workspaces, and an explicit workspace must never fall
// through to an app elsewhere (including the legacy creation-name fallback).
func TestRenamedServiceOutsideDefaultWorkspace(t *testing.T) {
	bob := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	for _, tc := range []struct {
		name, selected, target, want string
		duplicate                    bool
		wantErr                      error
	}{
		{name: "visible renamed service", target: "new", want: "srv-b"},
		{name: "explicit other workspace", selected: "tea-b", target: "new", want: "srv-b"},
		{name: "missing from selection", selected: "tea-a", target: "new", wantErr: ErrNotFound},
		{name: "legacy alias missing from selection", selected: "tea-a", target: "old", wantErr: ErrNotFound},
		{name: "renamed ambiguity", target: "new", duplicate: true, wantErr: ErrConflict},
		{name: "selection resolves renamed ambiguity", selected: "tea-b", target: "new", duplicate: true, want: "srv-b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			apps := []*appv1alpha1.App{shownAs(workspaceApp("tea-b", "old", "srv-b"), "new"), shownAs(workspaceApp("tea-foreign", "foreign", "srv-secret"), "new")}
			if tc.duplicate {
				apps = append(apps, shownAs(workspaceApp("tea-a", "other", "srv-a"), "new"))
			}
			base := &Base{Client: fakeAppClient(objectsOf(apps)...), Namespace: "default", Workspace: multiWorkspace{"bob": {"tea-a", "tea-b"}}, Authz: &fakeAllowChecker{}}
			ctx := bob
			if tc.selected != "" {
				ctx = WithWorkspace(ctx, tc.selected)
			}
			for _, resolve := range []func(context.Context, string, string) (*appv1alpha1.App, error){base.AuthorizeApp, base.GetApp} {
				got, err := resolve(ctx, RelCanView, tc.target)
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantErr == nil && (got == nil || got.Labels[LabelAppID] != tc.want) {
					t.Fatalf("got %v, want %s", got, tc.want)
				}
				if err != nil && strings.Contains(err.Error(), "srv-secret") {
					t.Fatal("foreign service leaked in error")
				}
			}
		})
	}
}

func TestDisplayedNameMembershipEnumerationFailsClosed(t *testing.T) {
	ctx := WithIdentity(context.Background(), Identity{Subject: "bob", Method: "session"})
	base := &Base{
		Client:    fakeAppClient(shownAs(workspaceApp("tea-a", "old", "srv-a"), "new")),
		Namespace: "default", Workspace: multiWorkspace{"bob": {"tea-a"}}, Authz: &fakeAllowChecker{},
		MemberWorkspaceIDs: func(context.Context, Identity) ([]string, error) {
			return nil, errors.New("membership store unavailable")
		},
	}
	for _, resolve := range []func(context.Context, string, string) (*appv1alpha1.App, error){base.AuthorizeApp, base.GetApp} {
		if _, err := resolve(ctx, RelCanView, "new"); !errors.Is(err, ErrAuthzUnavailable) {
			t.Fatalf("membership enumeration failed but lookup returned %v", err)
		}
	}
}
