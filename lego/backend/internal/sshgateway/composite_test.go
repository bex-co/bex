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

package sshgateway

import (
	"context"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

type recordingResolver struct {
	name   string
	seen   string
	target apps.SSHInstanceTarget
}

func (r *recordingResolver) ResolveSSHSession(_ context.Context, username string) (apps.SSHInstanceTarget, error) {
	r.seen = username
	return r.target, nil
}

func TestCompositeRoutesByIDKind(t *testing.T) {
	agsID := ids.New(ids.AgentSession)
	srvID := ids.New(ids.Service)
	cases := []struct {
		name     string
		username string
		wantAgg  bool // routed to AgentSessions?
	}{
		{"agent session id", agsID, true},
		{"bare service id", srvID, false},
		{"service instance id", srvID + "-abcd", false},
		{"malformed username", "not-an-id", false},
		{"empty username", "", false},
		{"single segment", "root", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			appsR := &recordingResolver{name: "apps"}
			agentR := &recordingResolver{name: "agent"}
			c := CompositeResolver{Apps: appsR, AgentSessions: agentR}
			if _, err := c.ResolveSSHSession(context.Background(), tc.username); err != nil {
				t.Fatalf("resolve: %v", err)
			}
			routedToAgent := agentR.seen == tc.username && appsR.seen == ""
			routedToApps := appsR.seen == tc.username && agentR.seen == ""
			if tc.wantAgg && !routedToAgent {
				t.Fatalf("username %q should route to AgentSessions (apps.seen=%q agent.seen=%q)", tc.username, appsR.seen, agentR.seen)
			}
			if !tc.wantAgg && !routedToApps {
				t.Fatalf("username %q should route to Apps (apps.seen=%q agent.seen=%q)", tc.username, appsR.seen, agentR.seen)
			}
		})
	}
}

// A nil AgentSessions resolver (feature not wired) must fall through to Apps for
// every username, including ags-… ones — never panic.
func TestCompositeNilAgentResolverFallsThroughToApps(t *testing.T) {
	appsR := &recordingResolver{name: "apps"}
	c := CompositeResolver{Apps: appsR}
	agsID := ids.New(ids.AgentSession)
	if _, err := c.ResolveSSHSession(context.Background(), agsID); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if appsR.seen != agsID {
		t.Fatalf("nil agent resolver should fall through to Apps, apps.seen=%q", appsR.seen)
	}
}
