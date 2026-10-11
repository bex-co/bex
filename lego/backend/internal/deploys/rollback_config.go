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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// staticReleaseSelection selects a static_site target's retained publication
// (w4/m197): the object-store prefix the operator recorded when that release
// published (appv1alpha1.StaticPublicationRecordName). The copy restores the
// exact published files, so the saved publish directory, root directory and
// build command are irrelevant to it and stay as saved. Image is carried only
// as the row's provenance; a direct (no-build) target has none.
func (s *Service) staticReleaseSelection(ctx context.Context, a *appv1alpha1.App, target store.Deploy) (*appv1alpha1.ReleaseConfigReference, error) {
	prefix, err := s.retainedStaticPublication(ctx, a, target)
	if err != nil {
		return nil, err
	}
	if prefix == "" {
		return nil, fmt.Errorf("%w: deploy %q has no retained static publication to restore: it was published before bex recorded static publications, "+
			"or is older than the retained history; deploy its commit instead", core.ErrConflict, target.ID)
	}
	return &appv1alpha1.ReleaseConfigReference{Image: target.ResolvedImage, StaticPrefix: prefix}, nil
}

// retainedStaticPublication returns target's recorded publication prefix, or
// "" when none is recorded for this App. The record must be controlled by this
// exact App, so a same-name predecessor's leftover can never be selected.
func (s *Service) retainedStaticPublication(ctx context.Context, a *appv1alpha1.App, target store.Deploy) (string, error) {
	if target.Generation <= 0 {
		return "", nil
	}
	rec := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: appv1alpha1.StaticPublicationRecordName(a.Name, target.Generation)}, rec)
	if apierrors.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	prefix := string(rec.Data[appv1alpha1.StaticPublicationPrefixKey])
	if !metav1.IsControlledBy(rec, a) || !appv1alpha1.ValidStaticPublicationPrefix(prefix) {
		return "", nil
	}
	return prefix, nil
}

// releaseSelector chooses the target row and its immutable runtime selection
// from the App as re-read under the trigger lock. openRelease runs it again on
// every optimistic-conflict re-read, so a selection is never frozen from an App
// state that has since moved on.
type releaseSelector func(a *appv1alpha1.App) (store.Deploy, *appv1alpha1.ReleaseConfigReference, error)

// openSelectedRelease is shared by rollback and snapshot-backed Restart. The
// selection rides the same optimistic, serialized patch as its release id; saved
// image, commands, environment and source configuration remain untouched.
func (s *Service) openSelectedRelease(ctx context.Context, a *appv1alpha1.App, selectRelease releaseSelector, rollback, disableAutoDeploy bool) (store.Deploy, error) {
	appID := appStoreID(a)
	var target store.Deploy
	var selected *appv1alpha1.ReleaseConfigReference
	return s.openRelease(ctx, a, appID, nil, func(a *appv1alpha1.App, release int64) error {
		var err error
		if target, selected, err = selectRelease(a); err != nil {
			return err
		}
		stampReleaseGeneration(a, release)
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
		selection := *selected
		selection.Generation = release
		a.Spec.ReleaseConfig = &selection
		if disableAutoDeploy {
			a.Spec.AutoDeploy = false
		}
		return nil
	}, func(release int64) (store.Deploy, error) {
		commit := store.CommitInfo{Hash: target.Commit, Message: target.CommitMessage}
		if rollback {
			return s.Store.CreateRollbackDeploy(ctx, appID, selected.Image, target.ID, release, commit, core.SubjectFrom(ctx))
		}
		return s.Store.CreateDeploy(ctx, appID, store.TriggerAPI, selected.Image, release, commit, core.SubjectFrom(ctx))
	})
}

// restartSelectedRelease restarts the artifact and configuration serving now.
// Old records without a resolved image retain the established commit-pinned
// fallback (nil, nil).
func (s *Service) restartSelectedRelease(ctx context.Context, a *appv1alpha1.App) (*DeployView, error) {
	if s.Store == nil || appStoreID(a) == "" || a.Spec.Type == appv1alpha1.TypeStaticSite {
		return nil, nil
	}
	if err := s.validateTrigger(a.Name, a, TriggerParams{restart: true}); err != nil {
		return nil, err
	}
	// This unlocked read only picks snapshot Restart over the legacy fallback;
	// the release actually restarted is selected again under the lock.
	if target, err := s.servingDeploy(ctx, a); err != nil || target.ResolvedImage == "" {
		return nil, err
	}
	d, err := s.openSelectedRelease(ctx, a, func(a *appv1alpha1.App) (store.Deploy, *appv1alpha1.ReleaseConfigReference, error) {
		target, selected, err := s.restartSelection(ctx, a)
		if err == nil && selected == nil {
			err = fmt.Errorf("%w: service %q changed its running release during the restart; retry", core.ErrConflict, a.Name)
		}
		return target, selected, err
	}, false, false)
	if err != nil {
		return nil, err
	}
	if store.IsOpenDeployStatus(d.Status) {
		s.notifyDeployStarted(ctx, a, a.Name)
	}
	result := view(d)
	return &result, nil
}

// restartSelection selects the serving release's row and immutable runtime
// inputs, or a nil selection for the legacy commit-pinned fallback.
func (s *Service) restartSelection(ctx context.Context, a *appv1alpha1.App) (store.Deploy, *appv1alpha1.ReleaseConfigReference, error) {
	target, err := s.servingDeploy(ctx, a)
	if err != nil || target.ResolvedImage == "" {
		return target, nil, err
	}
	selected, err := s.targetReleaseConfig(ctx, a, target, true)
	if err == nil && selected.SourceGeneration == 0 && target.Status != store.DeployLive {
		// Image-only would run the saved configuration, not the serving one.
		err = unresolvedServing(a, "its configuration record is unavailable")
	}
	return target, selected, err
}

// servingDeploy is the deploy row of the release serving traffic, named by
// status.activeRevision ("rev-N", N the generation the row is filed under) as
// soon as it serves — before history projects that row Live (w4/m201).
//
// A row still awaiting its Live projection has no resolved image yet; the
// active status image is that release's own only while status still names it
// the current release. When the serving row or image cannot be established the
// restart is refused retryably rather than falling back to an older Live row.
// Without a release generation or a parseable active revision (legacy status),
// the newest Live row is the only serving evidence there is.
func (s *Service) servingDeploy(ctx context.Context, a *appv1alpha1.App) (store.Deploy, error) {
	appID := appStoreID(a)
	generation, ok := appv1alpha1.RevisionGeneration(a.Status.ActiveRevision)
	if !ok || a.Status.ReleaseGeneration <= 0 {
		live, err := s.Store.ListDeploys(ctx, appID, store.DeployFilter{Statuses: []string{store.DeployLive}, Limit: 1})
		if err != nil || len(live) == 0 {
			return store.Deploy{}, err
		}
		return live[0], nil
	}
	id, err := s.Store.DeployIDByGeneration(ctx, appID, generation)
	if err != nil {
		return store.Deploy{}, err
	}
	if id == "" {
		return store.Deploy{}, unresolvedServing(a, "no deploy records that release")
	}
	d, err := s.Store.GetDeploy(ctx, appID, id)
	if err != nil {
		return store.Deploy{}, err
	}
	if d.Status == store.DeployLive {
		return d, nil
	}
	if a.Status.ReleaseGeneration != generation || a.Status.Image == "" {
		return store.Deploy{}, unresolvedServing(a, "its image is not recorded yet")
	}
	d.ResolvedImage = a.Status.Image
	return d, nil
}

func unresolvedServing(a *appv1alpha1.App, why string) error {
	return fmt.Errorf("%w: service %q is serving %s but %s; retry shortly", core.ErrConflict, a.Name, a.Status.ActiveRevision, why)
}
