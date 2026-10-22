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
	"net/http"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

type fakeProtectionStore struct {
	statuses map[string]string
	calls    int
}

func (s *fakeProtectionStore) GetEnvironmentProtectedStatus(_ context.Context, environmentID string) (string, error) {
	s.calls++
	if status := s.statuses[environmentID]; status != "" {
		return status, nil
	}
	return core.ProtectedStatusUnprotected, nil
}

func keyValueForProtection(id, name string, protected bool) *appv1alpha1.KeyValue {
	labels := map[string]string{}
	if protected {
		labels[core.LabelEnvironment] = "env-production"
	}
	return &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: id, Namespace: "default", Labels: labels},
		Spec:       appv1alpha1.KeyValueSpec{Name: name, Plan: "free"},
	}
}

func protectedKeyValueService(objects ...client.Object) (*Service, client.Client, *fakeProtectionStore) {
	svc, cl := newService(objects...)
	store := &fakeProtectionStore{statuses: map[string]string{"env-production": core.ProtectedStatusProtected}}
	svc.Protection = store
	return svc, cl, store
}

// denyWorkspaceManage allows every relation except the admin can_manage on a
// workspace — a Developer in the role ladder. Other objects' checks (the
// ordinary per-verb authorization) are untouched.
type denyWorkspaceManage struct{}

func (denyWorkspaceManage) Check(_ context.Context, _, relation, object string) (bool, error) {
	return !(relation == core.RelCanManage && strings.HasPrefix(object, "workspace:")), nil
}

type allowAllRelations struct{}

func (allowAllRelations) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

// ctxAsCaller is the identity every Authz-checked call needs.
func ctxAsCaller() context.Context {
	return core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "session"})
}

// The rule is Render's (w8/187): in a protected environment only a workspace
// Admin may delete, suspend or resume a Key Value, with no typed phrase;
// everything else is open to anyone who may do it elsewhere.
func TestProtectedKeyValueDeleteSuspendResumeNeedAdmin(t *testing.T) {
	t.Run("a non-admin is refused and nothing moves", func(t *testing.T) {
		kv := keyValueForProtection("red-sessions", "sessions", true)
		svc, cl, _ := protectedKeyValueService(kv)
		svc.Authz = denyWorkspaceManage{}
		if err := svc.DeleteKeyValue(ctxAsCaller(), kv.Name); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("DeleteKeyValue by a non-admin = %v, want ErrForbidden", err)
		}
		if _, err := svc.Suspend(ctxAsCaller(), kv.Name); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("Suspend by a non-admin = %v, want ErrForbidden", err)
		}
		var got appv1alpha1.KeyValue
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(kv), &got); err != nil || got.Spec.Suspended {
			t.Fatalf("a refused verb changed the store: suspended=%v err=%v", got.Spec.Suspended, err)
		}
		// The old phrase substitutes for nothing.
		ctx := core.WithConfirm(ctxAsCaller(), "sudo delete key value sessions")
		if err := svc.DeleteKeyValue(ctx, kv.Name); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("DeleteKeyValue with the old phrase = %v, want ErrForbidden", err)
		}
	})

	t.Run("an admin needs no phrase for delete, suspend or resume", func(t *testing.T) {
		kv := keyValueForProtection("red-sessions", "sessions", true)
		svc, cl, _ := protectedKeyValueService(kv)
		svc.Authz = allowAllRelations{}
		if _, err := svc.Suspend(ctxAsCaller(), kv.Name); err != nil {
			t.Fatalf("Suspend by an admin: %v", err)
		}
		var got appv1alpha1.KeyValue
		if err := cl.Get(context.Background(), client.ObjectKeyFromObject(kv), &got); err != nil || !got.Spec.Suspended {
			t.Fatalf("KeyValue not suspended: suspended=%v err=%v", got.Spec.Suspended, err)
		}
		if _, err := svc.Resume(ctxAsCaller(), kv.Name); err != nil {
			t.Fatalf("Resume by an admin: %v", err)
		}
		if err := svc.DeleteKeyValue(ctxAsCaller(), kv.Name); err != nil {
			t.Fatalf("DeleteKeyValue by an admin: %v", err)
		}
	})

	t.Run("a non-admin cannot resume a protected store", func(t *testing.T) {
		kv := keyValueForProtection("red-sessions", "sessions", true)
		kv.Spec.Suspended = true
		svc, _, _ := protectedKeyValueService(kv)
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.Resume(ctxAsCaller(), kv.Name); !errors.Is(err, core.ErrForbidden) {
			t.Fatalf("Resume by a non-admin = %v, want ErrForbidden", err)
		}
	})

	t.Run("environment-less resource skips store", func(t *testing.T) {
		kv := keyValueForProtection("red-dev", "dev", false)
		svc, _, store := protectedKeyValueService(kv)
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.Suspend(ctxAsCaller(), kv.Name); err != nil {
			t.Fatalf("environment-less Suspend: %v", err)
		}
		if store.calls != 0 {
			t.Fatalf("protection store calls = %d, want 0", store.calls)
		}
	})

	t.Run("unwired protection store is a no-op", func(t *testing.T) {
		kv := keyValueForProtection("red-hand-applied", "hand-applied", true)
		svc, _ := newService(kv)
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.Suspend(ctxAsCaller(), kv.Name); err != nil {
			t.Fatalf("DB-less Suspend: %v", err)
		}
	})

	t.Run("unprotected environment needs no admin", func(t *testing.T) {
		kv := keyValueForProtection("red-staging", "staging", true)
		svc, _, store := protectedKeyValueService(kv)
		store.statuses["env-production"] = core.ProtectedStatusUnprotected
		svc.Authz = denyWorkspaceManage{}
		if _, err := svc.Suspend(ctxAsCaller(), kv.Name); err != nil {
			t.Fatalf("unprotected Suspend: %v", err)
		}
	})

	t.Run("a protection lookup that fails refuses the verb", func(t *testing.T) {
		kv := keyValueForProtection("red-sessions", "sessions", true)
		svc, _, _ := protectedKeyValueService(kv)
		svc.Protection = failingProtection{}
		if _, err := svc.Suspend(ctxAsCaller(), kv.Name); err == nil {
			t.Fatal("Suspend with an unreadable protection status succeeded")
		}
	})
}

type failingProtection struct{}

func (failingProtection) GetEnvironmentProtectedStatus(context.Context, string) (string, error) {
	return "", errors.New("store down")
}

// Over REST the Admin role is the whole rule: a non-admin gets 403 and an admin
// succeeds with no ?confirm=.
func TestProtectedKeyValueOverREST(t *testing.T) {
	kv := keyValueForProtection("red-rest", "rest-kv", true)
	svc, _, _ := protectedKeyValueService(kv)
	svc.Authz = allowAllRelations{}
	if got := serveREST(svc, http.MethodPost, "/v1/key-value/red-rest/suspend", ""); got.Code != http.StatusAccepted {
		t.Fatalf("admin suspend status = %d: %s", got.Code, got.Body.String())
	}
}

// Renames, durability and eviction changes are not among the actions Render
// restricts in a protected environment (w4/m127 had gated them behind phrases).
func TestProtectedKeyValueSettingsAreOpenToDevelopers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		patch KeyValuePatch
	}{
		{"durability off", KeyValuePatch{PersistenceMode: strPtr("off")}},
		{"eviction policy", KeyValuePatch{MaxmemoryPolicy: strPtr("allkeys-lru")}},
		{"rename", KeyValuePatch{Name: strPtr("renamed")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kv := keyValueForProtection("red-cache", "cache", true)
			svc, _, _ := protectedKeyValueService(kv)
			svc.Authz = denyWorkspaceManage{}
			if _, err := svc.UpdateKeyValue(ctxAsCaller(), kv.Name, tc.patch); err != nil {
				t.Fatalf("%s by a non-admin on a protected store: %v", tc.name, err)
			}
		})
	}
}

func strPtr(v string) *string { return &v }
