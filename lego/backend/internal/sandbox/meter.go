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
	"math"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// MeterInterval bounds the phase-transition attribution error. Lifecycle
	// API calls also record immediately; polling catches upstream timeout/idle
	// transitions and sandboxes changed outside bex-api.
	MeterInterval = 30 * time.Second

	// AgentCore's public CPU/memory reference prices from ADR047 D6. A sandbox
	// shape is folded into milli-vCPU equivalents using this price ratio, so a
	// single additive meter preserves both resource dimensions.
	agentCoreCPUPerHour    = 0.0895
	agentCoreMemoryPerHour = 0.00945
)

// MeterStore is the durable cursor slice used by the sandbox meter. PGStore
// implements it; tests use a small fake.
type MeterStore interface {
	ListSandboxTenantKeys(context.Context) ([]store.SandboxTenantKey, error)
	ObserveSandboxMeter(context.Context, store.SandboxMeterObservation) error
	TerminateMissingSandboxMeters(context.Context, string, []string, time.Time) error
}

// Meter observes OpenSandbox's authoritative lifecycle state and advances the
// per-sandbox usage cursor. It is shared by request-path lifecycle observations
// and the background tenant-scoped poller.
type Meter struct {
	Client   *Client
	Store    MeterStore
	Interval time.Duration
	Now      func() time.Time
}

func (m *Meter) now() time.Time {
	if m.Now != nil {
		return m.Now().UTC()
	}
	return time.Now().UTC()
}

// computeWeightMilli returns the number of milli-vCPU-equivalent seconds
// accrued by one wall-clock running second. CPU is native millicores; memory
// GiB is converted at memory-price / CPU-price. The checked-in default
// 500m/512Mi shape therefore weighs 553 (500 + round(52.793)).
func computeWeightMilli(cpu, memory string) (int64, error) {
	cpuQ, err := resource.ParseQuantity(cpu)
	if err != nil || cpuQ.MilliValue() <= 0 {
		return 0, fmt.Errorf("invalid sandbox template CPU %q", cpu)
	}
	memQ, err := resource.ParseQuantity(memory)
	if err != nil || memQ.Value() <= 0 {
		return 0, fmt.Errorf("invalid sandbox template memory %q", memory)
	}
	memoryGiB := float64(memQ.Value()) / float64(int64(1)<<30)
	weight := float64(cpuQ.MilliValue()) + memoryGiB*(agentCoreMemoryPerHour/agentCoreCPUPerHour)*1000
	if weight > math.MaxInt32 {
		return 0, fmt.Errorf("sandbox template resource weight is too large")
	}
	return int64(math.Round(weight)), nil
}

func canonicalMeterPhase(state string) string {
	switch mapOpenSandboxStatus(state) {
	case StatusRunning:
		return "running"
	case StatusCreating:
		return "creating"
	case StatusSuspended:
		return "suspended"
	case StatusResuming:
		return "resuming"
	case StatusTerminated:
		return "terminated"
	default:
		return "errored"
	}
}

func meterObservation(raw osSandbox, observedAt time.Time) (store.SandboxMeterObservation, bool) {
	if raw.ID == "" || raw.Metadata[metadataWorkspace] == "" || raw.Metadata[metadataRegime] != metadataSandboxRegime {
		return store.SandboxMeterObservation{}, false
	}
	weight, err := strconv.ParseInt(raw.Metadata[metadataComputeWeight], 10, 64)
	if err != nil || weight <= 0 {
		// Sandboxes created before this meter shipped have no trustworthy shape;
		// do not invent a charge. New creates always stamp the weight.
		return store.SandboxMeterObservation{}, false
	}
	return store.SandboxMeterObservation{
		WorkspaceID: raw.Metadata[metadataWorkspace],
		SandboxID:   raw.ID,
		Phase:       canonicalMeterPhase(raw.Status.State),
		Tier:        raw.Metadata[metadataPlan],
		WeightMilli: weight,
		ObservedAt:  observedAt.UTC(),
	}, true
}

// Observe records one request-path phase sample. Metering failure is logged but
// never changes an already-successful OpenSandbox lifecycle operation into an
// API failure (which would invite a duplicate create or hide a successful stop).
func (m *Meter) Observe(ctx context.Context, raw osSandbox) {
	if m == nil || m.Store == nil {
		return
	}
	obs, ok := meterObservation(raw, m.now())
	if !ok {
		return
	}
	if err := m.Store.ObserveSandboxMeter(ctx, obs); err != nil {
		log.Printf("sandbox meter: observe workspace=%s sandbox=%s: %v", obs.WorkspaceID, obs.SandboxID, err)
	}
}

// Run polls every workspace through its tenant key. A successful complete list
// both advances present sandboxes and terminates missing cursors; a failed list
// does neither, preventing an upstream outage from being mistaken for sleep.
func (m *Meter) Run(ctx context.Context) {
	if m == nil || m.Client == nil || m.Store == nil {
		return
	}
	interval := m.Interval
	if interval <= 0 {
		interval = MeterInterval
	}
	m.poll(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.poll(ctx)
		}
	}
}

func (m *Meter) poll(ctx context.Context) {
	keys, err := m.Store.ListSandboxTenantKeys(ctx)
	if err != nil {
		log.Printf("sandbox meter: list tenant keys: %v", err)
		return
	}
	for _, key := range keys {
		raw, err := m.Client.List(ctx, key.APIKey)
		if err != nil {
			log.Printf("sandbox meter: list workspace=%s: %v", key.WorkspaceID, err)
			continue
		}
		at := m.now()
		seen := make([]string, 0, len(raw))
		for _, sb := range raw {
			obs, ok := meterObservation(sb, at)
			if !ok || obs.WorkspaceID != key.WorkspaceID {
				continue
			}
			seen = append(seen, obs.SandboxID)
			if err := m.Store.ObserveSandboxMeter(ctx, obs); err != nil {
				log.Printf("sandbox meter: observe workspace=%s sandbox=%s: %v", obs.WorkspaceID, obs.SandboxID, err)
			}
		}
		if err := m.Store.TerminateMissingSandboxMeters(ctx, key.WorkspaceID, seen, at); err != nil {
			log.Printf("sandbox meter: close missing workspace=%s: %v", key.WorkspaceID, err)
		}
	}
}
