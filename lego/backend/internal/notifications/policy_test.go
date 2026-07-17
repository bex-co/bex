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

package notifications

import (
	"context"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"testing"
)

func TestServiceNotificationPolicyDecisionTable(t *testing.T) {
	kinds := []struct {
		name string
		kind deployMailKind
	}{{"started", deployMailStarted}, {"succeeded", deployMailSucceeded}, {"failed", deployMailFailed}}
	cases := []struct {
		policy string
		want   map[deployMailKind]bool
	}{
		{"default", map[deployMailKind]bool{deployMailFailed: true}},
		{"failure", map[deployMailKind]bool{deployMailFailed: true}},
		{"all", map[deployMailKind]bool{deployMailStarted: true, deployMailSucceeded: true, deployMailFailed: true}},
		{"none", map[deployMailKind]bool{}},
	}
	for _, tc := range cases {
		for _, event := range kinds {
			t.Run(tc.policy+"/"+event.name, func(t *testing.T) {
				st := newFakeStore()
				st.recipients["tea-a"] = []store.NotifyRecipient{{Subject: "alice", DeployFailed: true}}
				mailer := &fakeMailer{}
				svc := newTestService(st, nil, mailer, fakeIdentities{"alice": "alice@example.com"})
				svc.notifyDeploy(context.Background(), "tea-a", "web", event.kind, tc.policy, deployDetails{})
				if got := len(mailer.sent) == 1; got != tc.want[event.kind] {
					t.Fatalf("sent=%v, want %v", got, tc.want[event.kind])
				}
			})
		}
	}
}

func TestFailureOnlyDefaultIsExplicit(t *testing.T) {
	if defaultSettings != (SettingsView{DeployStarted: false, DeploySucceeded: false, DeployFailed: true}) {
		t.Fatalf("defaultSettings = %+v", defaultSettings)
	}
}
