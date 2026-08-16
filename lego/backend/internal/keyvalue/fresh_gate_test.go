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
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// staleAllowChecker models the codex round-8 #8 window: the cached path (Check)
// still answers a warm positive while the source of truth (CheckFresh) already
// says the membership is gone — a member revoked on another replica inside
// PositiveTTL.
type staleAllowChecker struct{}

func (staleAllowChecker) Check(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func (staleAllowChecker) CheckFresh(context.Context, string, string, string) (bool, error) {
	return false, nil
}

// codex round-8 #8: the connection URIs embed the password — a revoked member
// riding a stale positive must not surface one last credential.
func TestKeyValueConnectionInfoFailsClosedOnFreshRevocation(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "fresh-kv")
	svc.Authz = staleAllowChecker{}

	_, err := svc.KeyValueConnectionInfo(ctxAs("user-a"), "fresh-kv")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("KeyValueConnectionInfo on a stale positive: %v, want ErrForbidden", err)
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("denial leaked the password: %v", err)
	}
}

// codex round-9 #7: deleting a managed key-value store (and its PVC) is
// irreversible — the fresh gate must stop a stale positive before the CR
// delete, leaving the store in place.
func TestDeleteKeyValueFailsClosedOnFreshRevocation(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "fresh-kvdel")
	svc.Authz = staleAllowChecker{}
	ctx := ctxAs("user-a")

	if err := svc.DeleteKeyValue(ctx, "fresh-kvdel"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("DeleteKeyValue on a stale positive: %v, want ErrForbidden", err)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "fresh-kvdel"}, &kv); err != nil {
		t.Fatalf("denied DeleteKeyValue must leave the CR in place: %v", err)
	}
}
