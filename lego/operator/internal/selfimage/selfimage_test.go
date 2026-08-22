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

package selfimage

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const managerImage = "ghcr.io/bex-co/bex-operator@sha256:" +
	"5efd2d8c754176992ec59cb688ae8aa19b8dc2d71bff542a1c91c76603c9a76e"

func pod(containers ...corev1.Container) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "bex-operator-abc123", Namespace: "bex-system"},
		Spec:       corev1.PodSpec{Containers: containers},
	}
}

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

// TestResolveReadsTheManagerContainerByName: the image must come from the
// container the operator actually runs in. A sidecar (a metrics proxy, a mesh
// injection) landing at index 0 must not become the image derived workloads
// pull — that would silently point the Key Value backup's encrypt stage at
// third-party code holding the plaintext RDB.
func TestResolveReadsTheManagerContainerByName(t *testing.T) {
	scheme := testScheme(t)
	p := pod(
		corev1.Container{Name: "istio-proxy", Image: "docker.io/istio/proxyv2:1.99"},
		corev1.Container{Name: ManagerContainer, Image: managerImage},
	)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(p).Build()

	got, err := Resolve(context.Background(), cl, p.Namespace, p.Name, ManagerContainer)
	if err != nil {
		t.Fatal(err)
	}
	if got != managerImage {
		t.Fatalf("resolved %q, want the manager container's own image", got)
	}
}

// TestResolveFailsRatherThanGuessing covers every way resolution can come up
// empty. None may return a usable-looking image: the caller treats "" as
// "encryption unavailable" and fails closed, which is only safe if this never
// invents a plausible default.
func TestResolveFailsRatherThanGuessing(t *testing.T) {
	scheme := testScheme(t)
	existing := pod(corev1.Container{Name: ManagerContainer, Image: managerImage})

	for _, tc := range []struct {
		name                 string
		namespace, podName   string
		container            string
		objects              []*corev1.Pod
		want                 string
		wantNoPodIdentityErr bool
	}{
		{
			name: "no downward API", namespace: "", podName: "", container: ManagerContainer,
			objects: []*corev1.Pod{existing}, want: "POD_NAME", wantNoPodIdentityErr: true,
		},
		{
			name: "namespace only", namespace: "bex-system", podName: "", container: ManagerContainer,
			objects: []*corev1.Pod{existing}, want: "POD_NAME", wantNoPodIdentityErr: true,
		},
		{
			name: "pod absent", namespace: "bex-system", podName: "gone", container: ManagerContainer,
			want: "read own pod",
		},
		{
			name: "container renamed", namespace: "bex-system", podName: existing.Name, container: "operator",
			objects: []*corev1.Pod{existing}, want: `no container named "operator"`,
		},
		{
			name: "image empty", namespace: "bex-system", podName: "bex-operator-abc123", container: ManagerContainer,
			objects: []*corev1.Pod{pod(corev1.Container{Name: ManagerContainer})}, want: "has no image",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			for _, o := range tc.objects {
				builder = builder.WithObjects(o.DeepCopy())
			}
			got, err := Resolve(context.Background(), builder.Build(), tc.namespace, tc.podName, tc.container)
			if err == nil {
				t.Fatalf("expected a failure, resolved %q", got)
			}
			if got != "" {
				t.Fatalf("a failed resolve must return no image, got %q", got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v does not mention %q", err, tc.want)
			}
			if tc.wantNoPodIdentityErr && !errors.Is(err, ErrNoPodIdentity) {
				t.Fatalf("running outside a Pod must be distinguishable via ErrNoPodIdentity, got %v", err)
			}
		})
	}
}
