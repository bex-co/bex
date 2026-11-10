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
	"strings"
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
	RecordResourceDisplayNames(context.Context, string, []store.ResourceDisplayName) error
	ClaimUnsizedSandboxStorage(ctx context.Context, now time.Time, limit int) ([]store.SandboxStorageObject, error)
	SetSandboxStorageBytes(ctx context.Context, workspaceID, objectID string, startedAt time.Time, bytes int64) error
	AccrueSandboxStorage(ctx context.Context, now time.Time, limit int) (int, error)
	SandboxStorageStats(ctx context.Context, now time.Time) (store.SandboxStorageStats, error)
	ListReapableSandboxStorage(context.Context, int) ([]store.SandboxStorageObject, error)
	MarkSandboxStorageReaped(ctx context.Context, workspaceID, objectID string, startedAt time.Time) error
	PruneSandboxStorage(ctx context.Context, now time.Time, limit int) (int64, error)
}

// Meter observes OpenSandbox's authoritative lifecycle state and advances the
// per-sandbox usage cursor. It is shared by request-path lifecycle observations
// and the background tenant-scoped poller.
type Meter struct {
	Client   *Client
	Store    MeterStore
	Interval time.Duration
	Now      func() time.Time
	// Sizer reads a retained image's stored size for the storage meter. Nil
	// turns sizing off: suspended sandboxes are recorded but never sized, so
	// nothing is billed for storage until it is configured (the feature is off,
	// not wrong); StorageMetrics' unsized gauge says so.
	Sizer SnapshotSizer
	// Reaper deletes the images of sandboxes that are gone (terminated, or
	// expired at pause + 30 days) from the registry, which keeps a repository's
	// five newest tags forever (ADR104 D11). Nil leaves them: the meter has
	// stopped charging for them either way, and the reap_pending gauge says how
	// many wait.
	Reaper ImageReaper
	// StorageMetrics, when set, reports the storage meter's health.
	StorageMetrics *StorageMetrics
}

const (
	// Each poll round bounds its registry reads, deletes, accrual transactions
	// and prunes; the rest wait for the next round.
	storageSizeBatch   = 20
	storageAccrueBatch = 500
	storageReapBatch   = 20
	storagePruneBatch  = 200

	// storageRegistryBudget bounds the time one round may spend on registry
	// calls, and storageRegistryFailures how many consecutive failures end it
	// early: a hung registry must not hold up the next poll of every workspace.
	storageRegistryBudget   = 20 * time.Second
	storageRegistryFailures = 3
)

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

// requestFor derives a conservative resource REQUEST (a quarter of the limit,
// with floors) so sandbox pods overcommit the node. Sandbox compute is bursty
// and mostly idle — a session spends most of its life waiting on the model — so
// scheduling on a full-size request (the agent template's 2 CPU / 4Gi) wastes
// the node: only ~2 pods fit even though actual use is a fraction of that
// ("Insufficient cpu/memory" scheduling failures at ~7% real load). Requesting a
// quarter lets several times as many pods schedule while each limit still caps a
// burst; CPU overcommit merely throttles, and memory stays conservative (a
// quarter, not a sliver) to bound OOM risk when several bursts coincide. An
// unparseable value falls through to the limit (previous behaviour).
func requestFor(cpu, memory string) (reqCPU, reqMem string) {
	reqCPU, reqMem = cpu, memory
	if q, err := resource.ParseQuantity(cpu); err == nil && q.MilliValue() > 0 {
		milli := q.MilliValue() / 4
		if milli < 50 {
			milli = 50
		}
		reqCPU = resource.NewMilliQuantity(milli, resource.DecimalSI).String()
	}
	if q, err := resource.ParseQuantity(memory); err == nil && q.Value() > 0 {
		bytes := q.Value() / 4
		if bytes < 128*1024*1024 {
			bytes = 128 * 1024 * 1024
		}
		reqMem = resource.NewQuantity(bytes, resource.BinarySI).String()
	}
	return reqCPU, reqMem
}

// memoryMiBOf is an allocated memory quantity ("2Gi", "512Mi") in whole MiB,
// rounded up; 0 for an unparseable or non-positive one, which the memory meter
// skips instead of guessing.
func memoryMiBOf(memory string) int64 {
	q, err := resource.ParseQuantity(memory)
	if err != nil || q.Value() <= 0 {
		return 0
	}
	const mib = int64(1) << 20
	return (q.Value() + mib - 1) / mib
}

// sandboxMemoryMiB is the memory the sandbox was created with: the stamped
// value, else the size of its plan for a sandbox created before the stamp. A
// sandbox with neither (a template-sized sandbox that predates the stamp) is 0,
// not guessed.
func sandboxMemoryMiB(raw osSandbox) int64 {
	if n, err := strconv.ParseInt(raw.Metadata[metadataMemoryMiB], 10, 64); err == nil && n > 0 {
		return n
	}
	if size, ok := planResources[Plan(raw.Metadata[metadataPlan])]; ok {
		return memoryMiBOf(size.Memory)
	}
	return 0
}

func canonicalMeterPhase(state string) string {
	// Unknown states already map to errored.
	return string(mapOpenSandboxStatus(state))
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
		// The canonical (tenant-facing) id, so a usage row names the sandbox the
		// same way the API does. Agent-session sandboxes are unstamped, so their
		// rows keep the substrate id the `agent_sessions` display-name join uses.
		SandboxID:   canonicalID(raw),
		Phase:       canonicalMeterPhase(raw.Status.State),
		Tier:        raw.Metadata[metadataPlan],
		WeightMilli: weight,
		MemoryMiB:   sandboxMemoryMiB(raw),
		StorageRef:  storageRef(raw),
		ObservedAt:  observedAt.UTC(),
	}, true
}

// storageRef is where the registry holds a sandbox's suspend image, or "" for a
// sandbox the storage meter leaves out: an agent-session sandbox's cost is its
// session's (ADR047 D6), not a Render sandbox's storage line.
func storageRef(raw osSandbox) string {
	if !idleTracked(raw) {
		return ""
	}
	return raw.ID
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

// Label retains a `plan · image` display name for a sandbox at create (w1/112).
// Charges outlive their sandboxes, and a sandbox outside any agent session had
// no other label, so fourteen charge rows all read `starter sandbox (deleted)`
// and a $78.61 one could not be told from $0.00 ones. The name goes into the
// same retained-name table usage already reads, so it survives reaping. Like
// Observe, a failure is logged and never fails the create.
func (m *Meter) Label(ctx context.Context, raw osSandbox) {
	if m == nil || m.Store == nil {
		return
	}
	obs, ok := meterObservation(raw, m.now())
	label := sandboxChargeLabel(obs.Tier, raw.Image.URI)
	if !ok || label == "" {
		return
	}
	record := store.ResourceDisplayName{Kind: store.ResourceKindSandbox, ID: obs.SandboxID, Name: label}
	if err := m.Store.RecordResourceDisplayNames(ctx, obs.WorkspaceID, []store.ResourceDisplayName{record}); err != nil {
		log.Printf("sandbox meter: label workspace=%s sandbox=%s: %v", obs.WorkspaceID, obs.SandboxID, err)
	}
}

// sandboxChargeLabel is `plan · image` with the image shortened to what a person
// reads: no digest, and no implicit Docker Hub prefix (`alpine:3`, not
// `docker.io/library/alpine:3@sha256:…`). The tag stays, so two sandboxes on
// different images never share a label.
func sandboxChargeLabel(plan, image string) string {
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	image = strings.TrimPrefix(strings.TrimPrefix(image, "docker.io/"), "library/")
	switch {
	case image == "":
		return ""
	case plan == "":
		return image
	}
	return plan + " · " + image
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
			obs.Polled = true // the authoritative list, not a request-path sample
			if err := m.Store.ObserveSandboxMeter(ctx, obs); err != nil {
				log.Printf("sandbox meter: observe workspace=%s sandbox=%s: %v", obs.WorkspaceID, obs.SandboxID, err)
			}
		}
		if err := m.Store.TerminateMissingSandboxMeters(ctx, key.WorkspaceID, seen, at); err != nil {
			log.Printf("sandbox meter: close missing workspace=%s: %v", key.WorkspaceID, err)
		}
	}
	m.settleStorage(ctx)
}

// settleStorage sizes the images the polls found retained, adds the seconds
// every sized object has been retained, and reports the meter's health. It runs
// after every poll round, so a registry outage delays a charge by the outage
// and loses nothing: the object keeps its start, and accrual covers it from
// there once it is sized.
func (m *Meter) settleStorage(ctx context.Context) {
	if m.Sizer != nil {
		m.sizeImages(ctx)
	}
	m.reapImages(ctx)
	if _, err := m.Store.AccrueSandboxStorage(ctx, m.now(), storageAccrueBatch); err != nil {
		log.Printf("sandbox meter: accrue storage: %v", err)
	}
	if _, err := m.Store.PruneSandboxStorage(ctx, m.now(), storagePruneBatch); err != nil {
		log.Printf("sandbox meter: prune storage: %v", err)
	}
	if stats, err := m.Store.SandboxStorageStats(ctx, m.now()); err != nil {
		log.Printf("sandbox meter: storage stats: %v", err)
	} else {
		m.StorageMetrics.set(stats)
	}
}

// sizeImages reads the size of the retained images the polls found. One failure
// is retried on a later round, and the rotation in the claim keeps it from
// holding the queue.
func (m *Meter) sizeImages(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, storageRegistryBudget)
	defer cancel()
	unsized, err := m.Store.ClaimUnsizedSandboxStorage(ctx, m.now(), storageSizeBatch)
	if err != nil {
		log.Printf("sandbox meter: claim unsized storage: %v", err)
		return
	}
	failed, consecutive := 0, 0
	var lastErr error
	for _, o := range unsized {
		if o.Ref == "" {
			continue
		}
		bytes, err := m.Sizer.SnapshotBytes(ctx, o.WorkspaceID, o.Ref)
		if err == nil {
			err = m.Store.SetSandboxStorageBytes(ctx, o.WorkspaceID, o.ObjectID, o.StartedAt, bytes)
		}
		if err == nil {
			consecutive = 0
			continue
		}
		failed++
		consecutive++
		lastErr = err
		m.StorageMetrics.sizeFailed()
		if consecutive >= storageRegistryFailures {
			break
		}
	}
	if failed > 0 {
		log.Printf("sandbox meter: %d of %d retained images not sized, retried on a later poll (last: %v)", failed, len(unsized), lastErr)
	}
}

// reapImages deletes the registry images of sandboxes that are gone. One failure
// does not stop the others, and the object stays queued for the next poll.
func (m *Meter) reapImages(ctx context.Context) {
	if m.Reaper == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, storageRegistryBudget)
	defer cancel()
	gone, err := m.Store.ListReapableSandboxStorage(ctx, storageReapBatch)
	if err != nil {
		log.Printf("sandbox meter: list images to reap: %v", err)
		return
	}
	consecutive := 0
	for _, o := range gone {
		if err := m.Reaper.DeleteImages(ctx, o.WorkspaceID, o.Ref); err != nil {
			m.StorageMetrics.reapFailed()
			log.Printf("sandbox meter: reap image workspace=%s sandbox=%s: %v", o.WorkspaceID, o.ObjectID, err)
			if consecutive++; consecutive >= storageRegistryFailures {
				return
			}
			continue
		}
		consecutive = 0
		if err := m.Store.MarkSandboxStorageReaped(ctx, o.WorkspaceID, o.ObjectID, o.StartedAt); err != nil {
			log.Printf("sandbox meter: mark reaped workspace=%s sandbox=%s: %v", o.WorkspaceID, o.ObjectID, err)
		}
	}
}
