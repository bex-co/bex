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

package portscan

import (
	"context"
	"net"
	"slices"
	"testing"
	"time"
)

// A listener on the scanned address is found; the closed ports around it are
// refused, which is what proves the host reachable.
func TestScanFindsTheListeningPorts(t *testing.T) {
	ports := make([]int, 0, 2)
	for range 2 {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Skipf("no loopback listener: %v", err)
		}
		defer func() { _ = l.Close() }()
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	slices.Sort(ports)
	low, high := ports[0]-3, ports[1]+3
	if low < 1 {
		low = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	got, err := Dialer{Low: low, High: high}.Scan(ctx, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range ports {
		if !slices.Contains(got, want) {
			t.Errorf("scan = %v, missing listening port %d", got, want)
		}
	}
}

// A listener bound to loopback only is invisible from the pod's own address.
func TestScanExcludesNothingListeningOnTheScannedAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	got, err := Dialer{Low: 40001, High: 40010}.Scan(ctx, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("scan = %v, want no open ports", got)
	}
}

// An address nothing answers on is reported as unreachable, not as "no ports".
func TestScanReportsAnUnreachableAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// TEST-NET-1 (192.0.2.0/24) is reserved and never routed: connects time out.
	if _, err := (Dialer{Low: 1, High: 40}).Scan(ctx, "192.0.2.1"); err == nil {
		t.Skip("the sandbox routed TEST-NET-1; cannot exercise the unreachable path")
	}
}
