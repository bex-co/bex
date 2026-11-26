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

package controller

import (
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"hash/fnv"
	"net/http"
	"slices"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	"github.com/bex-co/bex/lego/operator/internal/registry"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A running image service's image is checked, as Render does (w4/m671;
// appv1alpha1.ConditionImageAvailable, ADR004 § A running image service's
// image is checked). Without it, bex learned that an image had been deleted
// from its registry only when a restart, a scale-up or a node replacement
// pulled it again and failed.
//
// The app-image-check controller has its own App queue and one worker, and
// writes only status.imageCheck and the ImageAvailable condition. It never
// touches the Deployment, and it reads the registry with one manifest HEAD.
const (
	// DefaultImageCheckInterval is how often each running image service's
	// image is checked when BEX_IMAGE_CHECK_INTERVAL names no other interval.
	// Render publishes none.
	DefaultImageCheckInterval = 6 * time.Hour
	// imageCheckConfirm is how soon a first refusal is checked again, so an
	// image that is really gone is reported within minutes of being found,
	// not one interval later.
	imageCheckConfirm = 15 * time.Minute
	// imageCheckRetry is when a check the registry did not answer (a rate
	// limit, a server error, a failed request) is tried again.
	imageCheckRetry = 30 * time.Minute
	// imageCheckRefusalsToFail is how many consecutive refusals turn the
	// condition False: one 404 can be a registry's replication lag or a tag
	// re-pushed at that moment.
	imageCheckRefusalsToFail = 2
	// imageCheckSpacing spaces the checks: 30 a minute, which at the 6-hour
	// interval covers about 10,000 image services.
	imageCheckSpacing = 2 * time.Second
	// imageCheckWriteRetry is when a check whose answer could not be stored
	// runs again.
	imageCheckWriteRetry = time.Minute
)

// setupImageCheck registers the app-image-check controller, unless the
// runtime is not Kubernetes or the check is turned off.
func (r *AppReconciler) setupImageCheck(mgr ctrl.Manager) error {
	if r.Mode != ModeKubernetes || r.ImageCheckInterval <= 0 {
		return nil
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("app-image-check").
		For(&appv1alpha1.App{}, builder.WithPredicates(imageCheckPredicate{})).
		Complete(reconcile.Func(r.reconcileImageCheck))
}

// imageCheckPredicate wakes the check when what decides it changes: the image
// that serves, the phase, a suspend or resume. Between those the controller's
// own requeue paces it, and its own status writes never wake it.
type imageCheckPredicate struct{ predicate.Funcs }

func (imageCheckPredicate) Update(e event.UpdateEvent) bool {
	old, oldOK := e.ObjectOld.(*appv1alpha1.App)
	next, nextOK := e.ObjectNew.(*appv1alpha1.App)
	if !oldOK || !nextOK {
		return false
	}
	return old.Status.Image != next.Status.Image || old.Status.Phase != next.Status.Phase ||
		old.Spec.Suspended != next.Spec.Suspended
}

func (imageCheckPredicate) Delete(event.DeleteEvent) bool { return false }

// reconcileImageCheck checks the App's served image when its check is due,
// and requeues for the next one.
func (r *AppReconciler) reconcileImageCheck(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	app := &appv1alpha1.App{}
	if err := r.Get(ctx, req.NamespacedName, app); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !app.DeletionTimestamp.IsZero() || !r.canonicalNamespace(app) {
		return ctrl.Result{}, nil
	}
	// Status times are whole seconds once stored.
	now := r.imageCheckClock().UTC().Truncate(time.Second)
	next := app.Status.DeepCopy()
	dropStaleImageCheck(next)
	var res ctrl.Result
	if image, holder, err := r.imageCheckTarget(ctx, app); err != nil {
		logf.FromContext(ctx).Info("image check: the registry credential's holder could not be read", "app", app.Name, "error", err.Error())
		res.RequeueAfter = min(imageCheckRetry, r.ImageCheckInterval)
	} else if image != "" {
		if due := r.nextImageCheck(app.UID, next); due.After(now) {
			res.RequeueAfter = due.Sub(now)
		} else {
			res.RequeueAfter = r.checkServedImage(ctx, app, holder, image, next, now)
		}
	}
	if err := r.writeImageCheck(ctx, app, imageCheckStatusOf(next)); err != nil {
		logf.FromContext(ctx).Error(err, "image check: the answer could not be stored", "app", app.Name)
		return ctrl.Result{RequeueAfter: imageCheckWriteRetry}, nil
	}
	return res, nil
}

func (r *AppReconciler) imageCheckClock() time.Time {
	if r.imageCheckNow != nil {
		return r.imageCheckNow()
	}
	return time.Now()
}

// imageCheckTarget is the image the check reads for app, and the App whose
// registry credential reads it (externalImageCredentials), or "" when app is
// not checked now. Checked: a web service, private service or background
// worker that runs (or sleeps, a free web service whose wake pulls its image)
// and is not suspended, whose served image (status.image, the reference its
// pods pull: a digest for an image-port service, the deployed tag for a
// worker) is a prebuilt image outside bex's own registry. A cron job has no
// running instance, and a static site or build source runs nothing; a build
// in bex's registry is kept there by retention's live tag (w1/m183).
func (r *AppReconciler) imageCheckTarget(ctx context.Context, app *appv1alpha1.App) (string, *appv1alpha1.App, error) {
	if app.Spec.Suspended || !app.RollsDeployment() ||
		app.Status.Phase != appv1alpha1.PhaseRunning && app.Status.Phase != appv1alpha1.PhaseHibernated {
		return "", nil, nil
	}
	image := app.Status.Image
	if _, _, platform := r.platformRepoReference(image); image == "" || platform {
		return "", nil, nil
	}
	if _, _, _, _, ok := parseExternalImageReference(image); !ok {
		return "", nil, nil
	}
	holder, external, err := r.externalImageCredentials(ctx, app, image)
	if err != nil || !external {
		return "", nil, err
	}
	return image, holder, nil
}

// nextImageCheck is when status's served image is next due: at once when it
// has no answer yet, an interval after the last answer, or imageCheckConfirm
// after a first refusal. Each wait is spread over ±10%, the same for an App
// and answer across manager restarts, so the checks of many services do not
// come due together.
func (r *AppReconciler) nextImageCheck(uid types.UID, status *appv1alpha1.AppStatus) time.Time {
	check := status.ImageCheck
	if check == nil {
		return time.Time{}
	}
	wait := r.ImageCheckInterval
	if check.Refusals > 0 && !imageUnavailable(status) {
		wait = min(imageCheckConfirm, wait)
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(uid))
	_ = binary.Write(h, binary.BigEndian, check.CheckedAt.Unix())
	spread := 0.9 + 0.2*float64(h.Sum64()%1000)/1000
	return check.CheckedAt.Add(time.Duration(float64(wait) * spread))
}

// checkServedImage asks the registry for app's served image, read with
// holder's credential, and records a conclusive answer into status. It returns
// when to come back: the next check's time, or a retry for an answer that was
// no verdict.
func (r *AppReconciler) checkServedImage(ctx context.Context, app, holder *appv1alpha1.App, image string, status *appv1alpha1.AppStatus, now time.Time) time.Duration {
	log := logf.FromContext(ctx).WithValues("app", app.Name, "image", image)
	host, repo, _, reference, _ := parseExternalImageReference(image)
	var authorization string
	err := gatedRegistryRead(ctx, r.imageCheckReads(), host, app.Namespace, func() error {
		if err := r.imageCheckPace.wait(ctx); err != nil {
			return err
		}
		var err error
		if authorization, err = r.externalImageAuthHeader(ctx, holder, host); err != nil {
			return err
		}
		return registry.CheckImageManifest(ctx, r.HTTPClient, host, repo, reference, authorization)
	})
	busy, isBusy := errors.AsType[*registryBusyError](err)
	transient, isTransient := errors.AsType[*registry.TransientError](err)
	refused, isRefused := errors.AsType[*registry.RefusedError](err)
	switch {
	case err == nil:
		recordImageServed(status, image, now)
	case isRefused && imageGoneStatus(refused.Status):
		reason, msg := imageRefusalReason(image, refused.Status, authorization != "")
		recordImageRefused(status, image, now, reason, msg)
	case isBusy:
		return busy.busyRequeue()
	case isTransient:
		log.Info("image check: the registry did not answer; retrying", "error", err.Error())
		return max(min(imageCheckRetry, r.ImageCheckInterval), transient.RetryAfter)
	default:
		// An answer that says nothing about the image (a 400, a redirect the
		// policy refuses, a credential that cannot be read) will not change
		// soon: the count and the condition stay, and the next check is an
		// interval away.
		log.Info("image check: the registry gave no verdict", "error", err.Error())
		return r.ImageCheckInterval
	}
	return r.nextImageCheck(app.UID, status).Sub(now)
}

// imageGoneStatus reports a registry answer that counts toward a refusal: not
// found, or refused with the authorization the kubelet would pull with.
func imageGoneStatus(status int) bool {
	return status == http.StatusNotFound || status == http.StatusUnauthorized || status == http.StatusForbidden
}

// imageRefusalReason is the ImageAvailable reason and message for a refusal
// of image: Render's "Image not found (404 error)" or "Authentication error",
// in the deploy reason templates' words (predeploy.RegistryRefusal).
// credentialed says the check carried the release's registry credential.
func imageRefusalReason(image string, status int, credentialed bool) (reason, msg string) {
	reason = appv1alpha1.ReasonImageNotFound
	switch {
	case status != http.StatusNotFound && credentialed:
		reason = appv1alpha1.ReasonImageCredentialRefused
	case status != http.StatusNotFound:
		reason = appv1alpha1.ReasonImageAccessRefused
	}
	return reason, predeploy.RegistryRefusal(image, status, credentialed)
}

// recordImageServed records an answer that served image: the condition turns
// (or stays) True and the count of refusals starts over.
func recordImageServed(status *appv1alpha1.AppStatus, image string, now time.Time) {
	meta.SetStatusCondition(&status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionImageAvailable, Status: metav1.ConditionTrue, Reason: appv1alpha1.ReasonImageAvailable,
		Message: "the registry serves the image " + image, LastTransitionTime: metav1.NewTime(now),
	})
	status.ImageCheck = &appv1alpha1.ImageCheckStatus{Image: image, CheckedAt: metav1.NewTime(now)}
}

// recordImageRefused records a refusal of image with its reason and message.
// The condition turns False once imageCheckRefusalsToFail answers in a row
// refused, and keeps its transition time while the outage lasts, so bex-api
// records one event per outage. status.imageCheck is always image's here:
// dropStaleImageCheck has dropped any other.
func recordImageRefused(status *appv1alpha1.AppStatus, image string, now time.Time, reason, msg string) {
	refusals := int32(1)
	if prior := status.ImageCheck; prior != nil {
		refusals += prior.Refusals
	}
	if refusals >= imageCheckRefusalsToFail {
		meta.SetStatusCondition(&status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionImageAvailable, Status: metav1.ConditionFalse, Reason: reason,
			Message: msg, LastTransitionTime: metav1.NewTime(now),
		})
	}
	status.ImageCheck = &appv1alpha1.ImageCheckStatus{Image: image, CheckedAt: metav1.NewTime(now), Refusals: refusals}
}

// imageUnavailable reports an ImageAvailable condition that is False.
func imageUnavailable(status *appv1alpha1.AppStatus) bool {
	return meta.IsStatusConditionFalse(status.Conditions, appv1alpha1.ConditionImageAvailable)
}

// dropStaleImageCheck forgets the check of an image that no longer serves (a
// deploy, a rollback or a switch to a build), with its condition: the next
// image's outage is its own.
func dropStaleImageCheck(status *appv1alpha1.AppStatus) {
	if check := status.ImageCheck; check != nil && check.Image != status.Image {
		status.ImageCheck = nil
		meta.RemoveStatusCondition(&status.Conditions, appv1alpha1.ConditionImageAvailable)
	}
}

// writeImageCheck persists owned, the check's part of app's status, when it
// moved. A conflict re-reads the App and applies it again, unless another
// image serves by then: the change wakes the check for that one.
func (r *AppReconciler) writeImageCheck(ctx context.Context, app *appv1alpha1.App, owned imageCheckStatus) error {
	for attempt := 1; ; attempt++ {
		if owned.equal(imageCheckStatusOf(&app.Status)) {
			return nil
		}
		err := r.patchAppStatus(ctx, app, owned.applyTo)
		if !apierrors.IsConflict(err) || attempt == statusWriteAttempts {
			return err
		}
		fresh := &appv1alpha1.App{}
		if err := cmp.Or(r.APIReader, client.Reader(r.Client)).Get(ctx, client.ObjectKeyFromObject(app), fresh); err != nil {
			return client.IgnoreNotFound(err)
		}
		if fresh.Status.Image != app.Status.Image {
			return nil
		}
		*app = *fresh
	}
}

// imageCheckStatus is the part of an App's status the app-image-check
// controller owns: status.imageCheck and the ImageAvailable condition. A
// runtime status write retried over a newer App keeps both from that App
// (updateStatusRetrying, setNotReadyCondition), as it keeps the saved
// configuration status.
type imageCheckStatus struct {
	check     *appv1alpha1.ImageCheckStatus
	condition *metav1.Condition
}

func imageCheckStatusOf(status *appv1alpha1.AppStatus) imageCheckStatus {
	owned := imageCheckStatus{check: status.ImageCheck.DeepCopy()}
	if c := meta.FindStatusCondition(status.Conditions, appv1alpha1.ConditionImageAvailable); c != nil {
		owned.condition = c.DeepCopy()
	}
	return owned
}

func (s imageCheckStatus) applyTo(status *appv1alpha1.AppStatus) {
	status.ImageCheck = s.check.DeepCopy()
	if s.condition == nil {
		meta.RemoveStatusCondition(&status.Conditions, appv1alpha1.ConditionImageAvailable)
		return
	}
	i := slices.IndexFunc(status.Conditions, func(c metav1.Condition) bool { return c.Type == appv1alpha1.ConditionImageAvailable })
	if i < 0 {
		status.Conditions = append(status.Conditions, *s.condition.DeepCopy())
		return
	}
	status.Conditions[i] = *s.condition.DeepCopy()
}

func (s imageCheckStatus) equal(o imageCheckStatus) bool {
	return equality.Semantic.DeepEqual(s.check, o.check) && equality.Semantic.DeepEqual(s.condition, o.condition)
}

// imageCheckPacer spaces checks imageCheckSpacing apart (spacing, when set,
// in tests). The controller has one worker, so a due App waits its turn in
// the queue, not in a requeue loop.
type imageCheckPacer struct {
	mu      sync.Mutex
	last    time.Time
	spacing time.Duration
}

// wait returns once the spacing has passed since the previous check.
func (p *imageCheckPacer) wait(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if gap := cmp.Or(p.spacing, imageCheckSpacing) - time.Since(p.last); gap > 0 {
		timer := time.NewTimer(gap)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	p.last = time.Now()
	return nil
}

// imageCheckReads is the check's own read gate: the same bounds per registry
// host and per workspace and the same breaker as a deploy's metadata read, but
// apart from it, so a check never turns a deploy's read away and a registry
// failing checks does not back deploys off.
func (r *AppReconciler) imageCheckReads() *registryReadGate {
	r.imageCheckReadsOnce.Do(func() { r.imageCheckReadGate = newRegistryReadGate() })
	return r.imageCheckReadGate
}
