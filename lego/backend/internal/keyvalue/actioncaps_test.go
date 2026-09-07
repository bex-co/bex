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

package keyvalue

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func actionByID(t *testing.T, acts []core.ActionDecision, id string) core.ActionDecision {
	t.Helper()
	for _, a := range acts {
		if a.Action == id {
			return a
		}
	}
	t.Fatalf("no %q action in %+v", id, acts)
	return core.ActionDecision{}
}

// TestActionCapabilities_SuspendResumeOnly (ADR087, w6/m136): the Key Value
// projection offers exactly suspend/resume — no invented restart — with the
// protection precondition matching the execute path's guard.
func TestActionCapabilities_SuspendResumeOnly(t *testing.T) {
	kv := keyValueForProtection("red-cache", "cache", true)
	svc, _, _ := protectedKeyValueService(kv)

	acts, err := svc.ActionCapabilities(context.Background(), kv.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities: %v", err)
	}
	if len(acts) != 2 {
		t.Fatalf("want exactly suspend+resume, got %+v", acts)
	}
	for _, a := range acts {
		if a.Action == core.ActionRestart {
			t.Fatal("Key Value has no restart verb; the projection must not invent one")
		}
	}
	if s := actionByID(t, acts, core.ActionSuspend); s.Precondition != core.PrecondProtectedConfirmation {
		t.Fatalf("protected suspend = %+v, want protected_confirmation_required", s)
	}
	// The execute path enforces exactly what was projected.
	if _, err := svc.Suspend(context.Background(), kv.Name); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("unconfirmed Suspend on a protected store = %v, want ErrBadRequest", err)
	}

	plain := keyValueForProtection("red-scratch", "scratch", false)
	svc2, _, _ := protectedKeyValueService(plain)
	acts2, err := svc2.ActionCapabilities(context.Background(), plain.Name)
	if err != nil {
		t.Fatalf("ActionCapabilities (unprotected): %v", err)
	}
	if s := actionByID(t, acts2, core.ActionSuspend); s.Precondition != "" {
		t.Fatalf("unprotected suspend precondition = %q, want none", s.Precondition)
	}
}
