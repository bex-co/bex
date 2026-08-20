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

// Package egressmeter owns bex's direct App egress byte counter. A host-network
// netfilter BPF program observes POST_ROUTING after CNI pod policy and before
// source NAT, where allowed packets still carry their original Pod IP.
package egressmeter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/rlimit"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	PodLabelApp   = "app.bex.co/app"
	PodLabelAppID = "bex.co/app-id"

	// Run after the raw table but before conntrack, mangle, and source NAT.
	// Pod policy has already run at Cilium's host-facing veth ingress hook.
	nfMeterPriority = -299
)

type Config struct {
	NodeName        string
	PinPath         string
	CheckpointPath  string
	SourceIDPath    string
	LossStatePath   string
	ExcludedCIDRs   []string
	ReconcileEvery  time.Duration
	CheckpointEvery time.Duration
}

func (c *Config) defaults() {
	if c.PinPath == "" {
		c.PinPath = "/sys/fs/bpf/bex-egress-meter"
	}
	if c.CheckpointPath == "" {
		c.CheckpointPath = "/var/lib/bex-egress-meter/counters.json"
	}
	if c.SourceIDPath == "" {
		c.SourceIDPath = "/var/lib/bex-egress-meter/source-id"
	}
	if c.LossStatePath == "" {
		c.LossStatePath = "/var/lib/bex-egress-meter/counter-loss.json"
	}
	if c.ReconcileEvery <= 0 {
		c.ReconcileEvery = 5 * time.Second
	}
	if c.CheckpointEvery <= 0 {
		c.CheckpointEvery = 15 * time.Second
	}
}

type Meter struct {
	config Config
	client kubernetes.Interface

	mu       sync.RWMutex
	objects  *bpfObjects
	links    []link.Link
	loadErr  error
	lossMu   sync.Mutex
	stateErr error

	expectedPods    atomic.Int64
	attributedIPs   atomic.Int64
	healthy         atomic.Int64
	reconcileErrors atomic.Uint64
	lossEvents      atomic.Uint64
	lastLossUnix    atomic.Int64
	counterLoss     atomic.Bool
}

func New(config Config, client kubernetes.Interface) *Meter {
	config.defaults()
	meter := &Meter{config: config, client: client}
	state, err := readCounterLossState(config.LossStatePath)
	meter.stateErr = err
	meter.lossEvents.Store(state.Events)
	meter.lastLossUnix.Store(state.LastUnix)
	return meter
}

func (m *Meter) Load() error {
	if m.client == nil {
		return errors.New("kubernetes client is required")
	}
	if m.config.NodeName == "" {
		return errors.New("node name is required")
	}
	if m.stateErr != nil {
		return fmt.Errorf("read persistent counter-loss state: %w", m.stateErr)
	}
	if err := rlimit.RemoveMemlock(); err != nil {
		return fmt.Errorf("remove memlock limit: %w", err)
	}
	if err := os.MkdirAll(m.config.PinPath, 0o750); err != nil {
		return fmt.Errorf("create BPF pin path: %w", err)
	}
	objects := &bpfObjects{}
	options := &ebpf.CollectionOptions{Maps: ebpf.MapOptions{PinPath: m.config.PinPath}}
	if err := loadBpfObjects(objects, options); err != nil {
		return fmt.Errorf("load eBPF collection: %w", err)
	}
	m.mu.Lock()
	m.objects = objects
	m.mu.Unlock()
	cleanup := func(err error) error {
		m.mu.Lock()
		m.objects = nil
		m.mu.Unlock()
		_ = objects.Close()
		return err
	}
	if err := m.syncExcludedPrefixes(context.Background()); err != nil {
		return cleanup(err)
	}
	if err := m.restoreCheckpoint(); err != nil {
		return cleanup(err)
	}
	if err := m.reconcilePods(context.Background()); err != nil {
		return cleanup(err)
	}
	links, err := attachNetfilter(objects, m.config.PinPath)
	if err != nil {
		return cleanup(err)
	}
	m.mu.Lock()
	m.links = links
	m.loadErr = nil
	m.mu.Unlock()
	m.healthy.Store(1)
	return nil
}

func attachNetfilter(objects *bpfObjects, pinPath string) ([]link.Link, error) {
	type hook struct {
		family link.NetfilterProtocolFamily
		path   string
	}
	hooks := []hook{
		{family: link.NetfilterProtoIPv4, path: filepath.Join(pinPath, "postrouting-v1-ipv4")},
		{family: link.NetfilterProtoIPv6, path: filepath.Join(pinPath, "postrouting-v1-ipv6")},
	}
	var restored []link.Link
	replace := false
	for _, hook := range hooks {
		current, err := link.LoadPinnedLink(hook.path, nil)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			closeLinks(restored)
			return nil, fmt.Errorf("load pinned netfilter link %s: %w", hook.path, err)
		}
		info, err := current.Info()
		mapsMatch := false
		if err == nil {
			mapsMatch, err = programUsesMaps(info.Program,
				objects.PodV4Resources, objects.PodV6Resources, objects.ResourceBytes,
				objects.ExcludedV4, objects.ExcludedV6, objects.Diagnostics)
		}
		if err != nil || !mapsMatch || info.Netfilter() == nil || info.Netfilter().ProtocolFamily != hook.family || info.Netfilter().Hook != link.NetfilterInetPostRouting || info.Netfilter().Priority != nfMeterPriority {
			_ = current.Close()
			closeLinks(restored)
			restored = nil
			replace = true
			break
		}
		restored = append(restored, current)
	}
	if !replace && len(restored) == len(hooks) {
		return restored, nil
	}
	// A partial pair cannot account for both address families. Remove any old
	// half before creating a fresh pair so packets can never be double counted.
	closeLinks(restored)
	for _, hook := range hooks {
		existing, err := link.LoadPinnedLink(hook.path, nil)
		if err == nil {
			_ = existing.Unpin()
			_ = existing.Close()
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove stale netfilter link %s: %w", hook.path, err)
		}
	}

	var attached []link.Link
	for _, hook := range hooks {
		current, err := link.AttachNetfilter(link.NetfilterOptions{
			Program: objects.BexCountPublicEgress, ProtocolFamily: hook.family,
			Hook: link.NetfilterInetPostRouting, Priority: nfMeterPriority,
		})
		if err != nil {
			cleanupLinks(attached)
			return nil, fmt.Errorf("attach netfilter POST_ROUTING family %d: %w", hook.family, err)
		}
		if err := current.Pin(hook.path); err != nil {
			_ = current.Close()
			cleanupLinks(attached)
			return nil, fmt.Errorf("pin netfilter POST_ROUTING family %d: %w", hook.family, err)
		}
		attached = append(attached, current)
	}
	return attached, nil
}

func programUsesMaps(programID ebpf.ProgramID, maps ...*ebpf.Map) (bool, error) {
	program, err := ebpf.NewProgramFromID(programID)
	if err != nil {
		return false, err
	}
	defer func() { _ = program.Close() }()
	info, err := program.Info()
	if err != nil {
		return false, err
	}
	programMapIDs, ok := info.MapIDs()
	if !ok {
		return false, errors.New("kernel did not expose program map ids")
	}
	used := make(map[ebpf.MapID]bool, len(programMapIDs))
	for _, id := range programMapIDs {
		used[id] = true
	}
	for _, current := range maps {
		mapInfo, err := current.Info()
		if err != nil {
			return false, err
		}
		id, ok := mapInfo.ID()
		if !ok || !used[id] {
			return false, nil
		}
	}
	return true, nil
}

// closeLinks releases links without unpinning them — the restore path must
// leave the bpffs pins intact for the next attach attempt, unlike cleanupLinks.
func closeLinks(links []link.Link) {
	for _, existing := range links {
		_ = existing.Close()
	}
}

func cleanupLinks(links []link.Link) {
	for _, existing := range links {
		_ = existing.Unpin()
		_ = existing.Close()
	}
}

// Run keeps Pod-IP attribution current and checkpoints monotonically
// increasing totals onto node disk. Load failure remains visible as health=0
// instead of terminating the metrics endpoint and looking like a missing job.
func (m *Meter) Run(ctx context.Context) {
	if err := m.Load(); err != nil {
		m.mu.Lock()
		m.loadErr = err
		m.mu.Unlock()
		log.Printf("egress-meter: unsupported/unavailable: %v", err)
	}
	reconcileTicker := time.NewTicker(m.config.ReconcileEvery)
	checkpointTicker := time.NewTicker(m.config.CheckpointEvery)
	defer reconcileTicker.Stop()
	defer checkpointTicker.Stop()
	defer m.Close()
	for {
		select {
		case <-ctx.Done():
			_ = m.checkpoint()
			return
		case <-reconcileTicker.C:
			m.reconcile(ctx)
		case <-checkpointTicker.C:
			if err := m.checkpoint(); err != nil {
				m.fail("checkpoint", err)
			}
		}
	}
}

func (m *Meter) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, attached := range m.links {
		_ = attached.Close()
	}
	m.links = nil
	if m.objects != nil {
		_ = m.objects.Close()
		m.objects = nil
	}
}

func (m *Meter) Ready() bool { return m.healthy.Load() == 1 }

func (m *Meter) reconcile(ctx context.Context) {
	m.mu.RLock()
	available := m.objects != nil && len(m.links) == 2
	m.mu.RUnlock()
	if !available {
		if err := m.Load(); err != nil {
			m.mu.Lock()
			m.loadErr = err
			m.mu.Unlock()
			m.fail("reload accounting hooks", err)
			return
		}
		return
	}
	if m.counterLoss.Load() {
		m.healthy.Store(0)
		return
	}
	if err := m.syncExcludedPrefixes(ctx); err != nil {
		m.fail("sync excluded destinations", err)
		return
	}
	if err := m.reconcilePods(ctx); err != nil {
		m.fail("reconcile Pod attribution", err)
		return
	}
	m.healthy.Store(1)
}

func (m *Meter) reconcilePods(ctx context.Context) error {
	// App Pods live in canonical per-workspace namespaces. Keep this list
	// cluster-wide and constrain it by node plus the App label; a namespaced list
	// would silently omit every post-migration workspace.
	pods, err := m.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", m.config.NodeName).String(),
		LabelSelector: PodLabelApp,
	})
	if err != nil {
		return fmt.Errorf("list node-local App pods: %w", err)
	}
	v4, v6, expected, err := desiredPodResources(pods.Items)
	if err != nil {
		return err
	}
	m.expectedPods.Store(int64(expected))

	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.objects == nil {
		return errors.New("BPF maps unavailable")
	}
	for _, key := range v4 {
		if err := ensureCounter(m.objects.ResourceBytes, key); err != nil {
			return err
		}
	}
	for _, key := range v6 {
		if err := ensureCounter(m.objects.ResourceBytes, key); err != nil {
			return err
		}
	}
	if err := syncResourceMap4(m.objects.PodV4Resources, v4); err != nil {
		return err
	}
	if err := syncResourceMap16(m.objects.PodV6Resources, v6); err != nil {
		return err
	}
	m.attributedIPs.Store(int64(len(v4) + len(v6)))
	return nil
}

// desiredPodResources resolves the short-lived packet identity (Pod IP) through
// the immutable Pod UID before producing the low-cardinality App map. If a
// stale list ever presents one IP for two UIDs, accounting stops instead of
// depending on list order and cross-attributing bytes during churn.
func desiredPodResources(pods []corev1.Pod) (map[[4]byte]ResourceKey, map[[16]byte]ResourceKey, int, error) {
	v4 := map[[4]byte]ResourceKey{}
	v6 := map[[16]byte]ResourceKey{}
	v4Owners := map[[4]byte]types.UID{}
	v6Owners := map[[16]byte]types.UID{}
	expected := 0
	for i := range pods {
		pod := &pods[i]
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		if pod.UID == "" {
			return nil, nil, 0, fmt.Errorf("pod %s has no immutable UID", pod.Name)
		}
		expected++
		key, err := NewResourceKey(pod.Namespace, pod.Labels[PodLabelAppID])
		if err != nil {
			return nil, nil, 0, fmt.Errorf("pod %s: %w", pod.Name, err)
		}
		for _, podIP := range pod.Status.PodIPs {
			ip, err := netip.ParseAddr(podIP.IP)
			if err != nil {
				return nil, nil, 0, fmt.Errorf("pod %s IP %q: %w", pod.Name, podIP.IP, err)
			}
			if ip.Is4() {
				address := ip.As4()
				if owner, exists := v4Owners[address]; exists && owner != pod.UID {
					return nil, nil, 0, fmt.Errorf("pod IP %s belongs to both UID %s and %s", ip, owner, pod.UID)
				}
				v4Owners[address] = pod.UID
				v4[address] = key
			} else {
				address := ip.As16()
				if owner, exists := v6Owners[address]; exists && owner != pod.UID {
					return nil, nil, 0, fmt.Errorf("pod IP %s belongs to both UID %s and %s", ip, owner, pod.UID)
				}
				v6Owners[address] = pod.UID
				v6[address] = key
			}
		}
	}
	return v4, v6, expected, nil
}

func syncResourceMap4(m *ebpf.Map, desired map[[4]byte]ResourceKey) error {
	for ip, resource := range desired {
		if err := m.Update(ip, resource, ebpf.UpdateAny); err != nil {
			return err
		}
	}
	var ip [4]byte
	var resource ResourceKey
	var obsolete [][4]byte
	it := m.Iterate()
	for it.Next(&ip, &resource) {
		if _, ok := desired[ip]; !ok {
			obsolete = append(obsolete, ip)
		}
	}
	if err := it.Err(); err != nil {
		return err
	}
	for _, old := range obsolete {
		if err := m.Delete(old); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
			return err
		}
	}
	return nil
}

func syncResourceMap16(m *ebpf.Map, desired map[[16]byte]ResourceKey) error {
	for ip, resource := range desired {
		if err := m.Update(ip, resource, ebpf.UpdateAny); err != nil {
			return err
		}
	}
	var ip [16]byte
	var resource ResourceKey
	var obsolete [][16]byte
	it := m.Iterate()
	for it.Next(&ip, &resource) {
		if _, ok := desired[ip]; !ok {
			obsolete = append(obsolete, ip)
		}
	}
	if err := it.Err(); err != nil {
		return err
	}
	for _, old := range obsolete {
		if err := m.Delete(old); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
			return err
		}
	}
	return nil
}

func ensureCounter(counters *ebpf.Map, key ResourceKey) error {
	var value uint64
	err := counters.Lookup(key, &value)
	if err == nil {
		return nil
	}
	if !errors.Is(err, ebpf.ErrKeyNotExist) {
		return err
	}
	return counters.Update(key, uint64(0), ebpf.UpdateNoExist)
}

func (m *Meter) fail(action string, err error) {
	m.reconcileErrors.Add(1)
	m.healthy.Store(0)
	log.Printf("egress-meter: %s: %v", action, err)
}

func (m *Meter) syncExcludedPrefixes(ctx context.Context) error {
	prefixes, err := parsePrefixes(m.config.ExcludedCIDRs)
	if err != nil {
		return err
	}
	nodes, err := m.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("list node addresses: %w", err)
	}
	for _, node := range nodes.Items {
		for _, raw := range node.Spec.PodCIDRs {
			prefix, err := netip.ParsePrefix(raw)
			if err != nil {
				return fmt.Errorf("parse node %s pod CIDR %q: %w", node.Name, raw, err)
			}
			prefixes = append(prefixes, prefix.Masked())
		}
		for _, address := range node.Status.Addresses {
			if address.Type != corev1.NodeInternalIP && address.Type != corev1.NodeExternalIP {
				continue
			}
			ip, err := netip.ParseAddr(address.Address)
			if err == nil {
				prefixes = append(prefixes, netip.PrefixFrom(ip, ip.BitLen()))
			}
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.objects == nil {
		return errors.New("BPF maps unavailable")
	}
	v4 := map[lpmV4Key]struct{}{}
	v6 := map[lpmV6Key]struct{}{}
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() {
			v4[lpmV4Key{PrefixLen: uint32(prefix.Bits()), Addr: prefix.Addr().As4()}] = struct{}{}
		} else {
			v6[lpmV6Key{PrefixLen: uint32(prefix.Bits()), Addr: prefix.Addr().As16()}] = struct{}{}
		}
	}
	if err := syncLPMV4(m.objects.ExcludedV4, v4); err != nil {
		return err
	}
	return syncLPMV6(m.objects.ExcludedV6, v6)
}

func syncLPMV4(m *ebpf.Map, desired map[lpmV4Key]struct{}) error {
	one := uint8(1)
	for key := range desired {
		if err := m.Update(key, one, ebpf.UpdateAny); err != nil {
			return err
		}
	}
	var key lpmV4Key
	var value uint8
	var obsolete []lpmV4Key
	it := m.Iterate()
	for it.Next(&key, &value) {
		if _, keep := desired[key]; !keep {
			obsolete = append(obsolete, key)
		}
	}
	if err := it.Err(); err != nil {
		return err
	}
	for _, old := range obsolete {
		if err := m.Delete(old); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
			return err
		}
	}
	return nil
}

func syncLPMV6(m *ebpf.Map, desired map[lpmV6Key]struct{}) error {
	one := uint8(1)
	for key := range desired {
		if err := m.Update(key, one, ebpf.UpdateAny); err != nil {
			return err
		}
	}
	var key lpmV6Key
	var value uint8
	var obsolete []lpmV6Key
	it := m.Iterate()
	for it.Next(&key, &value) {
		if _, keep := desired[key]; !keep {
			obsolete = append(obsolete, key)
		}
	}
	if err := it.Err(); err != nil {
		return err
	}
	for _, old := range obsolete {
		if err := m.Delete(old); err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
			return err
		}
	}
	return nil
}

func (m *Meter) counterRows() ([]checkpointRow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.objects == nil {
		return nil, errors.New("BPF maps unavailable")
	}
	var key ResourceKey
	var bytes uint64
	var rows []checkpointRow
	iterator := m.objects.ResourceBytes.Iterate()
	for iterator.Next(&key, &bytes) {
		namespace, appID := key.Labels()
		rows = append(rows, checkpointRow{Namespace: namespace, AppID: appID, Bytes: bytes})
	}
	if err := iterator.Err(); err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace != rows[j].Namespace {
			return rows[i].Namespace < rows[j].Namespace
		}
		return rows[i].AppID < rows[j].AppID
	})
	return rows, nil
}

func (m *Meter) checkpoint() error {
	rows, err := m.counterRows()
	if err != nil {
		return err
	}
	previous, err := readCheckpoint(m.config.CheckpointPath)
	if err != nil {
		return err
	}
	if resource, oldBytes, newBytes, decreased := counterDecrease(previous, rows); decreased {
		if err := m.recordCounterLoss(); err != nil {
			m.healthy.Store(0)
			return fmt.Errorf("persist counter-loss event: %w", err)
		}
		m.counterLoss.Store(true)
		m.healthy.Store(0)
		return fmt.Errorf("resource counter %s decreased from %d to %d", resource, oldBytes, newBytes)
	}
	return writeCheckpoint(m.config.CheckpointPath, rows)
}

func counterDecrease(previous, current []checkpointRow) (string, uint64, uint64, bool) {
	now := make(map[string]uint64, len(current))
	for _, row := range current {
		now[row.Namespace+"/"+row.AppID] = row.Bytes
	}
	for _, row := range previous {
		key := row.Namespace + "/" + row.AppID
		if currentBytes, exists := now[key]; !exists || currentBytes < row.Bytes {
			return key, row.Bytes, currentBytes, true
		}
	}
	return "", 0, 0, false
}

func (m *Meter) restoreCheckpoint() error {
	rows, err := readCheckpoint(m.config.CheckpointPath)
	if err != nil {
		return err
	}
	type pendingRestore struct {
		key   ResourceKey
		bytes uint64
	}
	pending := make([]pendingRestore, 0, len(rows))
	for _, row := range rows {
		key, err := NewResourceKey(row.Namespace, row.AppID)
		if err != nil {
			return err
		}
		var current uint64
		err = m.objects.ResourceBytes.Lookup(key, &current)
		if errors.Is(err, ebpf.ErrKeyNotExist) || (err == nil && current < row.Bytes) {
			pending = append(pending, pendingRestore{key: key, bytes: row.Bytes})
		} else if err != nil {
			return err
		}
	}
	if len(pending) > 0 {
		if err := m.recordCounterLoss(); err != nil {
			return fmt.Errorf("persist checkpoint-restore loss event: %w", err)
		}
	}
	for _, restore := range pending {
		if err := m.objects.ResourceBytes.Update(restore.key, restore.bytes, ebpf.UpdateAny); err != nil {
			return err
		}
	}
	return nil
}
