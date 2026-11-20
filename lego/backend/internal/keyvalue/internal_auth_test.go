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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w9/m521: connection info answers Render's three shapes for every state a
// store can be in. The internal URL is credential-free exactly while the
// Service routes through the auth-injecting proxy and internal auth is off,
// and otherwise names the user and the store's id as its host. A public
// store's external URL and Render's CLI command (with bex's --sni) are
// answered whether or not its TLS listener is open, and a private store's
// command connects to the internal host. Every credentialed string names the
// store's id only once status says Valkey knows it, and no string carries the
// password while every string the store has could do without it.
func TestKeyValueConnectionStringsTakeRendersShapes(t *testing.T) {
	const password = "pa-ss_word-0123456789abcdefghijk" // base64url, like credpw's
	for _, tc := range []struct {
		name                                  string
		public, optional, internalAuth, named bool
		listener                              bool
		internal, external, cli               string
	}{{
		name: "private, routed through the proxy", optional: true,
		internal: "redis://red-kv:6379",
		cli:      " valkey-cli -h red-kv -p 6379",
	}, {
		name:     "private, Service not cut over yet (w8/204)",
		internal: "redis://default:" + password + "@red-kv:6379",
		cli:      " REDISCLI_AUTH=" + password + " valkey-cli --user default -h red-kv -p 6379",
	}, {
		name: "private, internal auth, id user", optional: true, internalAuth: true, named: true,
		internal: "redis://red-kv:" + password + "@red-kv:6379",
		cli:      " REDISCLI_AUTH=" + password + " valkey-cli --user red-kv -h red-kv -p 6379",
	}, {
		name: "public, listener closed, default user", public: true, optional: true,
		internal: "redis://red-kv:6379",
		external: "rediss://default:" + password + "@red-kv.kv.bex.co:6379",
		cli:      " REDISCLI_AUTH=" + password + " valkey-cli --user default -h red-kv.kv.bex.co --sni red-kv.kv.bex.co -p 6379 --tls",
	}, {
		name: "public, listener open, id user", public: true, optional: true, named: true, listener: true,
		internal: "redis://red-kv:6379",
		external: "rediss://red-kv:" + password + "@red-kv.kv.bex.co:6379",
		cli:      " REDISCLI_AUTH=" + password + " valkey-cli --user red-kv -h red-kv.kv.bex.co --sni red-kv.kv.bex.co -p 6379 --tls",
	}, {
		name: "public, internal auth, id user", public: true, optional: true, internalAuth: true, named: true, listener: true,
		internal: "redis://red-kv:" + password + "@red-kv:6379",
		external: "rediss://red-kv:" + password + "@red-kv.kv.bex.co:6379",
		cli:      " REDISCLI_AUTH=" + password + " valkey-cli --user red-kv -h red-kv.kv.bex.co --sni red-kv.kv.bex.co -p 6379 --tls",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			kv := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: "red-kv", Namespace: "default"},
				Spec:       appv1alpha1.KeyValueSpec{Public: tc.public, InternalAuth: tc.internalAuth},
				Status: appv1alpha1.KeyValueStatus{
					Phase: appv1alpha1.KVPhaseReady, Host: "red-kv.default.svc", Port: 6379,
					CredentialSecretName: "red-kv-auth", CredentialRevision: appv1alpha1.KeyValueCredentialRevision([]byte(password)),
					// The operator clears the flag while internal auth is on.
					InternalAuthOptional: tc.optional && !tc.internalAuth, TLSListener: tc.listener,
				},
			}
			if tc.named {
				kv.Status.Username = kv.Name
			}
			if tc.public {
				kv.Status.ExternalHost = "red-kv.kv.bex.co"
			}
			sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "red-kv-auth", Namespace: "default"}, Data: map[string][]byte{"password": []byte(password)}}
			svc, _ := newService(kv, sec)
			info, err := svc.KeyValueConnectionInfo(context.Background(), kv.Name)
			if err != nil {
				t.Fatal(err)
			}
			if info.InternalConnectionString != tc.internal || info.ExternalConnectionString != tc.external || info.CLICommand != tc.cli {
				t.Fatalf("connection info = %+v\nwant internal %q, external %q, cli %q", info, tc.internal, tc.external, tc.cli)
			}
			if tc.cli[0] != ' ' || strings.Contains(tc.cli, "redis-cli") {
				t.Fatalf("CLI command %q is not Render's shape", tc.cli)
			}
			if !tc.public && tc.optional && !tc.internalAuth && strings.Contains(fmt.Sprint(info), password) {
				t.Fatalf("a store with no credentialed string handed out its password: %+v", info)
			}
			wantCertificate := ""
			if tc.public {
				wantCertificate = core.ExternalCertificatePending
				if tc.listener {
					wantCertificate = core.ExternalCertificatePubliclyTrusted
				}
			}
			if info.ExternalCertificate != wantCertificate {
				t.Fatalf("externalCertificate = %q, want %q", info.ExternalCertificate, wantCertificate)
			}
		})
	}
}

// A status naming some other user never reaches a string: status.username
// counts only when it is the store's own id.
func TestConnectionStringsNameOnlyTheStoresOwnID(t *testing.T) {
	password := []byte("pw")
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "red-own", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{InternalAuth: true},
		Status: appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady, Host: "red-own.default.svc", Port: 6379,
			CredentialSecretName: "red-own-auth", Username: "bex",
		},
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "red-own-auth", Namespace: "default"}, Data: map[string][]byte{"password": password}}
	svc, _ := newService(kv, sec)
	info, err := svc.KeyValueConnectionInfo(context.Background(), kv.Name)
	if err != nil {
		t.Fatal(err)
	}
	if info.InternalConnectionString != "redis://default:pw@red-own:6379" || !strings.Contains(info.CLICommand, "--user default ") {
		t.Fatalf("a foreign status.username reached the strings: %+v", info)
	}
}

// Optional internal authentication changes the connection URL, never the
// sensitive credential-read authorization or immutable-authority revision gate.
func TestOptionalInternalAuthRetainsSensitiveReadGates(t *testing.T) {
	password := []byte("private-password")
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "optional-gates", Namespace: "default"},
		Status: appv1alpha1.KeyValueStatus{
			Phase: appv1alpha1.KVPhaseReady, Host: "optional-gates.default.svc", Port: 6379,
			CredentialSecretName: "optional-gates-auth", CredentialRevision: appv1alpha1.KeyValueCredentialRevision(password), InternalAuthOptional: true,
		},
	}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "optional-gates-auth", Namespace: "default"}, Data: map[string][]byte{"password": password}}
	t.Run("fresh revocation", func(t *testing.T) {
		svc, _ := newService(kv.DeepCopy(), sec.DeepCopy())
		svc.Authz = staleAllowChecker{}
		info, err := svc.KeyValueConnectionInfo(ctxAs("user-a"), kv.Name)
		if !errors.Is(err, core.ErrForbidden) || info != (KeyValueConnectionInfo{}) {
			t.Fatalf("fresh revocation leaked connection data: %+v %v", info, err)
		}
	})
	t.Run("unserved password", func(t *testing.T) {
		changed := sec.DeepCopy()
		changed.Data["password"] = []byte("unserved-password")
		svc, _ := newService(kv.DeepCopy(), changed)
		info, err := svc.KeyValueConnectionInfo(context.Background(), kv.Name)
		if !errors.Is(err, core.ErrConflict) || info != (KeyValueConnectionInfo{}) || strings.Contains(err.Error(), string(password)) {
			t.Fatalf("mismatched authority leaked connection data: %+v %v", info, err)
		}
	})
}
