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

// seam_test.go covers the blueprint apply path's seam (w1/m35): GroupNames,
// GroupIDsByName, ApplyEnvGroup (create/update by name, generate-once), and
// LinkEnvGroup (idempotent) — the methods apps' Blueprint flows drive
// envVarGroups/fromGroup through.

import (
	"context"
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestApplyEnvGroupCreatesAndIsIdempotent(t *testing.T) {
	st := newFakeStore()
	svc := newService(st)
	ctx := context.Background()

	if err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"LOG_LEVEL": "info"}, []string{"SESSION_SECRET"}); err != nil {
		t.Fatalf("ApplyEnvGroup create: %v", err)
	}
	gid, _, found, err := svc.findGroupByName(ctx, "shared")
	if err != nil || !found {
		t.Fatalf("group not created: found=%v err=%v", found, err)
	}
	env := st.m[envPath(gid)]
	if env["LOG_LEVEL"] != "info" {
		t.Errorf("literal not set: %q", env["LOG_LEVEL"])
	}
	minted := env["SESSION_SECRET"]
	if len(minted) != 44 { // base64 256-bit
		t.Errorf("generated value = %q (len %d), want 44-char base64", minted, len(minted))
	}

	// Re-apply the SAME set: no re-mint (the generated value persists), and no new
	// resource version churn on the env (idempotent).
	if err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"LOG_LEVEL": "info"}, []string{"SESSION_SECRET"}); err != nil {
		t.Fatalf("ApplyEnvGroup re-apply: %v", err)
	}
	gid2, _, _, _ := svc.findGroupByName(ctx, "shared")
	if gid2 != gid {
		t.Errorf("re-apply created a second group %q != %q", gid2, gid)
	}
	if got := st.m[envPath(gid)]["SESSION_SECRET"]; got != minted {
		t.Errorf("generated value re-minted on re-apply: %q -> %q", minted, got)
	}

	// A changed literal re-syncs; the generated value still persists.
	if err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"LOG_LEVEL": "debug"}, []string{"SESSION_SECRET"}); err != nil {
		t.Fatalf("ApplyEnvGroup change: %v", err)
	}
	env = st.m[envPath(gid)]
	if env["LOG_LEVEL"] != "debug" {
		t.Errorf("literal not re-synced: %q", env["LOG_LEVEL"])
	}
	if env["SESSION_SECRET"] != minted {
		t.Errorf("generated value changed on literal edit: %q", env["SESSION_SECRET"])
	}
}

func TestGroupNamesListsEveryGroup(t *testing.T) {
	svc := newService(newFakeStore())
	ctx := context.Background()
	for _, n := range []string{"beta", "alpha"} {
		if err := svc.ApplyEnvGroup(ctx, n, map[string]string{"K": "v"}, nil); err != nil {
			t.Fatalf("ApplyEnvGroup %s: %v", n, err)
		}
	}
	names, err := svc.GroupNames(ctx)
	if err != nil {
		t.Fatalf("GroupNames: %v", err)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("GroupNames = %v, want [alpha beta] (sorted)", names)
	}
	ids, err := svc.GroupIDsByName(ctx)
	if err != nil {
		t.Fatalf("GroupIDsByName: %v", err)
	}
	if ids["alpha"] == "" || ids["beta"] == "" || ids["alpha"] == ids["beta"] {
		t.Errorf("GroupIDsByName = %v, want distinct non-empty ids", ids)
	}
}

func TestLinkEnvGroupIsIdempotent(t *testing.T) {
	st := newFakeStore()
	svc := newService(st, sampleApp("web"))
	ctx := context.Background()

	if err := svc.ApplyEnvGroup(ctx, "shared", map[string]string{"K": "v"}, nil); err != nil {
		t.Fatalf("ApplyEnvGroup: %v", err)
	}
	if err := svc.LinkEnvGroup(ctx, "shared", "web"); err != nil {
		t.Fatalf("LinkEnvGroup: %v", err)
	}
	cl := svc.Client
	firstRA := getApp(t, cl, "web").Spec.RestartedAt
	gid, _, _, _ := svc.findGroupByName(ctx, "shared")
	a := getApp(t, cl, "web")
	if len(a.Spec.EnvFromSecrets) != 1 || a.Spec.EnvFromSecrets[0] != envSecretName(gid) {
		t.Errorf("link did not add the group's env secret: %+v", a.Spec.EnvFromSecrets)
	}

	// Re-link the same service: no spec churn, no restartedAt bump.
	if err := svc.LinkEnvGroup(ctx, "shared", "web"); err != nil {
		t.Fatalf("LinkEnvGroup re-link: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.RestartedAt; got != firstRA {
		t.Errorf("re-link bumped RestartedAt: %q -> %q", firstRA, got)
	}
}

func TestLinkEnvGroupUnknownGroupErrors(t *testing.T) {
	svc := newService(newFakeStore(), sampleApp("web"))
	err := svc.LinkEnvGroup(context.Background(), "ghost", "web")
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("LinkEnvGroup unknown group => ErrBadRequest, got %v", err)
	}
}
