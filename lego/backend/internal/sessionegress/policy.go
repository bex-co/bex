/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package sessionegress renders the fail-closed Cilium policy for one managed
// coding-agent session. The package deliberately owns policy mechanics only;
// the agent-session feature owns lifecycle, authorization, and persistence.
package sessionegress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const (
	SessionLabel = "bex.co/agent-session"

	managedByLabel  = "app.kubernetes.io/managed-by"
	managedByValue  = "bex-session-egress"
	phaseAnnotation = "bex.co/egress-phase"
	allowAnnotation = "bex.co/egress-allowlist"
	hashAnnotation  = "bex.co/egress-allowlist-hash"
	modelAnnotation = "bex.co/model-endpoint"

	credentialGatewayHost = "bex-ssh-gateway.bex-system.svc.cluster.local"
	credentialGatewayPort = uint16(8082)

	maxExtraDestinations = 32
)

var (
	ciliumPolicyGVK = schema.GroupVersionKind{Group: "cilium.io", Version: "v2", Kind: "CiliumNetworkPolicy"}
	// Registered model-provider API hosts are never tenant widening destinations:
	// model traffic must traverse the credential proxy even when the selected
	// profile uses a different provider.
	modelProviderDomains = []string{"api.anthropic.com", "api.openai.com", "generativelanguage.googleapis.com"}
)

// Phase is the network posture of an agent session. A phase transition is
// intentionally one-way: setup can become agent, but agent can never reopen
// package-registry access.
type Phase string

const (
	PhaseSetup Phase = "setup"
	PhaseAgent Phase = "agent"
)

// Config owns the platform-curated setup-only package registries. An empty list
// selects the built-in catalog; production can replace it through
// BEX_AGENT_SETUP_REGISTRIES. The per-session model endpoint is deliberately
// not process config: it comes from that session's selected agent/provider.
type Config struct {
	SetupRegistryDomains []string
	// ModelProxyPort, when non-zero (BEX_AGENT_MODEL_PROXY_URL wired), turns on the
	// ADR062 narrowing: the tenant pod's egress no longer admits the vendor model
	// host directly (the agent reaches it only through the gateway proxy), and a
	// gateway rule for this port is added instead. 0 retains the legacy policy
	// renderer for tests/migrations, but the agent-session service refuses new
	// mutation/rehydration work when the proxy is absent (ADR064).
	ModelProxyPort uint16
	// SnapshotStoreDomains are the exact public DNS hosts of the ADR059
	// hibernation object store (BEX_AGENT_SNAPSHOT_S3_ENDPOINT). The Completer's
	// snapshot/rehydrate scripts curl a time-boxed presigned URL from inside the
	// sandbox; without these names Cilium NXDOMAIN/timeouts the PUT (curl exit 6)
	// and reclaim falls back to Terminate. Empty when the Hibernated tier is off.
	// They are platform baseline (both phases), never tenant widening, and are
	// omitted from the allowlist identity hash so enabling the store does not
	// look like an extra-destination mutation.
	SnapshotStoreDomains []string
}

// Manager converges namespaced, per-session Cilium policies. A nil Client is
// not a permissive mode: every mutation fails closed.
type Manager struct {
	Client client.Client
	Config Config
}

// NewManager validates platform configuration at process startup rather than
// waiting for the first tenant session to discover a fail-closed refusal.
func NewManager(cl client.Client, config Config) (*Manager, error) {
	normalized, err := config.normalized()
	if err != nil {
		return nil, err
	}
	if cl == nil {
		return nil, errors.New("session egress: Kubernetes policy client is not configured")
	}
	return &Manager{Client: cl, Config: normalized}, nil
}

// RegistryConfig parses the comma-separated BEX_AGENT_SETUP_REGISTRIES value.
// Whitespace and empty members remain visible for startup validation instead of
// being silently discarded. Empty selects the curated default catalog.
func RegistryConfig(raw string) []string {
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// ExtraDestinations validates and canonicalizes a tenant's explicit widening.
// Only exact public DNS hostnames are supported. Schemes, ports, paths, IPs,
// wildcards, cluster-local names, duplicates, and overbroad single-label names
// receive one named 400 instead of being stripped or silently ignored.
func ExtraDestinations(entries []string) ([]string, error) {
	if len(entries) > maxExtraDestinations {
		return nil, invalidAllowlist("too_many_entries", "", fmt.Sprintf("at most %d destinations are allowed", maxExtraDestinations))
	}
	out := make([]string, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, raw := range entries {
		host := strings.ToLower(raw)
		switch {
		case raw == "":
			return nil, invalidAllowlist("empty", raw, "destination must not be empty")
		case raw != strings.TrimSpace(raw):
			return nil, invalidAllowlist("whitespace", raw, "destination must not contain surrounding whitespace")
		case strings.ContainsAny(raw, ":/*?#@"):
			return nil, invalidAllowlist("not_a_hostname", raw, "use an exact hostname without a scheme, port, path, query, or wildcard")
		case slices.Contains(modelProviderDomains, host):
			return nil, invalidAllowlist("already_baseline", raw, "model-provider APIs are reachable only through the credential proxy")
		case net.ParseIP(host) != nil:
			return nil, invalidAllowlist("ip_literal", raw, "IP literals are not supported")
		case len(validation.IsDNS1123Subdomain(host)) != 0 || !strings.Contains(host, "."):
			return nil, invalidAllowlist("not_a_hostname", raw, "destination must be a valid multi-label DNS hostname")
		case privateOrClusterHost(host):
			return nil, invalidAllowlist("private_name", raw, "private and cluster-local destinations cannot be allowlisted")
		}
		if _, exists := seen[host]; exists {
			return nil, invalidAllowlist("duplicate", raw, "duplicate destinations are not allowed")
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	slices.Sort(out)
	return out, nil
}

func invalidAllowlist(reason, entry, message string) error {
	params := map[string]any{"reason": reason}
	if entry != "" {
		params["entry"] = entry
	}
	return core.NewBadRequestError("AGENT_SESSION_EGRESS_ALLOWLIST_INVALID", message, params)
}

func (c Config) normalized() (Config, error) {
	if len(c.SetupRegistryDomains) == 0 {
		c.SetupRegistryDomains = slices.Clone(registryDomains)
	}
	validated, err := ExtraDestinations(c.SetupRegistryDomains)
	if err != nil {
		return Config{}, fmt.Errorf("session egress: BEX_AGENT_SETUP_REGISTRIES: %w", err)
	}
	c.SetupRegistryDomains = validated
	if len(c.SnapshotStoreDomains) > 0 {
		hosts, err := ExtraDestinations(c.SnapshotStoreDomains)
		if err != nil {
			return Config{}, fmt.Errorf("session egress: BEX_AGENT_SNAPSHOT_S3_ENDPOINT: %w", err)
		}
		c.SnapshotStoreDomains = hosts
	}
	return c, nil
}

// setupRegistryDomains returns the platform setup-only registry catalog. A
// Manager built through NewManager already holds the validated, normalized list;
// a zero-value Manager (used in rendering tests) falls back to the curated
// default so the setup phase is never emptied by an unset config. Reading the
// stored value avoids re-validating the static catalog on every render (m40/t005).
func (m *Manager) setupRegistryDomains() []string {
	if len(m.Config.SetupRegistryDomains) == 0 {
		return registryDomains
	}
	return m.Config.SetupRegistryDomains
}

func (m *Manager) snapshotStoreDomains() []string {
	if m == nil {
		return nil
	}
	return m.Config.SnapshotStoreDomains
}

// ModelEndpointHost validates one per-session provider endpoint and returns the
// exact public DNS host the Cilium policy must admit.
//
// ADR047 D5/D6 forward note: the model endpoint is deliberately derived per
// session from the caller's selected agent/provider, never a global constant.
// When the wave-2 metering LLM proxy (D6 phase 2) ships, the provider resolver
// must return only that single in-cluster proxy endpoint, so direct vendor model
// hosts disappear from newly rendered policies — one egress choke point then owns
// both token metering and exfiltration containment. No change is needed here: a
// narrower resolved endpoint simply renders a narrower baseline.
func ModelEndpointHost(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", core.NewBadRequestError("AGENT_SESSION_MODEL_ENDPOINT_INVALID", "modelEndpoint must be an absolute HTTPS endpoint", nil)
	}
	if u.Port() != "" && u.Port() != "443" {
		return "", core.NewBadRequestError("AGENT_SESSION_MODEL_ENDPOINT_INVALID", "modelEndpoint must use HTTPS port 443", nil)
	}
	modelHost := strings.ToLower(u.Hostname())
	if net.ParseIP(modelHost) != nil || len(validation.IsDNS1123Subdomain(modelHost)) != 0 ||
		!strings.Contains(modelHost, ".") || privateOrClusterHost(modelHost) {
		return "", core.NewBadRequestError("AGENT_SESSION_MODEL_ENDPOINT_INVALID", "modelEndpoint must use a DNS hostname", nil)
	}
	return modelHost, nil
}

// SnapshotEndpointHost returns the exact public DNS host a path-style
// presigned S3 URL will use. Empty input is the Hibernated-tier-off case.
// HTTPS (or HTTP with an empty/443 port) only: the Cilium FQDN rule admits
// TCP/443, matching the sandbox curl PUT/GET.
func SnapshotEndpointHost(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("BEX_AGENT_SNAPSHOT_S3_ENDPOINT must be an absolute http(s) origin (host only)")
	}
	if u.Port() != "" && u.Port() != "443" {
		return "", fmt.Errorf("BEX_AGENT_SNAPSHOT_S3_ENDPOINT must use port 443")
	}
	host := strings.ToLower(u.Hostname())
	if net.ParseIP(host) != nil || len(validation.IsDNS1123Subdomain(host)) != 0 ||
		!strings.Contains(host, ".") || privateOrClusterHost(host) {
		return "", fmt.Errorf("BEX_AGENT_SNAPSHOT_S3_ENDPOINT must use a public DNS hostname")
	}
	return host, nil
}

// privateOrClusterHost reports whether host is a loopback or cluster-local name
// that must never enter a tenant egress allowlist or be used as a model
// endpoint. Both validators share this reserved-suffix set so it lives in one
// place and cannot drift (m40/t005).
func privateOrClusterHost(host string) bool {
	return host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".cluster.local")
}

// PrepareSetup installs the setup posture before the sandbox is created. It is
// idempotent for the same input, but refuses to broaden a policy that already
// reached the agent phase.
func (m *Manager) PrepareSetup(ctx context.Context, namespace, sessionID, modelEndpoint string, extra []string) error {
	return m.apply(ctx, namespace, sessionID, PhaseSetup, modelEndpoint, extra)
}

// TransitionToAgent removes package registries from the session policy. It
// verifies that the immutable tenant widening is byte-for-byte equivalent to
// setup, so a transition cannot smuggle in a new destination.
func (m *Manager) TransitionToAgent(ctx context.Context, namespace, sessionID, modelEndpoint string, extra []string) error {
	return m.apply(ctx, namespace, sessionID, PhaseAgent, modelEndpoint, extra)
}

func (m *Manager) apply(ctx context.Context, namespace, sessionID string, phase Phase, modelEndpoint string, extra []string) error {
	if m == nil || m.Client == nil {
		return errors.New("session egress: Kubernetes policy client is not configured")
	}
	if len(validation.IsDNS1123Label(sessionID)) != 0 {
		return fmt.Errorf("session egress: invalid session id %q", sessionID)
	}
	extra, err := ExtraDestinations(extra)
	if err != nil {
		return err
	}
	desired, err := m.policy(namespace, sessionID, phase, modelEndpoint, extra)
	if err != nil {
		return err
	}
	key := client.ObjectKeyFromObject(desired)
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(ciliumPolicyGVK)
	err = m.Client.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		if phase == PhaseAgent {
			return core.NewConflictError("AGENT_SESSION_EGRESS_PHASE_INVALID", "setup egress policy does not exist", map[string]any{"sessionId": sessionID})
		}
		return m.Client.Create(ctx, desired)
	}
	if err != nil {
		return fmt.Errorf("get session egress policy: %w", err)
	}
	currentPhase := Phase(current.GetAnnotations()[phaseAnnotation])
	if currentPhase == PhaseAgent && phase == PhaseSetup {
		return core.NewConflictError("AGENT_SESSION_EGRESS_PHASE_INVALID", "agent sessions cannot return to the setup egress phase", map[string]any{"sessionId": sessionID})
	}
	if current.GetAnnotations()[hashAnnotation] != desired.GetAnnotations()[hashAnnotation] {
		return core.NewConflictError("AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE", "the session egress allowlist cannot change after setup starts", map[string]any{"sessionId": sessionID})
	}
	desired.SetResourceVersion(current.GetResourceVersion())
	return m.Client.Update(ctx, desired)
}

// Delete removes only the exact managed policy for a session. Kubernetes'
// structural sandbox default-deny remains selected throughout and afterward.
func (m *Manager) Delete(ctx context.Context, namespace, sessionID string) error {
	if m == nil || m.Client == nil {
		return errors.New("session egress: Kubernetes policy client is not configured")
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(ciliumPolicyGVK)
	obj.SetNamespace(namespace)
	obj.SetName(policyName(sessionID))
	if err := m.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj); apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("get session egress policy for deletion: %w", err)
	}
	labels := obj.GetLabels()
	if labels[managedByLabel] != managedByValue || labels[SessionLabel] != sessionID {
		return errors.New("delete session egress policy: existing object is not owned by this session")
	}
	if err := m.Client.Delete(ctx, obj); err != nil {
		return fmt.Errorf("delete session egress policy: %w", err)
	}
	return nil
}

func (m *Manager) policy(namespace, sessionID string, phase Phase, modelEndpoint string, extra []string) (*unstructured.Unstructured, error) {
	modelHost, err := ModelEndpointHost(modelEndpoint)
	if err != nil {
		return nil, err
	}
	for _, domain := range extra {
		if domain == modelHost || slices.Contains(modelProviderDomains, domain) || slices.Contains(githubDomains, domain) || slices.Contains(m.snapshotStoreDomains(), domain) {
			return nil, invalidAllowlist("already_baseline", domain, "destination is already part of the agent baseline")
		}
	}
	allowJSON, _ := json.Marshal(extra)
	// The model host stays in the immutable identity/hash (it is still the
	// session's logical model target) even when the proxy narrowing omits it from
	// the reachable domains, so the allowlist-immutability check is unaffected.
	identityJSON, _ := json.Marshal(struct {
		Model string   `json:"model"`
		Extra []string `json:"extra"`
	}{Model: modelHost, Extra: extra})
	digest := sha256.Sum256(identityJSON)
	domains := append([]string{}, githubDomains...)
	// ADR062 narrowing: with the model proxy on, the tenant pod cannot resolve or
	// reach the vendor host directly — the agent talks to it only through the
	// gateway proxy (the gateway rule below), so a stolen credential inside the
	// sandbox has no vendor destination to use it against.
	if m.Config.ModelProxyPort == 0 {
		domains = append(domains, modelHost)
	}
	if phase == PhaseSetup {
		domains = append(domains, m.setupRegistryDomains()...)
	}
	domains = append(domains, m.snapshotStoreDomains()...)
	domains = append(domains, extra...)
	domains = uniqueSorted(domains)
	dnsNames := append(slices.Clone(domains), credentialGatewayHost)
	dnsNames = uniqueSorted(dnsNames)

	egressRules := []any{
		dnsRule(dnsNames),
		fqdnRule(domains),
		gatewayRule(credentialGatewayPort),
	}
	if m.Config.ModelProxyPort != 0 {
		egressRules = append(egressRules, gatewayRule(m.Config.ModelProxyPort))
	}

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cilium.io/v2",
		"kind":       "CiliumNetworkPolicy",
		"metadata": map[string]any{
			"name":      policyName(sessionID),
			"namespace": namespace,
			"labels": map[string]any{
				managedByLabel: managedByValue,
				SessionLabel:   sessionID,
			},
			"annotations": map[string]any{
				phaseAnnotation: string(phase),
				allowAnnotation: string(allowJSON),
				hashAnnotation:  hex.EncodeToString(digest[:]),
				modelAnnotation: modelHost,
			},
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{"matchLabels": map[string]any{SessionLabel: sessionID}},
			"egress":           egressRules,
		},
	}}
	obj.SetGroupVersionKind(ciliumPolicyGVK)
	return obj, nil
}

// matchNames renders a domain list as Cilium's `{"matchName": <domain>}` form,
// shared by the L7 DNS filter and the toFQDNs allow.
func matchNames(domains []string) []any {
	names := make([]any, 0, len(domains))
	for _, domain := range domains {
		names = append(names, map[string]any{"matchName": domain})
	}
	return names
}

func dnsRule(domains []string) map[string]any {
	return map[string]any{
		"toEndpoints": []any{map[string]any{"matchLabels": map[string]any{
			"k8s:io.kubernetes.pod.namespace": "kube-system",
			"k8s:k8s-app":                     "kube-dns",
		}}},
		"toPorts": []any{map[string]any{
			"ports": []any{map[string]any{"port": "53", "protocol": "ANY"}},
			"rules": map[string]any{"dns": matchNames(domains)},
		}},
	}
}

func fqdnRule(domains []string) map[string]any {
	servers := make([]any, 0, len(domains))
	for _, domain := range domains {
		servers = append(servers, domain)
	}
	return map[string]any{
		"toFQDNs": matchNames(domains),
		"toPorts": []any{map[string]any{
			"ports":       []any{map[string]any{"port": "443", "protocol": "TCP"}},
			"serverNames": servers,
		}},
	}
}

func gatewayRule(port uint16) map[string]any {
	return map[string]any{
		"toEndpoints": []any{map[string]any{"matchLabels": map[string]any{
			"k8s:io.kubernetes.pod.namespace":         "bex-system",
			"k8s:io.cilium.k8s.policy.serviceaccount": "bex-ssh-gateway",
			"k8s:app.kubernetes.io/name":              "bex-ssh-gateway",
		}}},
		"toPorts": []any{map[string]any{"ports": []any{map[string]any{
			"port": strconv.Itoa(int(port)), "protocol": "TCP",
		}}}},
	}
}

func policyName(sessionID string) string {
	digest := sha256.Sum256([]byte(sessionID))
	return "agent-session-egress-" + hex.EncodeToString(digest[:8])
}

func uniqueSorted(in []string) []string {
	slices.Sort(in)
	return slices.Compact(in)
}

var githubDomains = []string{
	"api.github.com",
	"codeload.github.com",
	"github.com",
	"objects.githubusercontent.com",
	"raw.githubusercontent.com",
	"uploads.github.com",
}

var registryDomains = []string{
	"auth.docker.io",
	"crates.io",
	"download.docker.com",
	"files.pythonhosted.org",
	"index.rubygems.org",
	"production.cloudflare.docker.com",
	"proxy.golang.org",
	"pypi.org",
	"registry-1.docker.io",
	"registry.npmjs.org",
	"registry.yarnpkg.com",
	"repo.maven.apache.org",
	"rubygems.org",
	"static.crates.io",
	"sum.golang.org",
}
