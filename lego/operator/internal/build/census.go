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

package build

import (
	"context"
	"fmt"
	"sort"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

// Census is the build plane's live admission picture (docs/ADR060 D5) —
// recomputed from a full listing rather than accumulated, so a manager restart
// or a missed event self-heals on the next pass instead of stranding a gauge
// above zero forever.
type Census struct {
	// Active is every dispatched build that has not finished: queued ones
	// included. It is the quantity BEX_MAX_ACTIVE_BUILDS caps, so publishing it
	// is what makes the ceiling observable rather than merely enforced.
	Active int
	// Queued is the subset of Active whose pod the scheduler has not yet placed
	// on a node — waiting for capacity, doing no work. Always ≤ Active.
	Queued int
	// OldestQueued is how long the longest-waiting queued build has been waiting,
	// measured from its Job's creation. A single low-cardinality series can then
	// answer "has any build been Pending beyond N minutes?" exactly, which a
	// histogram of finished builds cannot: an alert must fire while the build is
	// still waiting, before the 18-minute deploy gate closes it.
	OldestQueued time.Duration
}

// CountBuilds takes the census of namespace at time now.
func CountBuilds(ctx context.Context, cl client.Client, namespace string, now time.Time) (Census, error) {
	sel := workspaceSelector("")
	jobs, err := listBuildJobs(ctx, cl, namespace, sel)
	if err != nil {
		return Census{}, err
	}
	inFlight := map[string]*batchv1.Job{}
	for i := range jobs {
		if !execution.JobFinished(&jobs[i]) {
			inFlight[jobs[i].Name] = &jobs[i]
		}
	}

	census := Census{Active: len(inFlight)}
	// The pod listing is the heavy one (a build pod carries the whole hardened
	// spec, and every finished build's pods linger for the Job's hour-long TTL),
	// so an idle build plane — the steady state — pays for it never.
	if len(inFlight) > 0 {
		var pods corev1.PodList
		if err := cl.List(ctx, &pods, client.InNamespace(namespace), sel); err != nil {
			return Census{}, fmt.Errorf("list build pods in %s: %w", namespace, err)
		}
		byJob := map[string][]corev1.Pod{}
		for i := range pods.Items {
			if job := pods.Items[i].Labels[jobNameLabel]; inFlight[job] != nil {
				byJob[job] = append(byJob[job], pods.Items[i])
			}
		}
		for name, job := range inFlight {
			if podsPlaced(byJob[name]) {
				continue
			}
			census.Queued++
			if waited := now.Sub(job.CreationTimestamp.Time); waited > census.OldestQueued {
				census.OldestQueued = waited
			}
		}
	}
	// kpack Images dispatch no Job we own, so they count toward Active (they hold
	// the same capacity) but never toward Queued — their pods carry kpack's own
	// labels, and inventing a queue state for them would report a wait bex cannot
	// see.
	kpackActive, err := activeKpackImages(ctx, cl, namespace, sel)
	if err != nil {
		return Census{}, err
	}
	census.Active += kpackActive
	return census, nil
}

// TrimOvershoot re-counts active builds after a create and deletes the NEWEST
// ones until the count is back at limit, returning the names it deleted.
//
// The per-workspace and cluster-wide gates both list-then-create, so two
// concurrent reconciles can each see a free slot and both dispatch. ADR060 D6
// asks for a self-correction rather than a distributed lock: re-count on the
// pass that created a build and shed the excess. Deleting the NEWEST is what
// makes it safe — the oldest build has been running longest, so the loser of the
// race is always the one that did the least work, and the ordering is total, so
// two racers converge on the same survivor instead of deleting each other.
//
// Only Jobs are trimmed. A kpack Image counts toward the limit (it holds the
// same capacity) but is never deleted here: kpack owns its own build objects,
// and its dispatch is still bounded by the pre-create gate.
func TrimOvershoot(ctx context.Context, cl client.Client, namespace, workspace string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}
	sel := workspaceSelector(workspace)
	jobs, err := listBuildJobs(ctx, cl, namespace, sel)
	if err != nil {
		return nil, err
	}
	live := make([]*batchv1.Job, 0, len(jobs))
	for i := range jobs {
		if !execution.JobFinished(&jobs[i]) {
			live = append(live, &jobs[i])
		}
	}
	// kpack Images hold the same capacity and so count toward the limit, even
	// though only Jobs can be shed below.
	kpackActive, err := activeKpackImages(ctx, cl, namespace, sel)
	if err != nil {
		return nil, err
	}
	excess := len(live) + kpackActive - limit
	if excess <= 0 {
		return nil, nil
	}
	// Newest first; ties broken by name descending so the choice is deterministic
	// across the racing reconciles that are each running this at the same moment.
	sort.Slice(live, func(a, b int) bool {
		return newerFirst(live[a].CreationTimestamp, live[a].Name, live[b].CreationTimestamp, live[b].Name)
	})

	deleted := make([]string, 0, excess)
	for _, job := range live {
		if len(deleted) >= excess {
			break
		}
		// Background propagation: the build pod must go with the Job, but the
		// caller is a reconcile that should not block on the kubelet's teardown.
		if err := cl.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			if apierrors.IsNotFound(err) {
				continue // another reconcile shed the same overshoot first
			}
			return deleted, fmt.Errorf("trim overshoot: delete build %s: %w", job.Name, err)
		}
		deleted = append(deleted, job.Name)
	}
	return deleted, nil
}

// newerFirst orders build objects newest-first, breaking creation-timestamp ties
// on name. The tiebreak is load-bearing rather than cosmetic: Kubernetes
// timestamps have one-second resolution, so two objects created in the same
// second must still order identically for every reader — otherwise racing
// reconciles could each pick a different "newest" and delete both.
func newerFirst(aTime metav1.Time, aName string, bTime metav1.Time, bName string) bool {
	if aTime.Equal(&bTime) {
		return aName > bName
	}
	return aTime.After(bTime.Time)
}
