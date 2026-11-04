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

// Package portscan finds the TCP ports a pod accepts connections on, from
// outside it. Render "scans the running container for an HTTP listener on
// 0.0.0.0" when a web service does not bind PORT (w8/177); connecting to the pod
// IP gives exactly that answer: a process bound only to loopback never accepts
// a connection on the pod's address, so it is not found.
package portscan

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	// dialTimeout bounds one connect. A closed port answers with a reset at
	// once on a reachable pod, so this only ever waits on an unreachable one.
	dialTimeout = 400 * time.Millisecond
	workers     = 512
	maxPort     = 65535
)

// ErrUnreachable means the pod did not answer at all (a closed port would have
// refused the connection), so no scan result can be trusted.
var ErrUnreachable = errors.New("portscan: the pod did not answer")

// ErrPartial means the scan's deadline passed before every port was tried.
// The open ports found so far are still returned beside it: likely HTTP ports
// are tried first, so a partial answer usually already holds the listener.
var ErrPartial = errors.New("portscan: deadline reached before the scan finished")

// LikelyHTTPPorts are tried before the ascending sweep: where HTTP servers
// actually bind, so a listener is found early even when closed ports drop
// probes instead of refusing them and the full sweep cannot finish in time.
var LikelyHTTPPorts = []int{80, 8080, 3000, 5000, 8000, 8888, 10000, 4000, 9000, 443, 5173, 8443}

// Scanner scans a pod IP. The interface lets the controller's tests supply a
// canned answer.
type Scanner interface {
	Scan(ctx context.Context, ip string) ([]int, error)
}

// Dialer connects to every port of an address. Zero value scans 1..65535.
type Dialer struct {
	// Low and High bound the scan (inclusive); zero means 1 and 65535.
	Low, High int
	// dial replaces the TCP connect in tests; nil uses net.Dialer.
	dial func(ctx context.Context, address string) (net.Conn, error)
}

func (d Dialer) dialer() func(ctx context.Context, address string) (net.Conn, error) {
	if d.dial != nil {
		return d.dial
	}
	var dialer net.Dialer
	return func(ctx context.Context, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", address)
	}
}

// ScanPorts connects to the listed ports of ip and returns the ones that
// accepted, sorted — the bounded first phase of a detection scan, so a
// dropped-packet path gets a verdict on the common HTTP ports well inside
// the caller's timeout instead of only after a full sweep (w4/393).
func (d Dialer) ScanPorts(ctx context.Context, ip string, list []int) ([]int, error) {
	return scan(ctx, ip, 64, list, d.dialer())
}

// Scan connects to each port of ip and returns the ones that accepted, sorted.
// LikelyHTTPPorts inside the range go first, then the rest ascending. A pod
// that neither accepts nor refuses anything is ErrUnreachable: a network
// policy or a dead pod is swallowing the probes, and silence proves nothing.
func (d Dialer) Scan(ctx context.Context, ip string) ([]int, error) {
	return scan(ctx, ip, workers, d.order(), d.dialer())
}

// order is the sweep order: LikelyHTTPPorts inside the range, then the rest
// ascending.
func (d Dialer) order() []int {
	low, high := d.Low, d.High
	if low <= 0 {
		low = 1
	}
	if high <= 0 || high > maxPort {
		high = maxPort
	}
	order := make([]int, 0, high-low+1)
	first := make(map[int]bool, len(LikelyHTTPPorts))
	for _, port := range LikelyHTTPPorts {
		if port >= low && port <= high && !first[port] {
			first[port] = true
			order = append(order, port)
		}
	}
	for port := low; port <= high; port++ {
		if !first[port] {
			order = append(order, port)
		}
	}
	return order
}

// scan dials ports in order with n workers. When ctx ends first it returns
// the ports found so far with ErrPartial (wrapping the context error) rather
// than discarding them.
func scan(ctx context.Context, ip string, n int, order []int, dial func(context.Context, string) (net.Conn, error)) ([]int, error) {
	var open []int
	var mu sync.Mutex
	var sawRefusal bool
	ports := make(chan int)
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			for port := range ports {
				dctx, cancel := context.WithTimeout(ctx, dialTimeout)
				conn, err := dial(dctx, net.JoinHostPort(ip, strconv.Itoa(port)))
				cancel()
				switch {
				case err == nil:
					_ = conn.Close()
					mu.Lock()
					open = append(open, port)
					mu.Unlock()
				case isRefused(err):
					mu.Lock()
					sawRefusal = true
					mu.Unlock()
				}
			}
		})
	}
feed:
	for _, port := range order {
		select {
		case ports <- port:
		case <-ctx.Done():
			break feed
		}
	}
	close(ports)
	wg.Wait()
	sort.Ints(open)
	if err := ctx.Err(); err != nil {
		if len(open) > 0 {
			return open, fmt.Errorf("%w: %w", ErrPartial, err)
		}
		return nil, err
	}
	if len(open) == 0 && !sawRefusal {
		return nil, ErrUnreachable
	}
	return open, nil
}

func isRefused(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }
