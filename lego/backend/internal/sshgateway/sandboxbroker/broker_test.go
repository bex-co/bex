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
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/bex-co/bex/lego/backend/internal/agentsession"
	"github.com/bex-co/bex/lego/backend/internal/sandbox"
	"github.com/bex-co/bex/lego/backend/internal/sessionegress"
	"github.com/bex-co/bex/lego/backend/internal/sshgateway"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func validSandboxPodObj(ns, ws, name, uid, ip, sbxID, grpID string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			UID:       types.UID(uid),
			Labels: map[string]string{
				appv1alpha1.LabelRegime:         appv1alpha1.RegimeSandbox,
				agentsession.LabelWorkspace:     ws,
				sessionegress.SandboxIDLabel:    sbxID,
				sessionegress.SandboxGroupLabel: grpID,
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: ip},
	}
}

func TestAuthorizeSandboxPodAndKubeResolver(t *testing.T) {
	now := metav1.Now()
	livePod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-live-0", "uid-live", "10.244.1.10", "sbx-live", "sbg-1")
	terminatingPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-term-0", "uid-term", "10.244.1.11", "sbx-term", "sbg-1")
	terminatingPod.DeletionTimestamp = &now
	agentPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "ags-1-0", "uid-ags", "10.244.1.12", "sbx-agent", "sbg-1")
	agentPod.Labels[agentsession.LabelSession] = "ags-1"
	missingGroupPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-nogrp-0", "uid-ng", "10.244.1.13", "sbx-nogrp", "sbg-1")
	delete(missingGroupPod.Labels, sessionegress.SandboxGroupLabel)
	wrongNSPod := validSandboxPodObj("tea-2-sandbox", "tea-1", "sbx-wrongns-0", "uid-wns", "10.244.1.14", "sbx-wrongns", "sbg-1")

	// Ambiguous IP across two workspaces.
	ambig1 := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-a1-0", "uid-a1", "10.244.1.20", "sbx-a1", "sbg-1")
	ambig2 := validSandboxPodObj("tea-2-sandbox", "tea-2", "sbx-a2-0", "uid-a2", "10.244.1.20", "sbx-a2", "sbg-2")

	client := fake.NewSimpleClientset(livePod, terminatingPod, agentPod, missingGroupPod, wrongNSPod, ambig1, ambig2)
	resolver := &KubeSandboxPodResolver{
		Client: client,
		Namespaces: func(context.Context) ([]string, error) {
			return []string{"tea-1-sandbox", "tea-2-sandbox"}, nil
		},
	}

	ctx := context.Background()
	pod, err := resolver.ResolveSandboxPod(ctx, "10.244.1.10")
	if err != nil {
		t.Fatalf("ResolveSandboxPod live: %v", err)
	}
	if pod.Workspace != "tea-1" || pod.SandboxID != "sbx-live" || pod.GroupID != "sbg-1" || pod.UID != "uid-live" {
		t.Fatalf("unexpected pod: %+v", pod)
	}
	// Repeat lookup exercises hint-cache fast path (verified via Get).
	pod2, err := resolver.ResolveSandboxPod(ctx, "10.244.1.10")
	if err != nil || pod2.UID != "uid-live" {
		t.Fatalf("hint-cached ResolveSandboxPod = (%+v, %v)", pod2, err)
	}

	// If the Pod starts terminating, the cached hint must fail closed immediately.
	livePod.DeletionTimestamp = &now
	if _, err := client.CoreV1().Pods("tea-1-sandbox").Update(ctx, livePod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveSandboxPod(ctx, "10.244.1.10"); !errors.Is(err, ErrPodForbidden) {
		t.Fatalf("terminating cached pod err = %v, want ErrPodForbidden", err)
	}

	for _, tc := range []struct {
		name string
		ip   string
	}{
		{"terminating pod", "10.244.1.11"},
		{"agent-session pod", "10.244.1.12"},
		{"missing sandbox-group label", "10.244.1.13"},
		{"namespace workspace mismatch", "10.244.1.14"},
		{"ambiguous IP across namespaces", "10.244.1.20"},
		{"unknown IP", "10.244.9.99"},
		{"invalid IP", "not-an-ip"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolver.ResolveSandboxPod(ctx, tc.ip); !errors.Is(err, ErrPodForbidden) {
				t.Fatalf("ResolveSandboxPod(%q) err = %v, want ErrPodForbidden", tc.ip, err)
			}
		})
	}
}

type staticResolver map[string][]netip.Addr

func (r staticResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	addrs, ok := r[strings.TrimSuffix(strings.ToLower(host), ".")]
	if !ok {
		return nil, errors.New("no such host")
	}
	return addrs, nil
}

func TestSSRFGuard(t *testing.T) {
	guard := SSRFGuard{
		Resolver: staticResolver{
			"public.example.com":  {netip.MustParseAddr("93.184.216.34")},
			"loopback.example":    {netip.MustParseAddr("127.0.0.1")},
			"metadata.example":    {netip.MustParseAddr("169.254.169.254")},
			"private10.example":   {netip.MustParseAddr("10.0.0.1")},
			"cgnat.example":       {netip.MustParseAddr("100.64.0.5")},
			"ula.example":         {netip.MustParseAddr("fc00::1")},
			"v4mapped.example":    {netip.MustParseAddr("::ffff:169.254.169.254")},
			"nat64.example":       {netip.MustParseAddr("64:ff9b::10.0.0.1")},
			"rebind-mix.example":  {netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("127.0.0.1")},
			"custom-deny.example": {netip.MustParseAddr("203.0.114.5")},
		},
		DenyPrefixes: []netip.Prefix{netip.MustParsePrefix("203.0.114.0/24")},
	}

	ctx := context.Background()
	addr, host, lit, err := guard.ResolveAndCheck(ctx, "PUBLIC.Example.COM", 443)
	if err != nil || lit || host != "public.example.com" || addr != netip.MustParseAddrPort("93.184.216.34:443") {
		t.Fatalf("public resolve = (%v, %q, %v, %v)", addr, host, lit, err)
	}

	blockedHosts := []string{
		"loopback.example",
		"metadata.example",
		"private10.example",
		"cgnat.example",
		"ula.example",
		"v4mapped.example",
		"nat64.example",
		"rebind-mix.example",
		"custom-deny.example",
		"127.0.0.1",
		"169.254.169.254",
		"10.0.0.1",
		"fc00::1",
		"localhost",
		"bex-api.bex-system.svc.cluster.local",
		"bex-api.bex-system.svc",
		"metadata.google.internal",
	}
	for _, h := range blockedHosts {
		if _, _, _, err := guard.ResolveAndCheck(ctx, h, 443); !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("ResolveAndCheck(%q) err = %v, want ErrSSRFBlocked", h, err)
		}
	}
	// ADR123 F1: zoned IPv6 addresses (e.g. ::1%lo, fe80::1%eth0) must NEVER
	// bypass BlockedEgressAddr / SSRFGuard (netip.Prefix.Contains returns false
	// for any zoned address).
	for _, zoned := range []string{"::1%lo", "fe80::1%eth0", "[::1%lo]", "[fe80::1%eth0]"} {
		if _, _, _, err := guard.ResolveAndCheck(ctx, zoned, 443); !errors.Is(err, ErrSSRFBlocked) {
			t.Errorf("ResolveAndCheck(%q) err = %v, want ErrSSRFBlocked", zoned, err)
		}
	}
	for _, zoned := range []string{"::1%lo", "fe80::1%eth0", "2001:4860:4860::8888%eth0"} {
		addr := netip.MustParseAddr(zoned)
		if !sshgateway.BlockedEgressAddr(addr, nil) {
			t.Errorf("BlockedEgressAddr(%q) = false, want true", zoned)
		}
	}
	if _, _, _, err := guard.ResolveAndCheck(ctx, "missing.example.com", 443); !errors.Is(err, ErrDNSLookupFailed) {
		t.Fatalf("missing host err = %v, want ErrDNSLookupFailed", err)
	}
}

// TestKubeSandboxPodResolverRejectsNonRunningHostNetworkAndTransientErrors
// pins ADR123 F2: exited Pods (Succeeded/Failed/Pending) that still hold their
// old Status.PodIP with DeletionTimestamp == nil after Cilium recycles the IP,
// hostNetwork Pods, and partial scans with transient apiserver errors must all
// fail closed with ErrPodForbidden.
func TestKubeSandboxPodResolverRejectsNonRunningHostNetworkAndTransientErrors(t *testing.T) {
	ctx := context.Background()

	runningPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-run-0", "uid-run", "10.244.2.10", "sbx-run", "sbg-1")
	failedPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-fail-0", "uid-fail", "10.244.2.11", "sbx-fail", "sbg-1")
	failedPod.Status.Phase = corev1.PodFailed
	succeededPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-done-0", "uid-done", "10.244.2.12", "sbx-done", "sbg-1")
	succeededPod.Status.Phase = corev1.PodSucceeded
	pendingPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-pend-0", "uid-pend", "10.244.2.13", "sbx-pend", "sbg-1")
	pendingPod.Status.Phase = corev1.PodPending
	hostNetPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-host-0", "uid-host", "10.244.2.14", "sbx-host", "sbg-1")
	hostNetPod.Spec.HostNetwork = true

	client := fake.NewSimpleClientset(runningPod, failedPod, succeededPod, pendingPod, hostNetPod)
	resolver := &KubeSandboxPodResolver{
		Client: client,
		Namespaces: func(context.Context) ([]string, error) {
			return []string{"tea-1-sandbox", "tea-2-sandbox"}, nil
		},
	}

	// Warm the hint cache on the running Pod.
	got, err := resolver.ResolveSandboxPod(ctx, "10.244.2.10")
	if err != nil || got.UID != "uid-run" {
		t.Fatalf("warm resolve = (%+v, %v), want uid-run", got, err)
	}

	// Simulate Pod exiting (Phase=Failed, DeletionTimestamp still nil, PodIP
	// unchanged) while Cilium reassigns 10.244.2.10 to another Pod. Both the
	// cached hint AND fallback scan must refuse the exited Pod.
	runningPod.Status.Phase = corev1.PodFailed
	if _, err := client.CoreV1().Pods("tea-1-sandbox").Update(ctx, runningPod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := resolver.ResolveSandboxPod(ctx, "10.244.2.10"); !errors.Is(err, ErrPodForbidden) {
		t.Fatalf("exited cached pod err = %v, want ErrPodForbidden", err)
	}

	for _, tc := range []struct {
		name string
		ip   string
	}{
		{"failed pod", "10.244.2.11"},
		{"succeeded pod", "10.244.2.12"},
		{"pending pod", "10.244.2.13"},
		{"hostNetwork pod", "10.244.2.14"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := resolver.ResolveSandboxPod(ctx, tc.ip); !errors.Is(err, ErrPodForbidden) {
				t.Fatalf("ResolveSandboxPod(%q) err = %v, want ErrPodForbidden", tc.ip, err)
			}
		})
	}

	// Transient apiserver error on a peer namespace must fail closed so a
	// partial scan cannot misattribute an IP.
	goodPod := validSandboxPodObj("tea-1-sandbox", "tea-1", "sbx-good-0", "uid-good", "10.244.2.99", "sbx-good", "sbg-1")
	if _, err := client.CoreV1().Pods("tea-1-sandbox").Create(ctx, goodPod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	client.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetNamespace() == "tea-2-sandbox" {
			return true, nil, errors.New("apiserver temporary 500")
		}
		return false, nil, nil
	})
	if _, err := resolver.ResolveSandboxPod(ctx, "10.244.2.99"); !errors.Is(err, ErrPodForbidden) {
		t.Fatalf("transient scan error err = %v, want ErrPodForbidden", err)
	}
}

type staticPodResolver map[string]SandboxPod

func (r staticPodResolver) ResolveSandboxPod(_ context.Context, ip string) (SandboxPod, error) {
	p, ok := r[ip]
	if !ok {
		return SandboxPod{}, ErrPodForbidden
	}
	return p, nil
}

type staticLookupClient struct {
	resp sandbox.BrokerLookupResponse
	err  error
}

func (c *staticLookupClient) Lookup(_ context.Context, _ sandbox.BrokerLookupRequest) (sandbox.BrokerLookupResponse, error) {
	return c.resp, c.err
}

func TestBrokerGuardrailsAndScrubbing(t *testing.T) {
	now := time.Now().UTC()
	certPEM, keyPEM := generateTestCAPEM(t, true, now.Add(-time.Hour), now.Add(24*time.Hour))
	ca, err := NewCA(certPEM, keyPEM, CAOptions{})
	if err != nil {
		t.Fatal(err)
	}

	broker := &Broker{
		Pods: staticPodResolver{
			"10.244.1.10": {Namespace: "tea-1-sandbox", Workspace: "tea-1", Name: "sbx-1-0", UID: "u1", SandboxID: "sbx-1", GroupID: "sbg-1"},
		},
		API: &staticLookupClient{
			resp: sandbox.BrokerLookupResponse{
				WorkspaceID:    "tea-1",
				SandboxID:      "sbx-1",
				SandboxGroupID: "sbg-1",
				NetworkPolicy:  sandbox.NetworkPolicy{Default: sandbox.NetworkPolicyAllowAll},
			},
		},
		CA:       ca,
		Resolver: staticResolver{"api.example.com": {netip.MustParseAddr("93.184.216.34")}},
	}

	// /healthz -> 200.
	rec := httptest.NewRecorder()
	broker.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz = %d, want 200", rec.Code)
	}

	// Plain HTTP proxy request -> 405 (never injected or relayed).
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://api.example.com/v1/x", nil)
	req.RemoteAddr = "10.244.1.10:55555"
	broker.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("plain HTTP = %d, want 405", rec.Code)
	}

	// Unknown source IP -> 403 (even if Proxy-Authorization is spoofed).
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodConnect, "https://api.example.com:443", nil)
	req.Host = "api.example.com:443"
	req.RemoteAddr = "10.244.9.9:55555"
	req.Header.Set("Proxy-Authorization", "Basic c3Bvb2Y6c3Bvb2Y=")
	broker.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unknown IP = %d, want 403", rec.Code)
	}

	// Malformed CONNECT target -> 400.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodConnect, "https://api.example.com", nil)
	req.Host = "api.example.com"
	req.RemoteAddr = "10.244.1.10:55555"
	broker.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing port = %d, want 400", rec.Code)
	}

	// Non-443 port refused by default -> 403.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodConnect, "https://api.example.com:8443", nil)
	req.Host = "api.example.com:8443"
	req.RemoteAddr = "10.244.1.10:55555"
	broker.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-443 port = %d, want 403", rec.Code)
	}

	// Concurrency limit -> 429.
	broker.Limits = sshgateway.NewSessionLimiter(1, 1)
	ok, _ := broker.Limits.Acquire("10.244.1.10")
	if !ok {
		t.Fatal("failed to pre-acquire limiter slot")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodConnect, "https://api.example.com:443", nil)
	req.Host = "api.example.com:443"
	req.RemoteAddr = "10.244.1.10:55555"
	broker.Handler().ServeHTTP(rec, req)
	broker.Limits.Release("10.244.1.10")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("concurrency exceeded = %d, want 429", rec.Code)
	}

	// Unconfigured broker -> 503.
	disabled := &Broker{}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodConnect, "https://api.example.com:443", nil)
	req.Host = "api.example.com:443"
	req.RemoteAddr = "10.244.1.10:55555"
	disabled.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled broker = %d, want 503", rec.Code)
	}

	// Error scrubbing must strip URLs containing query-injected secrets.
	urlErr := &url.Error{
		Op:  "Get",
		URL: "https://api.example.com/v1/x?api_key=TOP_SECRET_VALUE_999",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")},
	}
	scrubbed := scrubErrorCause(urlErr)
	if strings.Contains(scrubbed, "TOP_SECRET_VALUE_999") || strings.Contains(scrubbed, "api_key") {
		t.Fatalf("scrubErrorCause leaked secret URL: %q", scrubbed)
	}
}
