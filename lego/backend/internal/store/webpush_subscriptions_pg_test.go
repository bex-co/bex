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
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPGStoreWebPushSubscriptions(t *testing.T) {
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
	alice := "webpush-alice-" + stamp
	bob := "webpush-bob-" + stamp
	tenant, err := st.CreateWorkspace(ctx, "webpush-test-"+stamp, PlanHobby, alice)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	if err := st.AddMember(ctx, bob, tenant.ID, "viewer"); err != nil {
		t.Fatal(err)
	}

	secret := "https://fcm.googleapis.com/fcm/send/pg-secret-" + stamp
	created, err := st.UpsertWebPushSubscription(ctx, WebPushSubscription{
		TenantID: tenant.ID, Subject: alice, BrowserID: "wp-alice",
		Endpoint: secret, P256dh: "BNpg", Auth: "authsecret16bytes",
	})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(created)
	if strings.Contains(string(encoded), "pg-secret") || strings.Contains(string(encoded), created.EndpointDigest) {
		t.Fatalf("JSON leaked endpoint material: %s", encoded)
	}
	own, err := st.ListOwnWebPushSubscriptions(ctx, tenant.ID, alice)
	if err != nil || len(own) != 1 || own[0].Endpoint != "" || own[0].P256dh != "" || own[0].Auth != "" {
		t.Fatalf("own list = %+v err=%v", own, err)
	}

	if _, err := st.UpsertWebPushSubscription(ctx, WebPushSubscription{
		TenantID: tenant.ID, Subject: bob, BrowserID: "wp-bob",
		Endpoint: secret, P256dh: "BNpg", Auth: "authsecret16bytes",
	}); err != nil {
		t.Fatal(err)
	}
	aliceOwn, err := st.ListOwnWebPushSubscriptions(ctx, tenant.ID, alice)
	if err != nil || len(aliceOwn) != 0 {
		t.Fatalf("moved endpoint still on alice: %+v", aliceOwn)
	}

	active, err := st.ListActivePushSubscriptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, d := range active {
		if d.DeviceID == "wp-bob" && d.Provider == "webpush" && d.Token == secret && d.P256dh == "BNpg" {
			saw = true
		}
	}
	if !saw {
		t.Fatal("active destinations missing the webpush row")
	}
	changed, err := st.RevokeWebPushSubscription(ctx, tenant.ID, bob, "wp-bob")
	if err != nil || !changed {
		t.Fatalf("revoke = (%v, %v)", changed, err)
	}
}
