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
	"fmt"
	"testing"
	"time"
)

// TestPGWebhookReadsReturnTheSecret is w3/m282: Render's webhook object carries
// `secret` on list, get and patch, so every endpoint read selects it (the
// service decides who may see it).
func TestPGWebhookReadsReturnTheSecret(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	defer pool.Close()

	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	tenant, err := st.CreateWorkspace(ctx, "secret-"+stamp, PlanScale, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	const secret = "whsec_readback"
	created, err := st.CreateWebhookEndpoint(ctx, tenant.ID, "hook", "https://example.com/hook", secret, []string{}, true, "alice-"+stamp)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := st.GetWebhookEndpoint(ctx, tenant.ID, created.ID)
	if err != nil || got.Secret != secret {
		t.Fatalf("get = %q (err %v), want %q", got.Secret, err, secret)
	}
	list, err := st.ListWebhookEndpoints(ctx, []string{tenant.ID}, time.Time{}, "", 10)
	if err != nil || len(list) != 1 || list[0].Secret != secret {
		t.Fatalf("list = %+v (err %v), want the secret", list, err)
	}
	updated, err := st.UpdateWebhookEndpoint(ctx, tenant.ID, created.ID, "hook-2", "https://example.com/hook", []string{}, false)
	if err != nil || updated.Secret != secret {
		t.Fatalf("update = %q (err %v), want %q", updated.Secret, err, secret)
	}
	toggled, err := st.SetWebhookEndpointEnabled(ctx, tenant.ID, created.ID, true, "")
	if err != nil || toggled.Secret != secret {
		t.Fatalf("set enabled = %q (err %v), want %q", toggled.Secret, err, secret)
	}
}
