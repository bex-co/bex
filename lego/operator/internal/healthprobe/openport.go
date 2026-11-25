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
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/procnet"
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
	sockets := procnet.Listening(table)
	ports := make([]int, 0, len(sockets))
	for _, socket := range sockets {
		ports = append(ports, socket.Port)
	}
	return ports
}

// OpenPort returns the first port of listening, skip aside, on which addr
// accepts a TCP connection within timeout. skip holds the ports bex's own
// sidecars listen on inside the pod, which are not the service's. A failure
// names the last dial's error ("connect: connection refused", "i/o timeout"),
// as Render's restart events do (w6/m634).
func OpenPort(ctx context.Context, addr string, listening, skip []int, timeout time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var dialer net.Dialer
	var last error
	for _, port := range listening {
		if slices.Contains(skip, port) {
			continue
		}
		conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(addr, strconv.Itoa(port)))
		if err == nil {
			_ = conn.Close()
			return port, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	if last == nil {
		return 0, errors.New(NoOpenPorts)
	}
	return 0, fmt.Errorf("%s: %w", TCPFailed, last)
}
