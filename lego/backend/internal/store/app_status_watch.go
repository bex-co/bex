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

package store

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/maphash"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// AppStatusWatch kicks the projector when the operator changes an App status
// field the projector records (w5/m369, from w4/624). Without it the control
// plane learned a deploy's verdict only on its next resync: a build that failed
// in 3 s read "in progress" for ~45 s, and every webhook, push and email waited
// for the same pass. The resync stays the level-triggered backstop; this only
// makes a change arrive sooner, so a dropped watch costs latency, never a
// missed record.
//
// It compares a fingerprint of the lifecycle fields (statusFingerprint) rather
// than kicking on every update, so status fields that move without anything to
// record (port scans, network observations) cost nothing. A spec write still
// costs a second pass: the operator stamps the new generation on its
// conditions, which is when the new release's verdict can first be read.
type AppStatusWatch struct {
	Client client.WithWatch
	// Kick requests a projector pass (Reconciler.KickFromWatch, which also
	// spaces watch-driven passes).
	Kick func()
	// Metrics counts watch failures; nil means not metered.
	Metrics *ReconcilerMetrics
	// Backoff is the wait before restarting a failed watch. Zero means 2 s.
	Backoff time.Duration

	// seen is each watched App's last fingerprint hash; resourceVersion is
	// where the next watch resumes ("" starts over with a fresh listing).
	seen            map[types.UID]uint64
	resourceVersion string
}

// Run watches until ctx is done, restarting the watch when it ends.
func (w *AppStatusWatch) Run(ctx context.Context) {
	backoff := w.Backoff
	if backoff <= 0 {
		backoff = 2 * time.Second
	}
	core.Poll(ctx, "controlplane: App status watch", backoff, w.watchOnce)
}

// watchOnce runs one watch until it ends. It returns nil when the server
// closed the watch (its routine timeout), so the next starts at once, and an
// error when it failed, so the next waits a tick.
func (w *AppStatusWatch) watchOnce(ctx context.Context) error {
	if w.resourceVersion == "" {
		// A fresh listing replays every App as ADDED. After a gap something may
		// have changed unseen, and Apps deleted meanwhile sent no DELETED: start
		// the memory over, and kick once if there was any.
		if len(w.seen) > 0 {
			w.Kick()
		}
		w.seen = map[types.UID]uint64{}
	}
	wi, err := w.Client.Watch(ctx, &appv1alpha1.AppList{},
		client.MatchingLabels{LabelManagedBy: ManagedByValue},
		&client.ListOptions{Raw: &metav1.ListOptions{ResourceVersion: w.resourceVersion, AllowWatchBookmarks: true}})
	if err != nil {
		return w.failed(err)
	}
	defer wi.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-wi.ResultChan():
			if !ok {
				return nil
			}
			if ev.Type == watch.Error {
				return w.failed(apierrors.FromObject(ev.Object))
			}
			app, isApp := ev.Object.(*appv1alpha1.App)
			if !isApp {
				continue
			}
			w.resourceVersion = app.ResourceVersion
			switch ev.Type {
			case watch.Added, watch.Modified:
				fingerprint := statusFingerprint(&app.Status)
				before, known := w.seen[app.UID]
				w.seen[app.UID] = fingerprint
				// An App first seen through MODIFIED missed its ADDED (an App
				// created on this watch arrives as ADDED, and a fresh start
				// replays every App as ADDED), so what changed is unknown: kick.
				if (known && before != fingerprint) || (!known && ev.Type == watch.Modified) {
					w.Kick()
				}
			case watch.Deleted:
				delete(w.seen, app.UID)
			case watch.Bookmark:
				// Only advances resourceVersion, above.
			}
		}
	}
}

// failed counts a failed watch and returns its error. An expired resource
// version (410) makes the next watch start over with a fresh listing.
func (w *AppStatusWatch) failed(err error) error {
	w.Metrics.WatchFailure()
	if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) {
		w.resourceVersion = ""
	}
	return fmt.Errorf("watch: %w", err)
}

// statusFingerprint is the part of an App's status the projector turns into
// deploy rows and service events: the phase and condition verdicts, the build,
// pre-deploy and release records, rollout failures, instance exits, cron runs
// and autoscaling transitions. A change to any of them is something
// recordObservations may record; nothing else in the status is.
//
// Conditions are reduced to their verdicts (type, status, reason, generation):
// a message rewording or a re-stamped transition time is not a new fact.
//
// It is kept as a 64-bit hash: a collision costs one missed kick, which the
// resync covers.
func statusFingerprint(s *appv1alpha1.AppStatus) uint64 {
	type verdict struct {
		Type, Status, Reason string
		Generation           int64
	}
	conditions := make([]verdict, 0, len(s.Conditions))
	for _, c := range s.Conditions {
		conditions = append(conditions, verdict{c.Type, string(c.Status), c.Reason, c.ObservedGeneration})
	}
	b, err := json.Marshal(struct {
		Phase              appv1alpha1.AppPhase
		Conditions         []verdict
		BuildRun           *appv1alpha1.BuildRunStatus
		BuildHold          *appv1alpha1.BuildHoldStatus
		PreDeploy          *appv1alpha1.PreDeployStatus
		ReleaseStart       *appv1alpha1.ReleaseStart
		ReleaseGeneration  int64
		ActiveRevision     string
		RolloutFailure     *appv1alpha1.RolloutFailureStatus
		InstanceExits      []appv1alpha1.InstanceExit
		Runs               []appv1alpha1.CronRun
		Autoscaling        *appv1alpha1.AutoscalingStatus
		ManualRunHandledAt string
		CancelRunRecorded  string
	}{
		s.Phase, conditions, s.BuildRun, s.BuildHold, s.PreDeploy, s.ReleaseStart, s.ReleaseGeneration,
		s.ActiveRevision, s.RolloutFailure, s.InstanceExits, s.Runs, s.Autoscaling,
		s.ManualRunHandledAt, s.CancelRunRecorded,
	})
	if err != nil {
		// Every field above is plain data, so this cannot happen. If it ever
		// did, a constant compares equal to itself: the watch would stop
		// kicking and the resync would carry on.
		return 0
	}
	return maphash.Bytes(fingerprintSeed, b)
}

// fingerprintSeed is fixed for the process, so fingerprints compare across
// watch restarts.
var fingerprintSeed = maphash.MakeSeed()
