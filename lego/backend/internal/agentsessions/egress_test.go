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

package agentsessions

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestCreateEgressDerivesProviderAndCanonicalizesTenantInput(t *testing.T) {
	t.Parallel()

	endpoint, allowlist, err := createEgress("tea-a", AgentConfig{Agent: "codex"}, nil, []string{"Z.example.com", "a.example.com"})
	if err != nil {
		t.Fatalf("createEgress: %v", err)
	}
	if endpoint != "https://api.openai.com/v1" {
		t.Fatalf("endpoint = %q", endpoint)
	}
	if want := []string{"a.example.com", "z.example.com"}; !reflect.DeepEqual(allowlist, want) {
		t.Fatalf("allowlist = %v, want %v", allowlist, want)
	}
}

func TestCreateEgressReturnsNamedBadRequests(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		config AgentConfig
		extra  []string
		code   string
	}{
		{name: "unknown provider", config: AgentConfig{Agent: "custom"}, code: "AGENT_SESSION_MODEL_ENDPOINT_INVALID"},
		{name: "private endpoint", config: AgentConfig{Agent: "custom", ModelEndpoint: "https://model.internal/v1"}, code: "AGENT_SESSION_MODEL_ENDPOINT_INVALID"},
		{name: "invalid widening", config: AgentConfig{Agent: "codex"}, extra: []string{"https://example.com"}, code: "AGENT_SESSION_EGRESS_ALLOWLIST_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := createEgress("tea-a", tc.config, nil, tc.extra)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("error = %v, want bad request", err)
			}
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != tc.code {
				t.Fatalf("error = %#v, want code %s", err, tc.code)
			}
		})
	}
}

// The workspace tier's hosts merge with the request's explicit allowlist:
// duplicates across the two collapse, and the merged widening keeps the
// 32-destination cap with the session allowlist's typed error.
func TestCreateEgressMergesWorkspaceHostsWithTheRequestAllowlist(t *testing.T) {
	t.Parallel()

	_, allowlist, err := createEgress("tea-a", AgentConfig{Agent: "codex"}, []string{"b.example.com", "docs.example.com"}, []string{"Docs.Example.com", "a.example.com"})
	if err != nil {
		t.Fatalf("createEgress: %v", err)
	}
	if want := []string{"a.example.com", "b.example.com", "docs.example.com"}; !reflect.DeepEqual(allowlist, want) {
		t.Fatalf("allowlist = %v, want %v", allowlist, want)
	}

	workspace := make([]string, 20)
	requested := make([]string, 13)
	for i := range workspace {
		workspace[i] = fmt.Sprintf("w%d.example.com", i)
	}
	for i := range requested {
		requested[i] = fmt.Sprintf("r%d.example.com", i)
	}
	_, _, err = createEgress("tea-a", AgentConfig{Agent: "codex"}, workspace, requested)
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "AGENT_SESSION_EGRESS_ALLOWLIST_INVALID" || coded.Params["reason"] != "too_many_entries" {
		t.Fatalf("33 merged destinations error = %#v, want the typed cap", err)
	}
}
