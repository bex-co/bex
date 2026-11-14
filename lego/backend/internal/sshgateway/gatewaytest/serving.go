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

package gatewaytest

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ServingAppID is the service id of the NewServingApp fixture.
const ServingAppID = "srv-d5t5d4v8g3c73f5m9peg"

// ServingApp is the production apps.Service resolver over a fake cluster
// holding one paid web service with one Ready pod of its serving release
// (w4/m352). Revalidation suites drive it through a deploy, a suspension and
// the pod's deletion, so a live stream is judged by the real gate rather than
// a scripted resolver.
type ServingApp struct {
	Resolver *apps.Service
	client   client.Client
}

// NewServingApp builds the fixture: App "web" Running on rev-2, pod
// "web-rs-pod01" Ready on that revision and image.
func NewServingApp(t *testing.T) *ServingApp {
	t.Helper()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{
			core.LabelAppID: ServingAppID, core.LabelTenant: "tea-workspace",
		}, CreationTimestamp: metav1.NewTime(time.Now().UTC().Add(-time.Hour))},
		Spec:   appv1alpha1.AppSpec{Image: "example/web:v2", Type: appv1alpha1.TypeWebService, Tier: "starter", Replicas: 1},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, Image: "example/web:v2", ActiveRevision: "rev-2"},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-rs-pod01", Namespace: "default", Labels: map[string]string{
			core.PodLabelApp: "web", core.PodLabelRevision: "rev-2",
		}, CreationTimestamp: metav1.NewTime(time.Now().UTC())},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: core.AppContainer, Image: "example/web:v2"}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{
			Type: corev1.PodReady, Status: corev1.ConditionTrue,
		}}},
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, pod).Build()
	return &ServingApp{
		Resolver: &apps.Service{Base: &core.Base{Client: c, Namespace: "default"}, SSHHost: "ssh.bex.co"},
		client:   c,
	}
}

// Update applies mutate to the App, as the operator (phase, revision) or
// bex-api (suspension) would mid-session.
func (s *ServingApp) Update(t *testing.T, mutate func(*appv1alpha1.App)) {
	t.Helper()
	var app appv1alpha1.App
	if err := s.client.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "web"}, &app); err != nil {
		t.Fatal(err)
	}
	mutate(&app)
	if err := s.client.Update(context.Background(), &app); err != nil {
		t.Fatal(err)
	}
}

// DeletePod removes the serving pod: the instance the session runs on stops.
func (s *ServingApp) DeletePod(t *testing.T) {
	t.Helper()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "web-rs-pod01"}}
	if err := s.client.Delete(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
}
