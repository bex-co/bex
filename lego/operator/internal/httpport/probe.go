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
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxPorts = 75
	workers  = 8
	// portTimeout bounds one connect/request/status-line exchange. A cold
	// first request (RabbitMQ's management UI renders index.html) can take
	// more than a second on a small CPU share.
	portTimeout   = 3 * time.Second
	probeTimeout  = 10 * time.Second
	maxStatusLine = 4096
)

// Result is one bounded scan. Port is the lowest candidate that answered an
// HTTP/1.x status line, zero when none did. Unanswered is the lowest candidate
// below Port (any candidate when Port is zero) whose HTTP answer is still
// unknown: it accepted the connection but sent no status line within its
// budget, or the scan's own bound ended before it was tried. A caller that
// wants the lowest HTTP listener retries before committing a higher Port.
type Result struct {
	Port       int32
	Unanswered int32
}

type outcome uint8

const (
	unknown outcome = iota // accepted but silent, or never tried
	notHTTP                // refused, closed, or answered something else
	speaksHTTP
)

// Probe scans candidate ports for a parseable HTTP/1.x status line, regardless
// of the status code. Ports are tried lowest first; each exchange is bounded
// by portTimeout and the complete scan by probeTimeout and at most 75 ports.
// It sends only GET /, Host and Connection: close directly to the numeric pod
// IP: ambient HTTP proxies, credentials and platform HTTP clients are unused.
func Probe(parent context.Context, ip, host string, ports []int32) (Result, error) {
	if err := parent.Err(); err != nil {
		return Result{}, err
	}
	if len(ports) == 0 {
		return Result{}, nil
	}
	if len(ports) > maxPorts {
		return Result{}, errors.New("httpport: too many candidate ports")
	}
	address, err := netip.ParseAddr(ip)
	if err != nil {
		return Result{}, errors.New("httpport: a numeric pod IP is required")
	}
	if host == "" {
		host = address.String()
		if address.Is6() {
			host = "[" + host + "]"
		}
	}
	if len(host) > 1024 {
		return Result{}, errors.New("httpport: invalid Host header")
	}
	for _, c := range host {
		if c <= 0x20 || c >= 0x7f {
			return Result{}, errors.New("httpport: invalid Host header")
		}
	}
	for _, port := range ports {
		if port < 1 || port > 65535 {
			return Result{}, errors.New("httpport: candidate port is outside the TCP range")
		}
	}
	ports = slices.Clone(ports)
	slices.Sort(ports)
	ctx, cancel := context.WithTimeout(parent, probeTimeout)
	defer cancel()
	outcomes := make([]outcome, len(ports))
	jobs := make(chan int)
	var tasks sync.WaitGroup
	for range min(workers, len(ports)) {
		tasks.Go(func() {
			for i := range jobs {
				outcomes[i] = probePort(ctx, address.String(), host, ports[i])
			}
		})
	}
feed:
	for i := range ports {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	tasks.Wait()
	if err := parent.Err(); err != nil {
		return Result{}, err
	}
	// Ports are sorted, so the first decisive answer is the lowest HTTP port
	// and any unknown port before it is the lowest still unanswered.
	var result Result
	for i, port := range ports {
		switch outcomes[i] {
		case speaksHTTP:
			result.Port = port
			return result, nil
		case unknown:
			if result.Unanswered == 0 {
				result.Unanswered = port
			}
		}
	}
	return result, nil
}

func probePort(scan context.Context, ip, host string, port int32) outcome {
	ctx, cancel := context.WithTimeout(scan, portTimeout)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(int(port))))
	if err != nil {
		if scan.Err() != nil {
			return unknown // the scan's bound ended before this port was reached
		}
		return notHTTP // refused or unreachable: no listener to wait for
	}
	defer func() { _ = conn.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return silence(ctx, err)
	}
	if _, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host); err != nil {
		return silence(ctx, err)
	}
	// ReadSlice refuses an overlong line rather than allocating for a tenant's
	// response. No headers or body are needed to identify an HTTP listener.
	line, err := bufio.NewReaderSize(conn, maxStatusLine).ReadSlice('\n')
	if err != nil {
		return silence(ctx, err)
	}
	if !strings.HasSuffix(string(line), "\r\n") {
		return notHTTP
	}
	text := strings.TrimSuffix(string(line), "\r\n")
	protocol, status, ok := strings.Cut(text, " ")
	if !ok {
		return notHTTP
	}
	major, _, ok := http.ParseHTTPVersion(protocol)
	if !ok || major != 1 {
		return notHTTP
	}
	// Match the standard parser's status-code grammar without asking it to
	// parse an unbounded header block or create a response body reader.
	code, _, _ := strings.Cut(strings.TrimLeft(status, " "), " ")
	if len(code) != 3 {
		return notHTTP
	}
	if value, err := strconv.Atoi(code); err != nil || value < 100 || value > 999 {
		return notHTTP
	}
	return speaksHTTP
}

// silence classifies an accepted connection's I/O failure. Running out of time
// leaves the answer unknown; a reset, close or oversized line is a definite
// non-HTTP answer.
func silence(ctx context.Context, err error) outcome {
	var netErr net.Error
	if ctx.Err() != nil || (errors.As(err, &netErr) && netErr.Timeout()) {
		return unknown
	}
	return notHTTP
}
