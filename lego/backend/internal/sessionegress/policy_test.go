/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package sessionegress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestExtraDestinationsCanonicalizesAndRejectsEveryInvalidEntry(t *testing.T) {
	got, err := ExtraDestinations([]string{"Docs.Example.com", "api.example.net"})
	if err != nil {
		t.Fatalf("ExtraDestinations: %v", err)
	}
	if want := []string{"api.example.net", "docs.example.com"}; !slices.Equal(got, want) {
		t.Fatalf("destinations = %v, want %v", got, want)
	}

	for _, entries := range [][]string{
		{""},
		{" example.com"},
		{"https://example.com"},
		{"example.com:443"},
		{"*.example.com"},
		{"api.anthropic.com"},
		{"api.openai.com"},
		{"generativelanguage.googleapis.com"},
		{"192.0.2.1"},
		{"localhost"},
		{"service.cluster.local"},
		{"example.com", "EXAMPLE.COM"},
	} {
		_, err := ExtraDestinations(entries)
		var coded *core.CodedError
		if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != "AGENT_SESSION_EGRESS_ALLOWLIST_INVALID" {
			t.Errorf("ExtraDestinations(%q) error = %#v, want named 400", entries, err)
		}
	}

	tooMany := make([]string, maxExtraDestinations+1)
	for i := range tooMany {
		tooMany[i] = "host" + string(rune('a'+i%26)) + ".example.com"
	}
	if _, err := ExtraDestinations(tooMany); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("too many entries error = %v, want bad request", err)
	}
}

// ADR047 D5 amendment (2026-10-27): the package registries stay reachable
// through the agent phase, so an agent can install dependencies and run the
// build and tests of the change it delivers. Setup and agent render the same
// destinations; only the phase annotation differs.
func TestPolicyKeepsRegistriesInBothPhasesAndExactBaseline(t *testing.T) {
	m := &Manager{}
	setup, err := m.policy("tea-test-sandbox", "ags-test", PhaseSetup, "https://models.example.com/v1", Egress{Hosts: []string{"docs.example.com"}})
	if err != nil {
		t.Fatalf("setup policy: %v", err)
	}
	agent, err := m.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://models.example.com/v1", Egress{Hosts: []string{"docs.example.com"}})
	if err != nil {
		t.Fatalf("agent policy: %v", err)
	}

	setupNames := fqdnNames(t, setup)
	agentNames := fqdnNames(t, agent)
	for _, name := range append(slices.Clone(githubDomains), "models.example.com", "docs.example.com") {
		if !slices.Contains(setupNames, name) || !slices.Contains(agentNames, name) {
			t.Errorf("baseline/extra destination %q missing: setup=%v agent=%v", name, setupNames, agentNames)
		}
	}
	for _, registry := range registryDomains {
		if !slices.Contains(setupNames, registry) || !slices.Contains(agentNames, registry) {
			t.Errorf("registry %q missing: setup=%v agent=%v", registry, slices.Contains(setupNames, registry), slices.Contains(agentNames, registry))
		}
	}
	if !slices.Equal(setupNames, agentNames) || !slices.Equal(dnsNames(t, setup), dnsNames(t, agent)) {
		t.Fatalf("setup and agent destinations differ: setup=%v agent=%v", setupNames, agentNames)
	}
	if setup.GetAnnotations()[phaseAnnotation] != string(PhaseSetup) || agent.GetAnnotations()[phaseAnnotation] != string(PhaseAgent) {
		t.Fatalf("phase annotations = %q/%q", setup.GetAnnotations()[phaseAnnotation], agent.GetAnnotations()[phaseAnnotation])
	}
	if slices.Contains(agentNames, "api.openai.com") || slices.Contains(agentNames, "api.anthropic.com") {
		t.Fatalf("agent baseline included unconfigured model vendor: %v", agentNames)
	}
	selector, _, _ := unstructured.NestedStringMap(agent.Object, "spec", "endpointSelector", "matchLabels")
	if selector[SessionLabel] != "ags-test" || len(selector) != 1 {
		t.Fatalf("endpoint selector = %#v, want exact session identity", selector)
	}
	if !slices.Contains(dnsNames(t, agent), credentialGatewayHost) {
		t.Fatal("credential gateway DNS name missing from filtered DNS rule")
	}
	rules, _, _ := unstructured.NestedSlice(agent.Object, "spec", "egress")
	gateway := rules[2].(map[string]any)
	endpoint := gateway["toEndpoints"].([]any)[0].(map[string]any)["matchLabels"].(map[string]any)
	port := gateway["toPorts"].([]any)[0].(map[string]any)["ports"].([]any)[0].(map[string]any)["port"]
	if endpoint["k8s:io.cilium.k8s.policy.serviceaccount"] != "bex-ssh-gateway" || endpoint["k8s:io.kubernetes.pod.namespace"] != "bex-system" || port != "8082" {
		t.Fatalf("credential gateway rule = %#v port=%v", endpoint, port)
	}
}

// The per-session policy is purely additive on top of the clusterwide
// sandbox-egress-default-deny (node/metadata/private-network) floor. A render
// that ever emitted a toCIDR, toEntities, wildcard matchPattern, or broad
// endpoint selector could open a path the structural deny cannot re-close, so
// every rendered profile — setup, agent baseline, and agent+widened — must
// contain only the three exact allow shapes and never a floor-lifting escape.
func TestEveryRenderedProfileIsAdditiveOnlyAndPreservesTheDenyFloor(t *testing.T) {
	m := &Manager{}
	cases := []struct {
		name  string
		phase Phase
		extra []string
	}{
		{"setup", PhaseSetup, []string{"docs.example.com"}},
		{"agent baseline", PhaseAgent, nil},
		{"agent widened", PhaseAgent, []string{"docs.example.com"}},
	}
	for _, tc := range cases {
		obj, err := m.policy("tea-test-sandbox", "ags-test", tc.phase, "https://models.example.com/v1", Egress{Hosts: tc.extra})
		if err != nil {
			t.Fatalf("%s policy: %v", tc.name, err)
		}
		// The endpoint selector must pin exactly one immutable session identity —
		// never a broad or empty selector that would attach the allow to siblings.
		selector, _, _ := unstructured.NestedStringMap(obj.Object, "spec", "endpointSelector", "matchLabels")
		if len(selector) != 1 || selector[SessionLabel] != "ags-test" {
			t.Errorf("%s endpoint selector = %#v, want exactly the session identity", tc.name, selector)
		}
		rules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "egress")
		if len(rules) != 3 {
			t.Fatalf("%s egress = %d rules, want exactly dns+fqdn+gateway", tc.name, len(rules))
		}
		// No rendered rule may carry a floor-lifting primitive. toFQDNs/toEndpoints
		// (the additive DNS/FQDN/gateway shapes) are the only admitted keys.
		for i, raw := range rules {
			rule := raw.(map[string]any)
			for _, forbidden := range []string{"toCIDR", "toCIDRSet", "toEntities", "toGroups", "toRequires", "toServices"} {
				if _, present := rule[forbidden]; present {
					t.Errorf("%s egress rule %d contains floor-lifting key %q", tc.name, i, forbidden)
				}
			}
		}
		// No FQDN entry may be a wildcard: exact names only keep a tenant from
		// widening beyond a single validated destination.
		for _, name := range fqdnNames(t, obj) {
			if name == "" || name == "*" || strings.ContainsAny(name, "*?") {
				t.Errorf("%s rendered a wildcard/empty FQDN %q", tc.name, name)
			}
		}
		// enableDefaultDeny must never be flipped off on the per-session policy —
		// that structural posture belongs to the clusterwide floor, and a false
		// here would opt the endpoint out of default-deny entirely.
		if _, present, _ := unstructured.NestedFieldNoCopy(obj.Object, "spec", "enableDefaultDeny"); present {
			t.Errorf("%s per-session policy set enableDefaultDeny; the floor owns that posture", tc.name)
		}
	}
}

func TestAgentPolicyWithNoWideningIsExactlyTheBaseline(t *testing.T) {
	m := &Manager{}
	agent, err := m.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://models.example.com/v1", Egress{})
	if err != nil {
		t.Fatalf("agent policy: %v", err)
	}
	want := append(slices.Clone(githubDomains), "models.example.com")
	want = append(want, registryDomains...)
	slices.Sort(want)
	if got := fqdnNames(t, agent); !slices.Equal(got, want) {
		t.Fatalf("no-extra agent destinations = %v, want exact baseline %v", got, want)
	}
	for _, redundant := range []string{"github.com", "models.example.com"} {
		if _, err := m.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://models.example.com/v1", Egress{Hosts: []string{redundant}}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("redundant destination %q error = %v, want named bad request", redundant, err)
		}
	}
}

func TestConfigurableRegistryCatalogAndPerSessionModelEndpoint(t *testing.T) {
	m, err := NewManager(fakePolicyClient(t), Config{SetupRegistryDomains: RegistryConfig("packages.example.com")})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	setup, err := m.policy("tea-test-sandbox", "ags-test", PhaseSetup, "https://models-a.example.com/v1", Egress{})
	if err != nil {
		t.Fatalf("setup policy: %v", err)
	}
	names := fqdnNames(t, setup)
	if !slices.Contains(names, "packages.example.com") || slices.Contains(names, "registry.npmjs.org") || !slices.Contains(names, "models-a.example.com") {
		t.Fatalf("configured registry/per-session model not reflected: %v", names)
	}
	if _, err := NewManager(fakePolicyClient(t), Config{SetupRegistryDomains: RegistryConfig("good.example.com, bad.example.com")}); err == nil {
		t.Fatal("NewManager silently trimmed an invalid registry config member")
	}
}

func TestManagerTransitionPinsThePreparedAllowlist(t *testing.T) {
	cl := fakePolicyClient(t)
	m := &Manager{Client: cl}
	ctx := context.Background()
	const namespace, sessionID = "tea-test-sandbox", "ags-test"

	if err := m.PrepareSetup(ctx, namespace, sessionID, "https://models.example.com/v1", Egress{Hosts: []string{"docs.example.com"}}); err != nil {
		t.Fatalf("PrepareSetup: %v", err)
	}
	if err := m.TransitionToAgent(ctx, namespace, sessionID, "https://models.example.com/v1", Egress{Hosts: []string{"other.example.com"}}); err == nil {
		t.Fatal("TransitionToAgent changed the immutable allowlist")
	} else {
		var coded *core.CodedError
		if !errors.As(err, &coded) || coded.Code != "AGENT_SESSION_EGRESS_ALLOWLIST_IMMUTABLE" {
			t.Fatalf("allowlist mutation error = %#v", err)
		}
	}
	if err := m.TransitionToAgent(ctx, namespace, sessionID, "https://other-models.example.com/v1", Egress{Hosts: []string{"docs.example.com"}}); err == nil {
		t.Fatal("TransitionToAgent changed the immutable per-session model endpoint")
	}
	if err := m.TransitionToAgent(ctx, namespace, sessionID, "https://models.example.com/v1", Egress{Hosts: []string{"docs.example.com"}}); err != nil {
		t.Fatalf("TransitionToAgent: %v", err)
	}
	obj := getPolicy(t, cl, namespace, sessionID)
	if got := obj.GetAnnotations()[phaseAnnotation]; got != string(PhaseAgent) {
		t.Fatalf("phase annotation = %q, want agent", got)
	}
	if !slices.Contains(fqdnNames(t, obj), "registry.npmjs.org") {
		t.Fatal("persisted agent policy closed the package registries")
	}

	// The next sandbox generation (a steer, or the message after the idle
	// close) re-prepares the same session policy with the widening resolved
	// for that dispatch — e.g. after an admin changed the workspace hosts —
	// and its transition is then pinned to the new identity.
	if err := m.PrepareSetup(ctx, namespace, sessionID, "https://models.example.com/v1", Egress{Hosts: []string{"other.example.com"}}); err != nil {
		t.Fatalf("PrepareSetup for the next generation: %v", err)
	}
	obj = getPolicy(t, cl, namespace, sessionID)
	if got := obj.GetAnnotations()[phaseAnnotation]; got != string(PhaseSetup) {
		t.Fatalf("re-prepared phase annotation = %q, want setup", got)
	}
	if names := fqdnNames(t, obj); !slices.Contains(names, "other.example.com") || slices.Contains(names, "docs.example.com") {
		t.Fatalf("re-prepared destinations = %v, want the new generation's widening only", names)
	}
	if err := m.TransitionToAgent(ctx, namespace, sessionID, "https://models.example.com/v1", Egress{Hosts: []string{"docs.example.com"}}); err == nil {
		t.Fatal("TransitionToAgent restored the previous generation's allowlist")
	}
	if err := m.TransitionToAgent(ctx, namespace, sessionID, "https://models.example.com/v1", Egress{Hosts: []string{"other.example.com"}}); err != nil {
		t.Fatalf("TransitionToAgent for the next generation: %v", err)
	}
	if err := m.Delete(ctx, namespace, sessionID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: policyName(sessionID)}, obj); err == nil {
		t.Fatal("policy still exists after Delete")
	}
}

func TestTransitionWithoutSetupFailsClosed(t *testing.T) {
	m := &Manager{Client: fakePolicyClient(t)}
	err := m.TransitionToAgent(context.Background(), "tea-test-sandbox", "ags-test", "https://models.example.com/v1", Egress{})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "AGENT_SESSION_EGRESS_PHASE_INVALID" {
		t.Fatalf("transition error = %#v, want named conflict", err)
	}
}

func TestInvalidPerSessionModelEndpointIsNamedBadRequest(t *testing.T) {
	m := &Manager{Client: fakePolicyClient(t)}
	for _, endpoint := range []string{"", "http://models.example.com", "https://192.0.2.1", "https://models.example.com:8443", "https://models.svc.cluster.local"} {
		err := m.PrepareSetup(context.Background(), "tea-test-sandbox", "ags-test", endpoint, Egress{})
		var coded *core.CodedError
		if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != "AGENT_SESSION_MODEL_ENDPOINT_INVALID" {
			t.Errorf("endpoint %q error = %#v, want named 400", endpoint, err)
		}
	}
}

func TestDeleteRefusesAnObjectOutsideSessionOwnership(t *testing.T) {
	cl := fakePolicyClient(t)
	m := &Manager{Client: cl}
	ctx := context.Background()
	if err := m.PrepareSetup(ctx, "tea-test-sandbox", "ags-test", "https://models.example.com/v1", Egress{}); err != nil {
		t.Fatalf("PrepareSetup: %v", err)
	}
	obj := getPolicy(t, cl, "tea-test-sandbox", "ags-test")
	labels := obj.GetLabels()
	labels[SessionLabel] = "ags-other"
	obj.SetLabels(labels)
	if err := cl.Update(ctx, obj); err != nil {
		t.Fatalf("tamper fixture: %v", err)
	}
	if err := m.Delete(ctx, "tea-test-sandbox", "ags-test"); err == nil {
		t.Fatal("Delete removed a policy whose durable session identity disagreed")
	}
}

func fakePolicyClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(ciliumPolicyGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(ciliumPolicyGVK.GroupVersion().WithKind("CiliumNetworkPolicyList"), &unstructured.UnstructuredList{})
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func getPolicy(t *testing.T, cl client.Client, namespace, sessionID string) *unstructured.Unstructured {
	t.Helper()
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(ciliumPolicyGVK)
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: policyName(sessionID)}, obj); err != nil {
		t.Fatalf("get policy: %v", err)
	}
	return obj
}

// ADR062: with the model proxy on, the tenant pod must NOT be able to reach the
// vendor host directly — it disappears from the DNS and FQDN allows — and a
// gateway rule for the proxy port is added instead. Off (the default) is
// byte-identical to before.
func TestModelProxyNarrowingDropsVendorHostAndAddsGatewayPort(t *testing.T) {
	proxied := &Manager{Config: Config{ModelProxyPort: 8084}}
	agent, err := proxied.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://api.anthropic.com/v1", Egress{Hosts: []string{"docs.example.com"}})
	if err != nil {
		t.Fatalf("proxied policy: %v", err)
	}
	if slices.Contains(fqdnNames(t, agent), "api.anthropic.com") {
		t.Fatal("the vendor host is still directly reachable under the proxy narrowing")
	}
	if slices.Contains(dnsNames(t, agent), "api.anthropic.com") {
		t.Fatal("the vendor host is still DNS-resolvable under the proxy narrowing")
	}
	if !slices.Contains(fqdnNames(t, agent), "docs.example.com") {
		t.Fatal("an explicit tenant widening must survive the narrowing")
	}
	rules, _, _ := unstructured.NestedSlice(agent.Object, "spec", "egress")
	if len(rules) != 4 {
		t.Fatalf("egress = %d rules, want dns+fqdn+gitGateway+modelGateway", len(rules))
	}
	model := rules[3].(map[string]any)
	endpoint := model["toEndpoints"].([]any)[0].(map[string]any)["matchLabels"].(map[string]any)
	port := model["toPorts"].([]any)[0].(map[string]any)["ports"].([]any)[0].(map[string]any)["port"]
	if endpoint["k8s:app.kubernetes.io/name"] != "bex-ssh-gateway" || port != "8084" {
		t.Fatalf("model gateway rule = %#v port=%v, want the gateway on 8084", endpoint, port)
	}
	// The gateway host itself stays DNS-resolvable (the agent reaches the proxy by
	// that FQDN), and the digest still records the vendor host as the logical target.
	if !slices.Contains(dnsNames(t, agent), credentialGatewayHost) {
		t.Fatal("gateway host must remain DNS-resolvable so the agent can reach the proxy")
	}
	if agent.GetAnnotations()[modelAnnotation] != "api.anthropic.com" {
		t.Fatalf("model annotation = %q, want the vendor host recorded for audit", agent.GetAnnotations()[modelAnnotation])
	}
	// A tenant must not be able to re-open the vendor host by listing it as an
	// explicit widening — the already-baseline check rejects it even when narrowed.
	if _, err := proxied.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://api.anthropic.com/v1", Egress{Hosts: []string{"api.anthropic.com"}}); err == nil {
		t.Fatal("a tenant re-added the vendor host as an extra destination under the proxy")
	}
}

// ADR090 platform model route: an OpenRouter session's policy records
// openrouter.ai as its model target (the cluster admission policy admits that
// annotation), never opens it directly, and a tenant cannot add it as a widening.
func TestModelProxyNarrowingCoversTheOpenRouterRoute(t *testing.T) {
	proxied := &Manager{Config: Config{ModelProxyPort: 8084}}
	agent, err := proxied.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://openrouter.ai/api", Egress{})
	if err != nil {
		t.Fatalf("openrouter policy: %v", err)
	}
	if got := agent.GetAnnotations()[modelAnnotation]; got != "openrouter.ai" {
		t.Fatalf("model annotation = %q, want openrouter.ai", got)
	}
	if slices.Contains(fqdnNames(t, agent), "openrouter.ai") || slices.Contains(dnsNames(t, agent), "openrouter.ai") {
		t.Fatal("openrouter.ai is directly reachable; it must go through the model proxy")
	}
	if _, err := proxied.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://api.anthropic.com/v1", Egress{Hosts: []string{"openrouter.ai"}}); err == nil {
		t.Fatal("a tenant added openrouter.ai as an extra destination")
	}
}

func fqdnNames(t *testing.T, obj *unstructured.Unstructured) []string {
	t.Helper()
	rules, found, err := unstructured.NestedSlice(obj.Object, "spec", "egress")
	if err != nil || !found || len(rules) < 2 {
		t.Fatalf("egress rules missing: found=%v err=%v object=%#v", found, err, obj.Object)
	}
	rule, ok := rules[1].(map[string]any)
	if !ok {
		t.Fatalf("FQDN rule = %#v", rules[1])
	}
	raw, ok := rule["toFQDNs"].([]any)
	if !ok {
		t.Fatalf("toFQDNs = %#v", rule["toFQDNs"])
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, item.(map[string]any)["matchName"].(string))
	}
	return out
}

func dnsNames(t *testing.T, obj *unstructured.Unstructured) []string {
	t.Helper()
	rules, found, err := unstructured.NestedSlice(obj.Object, "spec", "egress")
	if err != nil || !found || len(rules) == 0 {
		t.Fatalf("DNS egress rule missing: found=%v err=%v", found, err)
	}
	rule := rules[0].(map[string]any)
	ports := rule["toPorts"].([]any)[0].(map[string]any)
	raw := ports["rules"].(map[string]any)["dns"].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		out = append(out, item.(map[string]any)["matchName"].(string))
	}
	return out
}

// Model-provider APIs are never directly reachable, in either phase and
// whatever the tenant widening: the registry catalog and a widening can
// only add public non-provider names.
func TestModelProvidersAreNeverDirectlyReachableInAnyPhase(t *testing.T) {
	proxied := &Manager{Config: Config{ModelProxyPort: 8084}}
	for _, phase := range []Phase{PhaseSetup, PhaseAgent} {
		for _, extra := range [][]string{nil, {"docs.example.com", "registry.npmjs.org"}} {
			obj, err := proxied.policy("tea-test-sandbox", "ags-test", phase, "https://api.anthropic.com/v1", Egress{Hosts: extra})
			if err != nil {
				t.Fatalf("%s policy: %v", phase, err)
			}
			for _, provider := range modelProviderDomains {
				if slices.Contains(fqdnNames(t, obj), provider) || slices.Contains(dnsNames(t, obj), provider) {
					t.Errorf("%s policy (extra=%v) admits model provider %q directly", phase, extra, provider)
				}
			}
		}
	}
	for _, provider := range modelProviderDomains {
		if _, err := ExtraDestinations([]string{provider}); err == nil {
			t.Errorf("ExtraDestinations accepted model provider %q", provider)
		}
		if !BaselineDestination(provider) {
			t.Errorf("BaselineDestination(%q) = false", provider)
		}
	}
	for _, host := range githubDomains {
		if !BaselineDestination(host) {
			t.Errorf("BaselineDestination(%q) = false", host)
		}
	}
	if BaselineDestination("docs.example.com") || BaselineDestination("registry.npmjs.org") {
		t.Fatal("BaselineDestination flagged an ordinary widening")
	}
}

// w10/m19: the full-internet posture renders no public FQDN rule at all — a
// direct connection to any public address is dropped — and exactly four
// endpoint rules: DNS filtered to the gateway host, and the Git, model and
// egress-proxy gateway hops. The bex-api-session-egress admission policy
// admits exactly this shape (and the 8085 hop only with the posture marker).
func TestFullInternetPostureIsOnlyDNSAndGatewayHops(t *testing.T) {
	proxied := &Manager{Config: Config{ModelProxyPort: 8084}}
	for _, phase := range []Phase{PhaseSetup, PhaseAgent} {
		obj, err := proxied.policy("tea-test-sandbox", "ags-test", phase, "https://api.anthropic.com/v1", Egress{FullInternet: true, Hosts: []string{"docs.example.com"}})
		if err != nil {
			t.Fatalf("%s full policy: %v", phase, err)
		}
		if got := obj.GetAnnotations()[postureAnnotation]; got != postureFull {
			t.Fatalf("posture annotation = %q, want %q", got, postureFull)
		}
		if got := obj.GetAnnotations()[allowAnnotation]; got != "[]" {
			t.Fatalf("allowlist annotation = %q, want [] (the proxy subsumes any widening)", got)
		}
		rules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "egress")
		if len(rules) != 4 {
			t.Fatalf("full egress = %d rules, want dns + 3 gateway hops", len(rules))
		}
		var ports []string
		for i, raw := range rules {
			rule := raw.(map[string]any)
			if _, ok := rule["toFQDNs"]; ok {
				t.Fatalf("rule %d is a public FQDN rule; the full posture must have none", i)
			}
			for _, forbidden := range []string{"toCIDR", "toCIDRSet", "toEntities", "toServices"} {
				if _, ok := rule[forbidden]; ok {
					t.Fatalf("rule %d carries %s", i, forbidden)
				}
			}
			port := rule["toPorts"].([]any)[0].(map[string]any)["ports"].([]any)[0].(map[string]any)["port"].(string)
			ports = append(ports, port)
			if port == "53" {
				continue
			}
			labels := rule["toEndpoints"].([]any)[0].(map[string]any)["matchLabels"].(map[string]any)
			if labels["k8s:app.kubernetes.io/name"] != "bex-ssh-gateway" || labels["k8s:io.cilium.k8s.policy.serviceaccount"] != "bex-ssh-gateway" || labels["k8s:io.kubernetes.pod.namespace"] != "bex-system" {
				t.Fatalf("rule %d endpoint = %#v, want the exact gateway identity", i, labels)
			}
		}
		if want := []string{"53", "8082", "8084", "8085"}; !slices.Equal(ports, want) {
			t.Fatalf("full posture ports = %v, want %v", ports, want)
		}
		if got := dnsNames(t, obj); !slices.Equal(got, []string{credentialGatewayHost}) {
			t.Fatalf("full posture DNS names = %v, want only the gateway host", got)
		}
	}
}

// The allowlist postures never carry the egress-proxy hop or the posture
// marker, and keep their pre-m19 identity hash (Full is omitted from the
// identity when false), so a session prepared before a rollout transitions
// after it.
func TestAllowlistPosturesNeverOpenTheEgressProxy(t *testing.T) {
	proxied := &Manager{Config: Config{ModelProxyPort: 8084, EgressProxyPort: 8085}}
	obj, err := proxied.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://api.anthropic.com/v1", Egress{Hosts: []string{"docs.example.com"}})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	if _, ok := obj.GetAnnotations()[postureAnnotation]; ok {
		t.Fatal("an allowlist posture carries the full-posture marker")
	}
	rules, _, _ := unstructured.NestedSlice(obj.Object, "spec", "egress")
	for _, raw := range rules {
		for _, port := range raw.(map[string]any)["toPorts"].([]any)[0].(map[string]any)["ports"].([]any) {
			if port.(map[string]any)["port"] == "8085" {
				t.Fatal("an allowlist posture opens the egress proxy hop")
			}
		}
	}
	// Golden pre-m19 identity: sha256 of {"model":...,"extra":[...]}.
	if got, want := obj.GetAnnotations()[hashAnnotation], legacyIdentityHash(t, "api.anthropic.com", []string{"docs.example.com"}); got != want {
		t.Fatalf("identity hash = %s, want the pre-m19 %s", got, want)
	}
	full, err := proxied.policy("tea-test-sandbox", "ags-test", PhaseAgent, "https://api.anthropic.com/v1", Egress{FullInternet: true})
	if err != nil {
		t.Fatalf("full policy: %v", err)
	}
	if full.GetAnnotations()[hashAnnotation] == obj.GetAnnotations()[hashAnnotation] {
		t.Fatal("full and allowlist postures share an identity hash")
	}
}

// A setup prepared under one posture cannot transition into the other: the
// posture is part of the immutable identity.
func TestTransitionCannotSwitchPosture(t *testing.T) {
	scheme := runtime.NewScheme()
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	m := &Manager{Client: cl, Config: Config{ModelProxyPort: 8084}}
	ctx := context.Background()
	if err := m.PrepareSetup(ctx, "tea-test-sandbox", "ags-test", "https://api.anthropic.com/v1", Egress{}); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if err := m.TransitionToAgent(ctx, "tea-test-sandbox", "ags-test", "https://api.anthropic.com/v1", Egress{FullInternet: true}); err == nil {
		t.Fatal("an allowlist setup transitioned into the full posture")
	}
	if err := m.TransitionToAgent(ctx, "tea-test-sandbox", "ags-test", "https://api.anthropic.com/v1", Egress{}); err != nil {
		t.Fatalf("same-posture transition: %v", err)
	}
}

// Without the model proxy there is no model route in the full posture, so it
// fails closed instead of rendering a policy the agent cannot use.
func TestFullInternetPostureRequiresTheModelProxy(t *testing.T) {
	if _, err := (&Manager{}).policy("tea-test-sandbox", "ags-test", PhaseSetup, "https://api.anthropic.com/v1", Egress{FullInternet: true}); err == nil {
		t.Fatal("full posture rendered without the model proxy")
	}
}

func legacyIdentityHash(t *testing.T, model string, extra []string) string {
	t.Helper()
	raw, err := json.Marshal(struct {
		Model string   `json:"model"`
		Extra []string `json:"extra"`
	}{Model: model, Extra: extra})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
