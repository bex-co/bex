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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// bigSecret holds five 120 KiB values: 600 KiB, under bex-api's per-set cap.
func bigSecret(name string) *corev1.Secret {
	data := map[string][]byte{}
	for _, key := range []string{"A", "B", "C", "D", "E"} {
		data[name+"_"+key] = []byte(strings.Repeat("x", 120<<10))
	}
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: snapshotTestNS}, Data: data}
}

// A release whose own variables and linked groups together pass the kernel's
// ARG_MAX is refused before its snapshot, so its template never rolls and the
// serving release keeps running; a release under it snapshots as before
// (w4/423).
func TestMergedEnvironmentPastArgMaxIsRefusedBeforeTheSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		groups []string
		refuse bool
	}{
		{"own set and two groups", []string{"g1", "g2"}, false},
		{"own set and three groups", []string{"g1", "g2", "g3"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: snapshotTestNS, UID: "fixture"},
				Spec:       appv1alpha1.AppSpec{EnvFromSecret: "api-env", EnvFromSecrets: tc.groups},
			}
			app.Status.ReleaseGeneration = 2
			objects := []client.Object{app, bigSecret("api-env")}
			for _, g := range tc.groups {
				objects = append(objects, bigSecret(g))
			}
			cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(objects...).WithStatusSubresource(app).Build()
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
			err := r.ensureSavedReleaseConfigSnapshot(context.Background(), app)
			if tc.refuse {
				if err == nil || !strings.Contains(err.Error(), "a process can start with") || app.Status.ConfigSnapshotGeneration != 0 {
					t.Fatalf("err=%v snapshot generation=%d, want a named refusal before the snapshot", err, app.Status.ConfigSnapshotGeneration)
				}
				return
			}
			if err != nil || app.Status.ConfigSnapshotGeneration != 2 {
				t.Fatalf("err=%v snapshot generation=%d, want the snapshot taken", err, app.Status.ConfigSnapshotGeneration)
			}
		})
	}
}

// runc's E2BIG is named as the environment's size, not a missing program.
func TestStartErrorNamesAnOversizedEnvironment(t *testing.T) {
	msg := startErrorMessage(`failed to create containerd task: exec: "/whoami": argument list too long`)
	if !strings.Contains(msg, "environment is too large") || !strings.Contains(msg, "/whoami") || strings.Contains(msg, "does not exist") {
		t.Fatalf("message = %q", msg)
	}
}
