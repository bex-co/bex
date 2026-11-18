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
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/108: Render answers connection-info for a store that is still creating,
// and its Terraform provider reads it straight after the create. A store the
// operator has reconciled once (host, port and credential Secret recorded) but
// whose pods have not served yet has no recorded revision, and its unchanging
// password is served on REST (both dialects) and GraphQL. MCP has no
// credential read, as Render's MCP server has none.
func TestConnectionInfoServesAStoreWithNoRecordedRevision(t *testing.T) {
	password := []byte("first-provision-password")
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-creating", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Name: "creating", Public: true},
		Status: appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseProvisioning, Host: "red-creating.default.svc", Port: 6379,
			SecretName: "red-creating", CredentialSecretName: "red-creating-auth",
			// The external host is set on the first pass; its TLS listener
			// opens once the certificate is issued (w8/237).
			ExternalHost: "red-creating.kv.bex.co",
		},
	}
	auth := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "red-creating-auth", Namespace: "default"},
		Data: map[string][]byte{"username": []byte("default"), "password": password}}
	svc, _ := newService(kv, auth)
	wantInternal := "redis://default:" + string(password) + "@red-creating.default.svc:6379"

	for path, cliField := range map[string]string{
		"/v1/key-value/red-creating/connection-info": "cliCommand",
		"/v1/redis/red-creating/connection-info":     "redisCLICommand",
	} {
		rec := serveREST(svc, http.MethodGet, path, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s => %d %s, want 200", path, rec.Code, rec.Body.String())
		}
		body := decodeMap(t, rec.Body.Bytes())
		if body["internalConnectionString"] != wantInternal {
			t.Errorf("GET %s internalConnectionString = %v, want %s", path, body["internalConnectionString"], wantInternal)
		}
		if ext, ok := body["externalConnectionString"]; !ok || ext != "" {
			t.Errorf("GET %s externalConnectionString = %v (present %v), want \"\" until the TLS listener opens", path, ext, ok)
		}
		if cli, _ := body[cliField].(string); !strings.Contains(cli, wantInternal) {
			t.Errorf("GET %s %s = %q, want one connecting to %s", path, cliField, cli, wantInternal)
		}
	}

	schema, err := kvGQLSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctxAs("user-a"),
		RequestString: `{ keyValueConnectionInfo(id: "red-creating") { internalConnectionString externalConnectionString cliCommand } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", res.Errors)
	}
	gql, _ := res.Data.(map[string]any)["keyValueConnectionInfo"].(map[string]any)
	if gql["internalConnectionString"] != wantInternal || gql["externalConnectionString"] != "" {
		t.Fatalf("GraphQL connection info = %v, want internal %s and an empty external string", gql, wantInternal)
	}
}

// newbornKeyValue is a store as the create leaves it: no status, no Secret.
func newbornKeyValue(name string, public bool) *appv1alpha1.KeyValue {
	return &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Name: name, Public: public},
	}
}

// firstPass does what the operator's first reconcile records for a new store:
// the credential Secret, then the host, Secret names and phase.
func firstPass(cl client.Client, name string, public bool, password []byte) error {
	ctx := context.Background()
	if err := cl.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-auth", Namespace: "default"},
		Data: map[string][]byte{"username": []byte("default"), "password": password}}); err != nil {
		return err
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, &kv); err != nil {
		return err
	}
	kv.Status = appv1alpha1.KeyValueStatus{
		Phase: appv1alpha1.KVPhaseProvisioning, Host: name + ".default.svc", Port: 6379,
		SecretName: name, CredentialSecretName: name + "-auth",
	}
	if public {
		kv.Status.ExternalHost = name + ".kv.bex.co"
	}
	return cl.Update(ctx, &kv)
}

// w9/108: the provider reads connection info a few milliseconds after the
// create returns, and the operator's first pass can land after that. Without
// waiting, the read answered 404 before the pass wrote the Secret, and a public
// store 503 between the Secret and its external host, so the create still
// failed. The read waits a bounded time for that pass.
func TestConnectionInfoWaitsForTheFirstReconcile(t *testing.T) {
	password := []byte("newborn-password")
	for _, public := range []bool{false, true} {
		t.Run(fmt.Sprintf("public=%v", public), func(t *testing.T) {
			name := fmt.Sprintf("red-newborn-%v", public)
			svc, cl := newService(newbornKeyValue(name, public))
			passed := make(chan error, 1)
			go func() {
				time.Sleep(100 * time.Millisecond)
				passed <- firstPass(cl, name, public, password)
			}()
			info, err := svc.KeyValueConnectionInfo(context.Background(), name)
			if passErr := <-passed; passErr != nil {
				t.Fatalf("first pass: %v", passErr)
			}
			want := "redis://default:" + string(password) + "@" + name + ".default.svc:6379"
			if err != nil || info.InternalConnectionString != want {
				t.Fatalf("connection info across the first reconcile = %+v, %v; want internal %s", info, err, want)
			}
		})
	}

	t.Run("a store never reconciled is answered as before once the wait is over", func(t *testing.T) {
		defer func(d time.Duration) { firstReconcileWait = d }(firstReconcileWait)
		firstReconcileWait = 150 * time.Millisecond
		svc, _ := newService(newbornKeyValue("red-stalled", true))
		start := time.Now()
		_, err := svc.KeyValueConnectionInfo(context.Background(), "red-stalled")
		if elapsed := time.Since(start); !errors.Is(err, core.ErrNotFound) || elapsed < firstReconcileWait || elapsed > 5*firstReconcileWait {
			t.Errorf("unreconciled store = %v after %v, want ErrNotFound (no Secret yet) after about %v", err, elapsed, firstReconcileWait)
		}
	})

	t.Run("a store deleted during the wait is not found", func(t *testing.T) {
		kv := newbornKeyValue("red-gone", false)
		svc, cl := newService(kv)
		go func() {
			time.Sleep(100 * time.Millisecond)
			_ = cl.Delete(context.Background(), kv.DeepCopy())
		}()
		start := time.Now()
		if _, err := svc.KeyValueConnectionInfo(context.Background(), kv.Name); !errors.Is(err, core.ErrNotFound) || time.Since(start) > time.Second {
			t.Fatalf("deleted store = %v after %v, want ErrNotFound at once", err, time.Since(start))
		}
	})
}
