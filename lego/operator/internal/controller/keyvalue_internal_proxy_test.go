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
	"encoding/hex"
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestKeyValueInternalProxyCutoverPreservesServerAndRouting(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-test", Namespace: "tea-test", UID: "test"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: "off"}}
	server := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}, Spec: appsv1.StatefulSetSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "valkey", Image: "existing", Args: []string{"--save", "", "--appendonly", "no"}}}}}}}
	original := server.DeepCopy()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(server).WithStatusSubresource(&appsv1.Deployment{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "bex@sha256:fixture"}
	intent := keyValueIntent{authSecretName: "auth", credentialRevision: "revision", tlsListener: true, public: true, labels: map[string]string{labelKeyValue: kv.Name}}
	ready, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent)
	if err != nil || ready {
		t.Fatalf("new front ready=%v err=%v", ready, err)
	}
	if err := r.reconcileKeyValueService(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	service := &corev1.Service{}
	key := client.ObjectKeyFromObject(kv)
	if err := cl.Get(ctx, key, service); err != nil {
		t.Fatal(err)
	}
	if service.Spec.Selector[labelKeyValue] != kv.Name {
		t.Fatal("cut over before proxy readiness")
	}
	backend := &corev1.Service{}
	if err := cl.Get(ctx, client.ObjectKey{Name: keyValueInternalBackendName(kv), Namespace: kv.Namespace}, backend); err != nil {
		t.Fatal(err)
	}
	if backend.Spec.Selector[labelKeyValue] != kv.Name || backend.Spec.Selector[labelKeyValueInternalProxy] != "" {
		t.Fatalf("backend can route to proxy: %v", backend.Spec.Selector)
	}
	dep := &appsv1.Deployment{}
	depKey := client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}
	if err := cl.Get(ctx, depKey, dep); err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec.Command[1] != "--check" {
		t.Fatal("front cutover must use authenticated readiness")
	}
	dep.Status = appsv1.DeploymentStatus{ObservedGeneration: dep.Generation, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	if err := cl.Status().Update(ctx, dep); err != nil {
		t.Fatal(err)
	}
	ready, err = r.reconcileKeyValueInternalProxy(ctx, kv, intent)
	if err != nil || !ready {
		t.Fatalf("ready front=%v err=%v", ready, err)
	}
	intent.internalProxy = ready
	if err := r.reconcileKeyValueService(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, key, service); err != nil {
		t.Fatal(err)
	}
	if service.Spec.Selector[labelKeyValueInternalProxy] != kv.Name || len(service.Spec.Ports) != 2 {
		t.Fatalf("front selector/ports=%v/%v", service.Spec.Selector, service.Spec.Ports)
	}
	// Even if status lags or the front is rolling, routing must never oscillate
	// back to the credential-only backend after a cutover.
	intent.internalProxy = false
	if err := r.reconcileKeyValueService(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, key, service); err != nil {
		t.Fatal(err)
	}
	if service.Spec.Selector[labelKeyValueInternalProxy] != kv.Name {
		t.Fatal("cutover reverted while readiness lagged")
	}
	if err := cl.Get(ctx, key, server); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(server.Spec, original.Spec) {
		t.Fatal("migration changed Off server template and would discard data")
	}
}

// proxyEnv reads a store's proxy Deployment and its container's env values.
func proxyEnv(t *testing.T, cl client.Client, kv *appv1alpha1.KeyValue) (*appsv1.Deployment, map[string]string) {
	t.Helper()
	dep := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}, dep); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	for _, e := range dep.Spec.Template.Spec.Containers[0].Env {
		env[e.Name] = e.Value
	}
	return dep, env
}

// markProxyRolledOut stands in for the Deployment controller: the proxy's
// current template is rolled out and Ready.
func markProxyRolledOut(t *testing.T, cl client.Client, dep *appsv1.Deployment) {
	t.Helper()
	dep.Status = appsv1.DeploymentStatus{ObservedGeneration: dep.Generation, Replicas: 1, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	if err := cl.Status().Update(context.Background(), dep); err != nil {
		t.Fatal(err)
	}
}

// w6/m463: enabling internal auth (w5/m268) keeps the stable Service on the
// store's proxy, so its tenants still meet the plan's limit, and switches the
// proxy to forward each client's own AUTH. The switch is a proxy rollout that
// readiness waits for; the internal URL stops being password-less at once.
func TestKeyValueInternalAuthKeepsTheProxyRoute(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-auth", Namespace: "tea-test", UID: "auth"}, Spec: appv1alpha1.KeyValueSpec{Public: true}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1.Deployment{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "bex@sha256:fixture"}
	intent := keyValueIntent{authSecretName: "auth", credentialRevision: "revision", public: true, tlsRuntime: true, labels: map[string]string{labelKeyValue: kv.Name}}
	if err := r.reconcileKeyValueInternalRoute(ctx, kv, &intent); err != nil {
		t.Fatal(err)
	}
	dep, env := proxyEnv(t, cl, kv)
	if _, set := env["BEX_KV_INTERNAL_AUTH"]; set {
		t.Fatalf("a store without internal auth got a client-auth proxy: %v", env)
	}
	markProxyRolledOut(t, cl, dep)
	if err := r.reconcileKeyValueInternalRoute(ctx, kv, &intent); err != nil {
		t.Fatal(err)
	}
	if !kv.Status.InternalAuthOptional {
		t.Fatal("precondition: the password-less cutover happened")
	}
	dep, _ = proxyEnv(t, cl, kv)
	settled := dep.Spec.Template.DeepCopy()

	kv.Spec.InternalAuth = true
	if err := r.reconcileKeyValueInternalRoute(ctx, kv, &intent); err != nil {
		t.Fatal(err)
	}
	if kv.Status.InternalAuthOptional {
		t.Fatal("internal auth left the internal URL password-less")
	}
	service := &corev1.Service{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(kv), service); err != nil {
		t.Fatal(err)
	}
	if service.Spec.Selector[labelKeyValueInternalProxy] != kv.Name || service.Spec.Selector[labelKeyValue] != "" || len(service.Spec.Ports) != 2 {
		t.Fatalf("an internal-auth store's <id> must select its proxy on both ports: %v %v", service.Spec.Selector, service.Spec.Ports)
	}
	dep, env = proxyEnv(t, cl, kv)
	if env["BEX_KV_INTERNAL_AUTH"] != "client" {
		t.Fatalf("the proxy of an internal-auth store would inject AUTH: %v", env)
	}
	if env["BEX_KV_MAX_CONNECTIONS"] == "" || env["BEX_KV_TLS_ADDR"] == "" {
		t.Fatalf("the client-auth proxy dropped the limit or the TLS listener: %v", env)
	}
	if probe := dep.Spec.Template.Spec.Containers[0].ReadinessProbe; probe == nil || probe.Exec.Command[1] != "--check" {
		t.Fatal("the client-auth proxy lost its authenticated readiness check")
	}
	// The switch is exactly the env var, so the proxy rolls for it and for
	// nothing else.
	switched := dep.Spec.Template.DeepCopy()
	switched.Spec.Containers[0].Env = slices.DeleteFunc(switched.Spec.Containers[0].Env, func(e corev1.EnvVar) bool { return e.Name == "BEX_KV_INTERNAL_AUTH" })
	if !reflect.DeepEqual(switched, settled) {
		t.Fatalf("enabling internal auth changed more of the proxy than its mode:\n%v\n%v", switched, settled)
	}
	// What the API server does on a template change (the fake client keeps
	// the generation): the rollout is pending until it is observed.
	dep.Generation++
	if err := cl.Update(ctx, dep); err != nil {
		t.Fatal(err)
	}
	if r.keyValueInternalProxyReady(ctx, kv) {
		t.Fatal("readiness did not wait for the proxy's switch to roll out")
	}
	markProxyRolledOut(t, cl, dep)
	if !r.keyValueInternalProxyReady(ctx, kv) {
		t.Fatal("the rolled-out client-auth proxy never read ready")
	}
	for _, obj := range []client.Object{
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: keyValueInternalBackendName(kv), Namespace: kv.Namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-internal-proxy-environment-ingress", Namespace: kv.Namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-internal-proxy-backend-ingress", Namespace: kv.Namespace}},
	} {
		if err := cl.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			t.Errorf("%T %s: %v", obj, obj.GetName(), err)
		}
	}
}

// A store that enabled internal auth before w6/m463 has no proxy and its <id>
// selects Valkey. It adopts a client-auth proxy and switches <id> only once
// the proxy is ready, so <id> is never without an endpoint.
func TestKeyValueInternalAuthStoreAdoptsTheProxyOnceReady(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-old", Namespace: "tea-test", UID: "old"}, Spec: appv1alpha1.KeyValueSpec{InternalAuth: true}}
	direct := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}, Spec: corev1.ServiceSpec{
		Type: corev1.ServiceTypeClusterIP, Selector: map[string]string{labelKeyValue: kv.Name},
		Ports: []corev1.ServicePort{{Port: kvPort, TargetPort: intstr.FromInt(kvPort), Name: keyValueContainerName}},
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(direct).WithStatusSubresource(&appsv1.Deployment{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "bex@sha256:fixture"}
	intent := keyValueIntent{authSecretName: "auth", credentialRevision: "revision", labels: map[string]string{labelKeyValue: kv.Name}}
	selector := func() map[string]string {
		t.Helper()
		service := &corev1.Service{}
		if err := cl.Get(ctx, client.ObjectKeyFromObject(kv), service); err != nil {
			t.Fatal(err)
		}
		return service.Spec.Selector
	}
	if err := r.reconcileKeyValueInternalRoute(ctx, kv, &intent); err != nil {
		t.Fatal(err)
	}
	if got := selector(); got[labelKeyValue] != kv.Name {
		t.Fatalf("<id> left Valkey before its proxy was ready: %v", got)
	}
	dep, env := proxyEnv(t, cl, kv)
	if env["BEX_KV_INTERNAL_AUTH"] != "client" {
		t.Fatalf("the adopted proxy would inject AUTH: %v", env)
	}
	markProxyRolledOut(t, cl, dep)
	if err := r.reconcileKeyValueInternalRoute(ctx, kv, &intent); err != nil {
		t.Fatal(err)
	}
	if got := selector(); got[labelKeyValueInternalProxy] != kv.Name {
		t.Fatalf("<id> did not move to the ready proxy: %v", got)
	}
	if kv.Status.InternalAuthOptional {
		t.Fatal("the cutover of an internal-auth store made its internal URL password-less")
	}
}

func TestKeyValueInternalProxyRejectsMixedRolloutReadiness(t *testing.T) {
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Spec: appsv1.DeploymentSpec{Replicas: new(int32(1))}, Status: appsv1.DeploymentStatus{ObservedGeneration: 2, Replicas: 2, UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}}
	if keyValueInternalDeploymentReady(dep) {
		t.Fatal("old available replica hid unready replacement")
	}
	dep.Status.Replicas = 1
	if !keyValueInternalDeploymentReady(dep) {
		t.Fatal("fully ready current generation was refused")
	}
	dep.Status.ObservedGeneration = 1
	if keyValueInternalDeploymentReady(dep) {
		t.Fatal("stale rollout acknowledged")
	}
}

// An isolation toggle alone must not roll the Key Value's one-replica internal
// proxy: the 2026-10-29 hosted check saw each toggle replace it and drop
// same-environment connections for seconds (w8/220). Its running pods are
// relabelled in place instead, and a template change for another reason still
// adopts the label.
func TestKeyValueInternalProxyIsolationToggleNeverRollsIt(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-iso", Namespace: "tea-test", UID: "iso", Labels: map[string]string{}}}
	ctrl := true
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "red-iso-internal-abc", Namespace: "tea-test",
		Labels:          map[string]string{labelKeyValueInternalProxy: kv.Name},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: "red-iso-internal-abc", UID: "rs", Controller: &ctrl}},
	}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv, pod).WithStatusSubresource(&appsv1.Deployment{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "bex@sha256:fixture"}
	intent := keyValueIntent{authSecretName: "auth", credentialRevision: "revision", labels: map[string]string{labelKeyValue: kv.Name}}
	depKey := client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}
	template := func() corev1.PodTemplateSpec {
		t.Helper()
		dep := &appsv1.Deployment{}
		if err := cl.Get(ctx, depKey, dep); err != nil {
			t.Fatal(err)
		}
		return dep.Spec.Template
	}
	if _, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, depKey, dep); err != nil {
		t.Fatal(err)
	}
	if dep.CreationTimestamp.IsZero() { // the fake client does not stamp it
		dep.CreationTimestamp = metav1.Now()
		if err := cl.Update(ctx, dep); err != nil {
			t.Fatal(err)
		}
	}
	settled := template()

	kv.Labels[labelNetworkIsolation] = "env-a"
	if _, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if got := template(); !reflect.DeepEqual(got, settled) {
		t.Fatalf("isolation on rolled the proxy template: %v", got.Labels)
	}
	if err := r.syncKeyValuePodIsolation(ctx, kv); err != nil {
		t.Fatal(err)
	}
	running := &corev1.Pod{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(pod), running); err != nil {
		t.Fatal(err)
	}
	if running.Labels[labelNetworkIsolation] != "env-a" {
		t.Fatalf("running proxy pod labels = %v, want it relabelled in place", running.Labels)
	}

	intent.credentialRevision = "rotated" // a real template change adopts the label
	if _, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if got := template().Labels[labelNetworkIsolation]; got != "env-a" {
		t.Fatalf("a rolling template did not adopt the isolation label: %q", got)
	}
}

// The proxy Deployment follows the same rule as the Valkey StatefulSet
// (w2/m247): a proxy created before the datastore PriorityClass existed is not
// rolled just to gain it, and adopts it on the next change that rolls it anyway.
func TestKeyValueInternalProxyAdoptsThePriorityClassOnlyWhenItRolls(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-test", Namespace: "tea-test", UID: "test"}, Spec: appv1alpha1.KeyValueSpec{PersistenceMode: "off"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1.Deployment{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "bex@sha256:fixture"}
	intent := keyValueIntent{authSecretName: "auth", credentialRevision: "rev-1", labels: map[string]string{labelKeyValue: kv.Name}}
	depKey := client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}
	class := func() string {
		t.Helper()
		dep := &appsv1.Deployment{}
		if err := cl.Get(ctx, depKey, dep); err != nil {
			t.Fatal(err)
		}
		return dep.Spec.Template.Spec.PriorityClassName
	}

	if _, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if got := class(); got != appv1alpha1.DatastorePriorityClassName {
		t.Fatalf("a new proxy carries class %q, want %q", got, appv1alpha1.DatastorePriorityClassName)
	}

	// A proxy that predates the class.
	dep := &appsv1.Deployment{}
	if err := cl.Get(ctx, depKey, dep); err != nil {
		t.Fatal(err)
	}
	dep.CreationTimestamp = metav1.Now()
	dep.Spec.Template.Spec.PriorityClassName = ""
	if err := cl.Update(ctx, dep); err != nil {
		t.Fatal(err)
	}
	if _, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if got := class(); got != "" {
		t.Fatalf("an unchanged proxy was rolled to adopt the class: %q", got)
	}

	// A credential rotation rolls the proxy anyway: the class rides along.
	intent.credentialRevision = "rev-2"
	if _, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent); err != nil {
		t.Fatal(err)
	}
	if got := class(); got != appv1alpha1.DatastorePriorityClassName {
		t.Fatalf("a rolling proxy did not adopt the class: %q", got)
	}
}

// w6/m456 t005: each store's proxy holds its plan's limit across both
// listeners, answers a refused TLS client with the store's own certificate,
// and has the memory to hold that many connections.
func TestKeyValueInternalProxyCarriesThePlansLimit(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	for _, c := range []struct {
		plan   string
		public bool
		memory string
	}{
		{"free", false, "128Mi"},
		{"starter", true, "128Mi"},
		{"pro", true, "272768Ki"},   // 32Mi + 5,000 × 48Ki
		{"50g", false, "2432768Ki"}, // 32Mi + 50,000 × 48Ki
	} {
		kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-" + c.plan, Namespace: "tea-test", UID: "test"}, Spec: appv1alpha1.KeyValueSpec{Plan: c.plan}}
		cl := fake.NewClientBuilder().WithScheme(scheme).Build()
		r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "bex@sha256:fixture"}
		plan, _ := resolveKVPlan(kv.Spec)
		intent := keyValueIntent{plan: plan, authSecretName: "auth", tlsRuntime: c.public, tlsSecretName: keyValueTLSName(kv), labels: map[string]string{labelKeyValue: kv.Name}}
		if _, err := r.reconcileKeyValueInternalProxy(ctx, kv, intent); err != nil {
			t.Fatal(err)
		}
		dep := &appsv1.Deployment{}
		if err := cl.Get(ctx, client.ObjectKey{Name: keyValueInternalProxyName(kv), Namespace: kv.Namespace}, dep); err != nil {
			t.Fatal(err)
		}
		proxy := dep.Spec.Template.Spec.Containers[0]
		env := map[string]string{}
		for _, e := range proxy.Env {
			env[e.Name] = e.Value
		}
		if got, want := env["BEX_KV_MAX_CONNECTIONS"], strconv.Itoa(int(plan.ConnectionLimit)); got != want {
			t.Errorf("%s: BEX_KV_MAX_CONNECTIONS %q, want the plan's %s", c.plan, got, want)
		}
		if got := proxy.Resources.Limits.Memory(); got.Cmp(resource.MustParse(c.memory)) != 0 {
			t.Errorf("%s: memory limit %s, want %s", c.plan, got, c.memory)
		}
		volumes := dep.Spec.Template.Spec.Volumes
		if !c.public {
			if len(volumes) != 0 || len(proxy.VolumeMounts) != 0 || env["BEX_KV_TLS_CERT_FILE"] != "" {
				t.Errorf("%s: a private store's proxy mounted a certificate: %v", c.plan, volumes)
			}
			continue
		}
		if len(volumes) != 1 || volumes[0].Secret == nil || volumes[0].Secret.SecretName != keyValueTLSName(kv) ||
			volumes[0].Secret.Optional == nil || !*volumes[0].Secret.Optional {
			t.Errorf("%s: the proxy must mount the store's certificate, optional: %+v", c.plan, volumes)
		}
		if len(proxy.VolumeMounts) != 1 || !proxy.VolumeMounts[0].ReadOnly || env["BEX_KV_TLS_CERT_FILE"] != kvTLSCertFile || env["BEX_KV_TLS_KEY_FILE"] != kvTLSKeyFile {
			t.Errorf("%s: mounts %+v, cert %q key %q", c.plan, proxy.VolumeMounts, env["BEX_KV_TLS_CERT_FILE"], env["BEX_KV_TLS_KEY_FILE"])
		}
	}
}

// w6/m512: every proxy drains on SIGTERM, and its pod's grace period outlasts
// the drain, so the process closes what is left itself before SIGKILL.
func TestKeyValueInternalProxyDrainsWithinItsGracePeriod(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	for _, internalAuth := range []bool{false, true} {
		kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-drain", Namespace: "tea-test", UID: "test"}, Spec: appv1alpha1.KeyValueSpec{Plan: "free", InternalAuth: internalAuth}}
		cl := fake.NewClientBuilder().WithScheme(scheme).Build()
		r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "bex@sha256:fixture"}
		plan, _ := resolveKVPlan(kv.Spec)
		if _, err := r.reconcileKeyValueInternalProxy(ctx, kv, keyValueIntent{plan: plan, authSecretName: "auth", labels: map[string]string{labelKeyValue: kv.Name}}); err != nil {
			t.Fatal(err)
		}
		dep, env := proxyEnv(t, cl, kv)
		drain, err := strconv.Atoi(env["BEX_KV_DRAIN_SECONDS"])
		if err != nil || drain <= 0 {
			t.Fatalf("internalAuth=%v: BEX_KV_DRAIN_SECONDS %q, want a drain", internalAuth, env["BEX_KV_DRAIN_SECONDS"])
		}
		grace := dep.Spec.Template.Spec.TerminationGracePeriodSeconds
		if grace == nil || *grace <= int64(drain) {
			t.Fatalf("internalAuth=%v: terminationGracePeriodSeconds %v does not outlast the %ds drain", internalAuth, grace, drain)
		}
	}
}

// w6/m463: enabling internal auth on a running store, through the whole
// reconcile, rolls its proxy and nothing of Valkey's: the StatefulSet's spec
// is unchanged (so its pod and run_id survive), <id> stays on the proxy, and
// the backup snapshot dials the backend, inside the server's reserve.
func TestEnablingInternalAuthRollsOnlyTheProxy(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	nn := types.NamespacedName{Name: "red-enable", Namespace: "default"}
	kv := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace, UID: "enable"}, Spec: appv1alpha1.KeyValueSpec{Name: "enable", Plan: "starter"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(kv).
		WithStatusSubresource(&appv1alpha1.KeyValue{}, &appsv1.StatefulSet{}, &appsv1.Deployment{}).Build()
	r := &KeyValueReconciler{Client: cl, Scheme: scheme, Backup: testKeyValueBackupStore, InternalProxyImage: "bex@sha256:fixture"}
	pass := func() {
		t.Helper()
		for range 3 {
			if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
				t.Fatal(err)
			}
		}
	}
	pass()
	dep, _ := proxyEnv(t, cl, kv)
	markProxyRolledOut(t, cl, dep)
	pass()
	service := &corev1.Service{}
	if err := cl.Get(ctx, nn, service); err != nil || service.Spec.Selector[labelKeyValueInternalProxy] != kv.Name {
		t.Fatalf("precondition: <id> selects the proxy: %v %v", service.Spec.Selector, err)
	}
	sts := &appsv1.StatefulSet{}
	if err := cl.Get(ctx, nn, sts); err != nil {
		t.Fatal(err)
	}
	before := sts.Spec.DeepCopy()

	if err := cl.Get(ctx, nn, kv); err != nil {
		t.Fatal(err)
	}
	kv.Spec.InternalAuth = true
	if err := cl.Update(ctx, kv); err != nil {
		t.Fatal(err)
	}
	pass()
	if err := cl.Get(ctx, nn, sts); err != nil {
		t.Fatal(err)
	}
	if !equality.Semantic.DeepEqual(&sts.Spec, before) {
		t.Fatalf("enabling internal auth changed the Valkey StatefulSet, restarting its pod:\n%v\n%v", sts.Spec.Template, before.Template)
	}
	if _, env := proxyEnv(t, cl, kv); env["BEX_KV_INTERNAL_AUTH"] != "client" {
		t.Fatalf("the proxy was not switched to client auth: %v", env)
	}
	if err := cl.Get(ctx, nn, service); err != nil || service.Spec.Selector[labelKeyValueInternalProxy] != kv.Name {
		t.Fatalf("<id> left the proxy: %v %v", service.Spec.Selector, err)
	}
	cron := &batchv1.CronJob{}
	if err := cl.Get(ctx, types.NamespacedName{Name: keyValueBackupName(kv.Name), Namespace: kv.Namespace}, cron); err != nil {
		t.Fatal(err)
	}
	for _, c := range cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers {
		for _, e := range c.Env {
			if c.Name == "snapshot" && e.Name == "VALKEY_HOST" && e.Value != keyValueInternalBackendName(kv) {
				t.Fatalf("the snapshot of an internal-auth store dials %q, a tenant connection", e.Value)
			}
		}
	}
}

// w6/m469: only a digest names the bytes a proxy runs. A tag (the local
// cluster's bex-operator:dev) is refused before any registry call, so the
// proxy rolls as it always did.
func TestInternalProxyBinaryFromRegistryNeedsADigest(t *testing.T) {
	binaryOf := InternalProxyBinaryFromRegistry()
	for _, image := range []string{"bex-operator:dev", "ghcr.io/bex-co/bex-operator:latest", "ghcr.io/bex-co/bex-operator@sha256:short"} {
		if sum, err := binaryOf(context.Background(), image); err == nil || !strings.Contains(err.Error(), "not pinned by digest") {
			t.Errorf("%s: %q, %v; want a refusal without a registry call", image, sum, err)
		}
	}
}

// w6/m512: once internal authentication is enabled, the operator closes the
// connections of the old, authenticating proxy pod as soon as it terminates,
// so its drain does not keep them, and only that pod's. A roll that keeps the
// proxy's mode, and a stale pod still serving, are left alone.
func TestKeyValueCutsOnlyATerminatingProxyOfTheOldMode(t *testing.T) {
	ctx := context.Background()
	scheme := keyValueBackupTestScheme(t)
	enabled := &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{Name: "red-cut", Namespace: "tea-test"}, Spec: appv1alpha1.KeyValueSpec{Plan: "free", InternalAuth: true}}
	unchanged := enabled.DeepCopy()
	unchanged.Spec.InternalAuth = false
	serving := terminatingProxyPod(enabled, "", "10.0.0.7")
	serving.Name, serving.DeletionTimestamp, serving.Finalizers = "red-cut-internal-serving", nil, nil
	current := terminatingProxyPod(enabled, "client", "10.0.0.8")
	current.Name, current.DeletionTimestamp, current.Finalizers = "red-cut-internal-new", nil, nil
	for _, c := range []struct {
		name        string
		kv          *appv1alpha1.KeyValue
		pods        []client.Object
		wantCut     []string
		wantPending bool
	}{
		{"enabling internal auth", enabled, []client.Object{terminatingProxyPod(enabled, "", "10.0.0.5"), current}, []string{"10.0.0.5"}, true},
		{"a binary-only roll", unchanged, []client.Object{terminatingProxyPod(unchanged, "", "10.0.0.5")}, nil, false},
		{"a roll after enabling", enabled, []client.Object{terminatingProxyPod(enabled, "client", "10.0.0.5"), current}, nil, false},
		{"the old pod still serving", enabled, []client.Object{serving, current}, nil, false},
		{"no address yet", enabled, []client.Object{terminatingProxyPod(enabled, "", "")}, nil, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			var cut []string
			r := &KeyValueReconciler{
				Client:             fake.NewClientBuilder().WithScheme(scheme).WithObjects(append(c.pods, valkeyServerPod(c.kv))...).Build(),
				InternalProxyImage: "bex@sha256:fixture",
				cutValkeyClients: func(_ context.Context, server *corev1.Pod, _ *corev1.Secret, ips map[string]bool) error {
					if server.Name != c.kv.Name+"-0" {
						t.Errorf("cut through %s, want the store's server", server.Name)
					}
					for ip := range ips {
						cut = append(cut, ip)
					}
					return nil
				},
			}
			if pending := r.cutStaleKeyValueProxyClients(ctx, c.kv, nil); pending != c.wantPending {
				t.Errorf("pending %v, want %v", pending, c.wantPending)
			}
			if !reflect.DeepEqual(cut, c.wantCut) {
				t.Errorf("cut %v, want %v", cut, c.wantCut)
			}
		})
	}
	// A stale pod is polled within a second, sooner than any other wait.
	if got := keyValueRuntimeRequeue(reconcile.Result{RequeueAfter: childHealthRequeue}, true, true).RequeueAfter; got != kvProxyCutRequeue {
		t.Errorf("requeue %v while a stale proxy terminates, want %v", got, kvProxyCutRequeue)
	}
	// Its events reach the store's reconcile.
	if got := keyValuePodRequests(ctx, serving); len(got) != 1 || got[0].Name != enabled.Name {
		t.Errorf("a proxy pod's event enqueued %v", got)
	}
}

// w6/m517: a store's proxy keeps the template it runs until the store's own
// change rolls it, so a change to what the operator renders reaches a store
// only then. The rendering is pinned, so that every change to it is a
// decision: let it wait for each store's next change, or bump
// kvInternalProxyRollRevision. The roll inputs' digest is pinned too, because
// changing how it is computed rolls every proxy at once.
func TestKeyValueInternalProxyRenderingIsPinned(t *testing.T) {
	type pin struct{ template, inputs string }
	pinned := map[string]pin{
		"plain":                    {"789e2112310f655a28e1ed052af24b6d02acd1ced1f5b2693c301af597f21a25", "0fca5894756134f2066a3f319f0944c6ba5d79ecadf1403d15022605ed3234cf"},
		"plain with internal auth": {"fbe431a3092e00fdb3f33c60ed2bab2b98c63897a3f2b8e0721efc975f077bad", "a5ed0a396b6923205712277b8ac65e333d31ff6e4fc6759c444f28b53018f65d"},
		"tls":                      {"84784e05fd395382739e6be9314d1195ddc5d052bb84f9dccf87fa4995cbba7d", "2a6015ddfc05d2562a2d9f1b05f340859fc5d7b7113e73e8a84e09af63043ff4"},
		"tls with internal auth":   {"2038b61d533defa1f22227b5638205efbdbdf05eb33482bfb96b7a7e2e929d38", "1cbe9908277765efd6ea6497ef7aac33d969590ea3a8f61d1ec1994fbf1c8417"},
	}
	scheme := keyValueBackupTestScheme(t)
	for name, want := range pinned {
		t.Run(name, func(t *testing.T) {
			kv := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: "red-pin", Namespace: "tea-pin", UID: "pin", Labels: map[string]string{labelWorkspace: "tea-pin"}},
				Spec:       appv1alpha1.KeyValueSpec{Plan: "starter", InternalAuth: strings.HasSuffix(name, "internal auth")},
			}
			plan, _ := resolveKVPlan(kv.Spec)
			intent := keyValueIntent{
				plan: plan, authSecretName: "red-pin-auth", credentialRevision: "rev-1",
				tlsRuntime: strings.HasPrefix(name, "tls"), tlsSecretName: keyValueTLSName(kv),
				labels: map[string]string{labelKeyValue: kv.Name},
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).Build()
			r := &KeyValueReconciler{Client: cl, Scheme: scheme, InternalProxyImage: "ghcr.io/bex-co/bex-operator@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
			if _, err := r.reconcileKeyValueInternalProxy(context.Background(), kv, intent); err != nil {
				t.Fatal(err)
			}
			dep, _ := proxyEnv(t, cl, kv)
			rendered, err := json.Marshal(dep.Spec.Template)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(rendered)
			if got := hex.EncodeToString(sum[:]); got != want.template {
				t.Errorf("the proxy's rendered pod template changed: %s now pins %q.\n"+
					"A store's proxy keeps the template it runs until the store's own change rolls it (w6/m517, ADR021 § 10), "+
					"so this change reaches each store only at its next plan change, internal-auth change, credential rotation or resume.\n"+
					"If that is not acceptable (a security fix, or a held template that can no longer work), bump kvInternalProxyRollRevision, "+
					"which rolls every store's proxy once on the next pass.\n"+
					"Either way, update the pin here.\nThe rendering: %s", name, got, rendered)
			}
			if got := dep.Annotations[kvInternalProxyInputsAnnotation]; got != want.inputs {
				t.Errorf("the proxy's roll inputs digest changed: %s now pins %q.\n"+
					"Every Deployment's recorded digest now differs, so the next pass rolls every store's proxy at once, as a bump of kvInternalProxyRollRevision does. "+
					"Make sure that is intended, then update the pin here.", name, got)
			}
		})
	}
}
