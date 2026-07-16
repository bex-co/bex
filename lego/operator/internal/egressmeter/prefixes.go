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

package egressmeter

import (
	"fmt"
	"net/netip"
)

var nonPublicPrefixes = []string{
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
	"169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24",
	"192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24",
	"224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8", "2001:db8::/32",
}

type lpmV4Key struct {
	PrefixLen uint32
	Addr      [4]byte
}

type lpmV6Key struct {
	PrefixLen uint32
	Addr      [16]byte
}

func parsePrefixes(extra []string) ([]netip.Prefix, error) {
	all := append(append([]string(nil), nonPublicPrefixes...), extra...)
	out := make([]netip.Prefix, 0, len(all))
	seen := map[netip.Prefix]bool{}
	for _, raw := range all {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			return nil, fmt.Errorf("parse excluded CIDR %q: %w", raw, err)
		}
		prefix = prefix.Masked()
		if !seen[prefix] {
			seen[prefix] = true
			out = append(out, prefix)
		}
	}
	return out, nil
}
