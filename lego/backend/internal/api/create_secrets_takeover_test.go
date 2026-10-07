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

package api

import (
	"context"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCreatingAServiceBesideANamesakesSecret (w5/128): a create that carries
// files while the deleted namesake's Secret awaits garbage collection answers
// 201 and mounts only its own files. A Secret another create prepared and has
// not adopted yet answers an uncoded 409, not 500.
func TestCreatingAServiceBesideANamesakesSecret(t *testing.T) {
	const body = `{"name":"web","type":"web_service","image":{"imagePath":"nginx:alpine","ownerId":""},"secretFiles":[{"name":"token","content":"new"}]}`
	held := func(owner types.UID) *corev1.Secret {
		sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "web-files", Namespace: "default"}, Data: map[string][]byte{"stale": []byte("old")}}
		if owner != "" {
			sec.OwnerReferences = []metav1.OwnerReference{{APIVersion: appv1alpha1.SchemeGroupVersion.String(), Kind: "App", Name: "web", UID: owner, Controller: new(true)}}
		}
		return sec
	}

	t.Run("a deleted namesake's", func(t *testing.T) {
		cl := fakeClient(held("uid-deleted"))
		h, _ := serverWith(t, &core.Base{Client: cl, Namespace: "default"}, Deps{Secrets: newMemSecretStore()})
		if res := do(t, h, http.MethodPost, "/v1/services", testToken, body); res.Code != http.StatusCreated {
			t.Fatalf("create = %d %s, want 201", res.Code, res.Body)
		}
		var sec corev1.Secret
		if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web-files"}, &sec); err != nil {
			t.Fatal(err)
		}
		if len(sec.Data) != 1 || string(sec.Data["token"]) != "new" || len(sec.OwnerReferences) != 1 || sec.OwnerReferences[0].UID == "uid-deleted" {
			t.Fatalf("Secret files %d, owners %v; want only the new file, owned by the new App", len(sec.Data), sec.OwnerReferences)
		}
	})
	t.Run("another create's", func(t *testing.T) {
		h, _ := serverWith(t, &core.Base{Client: fakeClient(held("")), Namespace: "default"}, Deps{Secrets: newMemSecretStore()})
		refused := codedRefusal{status: http.StatusConflict, msg: "conflict: a service of this name is still being created or deleted; retry shortly"}
		refused.onREST(t, h, http.MethodPost, "/v1/services", body)
	})
}
