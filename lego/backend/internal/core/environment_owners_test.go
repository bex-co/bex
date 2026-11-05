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

package core

import (
	"context"
	"slices"
	"testing"
)

// w4/500: an environmentId filter without ownerId searches the workspaces the
// environments live in, not just the caller's default workspace.
func TestEnvironmentOwners(t *testing.T) {
	workspaces := map[string]string{"evm-a": "tea-1", "evm-b": "tea-2", "evm-c": "tea-1"}
	resolve := func(_ context.Context, id string) (string, error) {
		switch id {
		case "evm-gone":
			return "", ErrNotFound
		case "evm-foreign":
			return "", ErrForbidden
		case "evm-down":
			return "", ErrAuthzUnavailable
		}
		return workspaces[id], nil
	}
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		owners []string
		envs   []string
		fn     func(context.Context, string) (string, error)
		want   []string
	}{
		{"owners from environments, deduplicated", nil, []string{"evm-a", "evm-b", "evm-c"}, resolve, []string{"tea-1", "tea-2"}},
		{"explicit ownerId is kept untouched", []string{"tea-9"}, []string{"evm-a"}, resolve, []string{"tea-9"}},
		{"no environment filter keeps the default", nil, nil, resolve, nil},
		{"an unwired resolver keeps the default", nil, []string{"evm-a"}, nil, nil},
		{"unknown and foreign environments contribute nothing", nil, []string{"evm-gone", "evm-foreign"}, resolve, nil},
		{"one resolvable among unresolvable", nil, []string{"evm-gone", "evm-b", "evm-foreign"}, resolve, []string{"tea-2"}},
		{"an unavailable dependency leaves the default list", nil, []string{"evm-a", "evm-down"}, resolve, nil},
	} {
		if got := EnvironmentOwners(ctx, tc.owners, tc.envs, tc.fn); !slices.Equal(got, tc.want) {
			t.Errorf("%s: owners = %v, want %v", tc.name, got, tc.want)
		}
	}
}
