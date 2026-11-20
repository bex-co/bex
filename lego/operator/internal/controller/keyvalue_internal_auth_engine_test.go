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
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/kvinternalauth"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestValkeyBehindAPlainPassthroughProxy puts kv-internal-proxy, as it fronts
// a store with internal authentication (w6/m463), before both pinned Valkey
// images started with a free store's own args (BEX_TEST_VALKEY=1):
//   - the proxy writes nothing of its own, so a client without the password
//     reads Valkey's NOAUTH and one that sends AUTH is served;
//   - the plan's 50 authenticated clients are admitted and the 51st reads
//     Valkey's refusal from the proxy;
//   - a client the server itself refuses, past its maxclients, reads the
//     server's own refusal through the proxy.
func TestValkeyBehindAPlainPassthroughProxy(t *testing.T) {
	if os.Getenv("BEX_TEST_VALKEY") != "1" {
		t.Skip("set BEX_TEST_VALKEY=1 to run pinned Valkey Docker internal-auth proxy tests")
	}
	for _, major := range []string{"7", "8"} {
		t.Run(major, func(t *testing.T) {
			e := startTLSSwitchEngine(t, major)
			free, _ := resolveKVPlan(appv1alpha1.KeyValueSpec{Plan: "free"})
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			serve := func(budget int) string {
				t.Helper()
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				go func() {
					_ = kvinternalauth.Serve(ctx, listener, kvinternalauth.Config{
						Upstream: e.hostPort("6379"), Password: fixturePassword,
						Mode: kvinternalauth.PlainPassthrough, Budget: kvinternalauth.NewBudget(budget),
					})
				}()
				return listener.Addr().String()
			}
			var held []net.Conn
			t.Cleanup(func() {
				for _, c := range held {
					_ = c.Close()
				}
			})
			// w9/m521: the client's own AUTH as the store's id user reaches
			// Valkey unchanged, as an internal-auth store's strings send it.
			named, replies := dialAuthPingAs(t, serve(1), "tls-engine")
			_ = named.Close()
			if replies != "+OK +PONG" {
				t.Fatalf("a client authenticating as the id user read %q", replies)
			}
			proxy := serve(int(free.ConnectionLimit))

			c, reply := dialPing(t, proxy)
			_ = c.Close()
			if reply != "-NOAUTH Authentication required." {
				t.Fatalf("a client without the password read %q, want Valkey's NOAUTH", reply)
			}
			for i := range int(free.ConnectionLimit) {
				c, replies := dialAuthPing(t, proxy)
				held = append(held, c)
				if replies != "+OK +PONG" {
					t.Fatalf("authenticated client %d of %d read %q", i+1, free.ConnectionLimit, replies)
				}
			}
			c, reply = dialPing(t, proxy)
			_ = c.Close()
			if reply != "-ERR max number of clients reached" {
				t.Fatalf("client %d read %q, want Valkey's refusal", free.ConnectionLimit+1, reply)
			}

			// A front whose budget is past the server's maxclients meets the
			// server's own refusal, and forwards it.
			wide := serve(1000)
			for admitted := 0; ; admitted++ {
				c, reply := dialPing(t, wide)
				held = append(held, c)
				if reply == "-NOAUTH Authentication required." {
					if admitted >= kvPlatformClientReserve {
						t.Fatalf("the server admitted %d past the plan's %d, more than its reserve", admitted+1, free.ConnectionLimit)
					}
					continue
				}
				if reply != "-ERR max number of clients reached" {
					t.Fatalf("past the server's maxclients a client read %q", reply)
				}
				break
			}
		})
	}
}

// dialAuthPing opens a client that authenticates as the default user and
// pings, and returns it with its two replies.
func dialAuthPing(t *testing.T, addr string) (net.Conn, string) {
	t.Helper()
	return dialAuthPingAs(t, addr, kvDefaultUser)
}

// dialAuthPingAs is dialAuthPing as user.
func dialAuthPingAs(t *testing.T, addr, user string) (net.Conn, string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprintf(c, "AUTH %s %s\r\nPING\r\n", user, fixturePassword); err != nil {
		return c, "write: " + err.Error()
	}
	r := bufio.NewReader(c)
	var replies [2]string
	for i := range replies {
		line, err := r.ReadString('\n')
		if err != nil {
			return c, "read: " + err.Error()
		}
		replies[i] = line[:len(line)-2]
	}
	return c, replies[0] + " " + replies[1]
}
