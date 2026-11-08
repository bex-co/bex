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

package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestPGWebhookEndpointCapAdmitsOneHundred is w3/056's regression: the cap was
// 25, below the 100 destinations Render (and bex's own plan gate) allow a Scale
// workspace, so the 26th create was refused. The 100th must commit and the
// 101st must be the typed limit refusal.
func TestPGWebhookEndpointCapAdmitsOneHundred(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "cap-"+stamp, PlanScale, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	create := func(i int) error {
		name := fmt.Sprintf("hook-%d", i)
		_, err := st.CreateWebhookEndpoint(ctx, tenant.ID, name, "https://example.com/"+name, "whsec_test", []string{}, true, "alice-"+stamp)
		return err
	}
	for i := 1; i <= 100; i++ {
		if err := create(i); err != nil {
			t.Fatalf("create #%d: %v", i, err)
		}
	}
	if err := create(101); !errors.Is(err, ErrWebhookEndpointLimit) {
		t.Fatalf("create #101 = %v, want ErrWebhookEndpointLimit", err)
	}
}
