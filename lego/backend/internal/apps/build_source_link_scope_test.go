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
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestBuildSourceLinkLetsScopeShortfallThrough: the build-source link every
// write verb funnels through (service create and update with buildSourceId,
// PATCH buildSourceId, and the Blueprint apply's service links) resolves the
// carrier with a can_view probe. A narrowed human OAuth token that holds the
// write capability but lacks read must read its scope shortfall
// (insufficient_scope), never the 400 "names no build source in this
// workspace" a genuinely missing or foreign id answers — the same
// write-only cell the org-key (#261) and audit-log (#264) mappings had to
// open. The capability check runs before existence, so even an unknown id
// answers insufficient_scope to such a token.
func TestBuildSourceLinkLetsScopeShortfallThrough(t *testing.T) {
	svc, _, ctx := newBuildSourceService(t)
	svc.Metadata = resourcemeta.Config{Region: "fsn1"} // default Render region = frankfurt

	src, err := svc.CreateBuildSource(ctx, CreateBuildSourceRequest{
		OwnerID: "tea-a", Name: "ImgSrc",
		Image: &BuildSourceImage{ImageURL: "nginx:1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, CreateRequest{
		OwnerID: "tea-a", Name: "web", Type: appv1alpha1.TypeWebService,
		Region: "frankfurt", Repo: "https://github.com/acme/web",
		Runtime: "node", BuildCommand: "npm ci", StartCommand: "node server.js",
	}); err != nil {
		t.Fatal(err)
	}

	writeOnly := core.WithIdentity(context.Background(), core.Identity{
		Subject: "user-a", Method: core.MethodOAuth2, Human: true,
		CanonicalScopes: core.ScopeWrite,
	})
	insufficient := func(err error) bool {
		var coded *core.CodedError
		return errors.As(err, &coded) && coded.Code == core.InsufficientScopeCode
	}

	// Every cell is asserted: the shortfall cells below fail while the
	// mapping swallows the capability refusal into the 400.
	for _, tc := range []struct {
		name string
		call func(context.Context) error
		want func(error) bool
	}{
		{
			name: "write-only token linking the existing carrier reads its scope shortfall",
			call: func(ctx context.Context) error {
				_, err := svc.LinkBuildSource(ctx, "web", src.ID, nil, false)
				return err
			},
			want: insufficient,
		},
		{
			name: "write-only token naming an unknown id still reads its scope shortfall, not absence",
			call: func(ctx context.Context) error {
				_, err := svc.LinkBuildSource(ctx, "web", "bds-missing", nil, false)
				return err
			},
			want: insufficient,
		},
		{
			name: "write-only token creating with a build source reads its scope shortfall",
			call: func(ctx context.Context) error {
				_, err := svc.Create(ctx, CreateRequest{
					OwnerID: "tea-a", Name: "linked", Type: appv1alpha1.TypeWebService,
					Region: "frankfurt", BuildSourceID: src.ID, StartCommand: "node server.js",
				})
				return err
			},
			want: insufficient,
		},
	} {
		if err := tc.call(writeOnly); !tc.want(err) {
			t.Errorf("%s: err = %v, want %s", tc.name, err, core.InsufficientScopeCode)
		}
	}
	after := getTenantApp(t, svc.Client, "tea-a", "web")
	if after.Spec.BuildSource != nil {
		t.Errorf("a refused link wrote a build source link: %+v", after.Spec.BuildSource)
	}

	// The absence cells keep their 400: an unknown id, and a non-carrier
	// service id, name no build source in this workspace to a caller who may
	// read them, so ids reveal nothing across workspaces.
	badRequest := func(err error) bool {
		return errors.Is(err, core.ErrBadRequest)
	}
	for _, tc := range []struct {
		name string
		call func(context.Context) error
	}{
		{
			name: "unknown id stays a 400",
			call: func(ctx context.Context) error {
				_, err := svc.LinkBuildSource(ctx, "web", "bds-missing", nil, true)
				return err
			},
		},
	} {
		if err := tc.call(ctx); !badRequest(err) {
			t.Errorf("%s: err = %v, want 400", tc.name, err)
		}
	}

	// The happy path still links on a dry run.
	if _, err := svc.LinkBuildSource(ctx, "web", src.ID, nil, true); err != nil {
		t.Errorf("dry-run link of the existing carrier: %v", err)
	}
}
