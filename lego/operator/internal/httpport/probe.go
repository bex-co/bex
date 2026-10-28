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

// Package httpport identifies a pod's plain HTTP listener without following
// redirects or interpreting response bodies. Image EXPOSE metadata can include
// binary protocols below the public web application's HTTP port.
package httpport

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxPorts      = 75
	workers       = 8
	portTimeout   = time.Second
	probeTimeout  = 10 * time.Second
	maxStatusLine = 4096
)

// Probe returns the lowest candidate port answering a parseable HTTP/1.x
// status line, regardless of the status code. Zero with no error means none
// spoke HTTP. Each connect/request/status-line exchange is bounded to a second;
// the complete scan is bounded to ten seconds and at most 75 candidate ports.
// It sends only GET /, Host and Connection: close directly to the numeric pod
// IP: ambient HTTP proxies, credentials and platform HTTP clients are unused.
func Probe(ctx context.Context, ip, host string, ports []int32) (int32, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if len(ports) == 0 {
		return 0, nil
	}
	if len(ports) > maxPorts {
		return 0, errors.New("httpport: too many candidate ports")
	}
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return 0, errors.New("httpport: a numeric pod IP is required")
	}
	if host == "" {
		host = address.String()
		if address.Is6() {
			host = "[" + host + "]"
		}
	}
	if len(host) > 1024 {
		return 0, errors.New("httpport: invalid Host header")
	}
	for _, c := range host {
		if c <= 0x20 || c >= 0x7f {
			return 0, errors.New("httpport: invalid Host header")
		}
	}
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return 0, errors.New("httpport: candidate port is outside the TCP range")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	jobs := make(chan int32)
	found := make(chan int32, len(ports))
	var tasks sync.WaitGroup
	for range min(workers, len(ports)) {
		tasks.Go(func() {
			for port := range jobs {
				if speaksHTTP(ctx, address.String(), host, port) {
					found <- port
				}
			}
		})
	}
feed:
	for _, port := range ports {
		select {
		case jobs <- port:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	tasks.Wait()
	close(found)
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var lowest int32
	for port := range found {
		if lowest == 0 || port < lowest {
			lowest = port
		}
	}
	return lowest, nil
}

func speaksHTTP(ctx context.Context, ip, host string, port int32) bool {
	ctx, cancel := context.WithTimeout(ctx, portTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(int(port))))
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return false
	}
	if _, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host); err != nil {
		return false
	}
	// ReadSlice refuses an overlong line rather than allocating for a tenant's
	// response. No headers or body are needed to identify an HTTP listener.
	line, err := bufio.NewReaderSize(conn, maxStatusLine).ReadSlice('\n')
	if err != nil || !strings.HasSuffix(string(line), "\r\n") {
		return false
	}
	text := strings.TrimSuffix(string(line), "\r\n")
	protocol, status, ok := strings.Cut(text, " ")
	if !ok {
		return false
	}
	major, _, ok := http.ParseHTTPVersion(protocol)
	if !ok || major != 1 {
		return false
	}
	// Match the standard parser's status-code grammar without asking it to
	// parse an unbounded header block or create a response body reader.
	code, _, _ := strings.Cut(strings.TrimLeft(status, " "), " ")
	if len(code) != 3 {
		return false
	}
	value, err := strconv.Atoi(code)
	return err == nil && value >= 100 && value <= 999
}
