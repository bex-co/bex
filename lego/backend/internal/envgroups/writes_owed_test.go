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

package envgroups

import (
	"errors"
	"maps"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
)

// w8/376: GroupWritesOwed answers the Blueprint plan's question for an
// existing group from ApplyEnvGroup's own diff, so a plan of an unchanged
// group reads no change and a plan that lists a change is one the apply
// makes.
func TestGroupWritesOwedAgreesWithApplyEnvGroup(t *testing.T) {
	st := newFakeStore()
	svc := newService(st)
	ctx := testCtx()
	owed := GroupWritesOwed(svc)
	literals := map[string]string{"LOG_LEVEL": "info", "EMPTY": ""}
	generates := []string{"TOKEN"}
	if got, err := owed(ctx, "shared", literals, generates); err != nil || !got {
		t.Fatalf("an absent group owes = %v, %v; want its creation", got, err)
	}
	if _, err := svc.ApplyEnvGroup(ctx, "shared", literals, generates); err != nil {
		t.Fatal(err)
	}
	stored := maps.Clone(st.m[st.key(ctx, envPath(idNamed(ctx, svc, "shared")))])
	for _, tc := range []struct {
		name      string
		literals  map[string]string
		generates []string
		want      bool
	}{
		{"unchanged", literals, generates, false},
		{"fewer declared", map[string]string{"LOG_LEVEL": "info"}, nil, false},
		{"changed literal", map[string]string{"LOG_LEVEL": "debug", "EMPTY": ""}, generates, true},
		{"new literal", map[string]string{"LOG_LEVEL": "info", "EMPTY": "", "REGION": "eu"}, generates, true},
		{"new empty literal", map[string]string{"LOG_LEVEL": "info", "EMPTY": "", "BLANK": ""}, generates, true},
		{"missing generate", literals, []string{"TOKEN", "SIGNING_KEY"}, true},
		{"generate over a literal", map[string]string{"EMPTY": ""}, []string{"LOG_LEVEL"}, false},
	} {
		got, err := owed(ctx, " shared ", tc.literals, tc.generates)
		if err != nil || got != tc.want {
			t.Errorf("%s: owed = %v, %v; want %v", tc.name, got, err, tc.want)
		}
		if after := st.m[st.key(ctx, envPath(idNamed(ctx, svc, "shared")))]; !maps.Equal(after, stored) {
			t.Fatalf("%s: the question wrote the group: %v -> %v", tc.name, stored, after)
		}
	}
	// Owing nothing, the apply writes nothing and owes no roll.
	roll, err := svc.ApplyEnvGroup(ctx, "shared", literals, generates)
	if err != nil || roll != nil {
		t.Fatalf("unchanged apply = roll %v, %v", roll != nil, err)
	}
	if after := st.m[st.key(ctx, envPath(idNamed(ctx, svc, "shared")))]; !maps.Equal(after, stored) {
		t.Fatalf("an apply that owed nothing wrote %v -> %v", stored, after)
	}
	if _, err := owed(ctx, "shared", map[string]string{"bad key": "x"}, nil); err == nil {
		t.Error("an invalid key was answered, want the apply's refusal")
	}
}

// Whether a declared value matches the stored one is a fact about the value:
// only a caller who may read the group's values gets the answer.
func TestGroupWritesOwedRefusesACallerWhoCannotReadTheValues(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := testCtx()
	literals := map[string]string{"API_KEY": "s3cret"}
	if _, err := svc.ApplyEnvGroup(ctx, "shared", literals, nil); err != nil {
		t.Fatal(err)
	}
	svc.Authz = refuseRelation{relation: core.RelCanViewSensitive}
	if _, err := GroupWritesOwed(svc)(ctx, "shared", literals, nil); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("a viewer's question = %v, want ErrForbidden", err)
	}
	svc.Authz = nil
	grants := coretest.KeyGrants{"key-1": core.KeyGrantFromEntries([]string{"env_groups:read_write", "env_group_values:write"})}
	svc.KeyGrants = grants
	key := core.WithIdentity(ctx, core.Identity{Subject: "key-1", ClientID: "key-1", Method: core.MethodOAuth2})
	if _, err := GroupWritesOwed(svc)(key, "shared", literals, nil); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("a key not granted the values = %v, want ErrForbidden", err)
	}
	grants["key-1"] = core.KeyGrantFromEntries([]string{"env_groups:read_write", "env_group_values:read_write"})
	if got, err := GroupWritesOwed(svc)(key, "shared", literals, nil); err != nil || got {
		t.Errorf("a key granted the values = %v, %v; want false", got, err)
	}
}
