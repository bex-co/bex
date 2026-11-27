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

package envgroups

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// StoredServiceEnvironment is the read the Blueprint plan judges a linked
// service by: the store's row first, the App label the reconciler re-stamps
// on its next pass only as fallback, a read failure as an error.
func TestStoredServiceEnvironmentFollowsTheStoreFirst(t *testing.T) {
	web := sampleApp("web")
	web.Labels[core.LabelAppID] = "srv-web"
	web.Labels[core.LabelEnvironment] = "env-alpha"

	storeErr := errors.New("store unavailable")
	for _, tc := range []struct {
		name    string
		hook    func(context.Context, string) (string, error)
		want    string
		wantErr string
	}{
		{"unwired hook reads the label", nil, "env-alpha", ""},
		{"the store wins over a lagging label", func(_ context.Context, id string) (string, error) {
			if id != "srv-web" {
				return "", core.ErrNotFound
			}
			return "env-beta", nil
		}, "env-beta", ""},
		{"no store record falls back to the label", func(context.Context, string) (string, error) {
			return "", core.ErrNotFound
		}, "env-alpha", ""},
		{"a store failure is an error, not a stale label", func(context.Context, string) (string, error) {
			return "", storeErr
		}, "", "store unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newService(newFakeStore(), web)
			svc.ServiceEnvironment = tc.hook
			got, err := svc.StoredServiceEnvironment(testCtx(), web)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("StoredServiceEnvironment = %q, %v, want error %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("StoredServiceEnvironment = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}
