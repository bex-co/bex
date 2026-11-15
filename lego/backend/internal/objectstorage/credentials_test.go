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

package objectstorage

import (
	"slices"
	"testing"
)

func TestCredentialsHalves(t *testing.T) {
	cases := []struct {
		name                string
		c                   Credentials
		enabled, configured bool
		missing             []string
	}{
		{"none", Credentials{}, false, false, nil},
		{"both", Credentials{AccessKey: "a", SecretKey: "s"}, true, true, nil},
		{"access key only", Credentials{AccessKey: "a"}, false, true, []string{"BEX_OBJECT_STORAGE_SECRET_KEY"}},
		{"secret key only", Credentials{SecretKey: "s"}, false, true, []string{"BEX_OBJECT_STORAGE_ACCESS_KEY"}},
	}
	for _, c := range cases {
		if c.c.Enabled() != c.enabled || c.c.Configured() != c.configured || !slices.Equal(c.c.Missing(), c.missing) {
			t.Errorf("%s: Enabled=%v Configured=%v Missing=%v", c.name, c.c.Enabled(), c.c.Configured(), c.c.Missing())
		}
	}
}

func TestNewStoresBuildsOneStorePerRegionOrNone(t *testing.T) {
	if stores := NewStores(Credentials{}); stores != nil {
		t.Fatalf("no credential must give no stores, got %v", stores)
	}
	if stores := NewStores(Credentials{AccessKey: "a"}); stores != nil {
		t.Fatalf("half a credential must give no stores, got %v", stores)
	}
	stores := NewStores(Credentials{AccessKey: "a", SecretKey: "s"})
	if len(stores) != len(Regions) {
		t.Fatalf("stores = %d, want one per region (%d)", len(stores), len(Regions))
	}
	for _, r := range Regions {
		if stores[r] == nil {
			t.Errorf("no store for %s", r)
		}
	}
}
