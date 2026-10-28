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
	"net/url"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestKeyValueConnectionInfoWaitsForInternalAuthCutover(t *testing.T) {
	for _, optional := range []bool{false, true} {
		for _, public := range []bool{false, true} {
			name := "legacy-private"
			if optional {
				name = "optional-private"
			}
			if public {
				name += "-public"
			}
			t.Run(name, func(t *testing.T) {
				password := []byte("controller-owned-secret")
				kv := &appv1alpha1.KeyValue{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
					Spec:       appv1alpha1.KeyValueSpec{Public: public},
					Status: appv1alpha1.KeyValueStatus{
						Phase: appv1alpha1.KVPhaseReady, Host: name + ".default.svc", Port: 6379,
						CredentialSecretName: name + "-auth", CredentialRevision: appv1alpha1.KeyValueCredentialRevision(password),
						InternalAuthOptional: optional,
					},
				}
				if public {
					kv.Status.ExternalHost = name + ".kv.bex.co"
				}
				sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name + "-auth", Namespace: "default"}, Data: map[string][]byte{"password": password}}
				svc, _ := newService(kv, sec)
				info, err := svc.KeyValueConnectionInfo(context.Background(), name)
				if err != nil {
					t.Fatal(err)
				}
				internal, err := url.Parse(info.InternalConnectionString)
				if err != nil {
					t.Fatal(err)
				}
				if optional {
					if info.InternalConnectionString != "redis://"+name+":6379" || internal.User != nil || strings.Contains(info.InternalConnectionString, string(password)) {
						t.Fatalf("cutover internal URL = %q, want credential-free stable hostname", info.InternalConnectionString)
					}
				} else if internal.User == nil || !strings.Contains(info.InternalConnectionString, string(password)) || internal.Host != kv.Status.Host+":6379" {
					t.Fatalf("legacy internal URL changed before cutover: %q", info.InternalConnectionString)
				}
				if public {
					if info.ExternalConnectionString != "rediss://default:"+string(password)+"@"+kv.Status.ExternalHost+":6379" || info.CLICommand != "redis-cli --sni "+kv.Status.ExternalHost+" -u "+info.ExternalConnectionString {
						t.Fatalf("public endpoint lost authenticated TLS: %+v", info)
					}
				} else if info.ExternalConnectionString != "" || info.CLICommand != "redis-cli -u "+info.InternalConnectionString {
					t.Fatalf("private CLI must use its internal URL: %+v", info)
				}
			})
		}
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
