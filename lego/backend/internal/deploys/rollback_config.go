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
	"cmp"
	"context"
	"encoding/json"
	"errors"
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
// configuration: the target's record, the port, shutdown delay and pre-deploy
// command it recorded, and the registry credential it pulled with. A legacy
// target with no record explicitly selects only its image; this is a
// compatibility fallback, not full Render configuration parity.
func (s *Service) targetReleaseConfig(ctx context.Context, a *appv1alpha1.App, target store.Deploy, restart bool) (*appv1alpha1.ReleaseConfigReference, error) {
	image := target.ResolvedImage
	if !restart {
		image = rollbackImage(target)
	}
	selected := &appv1alpha1.ReleaseConfigReference{Image: image, PreserveGroupValues: restart}
	if target.Generation <= 0 {
		return selected, nil
	}
	log := logf.FromContext(ctx).WithValues("app", a.Name, "deploy", target.ID, "generation", target.Generation)
	rec := &corev1.Secret{}
	err := s.Client.Get(ctx, client.ObjectKey{Namespace: a.Namespace, Name: appv1alpha1.ReleaseRecordName(a.Name, target.Generation)}, rec)
	if apierrors.IsNotFound(err) {
		log.Info("release selects image only: target configuration record is unavailable")
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
	// The record's port, shutdown delay and pre-deploy command, as Render's
	// Restart keeps the running configuration (w4/m433). A record written
	// before them leaves those to the saved spec.
	selected.Runtime = spec.Runtime
	if spec.Runtime == nil {
		log.Info("release runs the saved port, shutdown delay and pre-deploy command: target record predates them")
	}
	selected.Registry = recordedRegistryBinding(a, spec, rec)
	if selected.Registry == nil {
		log.Info("release pulls with the saved registry credential: target record predates its binding")
	}
	return selected, nil
}

// recordedRegistryBinding is the registry credential a release pulled its image
// with (w8/m529), named by that credential's own pull Secret: its record's
// binding, else the one in the spec it was adopted with, which records hold
// since w8/m121 (an unreadable one shows none). nil when it shows neither, or
// names a Secret no credential stands behind (a host-matched credential from
// before w4/m301): the release then pulls with the saved binding. A release
// that went live before per-credential Secrets moves off the shared one,
// which a later switch may have rewritten.
func recordedRegistryBinding(a *appv1alpha1.App, spec appv1alpha1.ReleaseRecordSpec, rec *corev1.Secret) *appv1alpha1.ReleaseRegistryBinding {
	binding := spec.Registry
	if raw := rec.Data[appv1alpha1.ReleaseRecordAdoptedSpecKey]; binding == nil && len(raw) > 0 {
		var adopted appv1alpha1.AppSpec
		if json.Unmarshal(raw, &adopted) == nil {
			binding = adopted.RegistryBinding()
		}
	}
	switch {
	case binding == nil || binding.PullSecret == "":
		return binding
	case binding.CredentialID == "":
		return nil
	}
	return &appv1alpha1.ReleaseRegistryBinding{CredentialID: binding.CredentialID,
		PullSecret: appv1alpha1.ExternalRegistryPullSecretName(a.Name, binding.CredentialID)}
}

// refreshedCredentials remembers, for one Restart or rollback, the pull Secret
// each credential was refreshed into ("" when it is gone).
type refreshedCredentials map[string]string

// refreshRegistryBinding refreshes the credential selected pulls with from its
// stored token before its release opens, so a rotated token is used, as a new
// pull on Render would. A credential deleted since leaves nothing to pull
// with, and the release pulls anonymously. It reads OpenBao and writes a
// Secret, so callers run it outside the trigger lock; done lets a selection
// made again under the lock reuse that refresh.
func (s *Service) refreshRegistryBinding(ctx context.Context, a *appv1alpha1.App, selected *appv1alpha1.ReleaseConfigReference, done refreshedCredentials) error {
	binding := selected.Registry
	if binding == nil || binding.CredentialID == "" || s.PullSecrets == nil {
		return nil
	}
	name, ok := done[binding.CredentialID]
	if !ok {
		var err error
		if name, err = s.PullSecrets.EnsureCredentialPullSecret(ctx, a, binding.CredentialID); err != nil {
			return fmt.Errorf("refresh registry credential %q for the selected release: %w", binding.CredentialID, err)
		}
		done[binding.CredentialID] = name
	}
	if name == "" {
		selected.Registry = &appv1alpha1.ReleaseRegistryBinding{}
	}
	return nil
}

// rollbackImage is the image reference a rollback to target runs. Render
// (render.com/docs/rollbacks § Registry-hosted Docker images): "The rollback
// uses the same image tag or digest as the target deploy. With a tag, Render
// pulls the latest image under that tag ... A digest always refers to the same
// image." So a registry image rolls back to the reference the target deployed
// (store.Deploy.Image), which the operator resolves again as a new release and
// records on the rollback's own row when it goes live (w4/m432). A build bex
// made from source is bex's own retained artifact: it runs the exact image the
// target ran (ResolvedImage), as Restart does.
func rollbackImage(target store.Deploy) string {
	if target.Image == "" || store.PlatformImage(target.Image) {
		return target.ResolvedImage
	}
	return target.Image
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

// selectedOpening is how openSelectedRelease opens its release: the deploy
// row's trigger (rollback, restart or service_resumed), whether the release
// write turns auto-deploy off, and a resume's own row write and App change,
// which ride that same write.
type selectedOpening struct {
	rowTrigger        string
	disableAutoDeploy bool
	// selectionLost substitutes this error when the serving release can no
	// longer be selected under the trigger lock, instead of refusing: the
	// row write already ran, so refusing would leave the row resumed while
	// the App still reads suspended. nil keeps the refusal (Restart has no
	// caller fallback). A resume passes errResumeWithoutDeploy, so the
	// caller resumes without a deploy — exactly as when the unlocked checks
	// find nothing to re-release.
	selectionLost error
	// rowWrite runs under the trigger lock before the release write, as
	// openRelease's rowWrite does; nil writes no row.
	rowWrite func() error
	// mutate changes the App in the release write; nil changes nothing else.
	mutate func(*appv1alpha1.App)
}

// openSelectedRelease is shared by rollback, snapshot-backed Restart and the
// deploy a Resume opens. The selection rides the same optimistic, serialized
// patch as its release id; saved image, commands, environment and source
// configuration remain untouched.
func (s *Service) openSelectedRelease(ctx context.Context, a *appv1alpha1.App, selectRelease releaseSelector, o selectedOpening) (store.Deploy, error) {
	appID := appStoreID(a)
	var target store.Deploy
	var selected *appv1alpha1.ReleaseConfigReference
	return s.openRelease(ctx, a, appID, o.rowWrite, func(a *appv1alpha1.App, release int64) error {
		var err error
		if target, selected, err = selectRelease(a); err != nil {
			return err
		}
		core.StampReleaseGeneration(a, release)
		a.Spec.RestartedAt = s.Now().UTC().Format(time.RFC3339Nano)
		selection := *selected
		selection.Generation = release
		a.Spec.ReleaseConfig = &selection
		if o.disableAutoDeploy {
			a.Spec.AutoDeploy = false
		}
		if o.mutate != nil {
			o.mutate(a)
		}
		return nil
	}, func(ctx context.Context, release int64) (store.Deploy, error) {
		// The restored release's build source build travels with its commit.
		commit := target.CommitInfo()
		commit.BuildID = target.BuildID
		switch o.rowTrigger {
		case store.TriggerRollback:
			return s.Store.CreateRollbackDeploy(ctx, appID, selected.Image, target.ID, release, commit, core.SubjectFrom(ctx))
		case store.TriggerRestart:
			return s.Store.CreateRestartDeploy(ctx, appID, selected.Image, release, commit, core.SubjectFrom(ctx))
		case store.TriggerServiceResumed:
			return s.Store.CreateDeploy(ctx, appID, o.rowTrigger, selected.Image, release, commit, core.SubjectFrom(ctx))
		default:
			return store.Deploy{}, fmt.Errorf("a selected release opens no %q deploy", o.rowTrigger)
		}
	})
}

// restartSelectedRelease restarts the artifact and configuration serving now:
// a runtime release's image and configuration record, or a static site's
// retained publication (w32/049). A release neither selects (an old record
// without a resolved image, a static release with no retained publication)
// retains the established commit-pinned fallback (nil, nil). Refusals name
// service, the caller's selector, as Trigger's do; a.Name is the internal
// tenant-prefixed CR name (w8/128).
func (s *Service) restartSelectedRelease(ctx context.Context, service string, a *appv1alpha1.App) (*DeployView, error) {
	if s.Store == nil || appStoreID(a) == "" {
		return nil, nil
	}
	if err := s.validateTrigger(service, a, TriggerParams{restart: true}); err != nil {
		return nil, err
	}
	refreshed, ok, err := s.selectServing(ctx, service, a)
	if !ok || err != nil {
		return nil, err
	}
	return s.openServing(ctx, service, a, refreshed, selectedOpening{rowTrigger: store.TriggerRestart})
}

// selectServing is the unlocked read of the release serving now: false when
// it cannot be selected (Restart then keeps the legacy commit-pinned
// fallback), else the serving release's registry credential refreshed before
// the trigger lock, which openServing's locked selection refreshes again only
// if it moved on.
func (s *Service) selectServing(ctx context.Context, service string, a *appv1alpha1.App) (refreshedCredentials, bool, error) {
	if selectable, err := s.restartSelectable(ctx, service, a); !selectable || err != nil {
		return nil, false, err
	}
	refreshed := refreshedCredentials{}
	if _, selected, err := s.restartSelection(ctx, service, a); err == nil && selected != nil {
		if err := s.refreshRegistryBinding(ctx, a, selected, refreshed); err != nil {
			return nil, false, err
		}
	}
	return refreshed, true, nil
}

// openServing opens a new release of the artifact and configuration serving
// now, selected again under the trigger lock, recorded as o says: Restart's,
// and the deploy a Resume opens.
func (s *Service) openServing(ctx context.Context, service string, a *appv1alpha1.App, refreshed refreshedCredentials, o selectedOpening) (*DeployView, error) {
	d, err := s.openSelectedRelease(ctx, a, func(a *appv1alpha1.App) (store.Deploy, *appv1alpha1.ReleaseConfigReference, error) {
		target, selected, err := s.restartSelection(ctx, service, a)
		if err == nil && selected == nil {
			err = fmt.Errorf("%w: service %q changed its running release during the restart; retry", core.ErrConflict, service)
		}
		if err == nil {
			err = s.refreshRegistryBinding(ctx, a, selected, refreshed)
		}
		if err != nil && o.selectionLost != nil {
			err = o.selectionLost
		}
		return target, selected, err
	}, o)
	if err != nil {
		return nil, err
	}
	if store.IsOpenDeployStatus(d.Status) {
		s.notifyDeployStarted(ctx, a)
	}
	result := view(a, d)
	return &result, nil
}

// ResumeRelease resumes a suspended service with a deploy of the release it
// was serving, as Render's resume opens a deploy: Render's deploy trigger enum
// carries service_resumed, and its dashboard opens the deploy (a cron job's
// build) its resumeService mutation answers (w3/m643,
// docs/render-artifacts/cron-dashboard.md). Like Restart it re-releases the
// artifact and configuration that ran, so nothing builds and saved changes
// wait for the next deploy. writeRow and resume are the resume's own row write
// and App change; they ride the release write, so the operator brings the
// service back once, onto the new release.
//
// "" with a nil error opens no deploy, and the caller resumes the service
// without one: it is not store-managed, a deploy is in flight (which brings
// it back itself, and a new release would replace it), or its serving
// release cannot be selected (it never went live, its release predates the
// records a Restart selects from, or reading it failed) — before the row
// write, when nothing is written, or after it, when the row already reads
// resumed and the caller's plain write converges the App onto it. An error
// is one from the writes themselves, once the serving release was selected
// under the trigger lock.
func (s *Service) ResumeRelease(ctx context.Context, service string, a *appv1alpha1.App, writeRow func() error, resume func(*appv1alpha1.App)) (string, error) {
	appID := appStoreID(a)
	if s.Store == nil || appID == "" {
		return "", nil
	}
	if inFlight, err := s.deployInFlight(ctx, appID); err != nil || inFlight {
		return "", err
	}
	refreshed, ok, err := s.selectServing(ctx, service, a)
	if err != nil {
		logf.FromContext(ctx).Info("resume opens no deploy: the serving release cannot be selected", "app", a.Name, "reason", err.Error())
	}
	if !ok || err != nil {
		return "", nil
	}
	d, err := s.openServing(ctx, service, a, refreshed, selectedOpening{
		rowTrigger: store.TriggerServiceResumed,
		// Checked again under the trigger lock, on the App as re-read there,
		// before anything is written: a deploy opened since keeps its
		// release, and a serving release that can no longer be selected
		// leaves the resume to the caller.
		//
		// Lost again while the release write itself selects, after the row
		// write ran, selectionLost leaves the resume to the caller too:
		// refusing then would report a failure that already half-happened
		// (the row reads resumed while the App still reads suspended).
		selectionLost: errResumeWithoutDeploy,
		rowWrite: func() error {
			if inFlight, err := s.deployInFlight(ctx, appID); err != nil || inFlight {
				return cmp.Or(err, errResumeWithoutDeploy)
			}
			if _, selected, err := s.restartSelection(ctx, service, a); err != nil || selected == nil {
				return errResumeWithoutDeploy
			}
			return writeRow()
		},
		mutate: resume,
	})
	if errors.Is(err, errResumeWithoutDeploy) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return d.ID, nil
}

// errResumeWithoutDeploy is ResumeRelease's locked finding that it should open
// no deploy; it answers "" with no error.
var errResumeWithoutDeploy = errors.New("the resume opens no deploy")

// deployInFlight reports an open deploy for appID: one executing, queued or
// waiting.
func (s *Service) deployInFlight(ctx context.Context, appID string) (bool, error) {
	open, err := s.Store.ListDeploys(ctx, appID, store.DeployFilter{Statuses: store.OpenDeployStatuses(), Limit: 1})
	return len(open) > 0, err
}

// restartSelectable reports whether Restart can select the release serving
// now; false keeps the legacy commit-pinned fallback.
func (s *Service) restartSelectable(ctx context.Context, service string, a *appv1alpha1.App) (bool, error) {
	if a.Spec.Type != appv1alpha1.TypeStaticSite {
		target, err := s.servingDeploy(ctx, service, a)
		return err == nil && target.ResolvedImage != "", err
	}
	target, selected, err := s.staticRestartSelection(ctx, service, a)
	if err == nil && selected == nil && target.ID != "" {
		logf.FromContext(ctx).Info("static restart rebuilds the live commit with the saved settings: the serving release has no retained publication",
			"app", a.Name, "deploy", target.ID, "generation", target.Generation)
	}
	return selected != nil, err
}

// restartSelection selects the serving release's row and immutable runtime
// inputs, or a nil selection for the legacy commit-pinned fallback.
func (s *Service) restartSelection(ctx context.Context, service string, a *appv1alpha1.App) (store.Deploy, *appv1alpha1.ReleaseConfigReference, error) {
	if a.Spec.Type == appv1alpha1.TypeStaticSite {
		return s.staticRestartSelection(ctx, service, a)
	}
	target, err := s.servingDeploy(ctx, service, a)
	if err != nil || target.ResolvedImage == "" {
		return target, nil, err
	}
	selected, err := s.targetReleaseConfig(ctx, a, target, true)
	if err == nil && selected.SourceGeneration == 0 && target.Status != store.DeployLive {
		// Image-only would run the saved configuration, not the serving one.
		err = unresolvedServing(service, a, "its configuration record is unavailable", retryShortly)
	}
	return target, selected, err
}

// staticRestartSelection selects a static site's serving release and its
// retained publication, the selection Rollback makes for a target
// (staticReleaseSelection, w4/m197). Render's Restart keeps "the exact same Git
// commit and configuration as the running instance", so the copy re-serves
// what is live: nothing builds, and the saved build command, root directory and
// publish directory wait for the next deploy (w32/049). A nil selection (a
// release published before w4/m197, or past the retained window) keeps the
// commit-pinned fallback.
func (s *Service) staticRestartSelection(ctx context.Context, service string, a *appv1alpha1.App) (store.Deploy, *appv1alpha1.ReleaseConfigReference, error) {
	target, err := s.servingDeploy(ctx, service, a)
	if err != nil || target.ID == "" {
		return target, nil, err
	}
	prefix, err := s.retainedStaticPublication(ctx, a, target)
	if err != nil || prefix == "" {
		return target, nil, err
	}
	return target, &appv1alpha1.ReleaseConfigReference{Image: target.ResolvedImage, StaticPrefix: prefix}, nil
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
func (s *Service) servingDeploy(ctx context.Context, service string, a *appv1alpha1.App) (store.Deploy, error) {
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
		// Not necessarily transient: a gated env-group link/unlink converges a
		// new revision without opening a deploy row (w2/m94), so waiting alone
		// may never resolve it (w4/264).
		return store.Deploy{}, unresolvedServing(service, a, "no deploy records that release, so Restart cannot preserve it", untrackedRelease)
	}
	d, err := s.Store.GetDeploy(ctx, appID, id)
	if err != nil {
		return store.Deploy{}, err
	}
	if d.Status == store.DeployLive {
		return d, nil
	}
	if a.Spec.Type == appv1alpha1.TypeStaticSite {
		// A static release restarts from its publication record, named by its
		// generation alone: a direct (no-build) publish has no image, and a
		// built one's is only the restart row's provenance (w32/049).
		if a.Status.ReleaseGeneration == generation {
			d.ResolvedImage = a.Status.Image
		}
		return d, nil
	}
	if a.Status.ReleaseGeneration != generation || a.Status.Image == "" {
		return store.Deploy{}, unresolvedServing(service, a, "its image is not recorded yet", retryShortly)
	}
	d.ResolvedImage = a.Status.Image
	return d, nil
}

// Recovery steps for unresolvedServing.
const (
	retryShortly     = "retry shortly"
	untrackedRelease = "if this persists, run a standard deploy to establish a tracked release (it applies the saved configuration and current source)"
)

func unresolvedServing(service string, a *appv1alpha1.App, why, next string) error {
	return fmt.Errorf("%w: service %q is serving %s but %s; %s", core.ErrConflict, service, a.Status.ActiveRevision, why, next)
}
