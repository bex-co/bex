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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w21/093: the pull request intake asks mayCreate and maySteer from a closure
// that linkedMember calls, so each recorded the closure's name
// ("agentsessions.startingMember.func1") until it named its own verb.
func TestPullRequestAccessProbesRecordTheirOwnAuditVerbs(t *testing.T) {
	svc, st, fga, _ := fixture()
	audit := &auditCapture{}
	svc.Audit = audit
	sessionID := seedLiveSession(st, "tea-a")
	fga.parents[sessionID] = "tea-a"

	probe := func(f func() (bool, error)) bool {
		// The intake runs the probe from a closure, as linkedMember's callback.
		done := make(chan bool)
		go func() { ok, err := f(); done <- ok && err == nil }()
		return <-done
	}
	if !probe(func() (bool, error) { return svc.mayCreate(caller("nobody"), "alice", "tea-a") }) {
		t.Fatal("a member of tea-a may not create sessions in it")
	}
	if probe(func() (bool, error) { return svc.mayCreate(caller("nobody"), "bob", "tea-a") }) {
		t.Fatal("a member of tea-b may create sessions in tea-a")
	}
	if !probe(func() (bool, error) { return svc.maySteer(caller("nobody"), "alice", sessionID) }) {
		t.Fatal("a member of tea-a may not steer its session")
	}
	if probe(func() (bool, error) { return svc.maySteer(caller("nobody"), "bob", sessionID) }) {
		t.Fatal("a member of tea-b may steer a session of tea-a")
	}

	audit.mu.Lock()
	defer audit.mu.Unlock()
	got := map[string]map[core.AuditOutcome]int{}
	for _, ev := range audit.events {
		if got[ev.Verb] == nil {
			got[ev.Verb] = map[core.AuditOutcome]int{}
		}
		got[ev.Verb][ev.Outcome]++
	}
	for _, verb := range []string{AuditVerbCheckCreateAccess, AuditVerbCheckSteerAccess} {
		if got[verb][core.AuditAllowed] != 1 || got[verb][core.AuditDenied] != 1 {
			t.Errorf("%s recorded %v, want one allowed and one denied row", verb, got[verb])
		}
		delete(got, verb)
	}
	if len(got) != 0 {
		t.Errorf("the probes also recorded %v", got)
	}
}
