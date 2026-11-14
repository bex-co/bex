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

package netutil_test

import (
	"net/netip"
	"testing"

	"github.com/bex-co/bex/lego/types/netutil"
)

func TestSourceKey(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"198.51.100.7", "198.51.100.7"},
		{"::ffff:198.51.100.7", "198.51.100.7"},
		{"2001:db8:1:2:aaaa:bbbb:cccc:dddd", "2001:db8:1:2::"},
		{"2001:db8:1:2::1", "2001:db8:1:2::"},
		{"fe80::1%eth0", "fe80::"},
	} {
		if got := netutil.SourceKey(netip.MustParseAddr(tc.in)); got.String() != tc.want {
			t.Errorf("SourceKey(%s) = %s, want %s", tc.in, got, tc.want)
		}
	}
	if netutil.SourceKey(netip.MustParseAddr("2001:db8:1:2::1")) == netutil.SourceKey(netip.MustParseAddr("2001:db8:1:3::1")) {
		t.Error("two different /64s share a key")
	}
}
