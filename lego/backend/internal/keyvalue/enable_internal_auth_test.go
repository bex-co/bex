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
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/m268: Render's Enable Internal Authentication — REST, GraphQL and MCP
// share one enable-only verb that writes spec.internalAuth and reports it.
func TestEnableInternalAuth(t *testing.T) {
	svc, cl := newService()
	seedKeyValue(t, cl, "red-auth")

	res := serveREST(svc, http.MethodPost, "/v1/key-value/red-auth/internal-auth", "")
	if res.Code != http.StatusAccepted {
		t.Fatalf("enable: %d %s", res.Code, res.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || body["internalAuth"] != true {
		t.Fatalf("enable body = %s", res.Body.String())
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "red-auth"}, &kv); err != nil {
		t.Fatal(err)
	}
	if !kv.Spec.InternalAuth {
		t.Fatal("spec.internalAuth was not written")
	}
	// Enabling again is a no-op that still answers the store.
	version := kv.ResourceVersion
	v, err := svc.EnableInternalAuth(context.Background(), "red-auth")
	if err != nil || !v.InternalAuth {
		t.Fatalf("second enable: %+v %v", v, err)
	}
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(&kv), &kv); err != nil || kv.ResourceVersion != version {
		t.Fatalf("second enable rewrote the store: %v", err)
	}
	if _, err := svc.EnableInternalAuth(context.Background(), "red-missing"); err == nil {
		t.Fatal("unknown store must be refused")
	}
}

// Once internal auth is enabled the internal URL carries the password even
// while status still says the proxy cutover happened: the Service may already
// route straight to Valkey, which requires AUTH.
func TestConnectionInfoCarriesThePasswordOnceInternalAuthIsEnabled(t *testing.T) {
	password := []byte("controller-owned-secret")
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-on", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{InternalAuth: true},
		Status: appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady, Host: "red-on.default.svc", Port: 6379,
			CredentialSecretName: "red-on-auth", CredentialRevision: appv1alpha1.KeyValueCredentialRevision(password),
			InternalAuthOptional: true, // not yet persisted as false
		},
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "red-on-auth", Namespace: "default"}, Data: map[string][]byte{"password": password}}
	svc, _ := newService(kv, sec)
	info, err := svc.KeyValueConnectionInfo(context.Background(), "red-on")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info.InternalConnectionString, string(password)) {
		t.Fatalf("internal URL = %q, want the password", info.InternalConnectionString)
	}
}
