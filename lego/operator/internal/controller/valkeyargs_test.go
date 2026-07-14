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

package controller

import (
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// A plain unit test (not the envtest Ginkgo suite): valkeyArgs is a pure function,
// so its persistence/eviction mapping is verifiable without a control plane.
func TestValkeyArgs(t *testing.T) {
	plan := tiers.ValkeyTier{Memory: "256Mi"} // 256*1024*1024 = 268435456 bytes

	has := func(args []string, flag, value string) bool {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == flag && args[i+1] == value {
				return true
			}
		}
		return false
	}

	t.Run("empty spec preserves the prior default (appendonly yes, no maxmemory)", func(t *testing.T) {
		args := valkeyArgs(appv1alpha1.KeyValueSpec{}, plan)
		if !has(args, "--appendonly", "yes") {
			t.Errorf("want --appendonly yes, got %v", args)
		}
		if slices.Contains(args, "--maxmemory") || slices.Contains(args, "--maxmemory-policy") {
			t.Errorf("empty MaxmemoryPolicy must not set maxmemory, got %v", args)
		}
	})

	t.Run("persistence off disables AOF and RDB", func(t *testing.T) {
		args := valkeyArgs(appv1alpha1.KeyValueSpec{PersistenceMode: "off"}, plan)
		if !has(args, "--appendonly", "no") || !has(args, "--save", "") {
			t.Errorf("want --appendonly no + --save \"\", got %v", args)
		}
	})

	t.Run("snapshot keeps RDB but no AOF", func(t *testing.T) {
		args := valkeyArgs(appv1alpha1.KeyValueSpec{PersistenceMode: "snapshot"}, plan)
		if !has(args, "--appendonly", "no") {
			t.Errorf("want --appendonly no, got %v", args)
		}
		if slices.Contains(args, "--save") {
			t.Errorf("snapshot must not clear save points, got %v", args)
		}
	})

	t.Run("maxmemory policy sets the budget to 80% of plan RAM", func(t *testing.T) {
		args := valkeyArgs(appv1alpha1.KeyValueSpec{MaxmemoryPolicy: "allkeys-lru"}, plan)
		if !has(args, "--maxmemory-policy", "allkeys-lru") {
			t.Errorf("want --maxmemory-policy allkeys-lru, got %v", args)
		}
		// 268435456 * 4 / 5 = 214748364
		if !has(args, "--maxmemory", "214748364") {
			t.Errorf("want --maxmemory 214748364 (80%% of 256Mi), got %v", args)
		}
	})

	t.Run("password flag is always first", func(t *testing.T) {
		args := valkeyArgs(appv1alpha1.KeyValueSpec{}, plan)
		if len(args) < 2 || args[0] != "--requirepass" || args[1] != "$(VALKEY_PASSWORD)" {
			t.Errorf("want --requirepass $(VALKEY_PASSWORD) first, got %v", args)
		}
	})
}
