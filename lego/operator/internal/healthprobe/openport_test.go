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

package healthprobe

import (
	"net"
	"slices"
	"strings"
	"testing"
	"time"
)

// Only LISTEN sockets count, on any address; established connections and the
// header do not (w4/m350).
func TestListeningPortsReadsOnlyListenSockets(t *testing.T) {
	table := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:2710 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1 1 0000000000000000 100 0 0 10 0
   1: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 2 1 0000000000000000 100 0 0 10 0
   2: 0A000005:2710 0A000009:D431 01 00000000:00000000 00:00000000 00000000  1000        0 3 1 0000000000000000 20 4 30 10 -1
   3: 00000000000000000000000000000000:4652 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 4 1 0000000000000000 100 0 0 10 0
`
	got := listeningPorts(strings.NewReader(table))
	if want := []int{10000, 8080, 18002}; !slices.Equal(got, want) {
		t.Fatalf("listening = %v, want %v", got, want)
	}
}

func listen(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l.Addr().(*net.TCPAddr).Port
}

// closedPort is a port nothing listens on: one a listener just released.
func closedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func TestOpenPortPassesOnAnyNonReservedListener(t *testing.T) {
	open, closed := listen(t), closedPort(t)
	got, err := OpenPort(t.Context(), "127.0.0.1", []int{closed, open}, nil, 2*time.Second)
	if err != nil || got != open {
		t.Fatalf("OpenPort = %d, %v; want %d", got, err, open)
	}
}

func TestOpenPortFailsWithNoListener(t *testing.T) {
	if got, err := OpenPort(t.Context(), "127.0.0.1", nil, nil, time.Second); err == nil {
		t.Fatalf("OpenPort with nothing listening = %d, want an error", got)
	}
	// A socket table entry that refuses the pod address (a loopback-only
	// listener seen from the pod IP) does not pass either.
	if got, err := OpenPort(t.Context(), "127.0.0.1", []int{closedPort(t)}, nil, time.Second); err == nil {
		t.Fatalf("OpenPort on a refused port = %d, want an error", got)
	}
}

func TestOpenPortIgnoresReservedPorts(t *testing.T) {
	sidecar := listen(t)
	if got, err := OpenPort(t.Context(), "127.0.0.1", []int{sidecar}, []int{sidecar}, time.Second); err == nil {
		t.Fatalf("OpenPort passed on a skipped sidecar port %d", got)
	}
}
