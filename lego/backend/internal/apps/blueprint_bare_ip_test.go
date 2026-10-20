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

package apps

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestManifestAllowEntriesAcceptsBareAddresses (w8/142): Render's Blueprint
// `source` is "the IP address or CIDR block", and its examples list
// `source: 198.51.100.1`. Every resource kind compiles a bare address to its
// single-host prefix; invalid sources keep their existing refusals.
func TestManifestAllowEntriesAcceptsBareAddresses(t *testing.T) {
	entries := []bexIPEntry{
		{Source: "203.0.113.4/30", Description: "office"},
		{Source: "198.51.100.1", Description: "home"},
		{Source: "2001:db8::1", Description: "home-v6"},
		{Source: "::ffff:192.0.2.9", Description: "mapped"},
	}
	want := []core.IPAllowListEntry{
		{CIDRBlock: "203.0.113.4/30", Description: "office"},
		{CIDRBlock: "198.51.100.1/32", Description: "home"},
		{CIDRBlock: "2001:db8::1/128", Description: "home-v6"},
		{CIDRBlock: "192.0.2.9/32", Description: "mapped"},
	}
	for _, noun := range []string{"service", "static site", "database", "key-value"} {
		got, err := manifestAllowEntries(entries, noun, "thunder")
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: entries = %+v (err %v), want %+v", noun, got, err, want)
		}
	}
	for source, wantErr := range map[string]string{
		"not-an-ip":      `"not-an-ip" is not a valid CIDR`,
		"203.0.113.0/99": `"203.0.113.0/99" is not a valid CIDR`,
		"fe80::1%eth0":   `"fe80::1%eth0" is not a valid CIDR`,
		" ":              "without a source",
	} {
		if _, err := manifestAllowEntries([]bexIPEntry{{Source: source}}, "key-value", "thunder"); err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("source %q: err = %v, want %q", source, err, wantErr)
		}
	}
}
