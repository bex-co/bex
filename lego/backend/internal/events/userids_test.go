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

package events

import (
	"context"
	"testing"
)

type fakeUserIDs map[string]string

func (f fakeUserIDs) LookupOwnerIDs(_ context.Context, subjects []string) (map[string]string, error) {
	out := map[string]string{}
	for _, s := range subjects {
		if id, ok := f[s]; ok {
			out[s] = id
		}
	}
	return out, nil
}

// w5/m277: the feed names a person by the usr- id the members list answers;
// an API-key client without a user id keeps its subject.
func TestEventUsersCarryTheirUsrID(t *testing.T) {
	svc := &Service{UserIDs: fakeUserIDs{"kratos-a": "own-00000000000000000007"}}
	events := []Event{
		{Details: Details{TriggeredByUser: "kratos-a", Trigger: &Trigger{User: &UserRef{ID: "kratos-a", Email: "a@example.com"}}}},
		{Details: Details{SuspendedByUser: &UserRef{ID: "kratos-a"}, TriggeredByUser: "client-1"}},
		{Details: Details{ResumedByUser: &UserRef{ID: "client-1"}}},
	}
	svc.resolveUserIDs(context.Background(), events)
	const want = "usr-00000000000000000007"
	if events[0].Details.TriggeredByUser != want || events[0].Details.Trigger.User.ID != want || events[0].Details.Trigger.User.Email != "a@example.com" {
		t.Errorf("trigger = %+v / %+v", events[0].Details.TriggeredByUser, events[0].Details.Trigger.User)
	}
	if events[1].Details.SuspendedByUser.ID != want || events[1].Details.TriggeredByUser != "client-1" {
		t.Errorf("suspend = %+v, triggered %q", events[1].Details.SuspendedByUser, events[1].Details.TriggeredByUser)
	}
	if events[2].Details.ResumedByUser.ID != "client-1" {
		t.Errorf("an API-key client keeps its subject: %+v", events[2].Details.ResumedByUser)
	}
	(&Service{}).resolveUserIDs(context.Background(), events) // nil lookup is a no-op
}
