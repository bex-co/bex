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
	"os"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestKeyValueLiveIntegration exercises the real Service verbs against a live
// cluster running the operator — Create → wait Ready → ConnectionInfo (internal
// + external strings, public path) → Delete → gone. Gated on BEX_TEST_KUBECONFIG
// (a kubeconfig for a cluster with the KeyValue CRD + reconciler), so it is
// skipped in CI and normal `go test`; the mirror of postgres's gated
// TestQueryReadOnlyIntegration. It talks only to the API server (no exec /
// port-forward), so it runs even where kubelet streaming is unavailable.
//
//	BEX_TEST_KUBECONFIG=/path/to/app.kubeconfig go test ./internal/keyvalue -run Live -v
func TestKeyValueLiveIntegration(t *testing.T) {
	kubeconfig := os.Getenv("BEX_TEST_KUBECONFIG")
	if kubeconfig == "" {
		t.Skip("set BEX_TEST_KUBECONFIG to run the live KeyValue integration test")
	}
	ns := os.Getenv("BEX_TEST_NAMESPACE")
	if ns == "" {
		ns = "default"
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("load kubeconfig: %v", err)
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	svc := &Service{Base: &core.Base{Client: cl, Namespace: ns}}
	ctx := context.Background()
	const name = "m7-live"

	// Best-effort clean slate + guaranteed teardown.
	_ = svc.DeleteKeyValue(ctx, name)
	t.Cleanup(func() {
		_ = svc.DeleteKeyValue(ctx, name)
	})

	// Create a public store (exercises the external connection-string path).
	view, err := svc.CreateKeyValue(ctx, CreateKeyValueRequest{Name: name, Plan: "free", Public: true})
	if err != nil {
		t.Fatalf("CreateKeyValue: %v", err)
	}
	if view.ID != name || view.Plan != "free" || !view.Public {
		t.Fatalf("create view wrong: %+v", view)
	}
	t.Logf("created %s (plan=%s public=%t)", view.ID, view.Plan, view.Public)

	// List + Get see it.
	list, err := svc.ListKeyValues(ctx)
	if err != nil {
		t.Fatalf("ListKeyValues: %v", err)
	}
	found := false
	for _, v := range list {
		if v.ID == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("created store not in list of %d", len(list))
	}

	// Wait for the operator to reconcile to available.
	deadline := time.Now().Add(3 * time.Minute)
	var got KeyValueView
	for {
		got, err = svc.GetKeyValue(ctx, name)
		if err != nil {
			t.Fatalf("GetKeyValue: %v", err)
		}
		t.Logf("status=%s", got.Status)
		if got.Status == "available" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("did not reach available (last status %q)", got.Status)
		}
		time.Sleep(5 * time.Second)
	}

	// ConnectionInfo: internal redis://, external rediss:// (public), cliCommand
	// over the external endpoint, and no leak of a standalone password field.
	ci, err := svc.KeyValueConnectionInfo(ctx, name)
	if err != nil {
		t.Fatalf("KeyValueConnectionInfo: %v", err)
	}
	if !strings.HasPrefix(ci.InternalConnectionString, "redis://:") {
		t.Errorf("internal string = %q", redact(ci.InternalConnectionString))
	}
	if !strings.HasPrefix(ci.ExternalConnectionString, "rediss://:") {
		t.Errorf("external string (public store) = %q", redact(ci.ExternalConnectionString))
	}
	if !strings.HasPrefix(ci.CLICommand, "redis-cli -u rediss://") {
		t.Errorf("cliCommand should use the external TLS endpoint, got %q", redact(ci.CLICommand))
	}
	t.Logf("connection-info ok: internal=%s external=%s", redact(ci.InternalConnectionString), redact(ci.ExternalConnectionString))

	// Delete → gone.
	if err := svc.DeleteKeyValue(ctx, name); err != nil {
		t.Fatalf("DeleteKeyValue: %v", err)
	}
	// Best-effort: also drop the StatefulSet's retained PVC (k8s never GCs a
	// StatefulSet's volumeClaimTemplate PVC — "data-<name>-0").
	_ = cl.Delete(ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data-" + name + "-0", Namespace: ns},
	})
	for i := 0; i < 12; i++ {
		if _, err := svc.GetKeyValue(ctx, name); err == core.ErrNotFound {
			t.Log("delete confirmed: store gone")
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("store still present after delete")
}

// redact hides the password embedded in a redis[s]://:<password>@host string.
func redact(s string) string {
	i, j := strings.Index(s, "://:"), strings.Index(s, "@")
	if i >= 0 && j > i {
		return s[:i+4] + "***" + s[j:]
	}
	return s
}
