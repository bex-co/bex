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

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProductActivityIdentityAndCanceledRequest(t *testing.T) {
	for _, tc := range []struct {
		id   Identity
		kind string
	}{
		{Identity{Subject: "account", Method: "session"}, "human"},
		{Identity{Subject: "account", Method: "oauth2", Human: true}, "human"},
		{Identity{Subject: "key", Method: "oauth2"}, "machine"},
		{Identity{}, "unknown"},
	} {
		t.Run(tc.kind+tc.id.Method, func(t *testing.T) {
			now := time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)
			var got ProductActivity
			b := &Base{Clock: func() time.Time { return now }, ProductActivity: func(ctx context.Context, e ProductActivity) error {
				if ctx.Err() != nil {
					t.Fatal("completion recording inherited request cancellation")
				}
				if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > time.Second {
					t.Fatal("missing bounded deadline")
				}
				got = e
				return errors.New("sink unavailable")
			}}
			ctx, cancel := context.WithCancel(WithIdentity(context.Background(), tc.id))
			cancel()
			b.ObserveProductActivity(ctx, ProductActivity{WorkspaceID: "workspace", ResourceID: "resource", ResourceType: "static_site", EventType: "created"})
			if got.ActorType != tc.kind || got.ActorID != tc.id.Subject || !got.At.Equal(now) {
				t.Fatalf("event: %+v", got)
			}
		})
	}
}
