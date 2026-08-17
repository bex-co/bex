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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPGStoreDevicePushSubscriptions(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	alice := "push-alice-" + stamp
	bob := "push-bob-" + stamp
	tenant, err := st.CreateWorkspace(ctx, "push-test-"+stamp, PlanHobby, alice)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	if err := st.AddMember(ctx, bob, tenant.ID, "viewer"); err != nil {
		t.Fatal(err)
	}
	settings, err := st.UpsertNotificationSettings(ctx, tenant.ID, alice, false, false, true)
	if err != nil {
		t.Fatal(err)
	}

	secret := "ExponentPushToken[pg-secret-" + stamp + "]"
	created, err := st.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
		TenantID: tenant.ID, Subject: alice, DeviceID: "alice-ios", SessionID: "session-alice",
		Provider: "expo", Platform: "ios", Token: secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.PreferenceID != settings.ID || created.CreatedAt.IsZero() || created.LastRegisteredAt.IsZero() {
		t.Fatalf("created subscription = %+v", created)
	}
	encoded, _ := json.Marshal(created)
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), created.TokenDigest) {
		t.Fatalf("JSON leaked token material: %s", encoded)
	}
	aliceOwn, err := st.ListOwnDevicePushSubscriptions(ctx, tenant.ID, alice)
	if err != nil || len(aliceOwn) != 1 || aliceOwn[0].Token != "" || aliceOwn[0].TokenDigest != "" {
		t.Fatalf("alice own list = %+v err=%v", aliceOwn, err)
	}
	bobOwn, err := st.ListOwnDevicePushSubscriptions(ctx, tenant.ID, bob)
	if err != nil || len(bobOwn) != 0 {
		t.Fatalf("bob saw alice devices = %+v err=%v", bobOwn, err)
	}

	// Moving one opaque capability to Bob (account switch on the same phone)
	// atomically revokes Alice's old destination.
	if _, err := st.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
		TenantID: tenant.ID, Subject: bob, DeviceID: "bob-ios", SessionID: "session-bob",
		Provider: "expo", Platform: "ios", Token: secret,
	}); err != nil {
		t.Fatal(err)
	}
	aliceOwn, _ = st.ListOwnDevicePushSubscriptions(ctx, tenant.ID, alice)
	bobOwn, _ = st.ListOwnDevicePushSubscriptions(ctx, tenant.ID, bob)
	if len(aliceOwn) != 0 || len(bobOwn) != 1 {
		t.Fatalf("account replacement alice=%+v bob=%+v", aliceOwn, bobOwn)
	}

	// Concurrent rotations for one device converge to one active row. Which
	// token wins is ordering-dependent; duplicate active destinations are not.
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = st.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
				TenantID: tenant.ID, Subject: bob, DeviceID: "bob-ios", SessionID: "session-bob",
				Provider: "expo", Platform: "ios", Token: fmt.Sprintf("ExponentPushToken[race-%s-%d]", stamp, i),
			})
		}(i)
	}
	close(start)
	wg.Wait()
	if errs[0] != nil || errs[1] != nil {
		t.Fatalf("rotation race errors = %v", errs)
	}
	bobOwn, err = st.ListOwnDevicePushSubscriptions(ctx, tenant.ID, bob)
	if err != nil || len(bobOwn) != 1 || bobOwn[0].DeviceID != "bob-ios" {
		t.Fatalf("rotation race result = %+v err=%v", bobOwn, err)
	}

	changed, err := st.RevokeDevicePushSubscription(ctx, tenant.ID, alice, "bob-ios")
	if err != nil || changed {
		t.Fatalf("cross-subject revoke changed=%v err=%v", changed, err)
	}
	changed, err = st.RevokeDevicePushSubscription(ctx, tenant.ID, bob, "bob-ios")
	if err != nil || !changed {
		t.Fatalf("owner revoke changed=%v err=%v", changed, err)
	}
	changed, err = st.RevokeDevicePushSubscription(ctx, tenant.ID, bob, "bob-ios")
	if err != nil || changed {
		t.Fatalf("idempotent revoke changed=%v err=%v", changed, err)
	}

	if _, err := st.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
		TenantID: tenant.ID, Subject: bob, DeviceID: "bob-a", SessionID: "session-bob",
		Provider: "expo", Platform: "android", Token: "ExponentPushToken[all-a-" + stamp + "]",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
		TenantID: tenant.ID, Subject: bob, DeviceID: "bob-b", SessionID: "session-bob",
		Provider: "expo", Platform: "android", Token: "ExponentPushToken[all-b-" + stamp + "]",
	}); err != nil {
		t.Fatal(err)
	}
	count, err := st.RevokeAllDevicePushSubscriptions(ctx, tenant.ID, bob)
	if err != nil || count != 2 {
		t.Fatalf("logout revoke count=%d err=%v", count, err)
	}
	if count, err = st.RevokeAllDevicePushSubscriptions(ctx, tenant.ID, bob); err != nil || count != 0 {
		t.Fatalf("idempotent logout revoke count=%d err=%v", count, err)
	}

	// Nine registrations plus two concurrent final attempts may create only ten
	// active devices. The workspace advisory lock serializes the count-and-write
	// boundary, so exactly one racer is refused instead of both slipping through.
	for i := 0; i < MaxActivePushDevicesPerSubject-1; i++ {
		if _, err := st.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
			TenantID: tenant.ID, Subject: bob, DeviceID: fmt.Sprintf("quota-%d", i), SessionID: "session-bob",
			Provider: "expo", Platform: "android", Token: fmt.Sprintf("ExponentPushToken[quota-%s-%d]", stamp, i),
		}); err != nil {
			t.Fatalf("seed quota device %d: %v", i, err)
		}
	}
	start = make(chan struct{})
	errs = make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = st.UpsertDevicePushSubscription(ctx, DevicePushSubscription{
				TenantID: tenant.ID, Subject: bob, DeviceID: fmt.Sprintf("quota-race-%d", i), SessionID: "session-bob",
				Provider: "expo", Platform: "android", Token: fmt.Sprintf("ExponentPushToken[quota-race-%s-%d]", stamp, i),
			})
		}(i)
	}
	close(start)
	wg.Wait()
	refused := 0
	for _, err := range errs {
		if errors.Is(err, ErrPushDeviceSubjectLimit) {
			refused++
		} else if err != nil {
			t.Fatalf("quota race unexpected error: %v", err)
		}
	}
	if refused != 1 {
		t.Fatalf("quota race errors = %v, want exactly one subject-limit refusal", errs)
	}
}
