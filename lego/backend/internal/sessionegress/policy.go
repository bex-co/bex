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
	// postureAnnotation marks the full-internet posture (w10/m19): no public
	// FQDN rule at all, only filtered DNS for the gateway and the gateway hops
	// (Git, model, egress proxy). The bex-api-session-egress admission policy
	// admits the egress-proxy hop only on a policy carrying this marker, and
	// such a policy may carry no FQDN rule. Allowlist postures carry none.
	postureAnnotation = "bex.co/egress-posture"
	postureFull       = "full"

	credentialGatewayHost = "bex-ssh-gateway.bex-system.svc.cluster.local"
	credentialGatewayPort = uint16(8082)
	// DefaultEgressProxyPort is the gateway egress proxy listener (w10/m19,
	// BEX_AGENT_EGRESS_PROXY_ADDR on the gateway).
	DefaultEgressProxyPort = uint16(8085)

	maxExtraDestinations = 32
)

var (
	ciliumPolicyGVK = schema.GroupVersionKind{Group: "cilium.io", Version: "v2", Kind: "CiliumNetworkPolicy"}
	// Registered model-provider API hosts are never tenant widening destinations:
	// model traffic must traverse the credential proxy even when the selected
	// profile uses a different provider.
	// openrouter.ai is the platform model route's host (ADR090); like the
	// others it is reached only through the gateway model proxy.
	modelProviderDomains = []string{"api.anthropic.com", "api.openai.com", "generativelanguage.googleapis.com", "openrouter.ai"}
)

// NetworkTierFull is the workspace agent-environment tier whose sessions
// reach any public host over HTTP(S) through the gateway egress proxy
// (w10/m19). It lives here, beside the policy it selects, so bex-api's tier
// store, the egress grant verb and the renderer share one literal.
const NetworkTierFull = "full"

// ModelProviderDomains is the registered model-provider API host list. These
// hosts are reachable only through the gateway model proxy: never a tenant
// widening and never an egress-proxy destination.
func ModelProviderDomains() []string { return slices.Clone(modelProviderDomains) }

// Egress is the destination set one session policy renders. FullInternet
// selects the proxy posture: no direct public egress at all, and every
// outbound HTTP(S) connection goes through the gateway egress proxy, which
// refuses model providers and private ranges itself. Otherwise Hosts is the
// validated tenant widening on top of GitHub and the registry catalog.
type Egress struct {
	FullInternet bool
	Hosts        []string
}

// Phase marks where one sandbox generation of an agent session is in its
// lifecycle: setup (clone + dependency install, before the agent runs) or agent.
// Both phases admit the same destinations — the package-registry catalog stays
// reachable for the whole session (ADR047 D5 amendment 2026-10-27), so agents
// can install dependencies and build and test what they change. What the phase
// still guards is the identity of the generation: the setup→agent transition
// must carry exactly the destinations setup was prepared with.
type Phase string

const (
	PhaseSetup Phase = "setup"
	PhaseAgent Phase = "agent"
)

// Config owns the platform-curated package registries every session may reach
// (setup and agent phase alike). An empty list selects the built-in catalog;
// production can replace it through BEX_AGENT_SETUP_REGISTRIES (the name
// predates the 2026-10-27 amendment that kept registries open after setup).
// The per-session model endpoint is deliberately not process config: it comes
// from that session's selected agent/provider.
type Config struct {
	SetupRegistryDomains []string
	// ModelProxyPort, when non-zero (BEX_AGENT_MODEL_PROXY_URL wired), turns on the
	// ADR062 narrowing: the tenant pod's egress no longer admits the vendor model
	// host directly (the agent reaches it only through the gateway proxy), and a
	// gateway rule for this port is added instead. 0 retains the legacy policy
	// renderer for tests/migrations, but the agent-session service refuses new
	// mutation work when the proxy is absent (ADR064).
	ModelProxyPort uint16
	// EgressProxyPort is the gateway egress proxy port a full-internet session
	// policy admits (BEX_AGENT_EGRESS_PROXY_URL's port). 0 selects
	// DefaultEgressProxyPort.
	EgressProxyPort uint16
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
		case PrivateOrClusterHost(host):
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

// Validate reports the configuration errors NewManager would surface, so
// cmd/api can reject a malformed environment during config load — before any
// startup side effect — rather than after migrations have run (.pm/w1/070.md).
func (c Config) Validate() error {
	_, err := c.normalized()
	return err
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
	if c.EgressProxyPort == 0 {
		c.EgressProxyPort = DefaultEgressProxyPort
	}
	return c, nil
}

// DefaultRegistryDomains is the built-in package-registry catalog (npm, PyPI,
// Go proxy, Maven, crates, RubyGems, Docker Hub) a session reaches when
// BEX_AGENT_SETUP_REGISTRIES is unset.
func DefaultRegistryDomains() []string { return slices.Clone(registryDomains) }

// RegistryDomains is the validated registry catalog this Manager renders into
// every session policy.
func (m *Manager) RegistryDomains() []string { return slices.Clone(m.setupRegistryDomains()) }

// BaselineDestination reports whether host is already part of every session's
// baseline (GitHub) or is a model-provider API that is reachable only through
// the gateway model proxy. Neither can be a tenant widening, so a workspace
// allowlist naming one is refused when it is saved, not at its next dispatch.
func BaselineDestination(host string) bool {
	host = strings.ToLower(host)
	return slices.Contains(githubDomains, host) || slices.Contains(modelProviderDomains, host)
}

// setupRegistryDomains returns the platform registry catalog. A
// Manager built through NewManager already holds the validated, normalized list;
// a zero-value Manager (used in rendering tests) falls back to the curated
// default so the catalog is never emptied by an unset config. Reading the
// stored value avoids re-validating the static catalog on every render (m40/t005).
func (m *Manager) setupRegistryDomains() []string {
	if len(m.Config.SetupRegistryDomains) == 0 {
		return registryDomains
	}
	return m.Config.SetupRegistryDomains
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
		!strings.Contains(modelHost, ".") || PrivateOrClusterHost(modelHost) {
		return "", core.NewBadRequestError("AGENT_SESSION_MODEL_ENDPOINT_INVALID", "modelEndpoint must use a DNS hostname", nil)
	}
	return modelHost, nil
}

// PrivateOrClusterHost reports whether host is a loopback or cluster-local name
// that must never enter a tenant egress allowlist or be used as a model
// endpoint. Both validators share this reserved-suffix set so it lives in one
// place and cannot drift (m40/t005).
//
// The gateway egress proxy refuses the same reserved names before it resolves
// anything.
func PrivateOrClusterHost(host string) bool {
	return host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") ||
		strings.HasSuffix(host, ".cluster.local")
}

// PrepareSetup installs the setup posture for a new sandbox generation before
// that sandbox is created. It is idempotent for the same input. A later
// generation of the same session (a steer, or the next message after the idle
// close) re-prepares the existing policy: it may carry a different tenant
// widening, because the workspace environment and the turn's explicit
// allowlist are resolved again for every dispatch. The setup posture admits
// nothing the agent posture does not, so re-entering setup never reopens a
// destination the agent phase had closed.
func (m *Manager) PrepareSetup(ctx context.Context, namespace, sessionID, modelEndpoint string, egress Egress) error {
	return m.apply(ctx, namespace, sessionID, PhaseSetup, modelEndpoint, egress)
}

// TransitionToAgent moves the session policy into the agent phase. It keeps the
// package registries (ADR047 D5 amendment 2026-10-27) and verifies that the
// tenant widening is byte-for-byte the one setup was prepared with, so a
// transition cannot smuggle in a new destination.
func (m *Manager) TransitionToAgent(ctx context.Context, namespace, sessionID, modelEndpoint string, egress Egress) error {
	return m.apply(ctx, namespace, sessionID, PhaseAgent, modelEndpoint, egress)
}

func (m *Manager) apply(ctx context.Context, namespace, sessionID string, phase Phase, modelEndpoint string, egress Egress) error {
	if m == nil || m.Client == nil {
		return errors.New("session egress: Kubernetes policy client is not configured")
	}
	if len(validation.IsDNS1123Label(sessionID)) != 0 {
		return fmt.Errorf("session egress: invalid session id %q", sessionID)
	}
	extra, err := ExtraDestinations(egress.Hosts)
	if err != nil {
		return err
	}
	egress.Hosts = extra
	desired, err := m.policy(namespace, sessionID, phase, modelEndpoint, egress)
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
	// Only the transition is pinned to the prepared identity: setup starts a new
	// sandbox generation and may re-resolve the widening.
	if phase == PhaseAgent && current.GetAnnotations()[hashAnnotation] != desired.GetAnnotations()[hashAnnotation] {
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

func (m *Manager) policy(namespace, sessionID string, phase Phase, modelEndpoint string, egress Egress) (*unstructured.Unstructured, error) {
	modelHost, err := ModelEndpointHost(modelEndpoint)
	if err != nil {
		return nil, err
	}
	extra := egress.Hosts
	if egress.FullInternet {
		// The proxy reaches every public host, so a widening adds nothing; it
		// is dropped rather than rendered as a direct FQDN rule.
		extra = []string{}
	}
	for _, domain := range extra {
		if domain == modelHost || slices.Contains(modelProviderDomains, domain) || slices.Contains(githubDomains, domain) {
			return nil, invalidAllowlist("already_baseline", domain, "destination is already part of the agent baseline")
		}
	}
	allowJSON, _ := json.Marshal(extra)
	// The model host stays in the immutable identity/hash (it is still the
	// session's logical model target) even when the proxy narrowing omits it from
	// the reachable domains, so the allowlist-immutability check is unaffected.
	// Full is omitted when false so every allowlist posture keeps the identity
	// hash it had before the full tier existed (a session prepared before a
	// rollout still transitions after it).
	identityJSON, _ := json.Marshal(struct {
		Model string   `json:"model"`
		Extra []string `json:"extra"`
		Full  bool     `json:"full,omitempty"`
	}{Model: modelHost, Extra: extra, Full: egress.FullInternet})
	digest := sha256.Sum256(identityJSON)
	annotations := map[string]any{
		phaseAnnotation: string(phase),
		allowAnnotation: string(allowJSON),
		hashAnnotation:  hex.EncodeToString(digest[:]),
		modelAnnotation: modelHost,
	}
	var egressRules []any
	if egress.FullInternet {
		// Full internet (w10/m19): no public FQDN rule, so a direct connection
		// to any public address is dropped by the sandbox default-deny. DNS
		// resolves only the gateway; HTTP(S) reaches the internet through the
		// egress proxy hop, model traffic through the model proxy hop and Git
		// through the Git hop. The model proxy is mandatory here: without it
		// the agent would have no model route at all.
		if m.Config.ModelProxyPort == 0 {
			return nil, errors.New("session egress: the full-internet posture requires the gateway model proxy")
		}
		annotations[postureAnnotation] = postureFull
		egressRules = []any{
			dnsRule([]string{credentialGatewayHost}),
			gatewayRule(credentialGatewayPort),
			gatewayRule(m.Config.ModelProxyPort),
			gatewayRule(m.egressProxyPort()),
		}
	} else {
		egressRules = m.allowlistRules(modelHost, extra)
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
			"annotations": annotations,
		},
		"spec": map[string]any{
			"endpointSelector": map[string]any{"matchLabels": map[string]any{SessionLabel: sessionID}},
			"egress":           egressRules,
		},
	}}
	obj.SetGroupVersionKind(ciliumPolicyGVK)
	return obj, nil
}

func (m *Manager) egressProxyPort() uint16 {
	if m.Config.EgressProxyPort != 0 {
		return m.Config.EgressProxyPort
	}
	return DefaultEgressProxyPort
}

// allowlistRules renders the registries/custom posture: filtered DNS, one
// exact FQDN+SNI rule (GitHub, the registry catalog and the tenant widening),
// and the Git and model gateway hops.
func (m *Manager) allowlistRules(modelHost string, extra []string) []any {
	domains := append([]string{}, githubDomains...)
	// ADR062 narrowing: with the model proxy on, the tenant pod cannot resolve or
	// reach the vendor host directly — the agent talks to it only through the
	// gateway proxy (the gateway rule below), so a stolen credential inside the
	// sandbox has no vendor destination to use it against.
	if m.Config.ModelProxyPort == 0 {
		domains = append(domains, modelHost)
	}
	// Package registries stay open in both phases (ADR047 D5 amendment
	// 2026-10-27): an agent that cannot install dependencies cannot build or
	// test the change it delivers.
	domains = append(domains, m.setupRegistryDomains()...)
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
	return egressRules
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
