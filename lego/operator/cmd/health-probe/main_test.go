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

package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeDrain points the checks at loopback and records the drain's sleep
// instead of taking it.
func fakeDrain(t *testing.T) *[]time.Duration {
	t.Helper()
	slept := &[]time.Duration{}
	oldAddrs, oldSleep := podAddrs, drainSleep
	podAddrs = func() []string { return []string{"127.0.0.1"} }
	drainSleep = func(d time.Duration) { *slept = append(*slept, d) }
	t.Cleanup(func() { podAddrs, drainSleep = oldAddrs, oldSleep })
	return slept
}

// serveStatus serves status on loopback and returns its port.
func serveStatus(t *testing.T, status int, delay time.Duration) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return strconv.Itoa(srv.Listener.Addr().(*net.TCPAddr).Port)
}

// closedPort is a loopback port nothing listens on.
func closedPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	_ = l.Close()
	return port
}

// w4/m353: the preStop drain waits only for an instance that passes its own
// health check. A failing one — out of rotation, or being restarted by its
// liveness probe — returns at once, and it has no failure exit, so the hook
// can never hold up termination.
func TestDrainWaitsOnlyForAPassingInstance(t *testing.T) {
	healthy := serveStatus(t, http.StatusOK, 0)
	failing := serveStatus(t, http.StatusInternalServerError, 0)
	slow := serveStatus(t, http.StatusOK, 2*time.Second)
	for _, tc := range []struct {
		name      string
		args      []string
		wantSleep bool
	}{
		{"http check passes", []string{"--seconds", "60", "check", "--port", healthy, "--path", "/health"}, true},
		{"tcp check passes", []string{"--seconds", "60", "tcp", "--port", healthy}, true},
		{"http check answers 500", []string{"--seconds", "60", "check", "--port", failing, "--path", "/health"}, false},
		{"http check refused", []string{"--seconds", "60", "check", "--port", closedPort(t), "--path", "/health"}, false},
		{"http check times out", []string{"--seconds", "60", "check", "--port", slow, "--timeout", "100ms"}, false},
		{"tcp check refused", []string{"--seconds", "60", "tcp", "--port", closedPort(t)}, false},
		{"no seconds", []string{"tcp", "--port", healthy}, false},
		{"no check", []string{"--seconds", "60"}, false},
		{"unknown check", []string{"--seconds", "60", "ping", "--port", healthy}, false},
		{"bad check flag", []string{"--seconds", "60", "check", "--port", "x"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			slept := fakeDrain(t)
			start := time.Now()
			drain(tc.args)
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("drain took %s outside its sleep, want it at once", elapsed)
			}
			if tc.wantSleep {
				if len(*slept) != 1 || (*slept)[0] != 60*time.Second {
					t.Fatalf("slept %v, want the 60 s drain", *slept)
				}
			} else if len(*slept) != 0 {
				t.Fatalf("slept %v, want no drain", *slept)
			}
		})
	}
}

// w6/m634: a failed check prints exactly one line, in Render's words, and a
// check slower than its 5 s timeout gives up soon enough to print it before the
// kubelet's own 5 s probe timeout kills the command and keeps no output.
func TestCheckPrintsOneLineInRendersWords(t *testing.T) {
	fakeDrain(t)
	unavailable := serveStatus(t, http.StatusServiceUnavailable, 0)
	slow := serveStatus(t, http.StatusOK, 6*time.Second)
	refused := closedPort(t)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"503", []string{"check", "--port", unavailable, "--path", "/health", "--timeout", "5s"},
			"HTTP health check failed with status code 503"},
		{"a 6-second answer", []string{"check", "--port", slow, "--path", "/health", "--timeout", "5s"},
			"HTTP health check failed (timed out after 5 seconds)"},
		{"a closed port", []string{"check", "--port", refused, "--path", "/health", "--timeout", "5s"},
			"HTTP health check failed: dial tcp 127.0.0.1:" + refused + ": connect: connection refused"},
		{"a closed port by TCP", []string{"tcp", "--port", refused, "--timeout", "5s"},
			"TCP health check failed: dial tcp 127.0.0.1:" + refused + ": connect: connection refused"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			old := failures
			failures = &out
			t.Cleanup(func() { failures = old })
			start := time.Now()
			if code := runCheck(tc.args); code != 1 {
				t.Fatalf("exit %d, want 1", code)
			}
			if elapsed := time.Since(start); elapsed >= 5*time.Second {
				t.Fatalf("the check took %s, not short of the kubelet's 5 s", elapsed)
			}
			if got := out.String(); got != tc.want+"\n" {
				t.Fatalf("printed %q, want the one line %q", got, tc.want)
			}
		})
	}
}
