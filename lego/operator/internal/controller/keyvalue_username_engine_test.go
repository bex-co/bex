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

package controller

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestValkeyRuntimeUsername drives the operator's real ACL SETUSER on both
// pinned Valkey images, started with the pod's own command (BEX_TEST_VALKEY=1,
// w9/m521 t004):
//   - a server started from a template held since before the id user does not
//     know it; the runtime add gives it exactly the default user's permissions
//     and password, in the same process (same run_id), and a Persistence Off
//     key survives;
//   - the id user may not CONFIG SET or ACL SETUSER, and AUTH works on the
//     plain listener and on the TLS listener the SNI edge passes through to;
//   - the default user keeps working, and the add is idempotent;
//   - a restart from the held template comes back without the user, and the
//     add brings it back;
//   - a server started from a new template knows the user from the start.
func TestValkeyRuntimeUsername(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY") != "1" {
		t.Skip("set BEX_TEST_VALKEY=1 to run pinned Valkey Docker ACL user tests")
	}
	const user = "tls-engine" // the engine store's id
	revision := appv1alpha1.KeyValueCredentialRevision([]byte(fixturePassword))
	for _, major := range []string{"7", "8"} {
		t.Run(major, func(t *testing.T) {
			e := startValkeyEngine(t, major, true)
			if err := e.tenant.Set(e.ctx, "kept", "yes", 0).Err(); err != nil {
				t.Fatal(err)
			}
			birth := e.runID()
			asUser := func(addr string, config *tls.Config) *redis.Client {
				c := redis.NewClient(&redis.Options{Addr: addr, Username: user, Password: fixturePassword, Protocol: 2,
					MaxRetries: -1, DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, TLSConfig: config})
				t.Cleanup(func() { _ = c.Close() })
				return c
			}
			if err := asUser(e.hostPort("6379"), nil).Ping(e.ctx).Err(); err == nil || !strings.Contains(err.Error(), "WRONGPASS") {
				t.Fatalf("a held server knew the id user before the add: %v", err)
			}

			add := func(step string) {
				t.Helper()
				if err := setValkeyUser(e.ctx, e.platform, user, revision); err != nil {
					t.Fatalf("%s: %v", step, err)
				}
			}
			add("the add")
			add("a repeated add")
			id := asUser(e.hostPort("6379"), nil)
			if got, err := id.Get(e.ctx, "kept").Result(); err != nil || got != "yes" {
				t.Fatalf("the id user reads %q, %v; want the tenant's keyspace", got, err)
			}
			if err := id.ConfigSet(e.ctx, "maxmemory", "1").Err(); err == nil || !strings.Contains(err.Error(), "NOPERM") {
				t.Fatalf("the id user ran CONFIG SET: %v", err)
			}
			if err := id.Do(e.ctx, "ACL", "SETUSER", "x", "on").Err(); err == nil || !strings.Contains(err.Error(), "NOPERM") {
				t.Fatalf("the id user ran ACL SETUSER: %v", err)
			}
			if err := id.ConfigGet(e.ctx, "maxmemory").Err(); err != nil {
				t.Fatalf("the id user lost CONFIG GET (w4/191): %v", err)
			}
			if err := redis.NewClient(&redis.Options{Addr: e.hostPort("6379"), Username: user, Password: "wrong", Protocol: 2, MaxRetries: -1}).Ping(e.ctx).Err(); err == nil {
				t.Fatal("the id user accepted a wrong password")
			}
			for _, field := range []string{"flags", "commands", "keys", "channels"} {
				def, err := e.platform.Do(e.ctx, "ACL", "GETUSER", kvDefaultUser).Slice()
				if err != nil {
					t.Fatal(err)
				}
				got, err := e.platform.Do(e.ctx, "ACL", "GETUSER", user).Slice()
				if err != nil {
					t.Fatal(err)
				}
				if a, b := aclField(def, field), aclField(got, field); a != b {
					t.Fatalf("ACL %s: default %s, id user %s", field, a, b)
				}
			}
			e.expectKey("the add")
			if e.runID() != birth {
				t.Fatal("the add restarted the process")
			}

			// The SNI edge passes TLS through to Valkey's own listener.
			e.projectCertificate()
			e.switchTo("enable", true, true, false)
			overTLS := asUser(e.tlsAddr, &tls.Config{RootCAs: e.roots, ServerName: "tls-engine.kv.example.test", MinVersion: tls.VersionTLS12})
			if err := overTLS.Ping(e.ctx).Err(); err != nil {
				t.Fatalf("AUTH as the id user over TLS: %v", err)
			}

			e.restart()
			if err := asUser(e.hostPort("6379"), nil).Ping(e.ctx).Err(); err == nil {
				t.Fatal("a restart from the held template kept the runtime user")
			}
			add("the add after a restart")
			if err := asUser(e.hostPort("6379"), nil).Ping(e.ctx).Err(); err != nil {
				t.Fatalf("after the add again: %v", err)
			}

			fresh := startValkeyEngine(t, major, false)
			if err := asUser(fresh.hostPort("6379"), nil).Ping(fresh.ctx).Err(); err != nil {
				t.Fatalf("a server from a new template does not know the id user: %v", err)
			}
		})
	}
}

// aclField is one field of an ACL GETUSER reply, printed for comparison.
func aclField(reply []any, field string) string {
	for i := 0; i+1 < len(reply); i += 2 {
		if reply[i] == field {
			return fmt.Sprint(reply[i+1])
		}
	}
	return "<missing>"
}
