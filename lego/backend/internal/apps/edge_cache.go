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
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Edge caching (w5/m151): Render's serviceDetails.cache.profile on a web
// service, and its POST /services/{id}/cache/purge. The operator fronts a
// caching web service with bex's edge-cache proxy (lego/operator/internal/
// edgecache); this file is the API half.

// viewCacheProfile is the profile a web service reads back, "no-cache" when
// unset, as Render's default; other types have none.
func viewCacheProfile(spec appv1alpha1.AppSpec, svcType string) string {
	if svcType != appv1alpha1.TypeWebService {
		return ""
	}
	if spec.CacheProfile == "" || freeTier(spec.Tier) {
		// A free web service never caches, so it reads as not caching.
		return appv1alpha1.CacheProfileNone
	}
	return spec.CacheProfile
}

func freeTier(tier string) bool { return tier == "" || tier == "free" }

// checkCacheProfile normalizes profile (Render's default no-cache is stored as
// unset, so setting it is no change) and refuses what Render refuses: an
// unknown profile, caching on a type other than a web service, and caching on
// a free web service ("Edge caching is not available for free web services").
func checkCacheProfile(a *appv1alpha1.App, profile string) (string, error) {
	switch profile {
	case appv1alpha1.CacheProfileNone, appv1alpha1.CacheProfileStatic, appv1alpha1.CacheProfileAll:
	default:
		return "", fmt.Errorf("%w: cache.profile must be one of %s, %s, %s", core.ErrBadRequest,
			appv1alpha1.CacheProfileNone, appv1alpha1.CacheProfileStatic, appv1alpha1.CacheProfileAll)
	}
	if profile == appv1alpha1.CacheProfileNone {
		return "", nil
	}
	if t := a.Spec.Type; t != "" && t != appv1alpha1.TypeWebService {
		return "", fmt.Errorf("%w: edge caching is available only for web services", core.ErrBadRequest)
	}
	if freeTier(a.Spec.Tier) {
		return "", fmt.Errorf("%w: edge caching is not available for free web services; choose a paid plan first", core.ErrBadRequest)
	}
	return profile, nil
}

// SetCacheProfile changes a web service's edge caching profile.
func (s *Service) SetCacheProfile(ctx context.Context, name, profile string) (AppView, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	profile, err = checkCacheProfile(a, profile)
	if err != nil {
		return AppView{}, err
	}
	return s.recordedPatch(ctx, core.AuditVerbSetCacheProfile, a, func(a *appv1alpha1.App) {
		a.Spec.CacheProfile = profile
	})
}

// PurgeCache empties a caching web service's edge cache, as Render's
// purge-cache does. The cache lives in each pod's edge-cache proxy, so the
// purge stamps the App and the operator rolls the pods with an empty cache
// (no deploy is opened). A service that does not cache has nothing to purge.
func (s *Service) PurgeCache(ctx context.Context, name string) (AppView, error) {
	a, err := s.AuthorizeApp(core.WithDeferredAllowedWriteAudit(ctx), core.RelCanOperate, name)
	if err != nil {
		return AppView{}, err
	}
	if !a.Spec.EdgeCaching() || freeTier(a.Spec.Tier) {
		return AppView{}, fmt.Errorf("%w: edge caching is not enabled for this service", core.ErrBadRequest)
	}
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	return s.recordedPatch(ctx, core.AuditVerbPurgeCache, a, func(a *appv1alpha1.App) {
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		a.Annotations[appv1alpha1.CachePurgedAtAnnotation] = stamp
	})
}
