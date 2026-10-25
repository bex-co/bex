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

// Scanner scans a pod IP. The interface lets the controller's tests supply a
// canned answer.
type Scanner interface {
	Scan(ctx context.Context, ip string) ([]int, error)
}

// Dialer connects to every port of an address. Zero value scans 1..65535.
type Dialer struct {
	// Low and High bound the scan (inclusive); zero means 1 and 65535.
	Low, High int
}

// ScanPorts connects to the listed ports of ip and returns the ones that
// accepted, sorted — the bounded first phase of a detection scan, so a
// dropped-packet path gets a verdict on the common HTTP ports well inside
// the caller's timeout instead of only after a full sweep (w4/393).
func (d Dialer) ScanPorts(ctx context.Context, ip string, list []int) ([]int, error) {
	var open []int
	var mu sync.Mutex
	var sawRefusal bool
	ports := make(chan int)
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			var dialer net.Dialer
			for port := range ports {
				dctx, cancel := context.WithTimeout(ctx, dialTimeout)
				conn, err := dialer.DialContext(dctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
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
	for _, port := range list {
		select {
		case ports <- port:
		case <-ctx.Done():
			break feed
		}
	}
	close(ports)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(open) == 0 && !sawRefusal {
		return nil, ErrUnreachable
	}
	sort.Ints(open)
	return open, nil
}

// Scan connects to each port of ip and returns the ones that accepted, sorted.
// It first proves the pod is reachable: a port nobody listens on must answer
// "connection refused"; a timeout there means a network policy or a dead pod is
// swallowing the probes, and the scan would only report silence.
func (d Dialer) Scan(ctx context.Context, ip string) ([]int, error) {
	low, high := d.Low, d.High
	if low <= 0 {
		low = 1
	}
	if high <= 0 || high > maxPort {
		high = maxPort
	}
	var open []int
	var mu sync.Mutex
	ports := make(chan int)
	var wg sync.WaitGroup
	var sawRefusal bool
	for range workers {
		wg.Go(func() {
			var dialer net.Dialer
			for port := range ports {
				dctx, cancel := context.WithTimeout(ctx, dialTimeout)
				conn, err := dialer.DialContext(dctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
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
	for port := low; port <= high; port++ {
		select {
		case ports <- port:
		case <-ctx.Done():
			break feed
		}
	}
	close(ports)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(open) == 0 && !sawRefusal {
		return nil, ErrUnreachable
	}
	sort.Ints(open)
	return open, nil
}

func isRefused(err error) bool { return errors.Is(err, syscall.ECONNREFUSED) }
