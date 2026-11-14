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

package netutil

import "net/netip"

// SourceKey is the address a per-source cap counts a client under (w21/m394
// t017, R25-12). An IPv4 address, or an IPv4-mapped IPv6 address, is itself. An
// IPv6 address is its /64, the smallest block one subscriber holds, so a client
// cannot dodge a cap by rotating through the addresses of its own prefix. A zone
// is dropped.
func SourceKey(addr netip.Addr) netip.Addr {
	addr = addr.Unmap().WithZone("")
	if addr.Is6() {
		prefix, _ := addr.Prefix(64) // a valid length for an IPv6 address
		return prefix.Addr()
	}
	return addr
}
