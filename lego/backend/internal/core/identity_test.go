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

package core

import "testing"

// IsAPIKey keys on how the credential is held (an OAuth2 token with no end user
// behind it), not on the subject: the workspace two-factor check and the
// lock-out guard both borrow the key's creator on it.
func TestIdentityIsAPIKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   Identity
		want bool
	}{
		{"machine token", Identity{Method: MethodOAuth2, Subject: "k", ClientID: "k"}, true},
		{"machine token with only a subject", Identity{Method: MethodOAuth2, Subject: "k"}, true},
		{"anonymous oauth2", Identity{Method: MethodOAuth2}, true},
		{"human oauth2 token", Identity{Method: MethodOAuth2, Human: true, Subject: "u", ClientID: "app"}, false},
		{"kratos session", Identity{Method: MethodSession, Human: true, Subject: "u"}, false},
		{"background session", Identity{Method: MethodSession, Subject: "u"}, false},
		{"gateway identity", Identity{Method: MethodSSH, Subject: "u"}, false},
		{"empty", Identity{}, false},
	} {
		if got := tc.id.IsAPIKey(); got != tc.want {
			t.Errorf("%s: IsAPIKey() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIdentityKeyClientID(t *testing.T) {
	if got := (Identity{ClientID: "client", Subject: "subject"}).KeyClientID(); got != "client" {
		t.Errorf("KeyClientID() = %q, want the client id when both are set", got)
	}
	if got := (Identity{Subject: "subject"}).KeyClientID(); got != "subject" {
		t.Errorf("KeyClientID() = %q, want the subject when no client id is set", got)
	}
	if got := (Identity{}).KeyClientID(); got != "" {
		t.Errorf("KeyClientID() = %q, want empty", got)
	}
}

// The object prefixes are what the funnel branches on, and the object builders
// must produce exactly them.
func TestObjectPrefixesMatchTheirBuilders(t *testing.T) {
	if got := WorkspaceObject("tea-1"); got != WorkspaceObjectPrefix+"tea-1" {
		t.Errorf("WorkspaceObject = %q, want the prefix constant + id", got)
	}
	if WorkspaceObjectPrefix != "workspace:" {
		t.Errorf("WorkspaceObjectPrefix = %q", WorkspaceObjectPrefix)
	}
	if AgentSessionObjectPrefix != "agent_session:" {
		t.Errorf("AgentSessionObjectPrefix = %q", AgentSessionObjectPrefix)
	}
}
