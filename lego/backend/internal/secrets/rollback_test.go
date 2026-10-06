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
	"maps"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func secretData(t *testing.T, cl client.Client, name string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for k, v := range getSecret(t, cl, name).Data {
		out[k] = string(v)
	}
	return out
}

// TestSingleKeyWriteRestoreKeepsAConcurrentCommittedWrite is w5/m119's first
// failure order. Call A CAS-writes its key; call B commits another key and
// answers 200; then A's App patch fails. A's restore wrote the map from before
// A over the store, erasing B's committed write while the Secret B projected
// still mounted it. A single-key write and a batch patch share the restore, so
// both are checked as call A.
func TestSingleKeyWriteRestoreKeepsAConcurrentCommittedWrite(t *testing.T) {
	ctx := context.Background()
	for name, writeA := range map[string]func(*Service) error{
		"single-key write": func(svc *Service) error {
			_, err := svc.SetEnvVar(ctx, "web", "A_KEY", EnvVarWrite{Value: "a"})
			return err
		},
		"batch patch": func(svc *Service) error {
			_, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeDeploy, EnvVars: []EnvVarPatch{{Key: "A_KEY", Value: "a"}}})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			store.m[envPath("web")] = map[string]string{"KEEP": "1"}
			store.versions[envPath("web")] = 1
			cl := &patchCountingClient{Client: fakeClient(sampleApp("web")), fail: errors.New("apiserver unavailable")}
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: store}
			var concurrent error
			cl.beforeFail = func() {
				_, concurrent = svc.SetEnvVar(ctx, "web", "B_KEY", EnvVarWrite{Value: "b"})
			}

			// A's write is superseded rather than undone: the answer says so.
			err := writeA(svc)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_RESTORATION_FAILED" || !errors.Is(err, core.ErrConflict) {
				t.Fatalf("A's failed App patch = %v, want 409 ENVIRONMENT_RESTORATION_FAILED", err)
			}
			if concurrent != nil {
				t.Fatalf("the concurrent write failed: %v", concurrent)
			}
			stored := store.m[envPath("web")]
			if stored["B_KEY"] != "b" {
				t.Fatalf("store after A's failure = %v: A's restore erased B's committed write", stored)
			}
			if mounted := secretData(t, cl, "web-env"); !maps.Equal(mounted, stored) {
				t.Fatalf("the Secret mounts %v while the store lists %v", mounted, stored)
			}
		})
	}
}

// TestSingleKeyWriteRollsBackItsSecretWhenTheAppPatchFails is the second
// order: the write's Secret update lands, then its App patch fails. The store
// was restored but the Secret kept the new value, mounted but not listed.
func TestSingleKeyWriteRollsBackItsSecretWhenTheAppPatchFails(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		path, secret string
		write        func(*Service) error
	}{
		"env var": {envPath("web"), "web-env", func(svc *Service) error {
			_, err := svc.SetEnvVar(ctx, "web", "NEW", EnvVarWrite{Value: "v"})
			return err
		}},
		"secret file": {filesPath("web"), "web-files", func(svc *Service) error {
			_, err := svc.SetSecretFile(ctx, "web", "new.txt", "v")
			return err
		}},
	} {
		t.Run(name, func(t *testing.T) {
			store := newVersionedFakeSecretStore()
			store.m[tc.path] = map[string]string{"KEEP": "1"}
			store.versions[tc.path] = 1
			app := sampleApp("web")
			app.Spec.EnvFromSecret = "web-env"
			app.Spec.FilesFromSecrets = []string{"web-files"}
			existing := func(name string) *corev1.Secret {
				return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Data: map[string][]byte{"KEEP": []byte("1")}}
			}
			cl := &patchCountingClient{Client: fakeClient(app, existing("web-env"), existing("web-files")), fail: errors.New("apiserver unavailable")}
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: store}

			if err := tc.write(svc); err == nil {
				t.Fatal("the failed App patch must fail the call")
			}
			want := map[string]string{"KEEP": "1"}
			if got := store.m[tc.path]; !maps.Equal(got, want) {
				t.Fatalf("store = %v, want the write undone: %v", got, want)
			}
			if got := secretData(t, cl, tc.secret); !maps.Equal(got, want) {
				t.Fatalf("the Secret mounts %v, but the store lists %v", got, want)
			}
		})
	}
}

// TestRestoredWriteKeepsTheSecretAConcurrentFirstWriteReferences: B's first
// env write commits, projects and references its Secret while A's write sits
// between its read and its CAS; then A's App patch fails. A's restore puts B's
// map back, and B's Secret with it. Deciding from the App A read, which
// referenced no Secret, deleted the one B's service mounts.
func TestRestoredWriteKeepsTheSecretAConcurrentFirstWriteReferences(t *testing.T) {
	ctx := context.Background()
	store := newVersionedFakeSecretStore()
	cl := &patchCountingClient{Client: fakeClient(sampleApp("web"))}
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default", Clock: fixedNow}, Store: store}
	var concurrent error
	store.afterGet = func() {
		_, concurrent = svc.SetEnvVar(ctx, "web", "B_KEY", EnvVarWrite{Value: "b"})
		cl.fail = errors.New("apiserver unavailable")
	}

	if _, err := svc.SetEnvVar(ctx, "web", "A_KEY", EnvVarWrite{Value: "a"}); err == nil {
		t.Fatal("A's failed App patch must fail the call")
	}
	if concurrent != nil {
		t.Fatalf("the concurrent write failed: %v", concurrent)
	}
	if ref := getApp(t, cl, "web").Spec.EnvFromSecret; ref != "web-env" {
		t.Fatalf("B's App patch referenced %q, want web-env", ref)
	}
	want := map[string]string{"B_KEY": "b"}
	if got := store.m[envPath("web")]; !maps.Equal(got, want) {
		t.Fatalf("store = %v, want B's write alone: %v", got, want)
	}
	if got := secretData(t, cl, "web-env"); !maps.Equal(got, want) {
		t.Fatalf("web-env mounts %v, want B's map %v", got, want)
	}
}

// TestBatchPatchWhoseEnvWriteWasSupersededSaysSo: a batch patch's env write
// commits, a concurrent write commits on top of it, then the patch's secret
// file is refused. The patch's env change survives in the newer map, so it
// answers ENVIRONMENT_RESTORATION_FAILED, not a 400 that says nothing was saved.
func TestBatchPatchWhoseEnvWriteWasSupersededSaysSo(t *testing.T) {
	ctx := context.Background()
	store := newVersionedFakeSecretStore()
	svc := &Service{Base: &core.Base{Client: fakeClient(sampleApp("web")), Namespace: "default", Clock: fixedNow}, Store: store}
	var concurrent error
	store.afterCAS = func(string, uint64) {
		_, concurrent = svc.SetEnvVar(ctx, "web", "B_KEY", EnvVarWrite{Value: "b"})
	}

	_, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{
		SaveMode:    SaveModeDeploy,
		EnvVars:     []EnvVarPatch{{Key: "A_KEY", Value: "a"}},
		SecretFiles: []SecretFilePatch{{Name: "big.bin", Content: strings.Repeat("x", maxSecretMapBytes)}},
	})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_RESTORATION_FAILED" || !errors.Is(err, core.ErrConflict) {
		t.Fatalf("PatchEnvironment = %v, want 409 ENVIRONMENT_RESTORATION_FAILED", err)
	}
	if concurrent != nil {
		t.Fatalf("the concurrent write failed: %v", concurrent)
	}
	want := map[string]string{"A_KEY": "a", "B_KEY": "b"}
	if got := store.m[envPath("web")]; !maps.Equal(got, want) {
		t.Fatalf("store = %v, want B's write on top of A's: %v", got, want)
	}
}
