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
	"fmt"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// static_rules.go is Render's single-rule static-site edge operations
// (add-headers, delete-header, add-route, patch-route, delete-route, w8/137)
// over the same spec lists the bulk SetRoutes/SetHeaders replace. A rule's id
// derives from its content (withRuleIDs), so each edit finds its rule by
// recomputing the ids of the list it is applied to: the fresh one inside the
// patch, so a concurrent edit is never overwritten.

// editStaticRules authorizes and applies one rule edit to a static_site, as
// SetRoutes/SetHeaders do. find validates the edit after authorization, against
// the current spec, so an unauthorized caller learns nothing from it; edit makes
// it on the patched spec.
func (s *Service) editStaticRules(ctx context.Context, name, verb string, find func(*appv1alpha1.App) error, edit func(*appv1alpha1.App)) (AppView, error) {
	// Routes and headers are content/security mutations (round-5 finding 11).
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanCreate, name)
	if err != nil {
		return AppView{}, err
	}
	if err := requireStaticSite(a, name); err != nil {
		return AppView{}, err
	}
	if err := find(a); err != nil {
		return AppView{}, err
	}
	return s.recordedPatch(ctx, verb, a, edit)
}

// routeIndex is the position of the route with routeID among a's routes, or -1.
func routeIndex(name string, a *appv1alpha1.App, routeID string) int {
	return slices.IndexFunc(toRenderRouteObjects(name, staticRouteViews(a.Spec.Routes)), func(r renderRoute) bool { return r.ID == routeID })
}

// headerIndex is the position of the header with headerID among a's headers, or -1.
func headerIndex(name string, a *appv1alpha1.App, headerID string) int {
	return slices.IndexFunc(toRenderHeaderObjects(name, staticHeaderViews(a.Spec.Headers)), func(h renderHeader) bool { return h.ID == headerID })
}

// AddRoute adds one redirect/rewrite rule (Render's add-route): appended, or
// inserted at priority, moving later rules down. A priority past the end
// appends. It also returns the position the rule landed at.
func (s *Service) AddRoute(ctx context.Context, name string, route StaticRouteView, priority *int) (AppView, int, error) {
	rule := routesFromViews([]StaticRouteView{route})[0]
	var at int
	view, err := s.editStaticRules(ctx, name, core.AuditVerbSetRoutes, func(a *appv1alpha1.App) error {
		if err := validateRoutes([]StaticRouteView{route}); err != nil {
			return err
		}
		if priority != nil && *priority < 0 {
			return fmt.Errorf("%w: priority must be 0 or more", core.ErrBadRequest)
		}
		if len(a.Spec.Routes) >= maxStaticRoutes {
			return fmt.Errorf("%w: routes must not exceed %d items", core.ErrBadRequest, maxStaticRoutes)
		}
		return nil
	}, func(a *appv1alpha1.App) {
		at = len(a.Spec.Routes)
		if priority != nil && *priority < at {
			at = *priority
		}
		a.Spec.Routes = slices.Insert(a.Spec.Routes, at, rule)
	})
	return view, at, err
}

// MoveRoute moves the rule with routeID to priority (Render's patch-route),
// shifting the rules between its old and new position by one.
func (s *Service) MoveRoute(ctx context.Context, name, routeID string, priority int) (AppView, error) {
	return s.editStaticRules(ctx, name, core.AuditVerbSetRoutes, func(a *appv1alpha1.App) error {
		if routeIndex(name, a, routeID) < 0 {
			return core.NotFound("route")
		}
		if priority < 0 || priority >= len(a.Spec.Routes) {
			return fmt.Errorf("%w: priority must be between 0 and %d", core.ErrBadRequest, len(a.Spec.Routes)-1)
		}
		return nil
	}, func(a *appv1alpha1.App) {
		from := routeIndex(name, a, routeID)
		if from < 0 || priority >= len(a.Spec.Routes) {
			return
		}
		rule := a.Spec.Routes[from]
		a.Spec.Routes = slices.Insert(slices.Delete(a.Spec.Routes, from, from+1), priority, rule)
	})
}

// DeleteRoute removes the rule with routeID (Render's delete-route).
func (s *Service) DeleteRoute(ctx context.Context, name, routeID string) error {
	_, err := s.editStaticRules(ctx, name, core.AuditVerbSetRoutes, func(a *appv1alpha1.App) error {
		if routeIndex(name, a, routeID) < 0 {
			return core.NotFound("route")
		}
		return nil
	}, func(a *appv1alpha1.App) {
		if i := routeIndex(name, a, routeID); i >= 0 {
			a.Spec.Routes = slices.Delete(a.Spec.Routes, i, i+1)
		}
	})
	return err
}

// AddHeader adds one custom response-header rule (Render's add-headers),
// appended after the existing ones.
func (s *Service) AddHeader(ctx context.Context, name string, header StaticHeaderView) (AppView, error) {
	rule := headersFromViews([]StaticHeaderView{header})[0]
	return s.editStaticRules(ctx, name, core.AuditVerbSetHeaders, func(a *appv1alpha1.App) error {
		if err := validateHeaders([]StaticHeaderView{header}); err != nil {
			return err
		}
		if len(a.Spec.Headers) >= maxStaticHeaders {
			return fmt.Errorf("%w: headers must not exceed %d items", core.ErrBadRequest, maxStaticHeaders)
		}
		return nil
	}, func(a *appv1alpha1.App) {
		a.Spec.Headers = append(a.Spec.Headers, rule)
	})
}

// DeleteHeader removes the rule with headerID (Render's delete-header).
func (s *Service) DeleteHeader(ctx context.Context, name, headerID string) error {
	_, err := s.editStaticRules(ctx, name, core.AuditVerbSetHeaders, func(a *appv1alpha1.App) error {
		if headerIndex(name, a, headerID) < 0 {
			return core.NotFound("header")
		}
		return nil
	}, func(a *appv1alpha1.App) {
		if i := headerIndex(name, a, headerID); i >= 0 {
			a.Spec.Headers = slices.Delete(a.Spec.Headers, i, i+1)
		}
	})
	return err
}
