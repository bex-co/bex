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

package logs

import (
	"context"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Public instance ids must round-trip: emit the canonical id, filter by it,
// and still select only that pod (w5/m87).
func TestInstanceFilterAcceptsPublicID(t *testing.T) {
	svc := newService(map[string][]string{
		webInst: {"2026-07-05T00:00:01Z only-me"},
		"web-2": {"2026-07-05T00:00:02Z other"},
	}, sampleApp("web"), podFor("web", webInst), podFor("web", "web-2"))

	public := ids.ServiceInstanceID("web", webInst)
	entries, err := svc.QueryLogs(context.Background(), LogQuery{
		App: "web", Instance: []string{public},
	})
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(entries) != 1 || entries[0].Message != "only-me" {
		t.Fatalf("filter by public id = %+v", entries)
	}
	if entries[0].Labels[LabelInstance] != public {
		t.Fatalf("emitted instance = %q, want %q", entries[0].Labels[LabelInstance], public)
	}

	// Legacy raw-name bookmarks still work as inputs.
	entries, err = svc.QueryLogs(context.Background(), LogQuery{
		App: "web", Instance: []string{webInst},
	})
	if err != nil || len(entries) != 1 || entries[0].Labels[LabelInstance] != public {
		t.Fatalf("legacy raw-name filter = %+v, err=%v", entries, err)
	}

	// Foreign selectors never broaden the query.
	entries, err = svc.QueryLogs(context.Background(), LogQuery{
		App: "web", Instance: []string{ids.ServiceInstanceID("web", "no-such-pod")},
	})
	if err != nil || len(entries) != 0 {
		t.Fatalf("foreign selector = %+v, err=%v", entries, err)
	}
}
