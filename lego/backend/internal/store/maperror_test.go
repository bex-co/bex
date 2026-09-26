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

package store

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/021: a store miss names its entity instead of every 404 reading "app not
// found" ("app not found: project: not found" for a missing project).
func TestMapErrorNamesTheMissingEntity(t *testing.T) {
	for _, tc := range []struct {
		in   error
		want string
	}{
		{fmt.Errorf("project: %w", ErrNotFound), "project not found"},
		{classify("deploy", pgx.ErrNoRows), "deploy not found"},
		{ErrNotFound, "not found"},
		{fmt.Errorf("a: b: %w", ErrNotFound), "not found"},
	} {
		got := MapError(tc.in)
		if !errors.Is(got, core.ErrNotFound) {
			t.Errorf("MapError(%v) = %v, want errors.Is core.ErrNotFound", tc.in, got)
		}
		if got.Error() != tc.want {
			t.Errorf("MapError(%v) = %q, want %q", tc.in, got.Error(), tc.want)
		}
	}
}
