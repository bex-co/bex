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

package mcputil

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestPreferString(t *testing.T) {
	if got := PreferString("a", "b"); got != "a" {
		t.Errorf("PreferString primary = %q", got)
	}
	if got := PreferString("", "b"); got != "b" {
		t.Errorf("PreferString legacy = %q", got)
	}
}

func TestPreferPtrOrZero(t *testing.T) {
	p, l := int32(40), int32(25)
	if got := PreferPtrOrZero(&p, &l); got != 40 {
		t.Errorf("both = %d, want 40", got)
	}
	if got := PreferPtrOrZero[int32](nil, &l); got != 25 {
		t.Errorf("legacy = %d, want 25", got)
	}
	if got := PreferPtrOrZero[int32](nil, nil); got != 0 {
		t.Errorf("neither = %d, want 0", got)
	}
}

func TestRequireAliasString(t *testing.T) {
	got, err := RequireAliasString("id", "legacy", "resourceId", "resource")
	if err != nil || got != "id" {
		t.Fatalf("primary: %q %v", got, err)
	}
	got, err = RequireAliasString("", "legacy", "resourceId", "resource")
	if err != nil || got != "legacy" {
		t.Fatalf("legacy: %q %v", got, err)
	}
	_, err = RequireAliasString("", "", "resourceId", "resource")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("empty: %v", err)
	}
}

func TestResourceIDs(t *testing.T) {
	got, err := ResourceIDs("a", []string{"b", "c"}, "resourceId", "resource")
	if err != nil || len(got) != 1 || got[0] != "a" {
		t.Fatalf("primary: %v %v", got, err)
	}
	got, err = ResourceIDs("", []string{"b", "c"}, "resourceId", "resource")
	if err != nil || len(got) != 2 {
		t.Fatalf("legacy: %v %v", got, err)
	}
	_, err = ResourceIDs("", nil, "resourceId", "resource")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("empty: %v", err)
	}
}
