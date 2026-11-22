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

package sandboxbroker

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	defaultNamespaceCacheTTL = 15 * time.Second
	namespaceRefreshMinGap   = time.Second
	maxPodHintEntries        = 4096
	maxConcurrentPodScans    = 8
)

// ErrPodForbidden is returned whenever a source IP cannot be bound to a live,
// non-terminating /v1/sandboxes Pod carrying valid immutable sandbox labels.
var ErrPodForbidden = errors.New("sandbox pod forbidden")

// SandboxPod is a verified /v1/sandboxes Pod resolved strictly from its direct
// TCP source IP.
type SandboxPod struct {
	Namespace string
	Workspace string
	Name      string
	UID       string
	SandboxID string
	GroupID   string
	Labels    map[string]string
}

// PodResolver resolves a TCP source IP to a verified /v1/sandboxes Pod.
type PodResolver interface {
	ResolveSandboxPod(ctx context.Context, sourceIP string) (SandboxPod, error)
}

// AuthorizeSandboxPod validates the immutable Kubernetes labels of a
// /v1/sandboxes Pod:
//   - app.bex.co/regime == sandbox
//   - bex.co/workspace non-empty and namespace == <workspace>-sandbox
//   - bex.co/sandbox-id present (sbx-*)
//   - bex.co/sandbox-group present (sbg-*)
//   - bex.co/agent-session ABSENT (agent-session pods are strictly refused)
func AuthorizeSandboxPod(namespace, podName, podUID string, labels map[string]string) (SandboxPod, error) {
	if namespace == "" || podName == "" || podUID == "" || len(labels) == 0 {
		return SandboxPod{}, ErrPodForbidden
	}
	if labels[appv1alpha1.LabelRegime] != appv1alpha1.RegimeSandbox {
		return SandboxPod{}, ErrPodForbidden
	}
	if labels[agentsession.LabelSession] != "" {
		return SandboxPod{}, ErrPodForbidden
	}
	workspace := labels[agentsession.LabelWorkspace]
	if workspace == "" || namespace != store.SandboxNamespace(workspace) {
		return SandboxPod{}, ErrPodForbidden
	}
	sandboxID := labels[sessionegress.SandboxIDLabel]
	groupID := labels[sessionegress.SandboxGroupLabel]
	if !strings.HasPrefix(sandboxID, "sbx-") || !strings.HasPrefix(groupID, "sbg-") {
		return SandboxPod{}, ErrPodForbidden
	}
	return SandboxPod{
		Namespace: namespace,
		Workspace: workspace,
		Name:      podName,
		UID:       podUID,
		SandboxID: sandboxID,
		GroupID:   groupID,
		Labels:    labels,
	}, nil
}

// KubeSandboxPodResolver resolves a source IP to a live /v1/sandboxes Pod
// using bex-ssh-gateway's existing per-tenant-namespace RBAC
// (`bex-tenant-ssh-gateway` in each `<ws>-sandbox` namespace) — requiring zero
// cluster-wide Pod list privileges.
//
// An in-memory IP -> (namespace, podName) hint cache makes repeat lookups O(1)
// (one direct Pod Get), and every hit re-verifies live PodIP and
// DeletionTimestamp == nil against the apiserver before trusting it.
type KubeSandboxPodResolver struct {
	Client     kubernetes.Interface
	Namespaces func(context.Context) ([]string, error)
	Now        func() time.Time

	nsMu        sync.Mutex
	cachedNS    []string
	nsFetchedAt time.Time

	hintMu sync.Mutex
	hints  map[string]podHint
}

type podHint struct {
	namespace string
	name      string
}

func (r *KubeSandboxPodResolver) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// isLivePodAtIP reports whether p is a live, running, non-terminating,
// non-hostNetwork Pod currently holding sourceIP. Exited Pods (Succeeded/Failed)
// retain Status.PodIP with DeletionTimestamp == nil even after Cilium recycles
// the IP to a different Pod, so Phase == PodRunning is mandatory (ADR123 F2).
func isLivePodAtIP(p *corev1.Pod, sourceIP string) bool {
	return p != nil &&
		p.DeletionTimestamp == nil &&
		p.Status.Phase == corev1.PodRunning &&
		!p.Spec.HostNetwork &&
		p.Status.PodIP == sourceIP
}

// ResolveSandboxPod binds sourceIP to a live, running, non-terminating
// /v1/sandboxes Pod. Fails closed on unknown IP, non-running/terminating Pod,
// hostNetwork Pod, ambiguous IP, transient apiserver errors, or missing
// sandbox labels.
func (r *KubeSandboxPodResolver) ResolveSandboxPod(ctx context.Context, sourceIP string) (SandboxPod, error) {
	if r == nil || r.Client == nil || r.Namespaces == nil || net.ParseIP(sourceIP) == nil {
		return SandboxPod{}, ErrPodForbidden
	}

	// Fast path: verify hint with a direct single-Pod GET so Pod IP reuse,
	// pod exit (Phase != Running), or termination is always caught immediately.
	if hint, ok := r.getHint(sourceIP); ok {
		pod, err := r.Client.CoreV1().Pods(hint.namespace).Get(ctx, hint.name, metav1.GetOptions{})
		if err == nil && isLivePodAtIP(pod, sourceIP) {
			verified, authErr := AuthorizeSandboxPod(pod.Namespace, pod.Name, string(pod.UID), pod.Labels)
			if authErr == nil {
				return verified, nil
			}
			// Pod at that IP is live but not a valid /v1/sandboxes pod (e.g.
			// agent-session pod reused the IP): refuse without falling through.
			r.delHint(sourceIP)
			return SandboxPod{}, ErrPodForbidden
		}
		r.delHint(sourceIP)
	}

	namespaces, err := r.listNamespaces(ctx, false)
	if err != nil {
		return SandboxPod{}, ErrPodForbidden
	}
	found, matched, err := r.scanNamespaces(ctx, namespaces, sourceIP)
	if err != nil {
		return SandboxPod{}, ErrPodForbidden
	}
	if !found {
		// Refresh tenant namespace list once in case a new workspace was just provisioned.
		refreshed, refreshErr := r.listNamespaces(ctx, true)
		if refreshErr != nil || len(refreshed) == 0 {
			return SandboxPod{}, ErrPodForbidden
		}
		found, matched, err = r.scanNamespaces(ctx, refreshed, sourceIP)
		if err != nil || !found {
			return SandboxPod{}, ErrPodForbidden
		}
	}
	r.setHint(sourceIP, podHint{namespace: matched.Namespace, name: matched.Name})
	return matched, nil
}

func (r *KubeSandboxPodResolver) scanNamespaces(ctx context.Context, namespaces []string, sourceIP string) (bool, SandboxPod, error) {
	if len(namespaces) == 0 {
		return false, SandboxPod{}, nil
	}
	selector := fields.OneTermEqualSelector("status.podIP", sourceIP).String()
	sem := make(chan struct{}, maxConcurrentPodScans)
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		matches   []SandboxPod
		ambiguous bool
		denied    bool
		scanErr   bool
	)
	for _, ns := range namespaces {
		if ns == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(namespace string) {
			defer wg.Done()
			defer func() { <-sem }()
			pods, err := r.Client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				FieldSelector: selector,
				Limit:         2,
			})
			if err != nil {
				// Ignore unprovisioned sandbox namespaces; fail closed on
				// transient apiserver errors so a partial scan cannot
				// misattribute a recycled IP.
				if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
					return
				}
				mu.Lock()
				scanErr = true
				mu.Unlock()
				return
			}
			for i := range pods.Items {
				p := &pods.Items[i]
				if !isLivePodAtIP(p, sourceIP) {
					continue
				}
				verified, authErr := AuthorizeSandboxPod(namespace, p.Name, string(p.UID), p.Labels)
				mu.Lock()
				if authErr != nil {
					denied = true
				} else {
					matches = append(matches, verified)
					if len(matches) > 1 {
						ambiguous = true
					}
				}
				mu.Unlock()
			}
		}(ns)
	}
	wg.Wait()
	if denied || ambiguous || scanErr {
		return false, SandboxPod{}, ErrPodForbidden
	}
	if len(matches) != 1 {
		return false, SandboxPod{}, nil
	}
	return true, matches[0], nil
}

func (r *KubeSandboxPodResolver) listNamespaces(ctx context.Context, forceRefresh bool) ([]string, error) {
	now := r.now()
	r.nsMu.Lock()
	if !forceRefresh && len(r.cachedNS) > 0 && now.Sub(r.nsFetchedAt) < defaultNamespaceCacheTTL {
		out := append([]string(nil), r.cachedNS...)
		r.nsMu.Unlock()
		return out, nil
	}
	if forceRefresh && now.Sub(r.nsFetchedAt) < namespaceRefreshMinGap {
		out := append([]string(nil), r.cachedNS...)
		r.nsMu.Unlock()
		return out, nil
	}
	r.nsMu.Unlock()

	ns, err := r.Namespaces(ctx)
	if err != nil {
		return nil, err
	}
	r.nsMu.Lock()
	r.cachedNS = append([]string(nil), ns...)
	r.nsFetchedAt = now
	r.nsMu.Unlock()
	return ns, nil
}

func (r *KubeSandboxPodResolver) getHint(ip string) (podHint, bool) {
	r.hintMu.Lock()
	defer r.hintMu.Unlock()
	h, ok := r.hints[ip]
	return h, ok
}

func (r *KubeSandboxPodResolver) setHint(ip string, h podHint) {
	r.hintMu.Lock()
	defer r.hintMu.Unlock()
	if r.hints == nil {
		r.hints = make(map[string]podHint)
	}
	if len(r.hints) >= maxPodHintEntries {
		clear(r.hints)
	}
	r.hints[ip] = h
}

func (r *KubeSandboxPodResolver) delHint(ip string) {
	r.hintMu.Lock()
	defer r.hintMu.Unlock()
	delete(r.hints, ip)
}
