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
	"crypto/sha256"
	"fmt"
	"io"
	"maps"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

type imageNetworkTransport func(*http.Request) (*http.Response, error)

func (f imageNetworkTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestImageNetworkUsesVerifiedBuildMetadata(t *testing.T) {
	config := `{"architecture":"amd64","os":"linux","config":{"ExposedPorts":{"8123/tcp":{},"9000/tcp":{},"9009/tcp":{}}}}`
	configDigest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(config)))
	manifest := fmt.Sprintf(`{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"digest":%q}}`, configDigest)
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(manifest)))
	requests := 0
	httpClient := &http.Client{Transport: imageNetworkTransport(func(request *http.Request) (*http.Response, error) {
		requests++
		var body string
		switch request.URL.Path {
		case "/v2/clickhouse/manifests/gen-1", "/v2/clickhouse/manifests/" + digest:
			body = manifest
		case "/v2/clickhouse/blobs/" + configDigest:
			body = config
		default:
			t.Fatalf("unexpected metadata request %s", request.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	r := &AppReconciler{Registry: "registry.test", HTTPClient: httpClient}
	app := projectionApp(func(a *appv1alpha1.App) {
		a.Name = "clickhouse"
		a.Generation = 1
		a.Spec.Type = appv1alpha1.TypePrivateService
		a.Spec.PortMode = appv1alpha1.PortModeImageV1
		a.Spec.Port = 3000
	})
	image, port, err := r.resolveImageNetwork(context.Background(), app, "registry.test/clickhouse:gen-1")
	if err != nil {
		t.Fatal(err)
	}
	if image != "registry.test/clickhouse@"+digest || port != 8123 {
		t.Fatalf("image=%q port=%d", image, port)
	}
	container := appContainer(app, deploymentParams{image: image, port: port, replicas: 1})
	if len(container.Ports) != 3 || container.StartupProbe.TCPSocket == nil || container.StartupProbe.TCPSocket.Port.IntVal != 8123 {
		t.Fatalf("wrong listener/probe projection: %+v", container)
	}
	if _, _, err := r.resolveImageNetwork(context.Background(), app, image); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("immutable metadata cache made %d requests", requests)
	}
	if _, _, err := r.resolveImageNetwork(context.Background(), app, "registry.test/someone-else:gen-1"); err == nil {
		t.Fatal("inspected another tenant's image")
	}
	if requests != 2 {
		t.Fatal("unauthorized image reached registry")
	}
	// An explicit port is distinct from an omitted 3000, and changing the
	// override must rederive the set so a prior manual listener does not linger.
	app.Spec.PortMode = appv1alpha1.PortModeImageConfiguredV1
	for _, primary := range []int32{9000, 3000, 8123} {
		app.Spec.Port = primary
		_, gotPort, err := r.resolveImageNetwork(t.Context(), app, image)
		if err != nil || int32(gotPort) != primary {
			t.Fatalf("explicit %d resolved %d: %v", primary, gotPort, err)
		}
		wantPorts := []int32{8123, 9000, 9009}
		if primary == 3000 {
			wantPorts = []int32{3000, 8123, 9000, 9009}
		}
		if !reflect.DeepEqual(app.Status.ImageNetwork.Ports, wantPorts) {
			t.Fatalf("explicit primary discarded or retained the wrong listeners: %v", app.Status.ImageNetwork.Ports)
		}
	}
}

func TestImageNetworkPromotesBothHostnames(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "clickhouse", Namespace: "default", UID: "fixture", Generation: 2},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypePrivateService, PortMode: appv1alpha1.PortModeImageV1, Subdomain: "clickhouse-qa", Port: 3000},
	}
	app.Status.ReleaseGeneration = 2
	app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: "old@digest", Revision: "rev-1", PrimaryPort: 3000, Ports: []int32{3000}}
	app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "new@digest", Revision: releaseRevision(app), PrimaryPort: 8123, Ports: []int32{8123, 9000, 9009}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}
	ctx := context.Background()
	for _, name := range []string{app.Name, app.Spec.Subdomain} {
		if err := r.applyClusterIPService(ctx, app, name, 8123); err != nil {
			t.Fatal(err)
		}
		var service corev1.Service
		if err := cl.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &service); err != nil {
			t.Fatal(err)
		}
		if len(service.Spec.Ports) != 1 || service.Spec.Ports[0].Port != 3000 || service.Spec.Selector[labelRevision] != "rev-1" {
			t.Fatalf("candidate changed active service: %+v", service.Spec)
		}
	}
	if app.InternalAddress() != "clickhouse-qa:3000" {
		t.Fatal("candidate changed active address")
	}
	if err := r.promoteImageNetwork(ctx, app, "new@digest", 8123); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{app.Name, app.Spec.Subdomain} {
		var service corev1.Service
		if err := cl.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &service); err != nil {
			t.Fatal(err)
		}
		ports := make([]int32, 0, len(service.Spec.Ports))
		for _, port := range service.Spec.Ports {
			ports = append(ports, port.Port)
			if port.Name == "" {
				t.Fatal("unnamed multiport service")
			}
		}
		if !reflect.DeepEqual(ports, []int32{8123, 9000, 9009}) || service.Spec.Selector[labelRevision] != releaseRevision(app) {
			t.Fatalf("wrong promoted service: %+v", service.Spec)
		}
	}
	if app.InternalAddress() != "clickhouse-qa:8123" {
		t.Fatal("serving address did not move")
	}
	if err := r.promoteImageNetwork(ctx, app, "mismatched@digest", 8123); err == nil {
		t.Fatal("mismatched image promoted")
	}
}

func TestImageNetworkFailedPublicationPreservesServingStatus(t *testing.T) {
	for _, failName := range []string{"clickhouse", "clickhouse-qa"} {
		t.Run(failName, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			_ = appv1alpha1.AddToScheme(scheme)
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "clickhouse", Namespace: "default", UID: "fixture", Generation: 2},
				Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypePrivateService,
					PortMode: appv1alpha1.PortModeImageV1, Subdomain: "clickhouse-qa"},
			}
			app.Status.ReleaseGeneration = 2
			app.Status.ServingNetwork = &appv1alpha1.ImageNetworkStatus{Image: "old", Revision: "rev-1", PrimaryPort: 3000, Ports: []int32{3000}}
			app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{Image: "new", Revision: "rev-2", PrimaryPort: 8123, Ports: []int32{8123, 9000}}
			old := app.Status.ServingNetwork.DeepCopy()
			cl := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
				Create: func(ctx context.Context, underlying client.WithWatch, object client.Object, opts ...client.CreateOption) error {
					if object.GetName() == failName {
						return fmt.Errorf("injected service write failure")
					}
					return underlying.Create(ctx, object, opts...)
				},
			}).Build()
			r := &AppReconciler{Client: cl, Scheme: scheme}
			if err := r.promoteImageNetwork(t.Context(), app, "new", 8123); err == nil {
				t.Fatal("service write unexpectedly succeeded")
			}
			if !reflect.DeepEqual(app.Status.ServingNetwork, old) || app.InternalAddress() != "clickhouse-qa:3000" {
				t.Fatal("failed publication changed the serving address")
			}
		})
	}
}

func TestImageNetworkLeavesLegacyServicesUnchanged(t *testing.T) {
	app := projectionApp()
	app.Status.ImageNetwork = &appv1alpha1.ImageNetworkStatus{PrimaryPort: 8123, Ports: []int32{8123, 9000}}
	selector, ports, err := privateServiceProjection(app, 3000)
	if _, revisioned := selector[labelRevision]; err != nil || revisioned || len(ports) != 1 || ports[0].Port != 3000 || ports[0].Name != "" {
		t.Fatalf("legacy routing changed: %v %v %v", selector, ports, err)
	}
	app.Spec.Type = appv1alpha1.TypePrivateService
	app.Spec.PortMode = appv1alpha1.PortModeImageV1
	app.Status.ImageNetwork = nil
	if app.InternalAddress() != "" {
		t.Fatal("unresolved service invented an address")
	}
	if _, _, err := privateServiceProjection(app, 3000); err == nil {
		t.Fatal("unresolved discovery silently used port 3000")
	}
}

func TestImageNetworkCancelRestoresServingRelease(t *testing.T) {
	for _, recorded := range []bool{false, true} {
		t.Run(fmt.Sprintf("recorded-template=%t", recorded), func(t *testing.T) {
			ctx := t.Context()
			serving := &appv1alpha1.ImageNetworkStatus{
				Image: "registry.test/clickhouse@sha256:served", Revision: "rev-1",
				PrimaryPort: 8123, Ports: []int32{8123, 9000, 9009},
			}
			candidate := &appv1alpha1.ImageNetworkStatus{
				Image: "registry.test/clickhouse@sha256:canceled", Revision: "rev-2",
				PrimaryPort: 9000, Ports: []int32{9000, 9009},
			}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name: "clickhouse", Namespace: "default", UID: "fixture", Generation: 2,
					Annotations: map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "2"},
				},
				Spec: appv1alpha1.AppSpec{
					Type: appv1alpha1.TypePrivateService, Subdomain: "clickhouse-qa",
					Repo: "https://github.com/render-examples/clickhouse", Port: 9000,
					PortMode: appv1alpha1.PortModeImageConfiguredV1,
				},
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseDeploying, ActiveRevision: "rev-1",
					ReleaseGeneration: 2, ObservedGeneration: 1,
					// A failed candidate has already reached reportRolloutProgress,
					// which records its image without making it the serving release.
					Image: candidate.Image, ArtifactImage: candidate.Image,
					ImageNetwork: candidate.DeepCopy(), ServingNetwork: serving.DeepCopy(),
				},
			}
			savedSpec := app.Spec.DeepCopy()
			cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(app).
				WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
			if recorded {
				prior := app.DeepCopy()
				prior.Status.ReleaseGeneration = 1
				prior.Status.ImageNetwork = serving.DeepCopy()
				dep := project(prior, deploymentParams{image: serving.Image, port: 8123, replicas: 1})
				if err := r.recordReleasePodTemplate(ctx, prior, dep.Spec.Template); err != nil {
					t.Fatal(err)
				}
			}
			if !prepareAppReleaseDecision(app).canceled {
				t.Fatal("fixture did not select the canceled release path")
			}
			if _, err := r.settleCanceledRelease(ctx, app, 9000); err != nil {
				t.Fatal(err)
			}
			var dep appsv1.Deployment
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &dep); err != nil {
				t.Fatal(err)
			}
			container := appContainerOf(t, &dep)
			if container.Image != serving.Image || container.StartupProbe.TCPSocket.Port.IntVal != 8123 || len(container.Ports) != 3 {
				t.Fatalf("canceled candidate changed the restored container: %+v", container)
			}
			dep.Status = appsv1.DeploymentStatus{
				ObservedGeneration: dep.Generation, UpdatedReplicas: 1,
				Replicas: 1, AvailableReplicas: 1, ReadyReplicas: 1,
			}
			if err := cl.Status().Update(ctx, &dep); err != nil {
				t.Fatal(err)
			}
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), app); err != nil {
				t.Fatal(err)
			}
			prepareAppReleaseDecision(app)
			if _, err := r.settleCanceledRelease(ctx, app, 9000); err != nil {
				t.Fatalf("restored release could not settle ready: %v", err)
			}
			if app.Status.Phase != appv1alpha1.PhaseRunning || app.Status.Image != serving.Image ||
				app.InternalAddress() != "clickhouse-qa:8123" || !reflect.DeepEqual(app.Status.ServingNetwork, serving) {
				t.Fatalf("cancel did not preserve the serving release: %+v", app.Status)
			}
			if !reflect.DeepEqual(&app.Spec, savedSpec) || app.Status.ArtifactImage != candidate.Image {
				t.Fatal("cancel lost the saved configuration or the artifact for a later deploy")
			}
			for _, name := range []string{app.Name, app.Spec.Subdomain} {
				var svc corev1.Service
				if err := cl.Get(ctx, client.ObjectKey{Namespace: app.Namespace, Name: name}, &svc); err != nil {
					t.Fatal(err)
				}
				if svc.Spec.Selector[labelRevision] != serving.Revision || len(svc.Spec.Ports) != 3 || svc.Spec.Ports[0].Port != 8123 {
					t.Fatalf("cancel changed %s routing: %+v", name, svc.Spec)
				}
			}
		})
	}
}

// TestServingSelectorExcludesJobPods is w4/m181: both stable Services used the
// bare labelApp selector, which pre-deploy Job pods also carry. Live, a ready
// migration pod (no listener) joined the endpoints and a share of public
// requests answered 502 while it ran. The selector must admit the runtime pod
// template, old and new revisions alike, and no Job pod of the same App.
func TestServingSelectorExcludesJobPods(t *testing.T) {
	for _, app := range []*appv1alpha1.App{
		projectionApp(func(a *appv1alpha1.App) { a.Labels = map[string]string{labelAppID: "srv-hello"} }),
		projectionApp(), // hand-applied: no public id, the name stands in
	} {
		raw, _, err := privateServiceProjection(app, 3000)
		if err != nil {
			t.Fatal(err)
		}
		selector := labels.SelectorFromSet(raw)
		serving := appPodLabels(app, false)
		if !selector.Matches(labels.Set(serving)) {
			t.Errorf("%s: selector %v drops the runtime pod %v", app.Name, raw, serving)
		}
		previous := maps.Clone(serving)
		previous[labelRevision] = "rev-1"
		if !selector.Matches(labels.Set(previous)) {
			t.Errorf("%s: selector %v drops the previous revision's pod during a rolling deploy", app.Name, raw)
		}
		jobs := []map[string]string{
			diskBackupLabels(app),
			execution.PodLabels(app.Name, "uid", predeploy.ComponentValue, "tea-1", app.Namespace, false),
			execution.PodLabels(app.Name, "uid", "build", "tea-1", app.Namespace, false),
		}
		for _, job := range jobs {
			if selector.Matches(labels.Set(job)) {
				t.Errorf("%s: selector %v admits a Job pod %v", app.Name, raw, job)
			}
		}
	}
}
