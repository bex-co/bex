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
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// denyingChecker allows every relation but one.
type denyingChecker struct{ deny string }

func (c denyingChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	return relation != c.deny, nil
}

// copyService ticks its clock on every read, so a restart a write records
// shows up as a new restartedAt.
func copyService(t *testing.T) (*Service, *fakeSecretStore) {
	t.Helper()
	store := newFakeSecretStore()
	svc := newService(store, sampleApp("database"), sampleApp("web"))
	var n atomic.Int64
	svc.Clock = func() time.Time { return time.Unix(1_000_000+n.Add(1), 0).UTC() }
	ctx := context.Background()
	for key, value := range map[string]string{"PASSWORD": "minted-1", "USER": "minted-user"} {
		if _, err := svc.SetEnvVar(ctx, "database", key, EnvVarWrite{Value: value}); err != nil {
			t.Fatal(err)
		}
	}
	return svc, store
}

var webCopies = []core.EnvVarCopy{
	{From: "database", FromKey: "PASSWORD", ToKey: "DB_PASSWORD"},
	{From: "database", FromKey: "USER", ToKey: "DB_USER"},
}

// w1/m175: a Blueprint's copies of a sibling's generated values land in the
// receiver's store in one write, and only for a caller who may reveal them.
func TestEnvCopierSeedsMissingCopiesInOneWriteAndLeavesEqualOnesAlone(t *testing.T) {
	svc, store := copyService(t)
	ctx := context.Background()
	copier := NewEnvCopier(svc)
	before := getApp(t, svc.Client, "web").Spec.RestartedAt

	if err := copier.CopyEnvVars(ctx, "web", webCopies); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if env := store.m[envPath("web")]; env["DB_PASSWORD"] != "minted-1" || env["DB_USER"] != "minted-user" {
		t.Fatalf("web env = %v, want both copies", env)
	}
	first := getApp(t, svc.Client, "web").Spec.RestartedAt
	if first == before {
		t.Fatal("the first copy did not roll web, so its pods never see the values")
	}

	// An unchanged sync writes nothing and restarts nothing.
	puts := store.puts
	if err := copier.CopyEnvVars(ctx, "web", webCopies); err != nil {
		t.Fatal(err)
	}
	if store.puts != puts || getApp(t, svc.Client, "web").Spec.RestartedAt != first {
		t.Fatalf("an unchanged copy wrote the store (%d puts, want %d) or restarted web", store.puts, puts)
	}

	// A changed source is copied again, as Render re-copies on each sync.
	if _, err := svc.SetEnvVar(ctx, "database", "PASSWORD", EnvVarWrite{Value: "rotated-2"}); err != nil {
		t.Fatal(err)
	}
	if err := copier.CopyEnvVars(ctx, "web", webCopies); err != nil {
		t.Fatal(err)
	}
	if got := store.m[envPath("web")]["DB_PASSWORD"]; got != "rotated-2" {
		t.Fatalf("after the source changed, web DB_PASSWORD = %q, want rotated-2", got)
	}
}

func TestEnvCopierRefusesACallerWhoCannotRevealTheSource(t *testing.T) {
	svc, store := copyService(t)
	svc.Authz = denyingChecker{deny: core.RelCanViewSensitive}

	if err := NewEnvCopier(svc).CopyEnvVars(context.Background(), "web", webCopies); !errors.Is(err, core.ErrForbidden) && !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err = %v, want the sensitive-read refusal", err)
	}
	if _, present := store.m[envPath("web")]["DB_PASSWORD"]; present {
		t.Fatal("the refused copy still wrote the receiver")
	}
}

func TestEnvCopierReportsAMissingSourceKey(t *testing.T) {
	svc, _ := copyService(t)
	err := NewEnvCopier(svc).CopyEnvVars(context.Background(), "web", []core.EnvVarCopy{{From: "database", FromKey: "ABSENT", ToKey: "X"}})
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
}

// The same against a real OpenBao (CI and /ship's gate provide one): the
// copy's read, compare and seed run over the store production writes to.
func TestEnvCopierAgainstOpenBao(t *testing.T) {
	bao := realBatchBao(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	source, receiver := "m175-db-"+suffix, "m175-web-"+suffix
	svc := newService(bao, sampleApp(source), sampleApp(receiver))
	var n atomic.Int64
	svc.Clock = func() time.Time { return time.Unix(1_000_000+n.Add(1), 0).UTC() }
	ctx := context.Background()
	if _, err := svc.SetEnvVar(ctx, source, "PASSWORD", EnvVarWrite{GenerateValue: true}); err != nil {
		t.Fatal(err)
	}
	minted, err := svc.GetEnvVar(ctx, source, "PASSWORD")
	if err != nil || len(minted.Value) != 44 {
		t.Fatalf("minted = %+v, %v", minted, err)
	}
	copies := []core.EnvVarCopy{{From: source, FromKey: "PASSWORD", ToKey: "DB_PASSWORD"}}
	copier := NewEnvCopier(svc)

	if err := copier.CopyEnvVars(ctx, receiver, copies); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if got, err := svc.GetEnvVar(ctx, receiver, "DB_PASSWORD"); err != nil || got.Value != minted.Value {
		t.Fatalf("receiver DB_PASSWORD = %+v err=%v, want the source's value", got, err)
	}
	restarted := getApp(t, svc.Client, receiver).Spec.RestartedAt
	if err := copier.CopyEnvVars(ctx, receiver, copies); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, svc.Client, receiver).Spec.RestartedAt; got != restarted {
		t.Fatalf("an unchanged copy restarted the receiver: %q -> %q", restarted, got)
	}
}
