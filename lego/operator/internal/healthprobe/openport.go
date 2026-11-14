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
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The any-open-port check (w4/m350) is Render's default health check: "By
// default, health checks are TCP socket probes to one of your service's open
// ports." The kubelet's TCP probe names one fixed port, so a web image whose
// HTTP port was still being discovered had its probe, and with it its pod
// template, rewritten once discovery chose the port: a second ReplicaSet on
// its first deploy. This check reads the pod's own listening sockets instead
// and passes when the pod address accepts a connection on any of them.

// procNetTCP are the kernel's socket tables for the pod's network namespace,
// which every container of the pod shares.
var procNetTCP = []string{"/proc/net/tcp", "/proc/net/tcp6"}

// tcpListen is the LISTEN state in a /proc/net/tcp table's st column.
const tcpListen = "0A"

// Listening reads the ports this pod's network namespace listens on, ascending.
// A table that cannot be read (IPv6 disabled) contributes nothing.
func Listening() []int {
	var ports []int
	for _, path := range procNetTCP {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		ports = append(ports, listeningPorts(f)...)
		_ = f.Close()
	}
	slices.Sort(ports)
	return slices.Compact(ports)
}

// listeningPorts parses one /proc/net/tcp-format table: the local port of
// every socket in LISTEN state, whatever address it is bound to. A
// loopback-only listener is left to OpenPort, whose connect to the pod address
// it refuses, as Render requires a listener on 0.0.0.0.
func listeningPorts(table io.Reader) []int {
	var ports []int
	scanner := bufio.NewScanner(table)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[3] != tcpListen {
			continue // the header, or a socket in another state
		}
		_, hexPort, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		if port, err := strconv.ParseUint(hexPort, 16, 16); err == nil && port > 0 {
			ports = append(ports, int(port))
		}
	}
	return ports
}

// OpenPort returns the first port of listening, skip aside, on which addr
// accepts a TCP connection within timeout. skip holds the ports bex's own
// sidecars listen on inside the pod, which are not the service's.
func OpenPort(ctx context.Context, addr string, listening, skip []int, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var dialer net.Dialer
	tried := 0
	for _, port := range listening {
		if slices.Contains(skip, port) {
			continue
		}
		tried++
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(addr, strconv.Itoa(port)))
		if err == nil {
			_ = conn.Close()
			return port, nil
		}
		if ctx.Err() != nil {
			break
		}
	}
	if tried == 0 {
		return 0, errors.New("no open port: nothing listens")
	}
	return 0, fmt.Errorf("no open port: none of the %d listening ports accepted a connection on %s", tried, addr)
}
