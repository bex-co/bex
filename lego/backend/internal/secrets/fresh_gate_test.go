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

package secrets

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

// codex round-8 #8: every env value is a secret reveal, so the masked list,
// single-key reveal, and paged list must all re-assert can_view_sensitive
// uncached — a stale positive must not surface one last value.
func TestEnvVarRevealsFailClosedOnFreshRevocation(t *testing.T) {
	store := newFakeSecretStore()
	store.m[envPath("web")] = map[string]string{"API_KEY": "topsecret"}
	svc := newService(store, sampleApp("web"))
	svc.Authz = staleAllowChecker{}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})

	if _, err := svc.ListEnvVars(ctx, "web"); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("ListEnvVars on a stale positive: %v, want ErrForbidden", err)
	}
	if _, err := svc.ListEnvVarsPage(ctx, "web", "", 10); !errors.Is(err, core.ErrForbidden) {
		t.Errorf("ListEnvVarsPage on a stale positive: %v, want ErrForbidden", err)
	}
	_, err := svc.GetEnvVar(ctx, "web", "API_KEY")
	if !errors.Is(err, core.ErrForbidden) {
		t.Errorf("GetEnvVar on a stale positive: %v, want ErrForbidden", err)
	}
	if err != nil && strings.Contains(err.Error(), "topsecret") {
		t.Errorf("denial leaked the value: %v", err)
	}
}

// codex round-8 #8: secret-file content is a reveal — GetSecretFile must fail
// closed on a fresh denial instead of surfacing one last file body.
func TestGetSecretFileFailsClosedOnFreshRevocation(t *testing.T) {
	store := newFakeSecretStore()
	store.m[filesPath("web")] = map[string]string{"ca.pem": "----CERT----"}
	svc := newService(store, sampleApp("web"))
	svc.Authz = staleAllowChecker{}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "client-1", Method: "oauth2"})

	_, err := svc.GetSecretFile(ctx, "web", "ca.pem")
	if !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("GetSecretFile on a stale positive: %v, want ErrForbidden", err)
	}
	if strings.Contains(err.Error(), "----CERT----") {
		t.Errorf("denial leaked the file body: %v", err)
	}
}
