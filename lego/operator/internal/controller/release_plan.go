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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A release on a Deployment is parked, woken, held behind the release that
// served, or rolled. planRelease decides which, once per pass, from facts the
// pass gathered first, and holdNewerRelease and the rollout carry the plan out.
// Each lifecycle fix (w1/m156, w1/m172, w6/m147, w6/076) is a rule in that one
// pure function, and release_plan_test.go checks every combination of facts
// against the invariant each fix established (w5/m123).

// releaseAct is what a pass does with its release.
type releaseAct int

const (
	// actRoll projects the current release onto the Deployment: the normal path.
	actRoll releaseAct = iota
	// actWake wakes an auto-hibernated App, once, for a newer release that has
	// not served (w6/076). A deploy is activity: Render marks a deploy live only
	// after the new instance passes its health check, and while parked the holds
	// keep the release off the Deployment until something wakes it. One wake per
	// release bounds a release that never rolls to one idle window.
	actWake
	// actGate runs the newer release's pre-deploy step while the served release
	// keeps serving (w1/m156). A step that passes on this pass plans again.
	actGate
	// actHoldStep keeps the served release's replicas and routing following the
	// App while the newer release's step has failed, or while the pass parks and
	// so does not run it: a parking pass would otherwise write the unmigrated
	// release onto the parked Deployment for the next wake to start (w1/m156).
	actHoldStep
	// actHoldVerdict puts the served release's template back after the newer
	// release's rollout settled failed, and keeps serving it (w1/m172). It ends
	// when a newer release moves the release generation past the verdict.
	actHoldVerdict
	// actHoldParked keeps the served release's template on a parked Deployment,
	// restored if a rollout was in flight when the park landed (w1/m172). Scaling
	// a Deployment up is not a rollout: a wake of a newer template starts its pods
	// with no progress deadline, and one that cannot become ready leaves the
	// service on the activator with no verdict to restore from.
	actHoldParked
	// actStartServed starts the served release's pods first on a wake or resume,
	// behind the activator. Once one is ready, or servedWakeBudget has passed, the
	// newer release rolls over it as a rolling update, which keeps the served pod
	// until the new one is ready and can fail into actHoldVerdict.
	actStartServed
)

// rolloutVerdict is the current release's Rollout condition.
type rolloutVerdict int

const (
	verdictNone     rolloutVerdict = iota // none recorded for the current release
	verdictRecorded                       // recorded, and not a failure over a release that served
	verdictFailed                         // settled failed over a release that served (failedRolloutOverServed)
)

// templateRev is the release the Deployment's pod template carries.
type templateRev int

const (
	templateOther   templateRev = iota // no Deployment, or neither release below
	templateServed                     // the release that served (status.activeRevision)
	templateCurrent                    // the current release, newer than the served one
)

// releaseFacts is everything planRelease decides from, gathered once per pass.
type releaseFacts struct {
	suspended bool
	// idle: auto-sleep eligible, the idle window elapsed and no traffic since.
	// desiredReplicas reads traffic only when it can decide the park.
	idle bool
	// newer: a release served, and the current one is newer
	// (newerReleaseUnserved).
	newer bool
	// servedRuntime: the served release's Deployment, and for a web or private
	// service its Service, exist, so a hold has something to keep serving.
	servedRuntime bool
	// wokeForRelease: the current release already had its one wake.
	wokeForRelease  bool
	verdict         rolloutVerdict
	preDeployPassed bool // no step, or it passed for the current release
	preDeployFailed bool // the step failed for the current release
	template        templateRev
	scaled          bool  // the Deployment's desired scale is above zero
	ready           bool  // one of its pods is ready
	replicas        int32 // the replicas the App runs awake
	// deadlineExceeded: the rollout passed its progress deadline, at deadlineAt.
	deadlineExceeded bool
	deadlineAt       time.Time
	// unavailableSince is when the Deployment last lost minimum availability
	// (Available=False): on a wake, when the served release's pods were asked
	// for. Zero while it is available or has not reported.
	unavailableSince time.Time
	// restorable is false once an executor found no served template to restore
	// (restoreServedTemplate); the pass then rolls.
	restorable bool
}

// releasePlan is planRelease's decision for one pass.
type releasePlan struct {
	act releaseAct
	// parked: the pass runs the workload at zero, suspended or auto-hibernating.
	// It is decided here once; every executor of the pass reads it.
	parked          bool
	autoHibernating bool
	// facts and now are the plan's inputs, kept for an executor that learns a
	// fact mid-pass and plans the next act.
	facts releaseFacts
	now   time.Time
}

// planRelease decides a pass from its facts alone.
func planRelease(f releaseFacts, now time.Time) releasePlan {
	autoHibernating := f.idle && !f.suspended && !f.awaitingVerdict(now)
	p := releasePlan{
		parked:          f.suspended || autoHibernating,
		autoHibernating: autoHibernating,
		facts:           f,
		now:             now,
	}
	p.act = p.nextAct()
	return p
}

// deploying reports whether the pass may stamp Deploying. Never while parked:
// the service is going to sleep, and the write would flash Deploying before
// Hibernated (w6/m147).
func (p releasePlan) deploying() bool {
	return !p.parked
}

// nextAct is the act for the plan's facts, in rollout order: the wake, then the
// three holds that keep a newer release off the pod template while the served
// release serves (a step that has not passed, a rollout that settled failed, a
// release that would otherwise start alone from zero), then the rollout.
func (p releasePlan) nextAct() releaseAct {
	f := p.facts
	switch {
	case p.autoHibernating && f.newer && !f.wokeForRelease && f.verdict != verdictFailed && !f.preDeployFailed:
		return actWake
	case !f.servedRuntime:
		return actRoll
	case !f.preDeployPassed:
		if f.preDeployFailed || p.parked {
			return actHoldStep
		}
		return actGate
	case f.verdict == verdictFailed && f.restorable:
		return actHoldVerdict
	case !f.newer:
		return actRoll
	case p.parked:
		if f.restorable {
			return actHoldParked
		}
		return actRoll
	case f.template == templateServed && !f.ready && f.replicas > 0 && !f.servedBudgetSpent(p.now):
		return actStartServed
	}
	return actRoll
}

// stepPassed records that the newer release's pre-deploy step passed on this
// pass, and plans the next act. The park decision stands.
func (p *releasePlan) stepPassed() {
	p.facts.preDeployPassed = true
	p.act = p.nextAct()
}

// unrestorable records that the served release has no template to restore
// from, and plans the next act. With nothing to keep, the pass rolls: a failed
// rollout then settles Failed rather than report a release no pod backs.
func (p *releasePlan) unrestorable() {
	p.facts.restorable = false
	p.act = p.nextAct()
}

// awaitingVerdict reports the current release, newer than the served one,
// rolling on an awake Deployment with no verdict yet. Such an App is not idle
// (w6/m147): the idle window equals the rollout budget, and a park before the
// verdict overwrote Ready with AutoHibernated, so the rollout never settled and
// its crash or probe diagnosis never reached the deploy row. Once
// settleFailedRollout records ConditionRollout the App may park; a settle that
// keeps failing stops deferring rolloutVerdictGrace past the deadline.
func (f releaseFacts) awaitingVerdict(now time.Time) bool {
	if !f.newer || f.verdict != verdictNone || f.template != templateCurrent || !f.scaled {
		return false
	}
	return !f.deadlineExceeded || now.Sub(f.deadlineAt) < rolloutVerdictGrace
}

// servedBudgetSpent reports that the served release has had servedWakeBudget to
// become available again.
func (f releaseFacts) servedBudgetSpent(now time.Time) bool {
	return !f.unavailableSince.IsZero() && now.Sub(f.unavailableSince) > servedWakeBudget
}

// rolloutVerdictGrace is how long past ProgressDeadlineExceeded a park still
// waits for settleFailedRollout's verdict, which normally lands the same pass.
const rolloutVerdictGrace = 2 * time.Minute

// servedWakeBudget is how long a newer release waits for the served release to
// become available before it rolls anyway, so a served release that can no
// longer start never blocks the release that might fix it. It counts from when
// the Deployment lost availability, not from pod age: pods its ReplicaSet cannot
// create never age, and an awake service whose only pod was just replaced would
// hold a hotfix for the full budget although it had been down for longer
// (w5/m123). The bound is the same for a wake, a resume and a paid service with
// no ready pod.
const servedWakeBudget = 5 * time.Minute

// wakeReadyPoll is how soon a held release's pass comes back while a woken free
// service waits for its first ready pod, so the public route leaves the
// activator promptly. The Deployment watch usually gets there first.
const wakeReadyPoll = 5 * time.Second

// priorRelease is the Deployment a serving prior release runs from and the port
// its Service exposes (0 for a background worker, which has none).
type priorRelease struct {
	dep  *appsv1.Deployment
	port int
}

// releaseObservation is the runtime a pass read for its release, once.
type releaseObservation struct {
	// dep is the App's Deployment; nil when it has none or nothing has served.
	dep *appsv1.Deployment
	// prior is the served release's runtime a hold keeps serving; nil when there
	// is none: nothing has served, or its Deployment, or a web or private
	// service's Service, is gone.
	prior *priorRelease
}

// observeRelease reads the App's Deployment and, for a web or private service,
// its Service. Nothing is read before a release has served: no hold applies,
// and no rollout can be awaiting a verdict over a served release.
func (r *AppReconciler) observeRelease(ctx context.Context, app *appv1alpha1.App) (releaseObservation, error) {
	if !releaseHasServed(app) {
		return releaseObservation{}, nil
	}
	key := client.ObjectKey{Namespace: app.Namespace, Name: app.Name}
	dep := &appsv1.Deployment{}
	if err := r.Get(ctx, key, dep); err != nil {
		return releaseObservation{}, client.IgnoreNotFound(err)
	}
	obs := releaseObservation{dep: dep}
	// A background worker has no Service or port: its runtime is the Deployment's
	// replicas alone (w1/m158).
	if !app.Spec.InternallyAddressable() {
		obs.prior = &priorRelease{dep: dep}
		return obs, nil
	}
	var svc corev1.Service
	if err := r.Get(ctx, key, &svc); err != nil {
		return obs, client.IgnoreNotFound(err)
	}
	// Route to the port the prior release's Service exposes: the held release may
	// have changed spec.port, and its pods are not the ones serving.
	if len(svc.Spec.Ports) > 0 {
		obs.prior = &priorRelease{dep: dep, port: int(svc.Spec.Ports[0].Port)}
	}
	return obs, nil
}

// facts derives the release's facts from app and the observed runtime. idle and
// replicas are the replica plan's to fill in (desiredReplicas).
func (obs releaseObservation) facts(app *appv1alpha1.App) releaseFacts {
	gen := releaseGeneration(app)
	f := releaseFacts{
		suspended:       app.Spec.Suspended,
		newer:           newerReleaseUnserved(app),
		servedRuntime:   obs.prior != nil,
		wokeForRelease:  app.Annotations[annotReleaseWakeGeneration] == strconv.FormatInt(gen, 10),
		verdict:         currentRolloutVerdict(app, gen),
		preDeployPassed: preDeployPassed(app),
		preDeployFailed: preDeployFailedFor(app, gen),
		restorable:      true,
	}
	if dep := obs.dep; dep != nil {
		switch dep.Spec.Template.Labels[labelRevision] {
		case app.Status.ActiveRevision:
			f.template = templateServed
		case releaseRevision(app):
			f.template = templateCurrent
		}
		f.scaled = dep.Spec.Replicas != nil && *dep.Spec.Replicas > 0
		f.ready = dep.Status.ReadyReplicas > 0
		if c := progressDeadlineExceeded(dep); c != nil {
			f.deadlineExceeded, f.deadlineAt = true, c.LastTransitionTime.Time
		}
		if c := deploymentCondition(dep, appsv1.DeploymentAvailable); c != nil && c.Status == corev1.ConditionFalse {
			f.unavailableSince = c.LastTransitionTime.Time
		}
	}
	return f
}

// startingServedCondition is Ready while the served release starts before a
// newer release rolls. When the Deployment reports pods its ReplicaSet cannot
// create (ReplicaFailure: the workspace quota, an admission webhook such as the
// image-signature check after a key rotation), Ready names the cause, which
// bex-api shows as the deploy's stall reason: the wait would otherwise read as
// ordinary progress until the newer release rolls without it.
func startingServedCondition(app *appv1alpha1.App, dep *appsv1.Deployment) metav1.Condition {
	condition := metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reasonRolloutProgressing,
		Message: "starting the previously deployed release before rolling the new one", ObservedGeneration: app.Generation,
	}
	if c := deploymentCondition(dep, appsv1.DeploymentReplicaFailure); c != nil && c.Status == corev1.ConditionTrue {
		condition.Reason = appv1alpha1.ReasonServedReleaseCannotStart
		condition.Message = fmt.Sprintf("the previously deployed release cannot create its pods: %s; the new release rolls anyway once it has been unavailable for %d minutes",
			strings.TrimSuffix(c.Message, "."), int(servedWakeBudget.Minutes()))
	}
	return condition
}

// currentRolloutVerdict classifies the Rollout condition of release gen.
func currentRolloutVerdict(app *appv1alpha1.App, gen int64) rolloutVerdict {
	if failedRolloutOverServed(app) {
		return verdictFailed
	}
	if c := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout); c != nil && c.ObservedGeneration == gen {
		return verdictRecorded
	}
	return verdictNone
}

// newerReleaseUnserved reports a current release newer than the one that served.
func newerReleaseUnserved(app *appv1alpha1.App) bool {
	return releaseHasServed(app) && successfulReleaseGeneration(app) != releaseGeneration(app)
}

// preDeployFailedFor reports a failed pre-deploy verdict stored for release gen.
func preDeployFailedFor(app *appv1alpha1.App, gen int64) bool {
	pd := app.Status.PreDeploy
	return pd != nil && pd.Generation == gen && pd.Status == appv1alpha1.PreDeployFailed
}

// holdNewerRelease carries out the plan's act for a pass whose newer release may
// be held off the pod template while the served release, plan.prior, serves.
// held=false hands the pass to the rollout. An executor that learns a fact
// mid-pass (a step that passed, a template it cannot restore) records it on
// the plan and carries out the act planned from it.
func (r *AppReconciler) holdNewerRelease(ctx context.Context, app *appv1alpha1.App, image string, port int, plan *replicaPlan) (bool, ctrl.Result, error) {
	held := func(res ctrl.Result, err error) (bool, ctrl.Result, error) {
		if err != nil {
			res, err = r.failStep(ctx, app, err)
		}
		return true, res, err
	}
	// Each learned fact removes the act that learned it, so the loop plans at most
	// three times.
	for {
		switch plan.act {
		case actWake:
			gen := releaseGeneration(app)
			if err := r.stampLastActive(ctx, app, time.Now(), [2]string{annotReleaseWakeGeneration, strconv.FormatInt(gen, 10)}); err != nil {
				return true, ctrl.Result{}, err
			}
			logf.FromContext(ctx).Info("waking app to roll out a new release", "name", app.Name, "releaseGeneration", gen)
			// The annotation patch alone triggers no reconcile.
			return true, ctrl.Result{RequeueAfter: wakeReadyPoll}, nil
		case actGate:
			res, halt, err := r.reconcilePreDeploy(ctx, app, image, port)
			if err != nil {
				return true, res, err
			}
			if !halt {
				plan.stepPassed()
				continue
			}
			plan.poll = res.RequeueAfter > 0
			return held(r.settleHeldRuntime(ctx, app, *plan, "", res.RequeueAfter))
		case actHoldStep:
			failed := ""
			if plan.facts.preDeployFailed {
				failed = "the latest pre-deploy command failed"
			}
			return held(r.settleHeldRuntime(ctx, app, *plan, failed, 0))
		case actHoldVerdict, actHoldParked:
			restored, err := r.restoreServedTemplate(ctx, app, plan.prior.dep)
			if err != nil {
				// Unrecorded: a read that failed says nothing about the serving release.
				return true, ctrl.Result{}, err
			}
			if !restored {
				plan.unrestorable()
				continue
			}
			if plan.act == actHoldParked {
				return held(r.convergeServingRuntime(ctx, app, *plan))
			}
			verdict := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout)
			return held(r.settleHeldRuntime(ctx, app, *plan, failedRolloutSummary(verdict.Message), 0))
		case actStartServed:
			res, err := r.convergeServingRuntime(ctx, app, *plan)
			if err != nil {
				return held(res, err)
			}
			app.Status.Phase = appv1alpha1.PhaseDeploying
			meta.SetStatusCondition(&app.Status.Conditions, startingServedCondition(app, plan.prior.dep))
			r.updateStatusRetrying(ctx, app, "releaseHeld")
			res.RequeueAfter = soonerRequeue(res.RequeueAfter, wakeReadyPoll)
			return true, res, nil
		default:
			return false, ctrl.Result{}, nil
		}
	}
}
