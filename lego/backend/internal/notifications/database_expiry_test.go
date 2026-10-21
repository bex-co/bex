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
	"strings"
	"testing"
	"time"
)

// w8/m126: each free-expiry stage mails every workspace owner with an address
// once, naming the dates that matter for that stage.
func TestFreeDatabaseExpiryMailsEveryOwner(t *testing.T) {
	expires := time.Date(2026, 12, 2, 9, 30, 0, 0, time.UTC)
	deletes := expires.Add(14 * 24 * time.Hour)
	cases := map[string]string{
		"warned":       "expires on 2026-12-02T09:30:00Z",
		"expired":      "before 2026-12-16T09:30:00Z",
		"final-warned": "deleted on 2026-12-16T09:30:00Z",
		"deleted":      "were deleted",
	}
	for stage, want := range cases {
		mailer := &fakeMailer{}
		svc := newTestService(newBillingOwners("alice", "bob", "carol"), nil, mailer, fakeIdentities{"alice": "alice@example.com", "bob": "bob@example.com"})
		svc.DashboardBaseURL = "https://dashboard.example.com"
		n := DatabaseExpiryNotifier{Service: svc}
		if err := n.NotifyFreeDatabaseExpiry(context.Background(), "tea-a", "dpg-a", "orders", stage, expires, deletes); err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		if len(mailer.sent) != 2 {
			t.Fatalf("%s: sent %d mails, want alice and bob", stage, len(mailer.sent))
		}
		body := mailer.sent[0].body
		if !strings.Contains(body, want) || !strings.Contains(body, "orders") {
			t.Errorf("%s mail lacks %q:\n%s", stage, want, body)
		}
		if link := strings.Contains(body, "https://dashboard.example.com/d/dpg-a"); link == (stage == "deleted") {
			t.Errorf("%s mail database link present = %v", stage, link)
		}
	}
}
