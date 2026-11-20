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

// Package procnet parses the kernel's TCP socket tables, /proc/net/tcp and
// /proc/net/tcp6, which list every socket of one network namespace. Both the
// in-pod health check (internal/healthprobe, w4/m350) and the node's listener
// reader (internal/listeners, w6/m530) read them; it imports nothing beyond
// the standard library because the health-probe binary is copied into tenant
// pods.
package procnet

import (
	"bufio"
	"encoding/binary"
	"io"
	"net/netip"
	"strconv"
	"strings"
)

// tcpListen is the LISTEN state in a table's st column.
const tcpListen = "0A"

// Socket is a listening socket's local address and port.
type Socket struct {
	Addr netip.Addr
	Port int
}

// Listening returns every socket of one /proc/net/tcp-format table that is in
// LISTEN state, in table order. Rows in another state, the header, and rows
// the kernel would never print are skipped.
func Listening(table io.Reader) []Socket {
	var sockets []Socket
	scanner := bufio.NewScanner(table)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[3] != tcpListen {
			continue
		}
		hexAddr, hexPort, ok := strings.Cut(fields[1], ":")
		if !ok {
			continue
		}
		port, err := strconv.ParseUint(hexPort, 16, 16)
		if err != nil || port == 0 {
			continue
		}
		addr, ok := parseAddr(hexAddr)
		if !ok {
			continue
		}
		sockets = append(sockets, Socket{Addr: addr, Port: int(port)})
	}
	return sockets
}

// parseAddr decodes a table's local address: one 32-bit word for IPv4, four
// for IPv6. The kernel prints each word as the hex of the address bytes read
// as a host-order integer, so 127.0.0.1 reads 0100007F on a little-endian node.
func parseAddr(hex string) (netip.Addr, bool) {
	switch len(hex) {
	case 8:
		var b [4]byte
		if !word(hex, b[:]) {
			return netip.Addr{}, false
		}
		return netip.AddrFrom4(b), true
	case 32:
		var b [16]byte
		for i := range 4 {
			if !word(hex[i*8:(i+1)*8], b[i*4:(i+1)*4]) {
				return netip.Addr{}, false
			}
		}
		return netip.AddrFrom16(b), true
	}
	return netip.Addr{}, false
}

// word writes one host-order hex word back as the four bytes it was printed from.
func word(hex string, dst []byte) bool {
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return false
	}
	binary.NativeEndian.PutUint32(dst, uint32(v))
	return true
}
