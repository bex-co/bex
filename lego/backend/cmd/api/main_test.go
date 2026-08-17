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

package main

import (
	"context"
	"testing"
)

type replayStoreStub struct{}

func (replayStoreStub) ClaimGitWebhookDelivery(context.Context, string) (bool, error) {
	return true, nil
}
func (replayStoreStub) ReleaseGitWebhookDelivery(context.Context, string) error { return nil }

func TestConfiguredWebhookRequiresDurableReplayStore(t *testing.T) {
	if err := validateWebhookReplayConfig("manual", "", nil); err == nil {
		t.Fatal("manual webhook without replay store was accepted")
	}
	if err := validateWebhookReplayConfig("", "github", nil); err == nil {
		t.Fatal("GitHub webhook without replay store was accepted")
	}
	if err := validateWebhookReplayConfig("manual", "", replayStoreStub{}); err != nil {
		t.Fatalf("configured replay store rejected: %v", err)
	}
	if err := validateWebhookReplayConfig("", "", nil); err != nil {
		t.Fatalf("disabled webhook should not need a store: %v", err)
	}
}

func TestSandboxTemplateRegistryIncludesPluggableAgentDriver(t *testing.T) {
	templates := sandboxTemplateRegistry("alpine:3", "agent:test")
	agent, ok := templates["agent"]
	if !ok {
		t.Fatal("agent template is not registered")
	}
	if agent.Image != "agent:test" || len(agent.Entrypoint) != 1 || agent.Entrypoint[0] != "bex-agent-driver" {
		t.Fatalf("agent template = %+v", agent)
	}
	if agent.CPU != "2" || agent.Memory != "4Gi" {
		t.Fatalf("agent resource limits = %s/%s, want 2/4Gi", agent.CPU, agent.Memory)
	}
	if base := templates["base"]; base.Image != "alpine:3" {
		t.Fatalf("base template image = %q", base.Image)
	}
}

// requireCPAuth is the w1/m53 fail-closed gate on the internal control-plane
// API (:8091): an empty BEX_CP_TOKEN must abort startup unless the explicit
// local-dev override is set.
func TestRequireCPAuth(t *testing.T) {
	cases := []struct {
		name     string
		token    string
		insecure string
		wantErr  bool
	}{
		{"token set", "s3kret", "", false},
		{"token set, insecure ignored", "s3kret", "1", false},
		{"empty token fails closed", "", "", true},
		{"empty token, insecure!=1 still fails", "", "0", true},
		{"empty token, insecure!=1 word still fails", "", "yes", true},
		{"empty token, insecure=1 override", "", "1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireCPAuth(tc.token, tc.insecure)
			if (err != nil) != tc.wantErr {
				t.Fatalf("requireCPAuth(%q,%q) err=%v, wantErr=%v", tc.token, tc.insecure, err, tc.wantErr)
			}
		})
	}
}
