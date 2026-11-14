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
	"errors"
	"net"
	"slices"
	"strconv"
	"sync"
	"syscall"
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
// Every probe runs into its own dial timeout, as on a path that drops packets:
// a timeout is no refusal, so it must not read as "nothing listening". The
// injected dialer does not depend on the test host's routing to TEST-NET-1.
func TestScanReportsAnUnreachableAddress(t *testing.T) {
	got, err := Dialer{Low: 1, High: 40, dial: droppingDial()}.Scan(context.Background(), "192.0.2.1")
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("err = %v, want ErrUnreachable", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want no open ports", got)
	}
}

// droppingDial answers on the open ports and, like a path that drops probes
// to closed ports, never answers anywhere else until the dial's own timeout.
func droppingDial(open ...int) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, address string) (net.Conn, error) {
		_, portText, _ := net.SplitHostPort(address)
		port, _ := strconv.Atoi(portText)
		if slices.Contains(open, port) {
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
}

// Closed ports that drop probes make a full sweep far longer than any
// detection budget. The listener on a likely HTTP port is still found, and
// the ports found before the deadline are returned, not discarded (w9/072).
func TestScanReturnsWhatItFoundWhenTheDeadlineHits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	got, err := Dialer{dial: droppingDial(3000, 60000)}.Scan(ctx, "10.0.0.1")
	if !errors.Is(err, ErrPartial) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want ErrPartial wrapping the deadline", err)
	}
	if !slices.Equal(got, []int{3000}) {
		t.Fatalf("got %v, want [3000]: the likely port answers, the late one is never reached", got)
	}
}

func TestScanTriesLikelyHTTPPortsFirst(t *testing.T) {
	var mu sync.Mutex
	var order []int
	dial := func(_ context.Context, address string) (net.Conn, error) {
		_, portText, _ := net.SplitHostPort(address)
		port, _ := strconv.Atoi(portText)
		mu.Lock()
		order = append(order, port)
		mu.Unlock()
		return nil, syscall.ECONNREFUSED
	}
	// One worker makes the dial order the feed order.
	if _, err := scan(context.Background(), "10.0.0.1", 1, Dialer{Low: 1, High: 12000}.order(), dial); err != nil {
		t.Fatal(err)
	}
	want := []int{80, 8080, 3000, 5000, 8000, 8888, 10000, 4000, 9000, 443, 5173, 8443, 1, 2}
	if !slices.Equal(order[:len(want)], want) {
		t.Fatalf("first dials = %v, want %v", order[:len(want)], want)
	}
	if len(order) != 12000 {
		t.Fatalf("dialed %d ports, want every port in range exactly once", len(order))
	}
}
