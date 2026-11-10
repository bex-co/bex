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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/bex-co/bex/lego/operator/internal/build"
)

// defaultBuildCensusInterval paces the recount. The gauges answer "is capacity
// short right now?", and the alert built on them (BuildQueuedTooLong) has to
// fire minutes before the 18-minute deploy gate — so a resolution of half a
// minute is ample, while the listing itself goes to the UNCACHED build-plane
// client and should not be run per-second against the API server.
const defaultBuildCensusInterval = 30 * time.Second

// BuildCensusCollector republishes the build plane's live admission gauges on an
// interval (docs/ADR060 D5). It is a Runnable rather than reconcile-driven work
// for two reasons: the gauges are CLUSTER-wide, so no single App's reconcile can
// compute them; and recomputing from a full listing is what makes them
// restart-safe — after a manager restart the very first tick republishes the
// truth, where an edge-driven counter would have lost its accumulated state.
type BuildCensusCollector struct {
	// Client reads the build plane. Nil disables the collector.
	Client client.Client
	// Namespace is the build namespace (BEX_BUILD_NAMESPACE). Empty means builds
	// run in each App's own namespace, which this collector cannot enumerate from
	// one place — it then stays off rather than publishing a partial census.
	Namespace string
	// Apps reads App status for the releases held before dispatch
	// (status.buildHold, w6/m306): they have no build Job, so the build-plane
	// listing above cannot see them. Nil leaves the held gauges unpublished.
	Apps client.Reader
	// Interval overrides defaultBuildCensusInterval (tests).
	Interval time.Duration
}

// NeedLeaderElection keeps exactly one replica publishing. Two leaders would not
// corrupt the gauges (each Set is idempotent and both observe the same cluster),
// but a non-leader standby that stops ticking would freeze its own last value
// forever — so only the instance actually doing the work reports.
func (c *BuildCensusCollector) NeedLeaderElection() bool { return true }

// Start runs until ctx is cancelled. A failed listing logs and leaves the
// previous values in place rather than zeroing them: a transient API error is
// not evidence that the build plane went idle, and zeroing would resolve a live
// queueing alert on nothing.
func (c *BuildCensusCollector) Start(ctx context.Context) error {
	if c.Client == nil || c.Namespace == "" {
		return nil
	}
	interval := c.Interval
	if interval <= 0 {
		interval = defaultBuildCensusInterval
	}
	ctx = logf.IntoContext(ctx, logf.FromContext(ctx).WithName("build-census"))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		c.collect(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (c *BuildCensusCollector) collect(ctx context.Context) {
	census, err := build.CountBuilds(ctx, c.Client, c.Namespace, time.Now())
	if err != nil {
		logf.FromContext(ctx).Error(err, "build census failed; leaving the previous gauge values in place")
		return
	}
	publishBuildCensus(census)
	if c.Apps == nil {
		return
	}
	held, err := countHeldBuilds(ctx, c.Apps, time.Now())
	if err != nil {
		logf.FromContext(ctx).Error(err, "held-build census failed; leaving the previous gauge values in place")
		return
	}
	publishHoldCensus(held)
}

// holdCensus is the held-release half of the admission picture.
type holdCensus struct {
	held   map[string]int // by cause
	oldest time.Duration
}

// countHeldBuilds counts the releases whose build waits before dispatch
// (heldBuild), by cause, and the age of the oldest.
func countHeldBuilds(ctx context.Context, apps client.Reader, now time.Time) (holdCensus, error) {
	held, err := heldBuilds(ctx, apps, "")
	if err != nil {
		return holdCensus{}, err
	}
	census := holdCensus{held: map[string]int{}}
	for _, app := range held {
		hold := app.Status.BuildHold
		census.held[hold.Cause]++
		if waited := now.Sub(hold.Since.Time); waited > census.oldest {
			census.oldest = waited
		}
	}
	return census, nil
}
