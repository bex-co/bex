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

package sandbox

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/egressquery"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The used-CPU meter (w22/m314, ADR104 D8, ADR023 § Sandbox CPU billed on use).
//
// A sandbox billed on use pays for the CPU its pod used, as on Render, so an
// idle sandbox's CPU charge is close to nothing. An agent session's sandbox
// keeps the weighted meter, and its CPU is read the same way and recorded
// unbilled beside it (w6/m566). The figure is the pod-level
// cgroup counter: under gVisor the workload container has no CPU series of its
// own, and the pod's includes the sentry's overhead, which is billed as
// measured (ADR104 D8). Prometheus keeps that counter for sandbox namespaces as
// SandboxPodCPUMetric (deploy/gitops/base/prometheus.yaml).
//
// The store holds one running interval per stretch a sandbox ran on one pod.
// Each poll the meter reads the next ended window of every interval (one UTC
// hour at most) and adds it to sandbox_cpu_seconds. A window Prometheus cannot
// answer for stays where it is and is read again later: an outage delays a
// charge and never writes a zero.

// SandboxPodCPUMetric is the pod-level CPU counter of a sandbox pod, in
// CPU-seconds. The kubernetes-cadvisor job renames the pod cgroup's
// container_cpu_usage_seconds_total row (container="") to it for *-sandbox
// namespaces and drops that row everywhere else.
const SandboxPodCPUMetric = "sandbox_pod_cpu_usage_seconds_total"

const (
	// cpuWindowBatch bounds the windows one poll reads; the rest wait for the
	// next one.
	cpuWindowBatch = 200
	// cpuScrapeLag is how long after a window ended the meter reads it: four
	// 15 s scrapes, so the window's last sample is in Prometheus.
	cpuScrapeLag = time.Minute
	// cpuUnsampledWindow is the longest window that may hold no sample without
	// anything being wrong: the pod ran for less than three scrapes of it.
	cpuUnsampledWindow = 45 * time.Second
	// cpuGiveUpAfter is how long a window with no reading is retried. It ends
	// inside the 48 h after which billing exports the hour, so a late reading
	// is still billed; past it Prometheus will not produce one.
	cpuGiveUpAfter = 36 * time.Hour
	// cpuReadBudget bounds the time one poll spends on Prometheus, and
	// cpuReadFailures how many consecutive failures end it early: a hung
	// Prometheus must not hold up the next poll of every workspace.
	cpuReadBudget   = 20 * time.Second
	cpuReadFailures = 3
	// cpuMaxCores rejects a reading no sandbox pod can produce (the largest
	// plan is far below it), so a corrupt counter is deferred, not billed.
	cpuMaxCores = 256
)

// CPUReader reads the CPU a pod used over a window that has ended. started is
// when the window's running interval began: a window that starts soon after it
// is reading a new pod, whose counter may be read from zero. sampled is false when the source holds no sample of the
// pod in the window: there is no reading, which is not the same as zero CPU.
type CPUReader interface {
	PodCPUSeconds(ctx context.Context, namespace, pod string, started, from, to time.Time) (seconds float64, sampled bool, err error)
}

// PrometheusCPU reads SandboxPodCPUMetric through the Prometheus HTTP API.
type PrometheusCPU struct {
	Base string // BEX_PROM_URL
	HTTP *http.Client
}

// kubeNameRE is what a namespace or pod name may contain; both go into a
// PromQL string matcher.
var kubeNameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

// cpuBaselineLookback is how far before a window the read looks for the
// counter's value at the window's start: twenty 15 s scrapes.
const cpuBaselineLookback = 5 * time.Minute

// PodCPUSeconds is how far the pod's counter moved between its scraped samples
// over (from, to], both taken to the whole second so that consecutive windows
// meet exactly: its last sample in the window minus its last sample at or
// before the window's start. Nothing is extrapolated to the window's edges
// (w22/024): increase() fills the time after the last scrape at the window's
// average rate, which billed a sandbox that went idle before it stopped for
// more CPU than its pod's counter ever showed. Consecutive windows therefore
// add up to the pod's last scraped value at most, and what the pod used after
// its last scrape is not read.
//
// A series with no sample in the cpuBaselineLookback before the window is
// counted from zero only when the window starts within that lookback of the
// interval's start: the pod is new, and its cgroup counter started at zero
// with it. In a later window it is counted from its first sample in the
// window: a scrape gap, a relabelled series or a Prometheus that lost its
// history must never bill a counter's whole past again. A counter that restarts counts from zero from the restart.
// A recreated pod is a second series, read the same way. One sample of a
// series with nothing before it is a reading of no measurable increase; no
// sample in the window is no reading.
func (p PrometheusCPU) PodCPUSeconds(ctx context.Context, namespace, pod string, started, from, to time.Time) (float64, bool, error) {
	if !kubeNameRE.MatchString(namespace) || !kubeNameRE.MatchString(pod) {
		return 0, false, fmt.Errorf("invalid sandbox pod identity")
	}
	start, end := float64(from.Unix()), float64(to.Unix())
	if end <= start {
		return 0, true, nil
	}
	selector := fmt.Sprintf(`%s{namespace=%q,pod=%q}[%ds]`, SandboxPodCPUMetric, namespace, pod,
		int64(end-start)+int64(cpuBaselineLookback/time.Second))
	series, err := egressquery.RawSamples(ctx, p.HTTP, p.Base, selector, to)
	if err != nil {
		return 0, false, err
	}
	newPod := from.Sub(started) <= cpuBaselineLookback
	var used float64
	sampled := false
	for _, samples := range series {
		last, known := 0.0, newPod
		for _, s := range samples {
			if s.Time > start {
				sampled = true
				if known && s.Value >= last {
					used += s.Value - last
				} else if known {
					used += s.Value // the counter restarted
				}
			}
			last, known = s.Value, true
		}
	}
	return used, sampled, nil
}

// settleCPU reads the windows that are due and reports the meter's health. It
// runs after every poll round. Nil CPU turns the meter off: intervals are
// recorded and nothing is read (the feature is off, not wrong).
func (m *Meter) settleCPU(ctx context.Context) {
	if _, err := m.Store.PruneSandboxIntervals(ctx, m.now(), storagePruneBatch); err != nil {
		log.Printf("sandbox meter: prune running intervals: %v", err)
	}
	if m.CPU != nil {
		m.readCPUWindows(ctx)
	}
	if stats, err := m.Store.SandboxCPUStats(ctx); err != nil {
		log.Printf("sandbox meter: cpu stats: %v", err)
	} else {
		m.CPUMetrics.set(stats)
	}
}

func (m *Meter) readCPUWindows(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, cpuReadBudget)
	defer cancel()
	now := m.now()
	windows, err := m.Store.ClaimSandboxCPUWindows(ctx, now, now.Add(-cpuScrapeLag), cpuWindowBatch)
	if err != nil {
		log.Printf("sandbox meter: claim cpu windows: %v", err)
		return
	}
	failed, consecutive := 0, 0
	var lastErr error
	for _, w := range windows {
		err := m.readCPUWindow(ctx, w, now)
		if err == nil {
			consecutive = 0
			continue
		}
		failed++
		lastErr = err
		m.CPUMetrics.readFailed()
		if consecutive++; consecutive >= cpuReadFailures {
			break
		}
	}
	if failed > 0 {
		log.Printf("sandbox meter: %d of %d cpu windows not read, retried on a later poll (last: %v)", failed, len(windows), lastErr)
	}
}

// readCPUWindow reads one window and either adds its CPU-seconds or defers it.
// It returns an error only for a failed read or write, which the caller counts.
func (m *Meter) readCPUWindow(ctx context.Context, w store.SandboxCPUWindow, now time.Time) error {
	seconds, sampled, err := m.CPU.PodCPUSeconds(ctx, store.SandboxNamespace(w.WorkspaceID), w.PodName, w.StartedAt, w.From, w.To)
	length := w.To.Sub(w.From)
	if err == nil && sampled && seconds > length.Seconds()*cpuMaxCores {
		err = fmt.Errorf("implausible reading of %.0f CPU-seconds over %s", seconds, length)
	}
	switch {
	case err != nil:
		if deferErr := m.Store.DeferSandboxCPUWindow(ctx, w, now); deferErr != nil {
			log.Printf("sandbox meter: defer cpu window workspace=%s sandbox=%s: %v", w.WorkspaceID, w.SandboxID, deferErr)
		}
		return err
	case sampled:
	case length < cpuUnsampledWindow:
		// Too short to have been scraped: nothing to read, and nothing wrong.
	case now.Sub(w.To) > cpuGiveUpAfter:
		// Prometheus never held this window. Close it unbilled, loudly: leaving
		// it would hold the interval's later windows back for good.
		m.CPUMetrics.windowAbandoned()
		log.Printf("sandbox meter: cpu window workspace=%s sandbox=%s pod=%s %s..%s has no samples after %s; closed unbilled",
			w.WorkspaceID, w.SandboxID, w.PodName, w.From.Format(time.RFC3339), w.To.Format(time.RFC3339), cpuGiveUpAfter)
	default:
		// A running pod with no sample: the scrape is missing, not the usage.
		return m.Store.DeferSandboxCPUWindow(ctx, w, now)
	}
	_, err = m.Store.AccrueSandboxCPUWindow(ctx, w, seconds)
	return err
}

// sandboxPodName is the pod that runs a sandbox: the pinned OpenSandbox
// controller names a non-pooled BatchSandbox's only pod `<name>-0`, and the
// BatchSandbox is named after the substrate sandbox id. A resume that creates
// a new substrate sandbox therefore changes the name, and one that recreates
// the pod under the same name is told apart by its running interval.
func sandboxPodName(raw osSandbox) string {
	if raw.ID == "" {
		return ""
	}
	return raw.ID + "-0"
}
