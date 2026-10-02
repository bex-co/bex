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

package deploys

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// RollbackOptions carries the documented per-surface auto-deploy distinction.
type RollbackOptions struct {
	// DisableAutoDeploy is true for dashboard rollback, false for API rollback.
	DisableAutoDeploy bool
}

// targetReleaseConfig selects immutable runtime inputs without writing any saved
// configuration. A legacy target with no record explicitly selects only its image;
// this is a compatibility fallback, not full Render configuration parity.
func (s *Service) targetReleaseConfig(ctx context.Context, a *appv1alpha1.App, target store.Deploy, restart bool) (*appv1alpha1.ReleaseConfigReference, error) {
	selected := &appv1alpha1.ReleaseConfigReference{Image: target.ResolvedImage, PreserveGroupValues: restart}
	if target.Generation <= 0 {
		return selected, nil
	}
	rec := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: appv1alpha1.ReleaseRecordName(a.Name, target.Generation)}, rec)
	if apierrors.IsNotFound(err) {
		logf.FromContext(ctx).Info("release selects image only: target configuration record is unavailable", "app", a.Name, "deploy", target.ID, "generation", target.Generation)
		return selected, nil
	}
	if err != nil {
		return nil, err
	}
	var spec appv1alpha1.ReleaseRecordSpec
	if raw := rec.Data[appv1alpha1.ReleaseRecordSpecKey]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &spec); err != nil {
			return nil, fmt.Errorf("decode release record for generation %d: %w", target.Generation, err)
		}
	}
	selected.SourceGeneration = target.Generation
	return selected, nil
}

// openSelectedRelease is shared by rollback and snapshot-backed Restart. The
// selection rides the same optimistic, serialized patch as its release id; saved
// image, commands, environment and source configuration remain untouched.
func (s *Service) openSelectedRelease(ctx context.Context, a *appv1alpha1.App, target store.Deploy, selected *appv1alpha1.ReleaseConfigReference, rollback, disableAutoDeploy bool) (store.Deploy, error) {
	appID := appStoreID(a)
	return s.openRelease(ctx, a, appID, nil, func(a *appv1alpha1.App, release int64) {
		stampReleaseGeneration(a, release)
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
		selection := *selected
		selection.Generation = release
		a.Spec.ReleaseConfig = &selection
		if disableAutoDeploy {
			a.Spec.AutoDeploy = false
		}
	}, func(release int64) (store.Deploy, error) {
		commit := store.CommitInfo{Hash: target.Commit, Message: target.CommitMessage}
		if rollback {
			return s.Store.CreateRollbackDeploy(ctx, appID, selected.Image, target.ID, release, commit, core.SubjectFrom(ctx))
		}
		return s.Store.CreateDeploy(ctx, appID, store.TriggerAPI, selected.Image, release, commit, core.SubjectFrom(ctx))
	})
}

// restartSelectedRelease prefers the actual live artifact and configuration. Old
// records without a resolved image retain the established commit-pinned fallback.
func (s *Service) restartSelectedRelease(ctx context.Context, a *appv1alpha1.App) (*DeployView, error) {
	if s.Store == nil || appStoreID(a) == "" || a.Spec.Type == appv1alpha1.TypeStaticSite {
		return nil, nil
	}
	if err := s.validateTrigger(a.Name, a, TriggerParams{restart: true}); err != nil {
		return nil, err
	}
	live, err := s.Store.ListDeploys(ctx, appStoreID(a), store.DeployFilter{Statuses: []string{store.DeployLive}, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(live) == 0 || live[0].ResolvedImage == "" {
		return nil, nil
	}
	selected, err := s.targetReleaseConfig(ctx, a, live[0], true)
	if err != nil {
		return nil, err
	}
	d, err := s.openSelectedRelease(ctx, a, live[0], selected, false, false)
	if err != nil {
		return nil, err
	}
	if store.IsOpenDeployStatus(d.Status) {
		s.notifyDeployStarted(ctx, a, a.Name)
	}
	result := view(d)
	return &result, nil
}
