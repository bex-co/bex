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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A Render Build Source is a build_source App CR (w2/m254, ADR097): built by
// the operator, never run, and never a service. Every App enumeration that
// means "the services" filters carriers out with ServiceApps; the guard in
// build_source_guard_test.go makes each enumeration declare which it means.

// IsBuildSource reports whether app is a build source carrier.
func IsBuildSource(app *appv1alpha1.App) bool {
	return app.Spec.Type == appv1alpha1.TypeBuildSource
}

// ServiceApps drops build source carriers from items, in place.
func ServiceApps(items []appv1alpha1.App) []appv1alpha1.App {
	out := items[:0]
	for i := range items {
		if !IsBuildSource(&items[i]) {
			out = append(out, items[i])
		}
	}
	return out
}

type buildSourcesKey struct{}

// WithBuildSources lets the core App resolvers return build source carriers.
// Only the build-source surface sets it, after checking that the id it
// resolves names a carrier, so it can reuse the service write paths (a source
// patch, a build setting) on the carrier; every other caller keeps resolving
// services only.
func WithBuildSources(ctx context.Context) context.Context {
	return context.WithValue(ctx, buildSourcesKey{}, true)
}

func buildSourcesAllowed(ctx context.Context) bool {
	allowed, _ := ctx.Value(buildSourcesKey{}).(bool)
	return allowed
}

// serviceAppsFor is ServiceApps unless ctx is the build-source surface's.
func serviceAppsFor(ctx context.Context, items []appv1alpha1.App) []appv1alpha1.App {
	if buildSourcesAllowed(ctx) {
		return items
	}
	return ServiceApps(items)
}

// notServiceAppFor turns a successful read of a build source carrier into the
// not-found a service lookup answers, so a service verb never acts on one —
// unless ctx is the build-source surface's.
func notServiceAppFor(ctx context.Context, err error, app *appv1alpha1.App) error {
	if err == nil && IsBuildSource(app) && !buildSourcesAllowed(ctx) {
		return apierrors.NewNotFound(schema.GroupResource{Group: appv1alpha1.SchemeGroupVersion.Group, Resource: "apps"}, app.Name)
	}
	return err
}

// BuildSourceOrNotFound refuses an App that is not a build source carrier
// as a missing build source. Callers authorize first (AuthorizeApp under
// WithBuildSources) so a denial stays a denial, audited under their own verb.
func BuildSourceOrNotFound(a *appv1alpha1.App) error {
	if !IsBuildSource(a) {
		return NotFound("build source")
	}
	// A deleting carrier reads as gone the moment its delete is accepted, as a
	// service does, instead of lingering until its finalizer ends (w4/473).
	if NotFoundIfDeleting(a) != nil {
		return NotFound("build source")
	}
	return nil
}

// BuildSourceApp authorizes relation on build source id and returns the
// carrier under the build-source context. The apps and deploys surfaces both
// resolve a build source this way before acting on it, so the composition —
// WithBuildSources, then AuthorizeApp, then BuildSourceOrNotFound — lives once
// (w10/unify-duplicates). Keeping the callers' unexported buildSourceApp
// wrappers means the audit verb walk still names the exported verb that acts.
func (b *Base) BuildSourceApp(ctx context.Context, relation, id string) (context.Context, *appv1alpha1.App, error) {
	ctx = WithBuildSources(ctx)
	a, err := b.AuthorizeApp(ctx, relation, id)
	if err != nil {
		return ctx, nil, err
	}
	return ctx, a, BuildSourceOrNotFound(a)
}

// BuildSourceScope is the context the build-source surface acts in on id: it
// resolves id under WithBuildSources and refuses anything that is not a build
// source carrier, so a /v1/build-sources route can delegate to a service
// write path (which then authorizes and audits as usual) without ever
// reaching a service.
func (b *Base) BuildSourceScope(ctx context.Context, id string) (context.Context, error) {
	// Resolve the acting workspace onto the returned context, so the verb's
	// own authorization reuses it instead of resolving it again.
	ctx, _, err := b.resolveWorkspaceMemo(WithBuildSources(ctx))
	if err != nil {
		return ctx, err
	}
	app, err := b.GetApp(ctx, RelCanView, id)
	if err != nil {
		return ctx, err
	}
	return ctx, BuildSourceOrNotFound(app)
}

// ScopeBuildSourceID is BuildSourceScope for an id that names a build source
// (bds-…) and ctx unchanged for any other id. GraphQL and MCP reach a build
// source's build-time environment through the service verbs' serviceId
// argument; REST keeps its own /v1/build-sources routes (ADR097).
func (b *Base) ScopeBuildSourceID(ctx context.Context, id string) (context.Context, error) {
	if k, ok := ids.KindOf(id); ok && k == ids.BuildSource {
		return b.BuildSourceScope(ctx, id)
	}
	return ctx, nil
}

// CurrentBuild is a carrier's built image once it has built, else "".
func CurrentBuild(carrier *appv1alpha1.App) string {
	if carrier.Status.ActiveRevision == "" {
		return ""
	}
	return carrier.Status.Image
}

// BuildSourceLinkTo is the link a service deploying from carrier carries: the
// carrier and its current build.
func BuildSourceLinkTo(carrier *appv1alpha1.App) *appv1alpha1.BuildSourceLink {
	return &appv1alpha1.BuildSourceLink{Name: carrier.Name, Image: CurrentBuild(carrier)}
}

// BuildSourceIDOf is the public id of the build source a linked App deploys
// from: its carrier is named CRName(tenant, id) (ADR097).
func BuildSourceIDOf(a *appv1alpha1.App) string {
	return BuildSourceIDOfLink(a.Labels[LabelTenant], a.Spec.BuildSource)
}

// BuildSourceIDOfLink is the public id of the build source link names, in
// workspace tenantID; "" for no link.
func BuildSourceIDOfLink(tenantID string, link *appv1alpha1.BuildSourceLink) string {
	if link == nil {
		return ""
	}
	return strings.TrimPrefix(link.Name, tenantID+"-")
}
