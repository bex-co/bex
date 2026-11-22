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
// and its Terraform provider reads it straight after the create. w9/m521: in
// Render's shapes. A store the operator has reconciled once (host, port and
// credential Secret recorded, its Service routed through the proxy from
// birth, its id user in the new template) but whose pods have not served yet
// answers on REST (both dialects) and GraphQL:
//
//   - the external URL with the id as its user, though the TLS listener is
//     not open yet, with externalCertificate pending;
//   - the internal URL with no credentials;
//   - Render's CLI command over the external endpoint, with bex's --sni.
//
// w9/134: a private store answers the same three strings over the host its
// endpoint keeps, as Render answers a store created with ipAllowList: []
// (the provider's cassettes at pin 06af456), and no externalCertificate: it
// has no Certificate, and the edge routes it only once a rule makes it public.
//
// MCP has no credential read, as Render's MCP server has none.
func TestConnectionInfoServesAStoreWithNoRecordedRevision(t *testing.T) {
	password := []byte("first-provision-password")
	for _, public := range []bool{true, false} {
		t.Run(fmt.Sprintf("public=%v", public), func(t *testing.T) {
			kv := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: "red-creating", Namespace: "default"},
				Spec:       appv1alpha1.KeyValueSpec{Name: "creating", Public: public},
				Status: appv1alpha1.KeyValueStatus{
					Phase: appv1alpha1.KVPhaseProvisioning, Host: "red-creating.default.svc", Port: 6379,
					SecretName: "red-creating", CredentialSecretName: "red-creating-auth",
					InternalAuthOptional: true, Username: "red-creating",
					ReservedExternalHost: "red-creating.kv.bex.co",
				},
			}
			wantCertificate := ""
			if public {
				// The external host is set on the first pass; its TLS
				// listener opens once the certificate is issued.
				kv.Status.ExternalHost = "red-creating.kv.bex.co"
				wantCertificate = core.ExternalCertificatePending
			}
			auth := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "red-creating-auth", Namespace: "default"},
				Data: map[string][]byte{"username": []byte("default"), "password": password}}
			svc, _ := newService(kv, auth)
			want := map[string]string{
				"internalConnectionString": "redis://red-creating:6379",
				"externalConnectionString": "rediss://red-creating:" + string(password) + "@red-creating.kv.bex.co:6379",
				"cliCommand": " REDISCLI_AUTH=" + string(password) +
					" valkey-cli --user red-creating -h red-creating.kv.bex.co --sni red-creating.kv.bex.co -p 6379 --tls",
			}

			for path, cliField := range map[string]string{
				"/v1/key-value/red-creating/connection-info": "cliCommand",
				"/v1/redis/red-creating/connection-info":     "redisCLICommand",
			} {
				rec := serveREST(svc, http.MethodGet, path, "")
				if rec.Code != http.StatusOK {
					t.Fatalf("GET %s => %d %s, want 200", path, rec.Code, rec.Body.String())
				}
				body := decodeMap(t, rec.Body.Bytes())
				for field, value := range want {
					if field == "cliCommand" {
						field = cliField
					}
					if body[field] != value {
						t.Errorf("GET %s %s = %q, want %q", path, field, body[field], value)
					}
				}
				if got, _ := body["externalCertificate"].(string); path == "/v1/key-value/red-creating/connection-info" && got != wantCertificate {
					t.Errorf("GET %s externalCertificate = %q, want %q", path, got, wantCertificate)
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
			for field, value := range want {
				if gql[field] != value {
					t.Errorf("GraphQL %s = %q, want %q", field, gql[field], value)
				}
			}
		})
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
		InternalAuthOptional: true, Username: name,
		ReservedExternalHost: name + ".kv.bex.co",
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
			// A private store answers Render's external strings too (w9/134).
			want := KeyValueConnectionInfo{InternalConnectionString: "redis://" + name + ":6379",
				ExternalConnectionString: "rediss://" + name + ":" + string(password) + "@" + name + ".kv.bex.co:6379",
				CLICommand:               " REDISCLI_AUTH=" + string(password) + " valkey-cli --user " + name + " -h " + name + ".kv.bex.co --sni " + name + ".kv.bex.co -p 6379 --tls"}
			if public {
				want.ExternalCertificate = core.ExternalCertificatePending
			}
			if err != nil || info != want {
				t.Fatalf("connection info across the first reconcile = %+v, %v; want %+v", info, err, want)
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
