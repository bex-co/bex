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

package controller

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCopyCloneSecretAcrossNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	src := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "web-clone", Namespace: "default"},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{"token": []byte("ghs_abc")},
	}
	cachedClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	buildClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(src).Build()
	r := &AppReconciler{Client: cachedClient, BuildClient: buildClient}
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "uid-web"}}

	// Copy default/web-clone -> bex-system/web-clone (the build namespace).
	if err := r.copyCloneSecret(context.Background(), app, "default", "bex-system", "web-clone"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	var dst corev1.Secret
	if err := buildClient.Get(context.Background(), client.ObjectKey{Namespace: "bex-system", Name: "web-clone"}, &dst); err != nil {
		t.Fatalf("dst not created: %v", err)
	}
	if string(dst.Data["token"]) != "ghs_abc" {
		t.Errorf("copied token = %q, want ghs_abc", dst.Data["token"])
	}
	if dst.Type != corev1.SecretTypeOpaque {
		t.Errorf("copied type = %v", dst.Type)
	}
	if dst.Labels["app.bex.co/app-uid"] != "uid-web" || dst.Labels["app.bex.co/component"] != "copied-secret" {
		t.Fatalf("copied Secret missing canonical artifact labels: %v", dst.Labels)
	}

	// Idempotent + refreshes: change the source, copy again, dst updates.
	src.Data["token"] = []byte("ghs_fresh")
	if err := buildClient.Update(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if err := r.copyCloneSecret(context.Background(), app, "default", "bex-system", "web-clone"); err != nil {
		t.Fatal(err)
	}
	_ = buildClient.Get(context.Background(), client.ObjectKey{Namespace: "bex-system", Name: "web-clone"}, &dst)
	if string(dst.Data["token"]) != "ghs_fresh" {
		t.Errorf("refresh failed: token = %q, want ghs_fresh", dst.Data["token"])
	}
}
