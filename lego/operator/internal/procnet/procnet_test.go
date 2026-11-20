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

package procnet

import (
	"encoding/binary"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// The tables below are written as a little-endian node prints them, which is
// every node bex runs on (amd64, arm64); on a big-endian host the kernel would
// print the words the other way round and parseAddr would read them back the
// same, so the expectations only hold where NativeEndian is little.
func requireLittleEndian(t *testing.T) {
	t.Helper()
	var b [2]byte
	binary.NativeEndian.PutUint16(b[:], 1)
	if b[0] != 1 {
		t.Skip("fixtures are little-endian kernel output")
	}
}

func TestListeningDecodesIPv4AndIPv6Addresses(t *testing.T) {
	requireLittleEndian(t)
	table := `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:2710 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1 1 0000000000000000 100 0 0 10 0
   1: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 2 1 0000000000000000 100 0 0 10 0
   2: 0501F40A:2710 0901F40A:D431 01 00000000:00000000 00:00000000 00000000  1000        0 3 1 0000000000000000 20 4 30 10 -1
   3: 0501F40A:18EB 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 4 1 0000000000000000 100 0 0 10 0
   4: 00000000000000000000000000000000:4652 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 5 1 0000000000000000 100 0 0 10 0
   5: 00000000000000000000000001000000:0050 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 6 1 0000000000000000 100 0 0 10 0
   6: 0000000000000000FFFF00000100007F:0051 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 7 1 0000000000000000 100 0 0 10 0
   7: 0000000000000000FFFF00000501F40A:0052 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 8 1 0000000000000000 100 0 0 10 0
   8: 0000000000000000:0053 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 9 1 0000000000000000 100 0 0 10 0
   9: 00000000:0000 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 10 1 0000000000000000 100 0 0 10 0
`
	want := []Socket{
		{netip.MustParseAddr("0.0.0.0"), 10000},
		{netip.MustParseAddr("127.0.0.1"), 8080},
		{netip.MustParseAddr("10.244.1.5"), 6379},
		{netip.MustParseAddr("::"), 18002},
		{netip.MustParseAddr("::1"), 80},
		{netip.MustParseAddr("::ffff:127.0.0.1"), 81},
		{netip.MustParseAddr("::ffff:10.244.1.5"), 82},
	}
	if got := Listening(strings.NewReader(table)); !slices.Equal(got, want) {
		t.Fatalf("Listening = %v\nwant %v", got, want)
	}
}

// Only the LISTEN state counts: an established connection's local port is not
// a listener, nor is a header or a truncated row.
func TestListeningSkipsOtherStatesAndJunk(t *testing.T) {
	table := "  sl  local_address rem_address   st\n" +
		"   0: 00000000:1F90 00000000:0000 06\n" + // TIME_WAIT
		"   1: 00000000:1F91\n" +
		"   2: 00000000 00000000:0000 0A\n" +
		"   3: 0000000Z:1F92 00000000:0000 0A\n" +
		"   4: 00000000:XYZ0 00000000:0000 0A\n"
	if got := Listening(strings.NewReader(table)); len(got) != 0 {
		t.Fatalf("Listening = %v, want none", got)
	}
}
