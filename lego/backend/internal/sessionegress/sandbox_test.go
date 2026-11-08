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

package sessionegress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestSandboxAllowListAcceptsRenderHostnames(t *testing.T) {
	got, err := SandboxAllowList([]string{"registry.npmjs.org", "*.githubusercontent.com", "api.example.co.uk"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"*.githubusercontent.com", "api.example.co.uk", "registry.npmjs.org"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want sorted %v", got, want)
	}
}

func TestSandboxAllowListRefusesEachInvalidFormByName(t *testing.T) {
	tooMany := make([]string, MaxSandboxRules+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("h%d.example.com", i)
	}
	for _, tc := range []struct {
		name    string
		domains []string
		reason  string
	}{
		{"none", nil, "rules_required"},
		{"too many", tooMany, "too_many_rules"},
		{"empty", []string{""}, "empty"},
		{"scheme", []string{"https://example.com"}, "not_a_hostname"},
		{"port", []string{"example.com:443"}, "not_a_hostname"},
		{"path", []string{"example.com/x"}, "not_a_hostname"},
		{"uppercase", []string{"Example.com"}, "not_lowercase"},
		{"ipv4", []string{"93.184.216.34"}, "ip_literal"},
		{"single label", []string{"localhost"}, "not_a_hostname"},
		{"inner wildcard", []string{"api.*.example.com"}, "not_a_hostname"},
		{"double wildcard", []string{"**.example.com"}, "not_a_hostname"},
		{"bare wildcard", []string{"*"}, "not_a_hostname"},
		{"wildcard tld", []string{"*.com"}, "not_a_hostname"},
		{"cluster local", []string{"kubernetes.default.svc.cluster.local"}, "private_name"},
		{"internal", []string{"metadata.google.internal"}, "private_name"},
		{"duplicate", []string{"example.com", "example.com"}, "duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := SandboxAllowList(tc.domains)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != "SANDBOX_NETWORK_POLICY_INVALID" || coded.Params["reason"] != tc.reason {
				t.Fatalf("err = %#v, want SANDBOX_NETWORK_POLICY_INVALID reason %s", err, tc.reason)
			}
		})
	}
}

// The rendered object is the narrowest shape: filtered DNS for exactly the
// listed names and TLS 443 with matching SNI, selecting one sandbox's Pod.
func TestRenderSandboxPolicyIsExactDNSAndTLSForOneSandbox(t *testing.T) {
	obj, err := RenderSandboxPolicy("tea-a-sandbox", "sbx-abc", []string{"registry.npmjs.org", "*.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(obj.Object)
	var policy struct {
		Metadata struct {
			Name      string            `json:"name"`
			Namespace string            `json:"namespace"`
			Labels    map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			EndpointSelector map[string]map[string]string `json:"endpointSelector"`
			Egress           []map[string]json.RawMessage `json:"egress"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(policy.Metadata.Name, "sandbox-egress-") || policy.Metadata.Namespace != "tea-a-sandbox" {
		t.Fatalf("identity = %s/%s", policy.Metadata.Namespace, policy.Metadata.Name)
	}
	if policy.Metadata.Labels["app.kubernetes.io/managed-by"] != "bex-sandbox-egress" || policy.Metadata.Labels[SandboxIDLabel] != "sbx-abc" {
		t.Fatalf("labels = %v", policy.Metadata.Labels)
	}
	if sel := policy.Spec.EndpointSelector["matchLabels"]; len(sel) != 1 || sel[SandboxIDLabel] != "sbx-abc" {
		t.Fatalf("selector = %v, want exactly this sandbox", policy.Spec.EndpointSelector)
	}
	if len(policy.Spec.Egress) != 2 {
		t.Fatalf("egress rules = %d, want DNS + TLS", len(policy.Spec.Egress))
	}
	for _, rule := range policy.Spec.Egress {
		for key := range rule {
			if key == "toCIDR" || key == "toCIDRSet" || key == "toEntities" || key == "toServices" {
				t.Fatalf("rule carries %s: %v", key, rule)
			}
		}
	}
	want := `[{"matchPattern":"*.example.com"},{"matchName":"registry.npmjs.org"}]`
	if got := string(policy.Spec.Egress[1]["toFQDNs"]); got != want {
		t.Fatalf("toFQDNs = %s, want %s", got, want)
	}
	if got := string(policy.Spec.Egress[1]["toPorts"]); got != `[{"ports":[{"port":"443","protocol":"TCP"}],"serverNames":["*.example.com","registry.npmjs.org"]}]` {
		t.Fatalf("TLS toPorts = %s", got)
	}
	if got := string(policy.Spec.Egress[0]["toPorts"]); !strings.Contains(got, `"dns":[{"matchPattern":"*.example.com"},{"matchName":"registry.npmjs.org"}]`) || !strings.Contains(got, `"port":"53"`) {
		t.Fatalf("DNS toPorts = %s", got)
	}
}

func TestSandboxManagerAppliesReadsAndDeletesOnlyItsOwnPolicy(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	m := &SandboxManager{Client: cl}
	ctx := context.Background()
	if err := m.Apply(ctx, "tea-a-sandbox", "sbx-one", []string{"registry.npmjs.org", "*.example.com"}); err != nil {
		t.Fatal(err)
	}
	// Re-applying the same sandbox updates its own policy in place.
	if err := m.Apply(ctx, "tea-a-sandbox", "sbx-one", []string{"example.com"}); err != nil {
		t.Fatal(err)
	}
	rules, err := m.Rules(ctx, "tea-a-sandbox", "sbx-one")
	if err != nil || !slices.Equal(rules, []string{"example.com"}) {
		t.Fatalf("rules = %v, %v", rules, err)
	}
	if rules, err := m.Rules(ctx, "tea-a-sandbox", "sbx-missing"); err != nil || rules != nil {
		t.Fatalf("missing policy rules = %v, %v; want none", rules, err)
	}

	// A foreign object at the derived name is never adopted or deleted.
	foreign := &unstructured.Unstructured{}
	foreign.SetGroupVersionKind(ciliumPolicyGVK)
	foreign.SetNamespace("tea-a-sandbox")
	foreign.SetName(sandboxPolicyName("sbx-two"))
	if err := cl.Create(ctx, foreign); err != nil {
		t.Fatal(err)
	}
	if err := m.Apply(ctx, "tea-a-sandbox", "sbx-two", []string{"example.com"}); err == nil {
		t.Fatal("Apply adopted a policy it does not own")
	}
	if err := m.Delete(ctx, "tea-a-sandbox", "sbx-two"); err == nil {
		t.Fatal("Delete removed a policy it does not own")
	}

	if err := m.Delete(ctx, "tea-a-sandbox", "sbx-one"); err != nil {
		t.Fatal(err)
	}
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(ciliumPolicyGVK)
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "tea-a-sandbox", Name: sandboxPolicyName("sbx-one")}, obj); err == nil {
		t.Fatal("policy still exists after Delete")
	}
	if err := m.Delete(ctx, "tea-a-sandbox", "sbx-one"); err != nil {
		t.Fatalf("second Delete = %v, want nil (gone is the goal)", err)
	}
}

func TestSandboxManagerWithoutClientFailsClosed(t *testing.T) {
	var m *SandboxManager
	if err := m.Apply(context.Background(), "ns", "sbx-x", []string{"example.com"}); err == nil {
		t.Fatal("nil manager applied a policy")
	}
}
