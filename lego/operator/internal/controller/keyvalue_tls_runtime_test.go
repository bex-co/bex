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
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// useStaticTLSLayout rewrites a reconciled store into the layout an operator
// before w8/m134 wrote: the static listener (or none) in the template and no
// runtime switch.
func useStaticTLSLayout(t *testing.T, cl client.Client, kv *appv1alpha1.KeyValue, listening bool) {
	t.Helper()
	ctx := context.Background()
	var sts appsv1.StatefulSet
	if err := cl.Get(ctx, client.ObjectKeyFromObject(kv), &sts); err != nil {
		t.Fatal(err)
	}
	spec := &sts.Spec.Template.Spec
	c := &spec.Containers[slices.IndexFunc(spec.Containers, func(c corev1.Container) bool { return c.Name == keyValueContainerName })]
	if len(c.Args) > 4 && c.Args[0] == "sh" {
		c.Args = slices.Clone(c.Args[4:])
	}
	spec.Volumes = nil
	c.VolumeMounts = []corev1.VolumeMount{{Name: "data", MountPath: kvDataPath}}
	c.Ports = []corev1.ContainerPort{{ContainerPort: kvPort, Name: keyValueContainerName}}
	if listening {
		c.Args = append(c.Args, valkeyTLSArgs()...)
		c.Ports = append(c.Ports, corev1.ContainerPort{ContainerPort: kvTLSPort, Name: "valkey-tls"})
		c.VolumeMounts = append(c.VolumeMounts, corev1.VolumeMount{Name: kvTLSVolume, MountPath: kvTLSMountPath, ReadOnly: true})
		spec.Volumes = []corev1.Volume{{Name: kvTLSVolume, VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: keyValueTLSName(kv)},
		}}}
	}
	if err := cl.Update(ctx, &sts); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(kv), kv); err != nil {
		t.Fatal(err)
	}
	kv.Status.TLSRuntimeSwitch = false
	kv.Status.TLSListener = listening
	if err := cl.Status().Update(ctx, kv); err != nil {
		t.Fatal(err)
	}
}

// runtimeTLSFixture is a store reconciled through a fake API server, with the
// runtime switch stubbed to record each call.
type runtimeTLSFixture struct {
	t        *testing.T
	ctx      context.Context
	cl       client.Client
	r        *KeyValueReconciler
	kv       *appv1alpha1.KeyValue
	nn       types.NamespacedName
	result   reconcile.Result
	calls    []bool
	template *corev1.PodTemplateSpec
	version  string
}

func newRuntimeTLSFixture(t *testing.T) *runtimeTLSFixture {
	f := &runtimeTLSFixture{t: t, ctx: context.Background(), nn: types.NamespacedName{Name: "cache-kv", Namespace: "default"}}
	f.kv = &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "cache-kv", Namespace: "default", UID: "kv-runtime-tls"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free", PersistenceMode: appv1alpha1.KeyValuePersistenceOff},
	}
	// The API server assigns every object a UID; the Certificate's lifetime
	// cleanup relies on it.
	f.cl = fake.NewClientBuilder().WithScheme(keyValueBackupTestScheme(t)).WithObjects(f.kv).
		WithStatusSubresource(&appv1alpha1.KeyValue{}).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if obj.GetUID() == "" {
				obj.SetUID(types.UID("uid-" + obj.GetName()))
			}
			return c.Create(ctx, obj, opts...)
		},
	}).Build()
	f.r = &KeyValueReconciler{Client: f.cl, Scheme: f.cl.Scheme(), KvDomain: "kv.example.test", ClusterIssuer: "letsencrypt-prod",
		switchTLS: func(_ context.Context, pod *corev1.Pod, platform *corev1.Secret, on bool) (bool, error) {
			if pod.Status.PodIP != "10.0.0.7" || platform.Name != "cache-kv-platform" {
				t.Fatalf("switch dialed pod %q as %q", pod.Status.PodIP, platform.Name)
			}
			f.calls = append(f.calls, on)
			return on, nil
		}}
	return f
}

func (f *runtimeTLSFixture) reconcileTwice() {
	f.t.Helper()
	for range 2 {
		var err error
		if f.result, err = f.r.Reconcile(f.ctx, reconcile.Request{NamespacedName: f.nn}); err != nil {
			f.t.Fatalf("reconcile: %v", err)
		}
	}
	if err := f.cl.Get(f.ctx, f.nn, f.kv); err != nil {
		f.t.Fatal(err)
	}
}

func (f *runtimeTLSFixture) setPublic(public bool) {
	f.t.Helper()
	if err := f.cl.Get(f.ctx, f.nn, f.kv); err != nil {
		f.t.Fatal(err)
	}
	f.kv.Spec.Public = public
	f.kv.Spec.IPAllowList = nil
	if public {
		f.kv.Spec.IPAllowList = []appv1alpha1.IPAllowEntry{{CIDR: "203.0.113.7/32", Description: "qa"}}
	}
	if err := f.cl.Update(f.ctx, f.kv); err != nil {
		f.t.Fatal(err)
	}
	f.reconcileTwice()
}

func (f *runtimeTLSFixture) statefulSet() *appsv1.StatefulSet {
	f.t.Helper()
	sts := &appsv1.StatefulSet{}
	if err := f.cl.Get(f.ctx, f.nn, sts); err != nil {
		f.t.Fatal(err)
	}
	return sts
}

// startPod stands in for the StatefulSet controller: one serving pod.
func (f *runtimeTLSFixture) startPod() {
	f.t.Helper()
	sts := f.statefulSet()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "cache-kv-0", Namespace: "default", Labels: map[string]string{labelKeyValue: "cache-kv"},
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(sts, appsv1.SchemeGroupVersion.WithKind("StatefulSet"))}},
		Spec:   *sts.Spec.Template.Spec.DeepCopy(),
		Status: corev1.PodStatus{PodIP: "10.0.0.7", Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
	if err := f.cl.Create(f.ctx, pod); err != nil {
		f.t.Fatal(err)
	}
	f.reconcileTwice()
	sts = f.statefulSet()
	f.template, f.version = sts.Spec.Template.DeepCopy(), sts.ResourceVersion
}

// issueCertificate stands in for cert-manager writing the Secret.
func (f *runtimeTLSFixture) issueCertificate() {
	f.t.Helper()
	if err := f.cl.Create(f.ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: keyValueTLSName(f.kv), Namespace: "default", Labels: map[string]string{execution.LabelKeyValueUID: string(f.kv.UID)}},
		Type:       corev1.SecretTypeTLS,
		Data:       map[string][]byte{corev1.TLSCertKey: []byte("cert"), corev1.TLSPrivateKeyKey: []byte("key")},
	}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *runtimeTLSFixture) certificateErr() error {
	c := &unstructured.Unstructured{}
	c.SetGroupVersionKind(certManagerCertificateGVK)
	return f.cl.Get(f.ctx, types.NamespacedName{Name: keyValueTLSName(f.kv), Namespace: "default"}, c)
}

// unrolled fails the test if the StatefulSet changed since the pod started:
// any template change restarts the store and empties a Persistence Off one.
func (f *runtimeTLSFixture) unrolled(step string) {
	f.t.Helper()
	sts := f.statefulSet()
	if !equality.Semantic.DeepEqual(sts.Spec.Template, *f.template) || sts.ResourceVersion != f.version {
		f.t.Fatalf("%s changed the StatefulSet, which restarts the store:\nbefore %+v\nafter  %+v", step, f.template.Spec, sts.Spec.Template.Spec)
	}
}

func (f *runtimeTLSFixture) lastCall() any {
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1]
}

// TestKeyValueRuntimeTLSSwitchRestartsNothing is w8/m134 t003: a store created
// private, with persistence off, starts with no TLS listener and no
// Certificate. Its first external enable opens the listener in the running
// process once the Secret exists, and disabling closes it; neither changes the
// pod template, so the StatefulSet never rolls and the keyspace survives.
func TestKeyValueRuntimeTLSSwitchRestartsNothing(t *testing.T) {
	f := newRuntimeTLSFixture(t)
	f.reconcileTwice()
	if err := f.certificateErr(); !apierrors.IsNotFound(err) {
		t.Fatalf("a private store requested a certificate: %v", err)
	}
	f.startPod()
	if f.kv.Status.TLSListener || !f.kv.Status.TLSRuntimeSwitch || slices.Contains(f.calls, true) {
		t.Fatalf("private store: status %+v, switch calls %v", f.kv.Status, f.calls)
	}

	// Enabled: the Certificate is requested; until its Secret exists the
	// listener cannot open, so the store keeps serving and polls quickly.
	f.setPublic(true)
	f.unrolled("the first enable")
	if slices.Contains(f.calls, true) || f.kv.Status.TLSListener || f.kv.Status.ExternalHost != "cache-kv.kv.example.test" {
		t.Fatalf("enable before the certificate: status %+v, switch calls %v", f.kv.Status, f.calls)
	}
	if f.result.RequeueAfter == 0 || f.result.RequeueAfter > kvTLSPendingRequeue {
		t.Fatalf("a pending listener requeues after %v", f.result.RequeueAfter)
	}
	if err := f.certificateErr(); err != nil {
		t.Fatalf("the enable requested no certificate: %v", err)
	}
	f.issueCertificate()
	f.reconcileTwice()
	f.unrolled("opening the listener")
	if !f.kv.Status.TLSListener || f.lastCall() != true || f.kv.Status.Phase == appv1alpha1.KVPhaseFailed {
		t.Fatalf("issued certificate: status %+v, switch calls %v", f.kv.Status, f.calls)
	}

	// Disabled: tls-port 0 in the running process, and the Certificate goes.
	f.setPublic(false)
	f.unrolled("disabling external access")
	if f.kv.Status.TLSListener || f.lastCall() != false || f.kv.Status.ExternalHost != "" {
		t.Fatalf("disabled store: status %+v, switch calls %v", f.kv.Status, f.calls)
	}
	if err := f.certificateErr(); !apierrors.IsNotFound(err) {
		t.Fatalf("a private store kept its certificate: %v", err)
	}

	// Enabled again: the kept Secret opens the listener straight away.
	f.setPublic(true)
	f.unrolled("enabling external access again")
	if !f.kv.Status.TLSListener || f.lastCall() != true {
		t.Fatalf("re-enabled store: status %+v, switch calls %v", f.kv.Status, f.calls)
	}
}

// TestKeyValueTLSWrapperStartsTheListenerFromTheMountedCertificate runs the
// pod's real start command with a stand-in valkey-server: a pod (re)started
// while the certificate is mounted starts the listener, one started without
// it does not, and the server args pass through verbatim either way.
func TestKeyValueTLSWrapperStartsTheListenerFromTheMountedCertificate(t *testing.T) {
	kv := &appv1alpha1.KeyValue{
		ObjectMeta: metav1.ObjectMeta{Name: "wrap-kv", Namespace: "default"},
		Spec:       appv1alpha1.KeyValueSpec{Plan: "free", PersistenceMode: appv1alpha1.KeyValuePersistenceOff},
	}
	plan, storage := resolveKVPlan(kv.Spec)
	sts := &appsv1.StatefulSet{}
	applyKeyValueStatefulSet(sts, kv, keyValueIntentFor(kv, plan, storage, "kv.example.test", false, true))
	container := sts.Spec.Template.Spec.Containers[0]
	volume := keyValueTLSVolume(sts.Spec.Template.Spec.Volumes)
	if volume == nil || volume.Secret.SecretName != keyValueTLSName(kv) || volume.Secret.Optional == nil || !*volume.Secret.Optional {
		t.Fatalf("certificate volume %+v, want the store's TLS Secret mounted optional", volume)
	}
	if !slices.ContainsFunc(container.VolumeMounts, func(m corev1.VolumeMount) bool {
		return m.Name == kvTLSVolume && m.MountPath == kvTLSMountPath && m.ReadOnly
	}) {
		t.Fatalf("server mounts %+v", container.VolumeMounts)
	}
	if len(container.Command) != 0 || container.Args[0] != "sh" || container.Args[1] != "-c" || container.Args[3] != "valkey-server" {
		t.Fatalf("the server must start through the image entrypoint and the wrapper: %q %q", container.Command, container.Args[:4])
	}
	server := container.Args[4:]
	if !slices.Equal(server, append(valkeyArgs(kv.Spec, plan), "--dir", "/tmp")) {
		t.Fatalf("server args %q", server)
	}

	dir := t.TempDir()
	tlsDir, bin := filepath.Join(dir, "tls"), filepath.Join(dir, "bin")
	for _, d := range []string{tlsDir, bin} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// The stand-in prints one argument per line, so empty ones stay visible.
	if err := os.WriteFile(filepath.Join(bin, "valkey-server"), []byte("#!/bin/sh\nfor a in \"$@\"; do printf '[%s]\\n' \"$a\"; done\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	start := func() []string {
		t.Helper()
		script := strings.ReplaceAll(container.Args[2], kvTLSMountPath+"/", tlsDir+"/")
		cmd := exec.Command("sh", append([]string{"-c", script, "valkey-server"}, server...)...)
		cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("wrapper: %v\n%s", err, out)
		}
		return strings.Split(strings.TrimSpace(string(out)), "\n")
	}
	bracket := func(args []string) []string {
		out := make([]string, len(args))
		for i, a := range args {
			out[i] = "[" + a + "]"
		}
		return out
	}
	if got := start(); !slices.Equal(got, bracket(server)) {
		t.Fatalf("started without a certificate: %q, want the server args only", got)
	}
	// The kubelet projects the Secret's keys as files.
	for _, f := range []string{corev1.TLSCertKey, corev1.TLSPrivateKeyKey} {
		if err := os.WriteFile(filepath.Join(tlsDir, f), []byte("pem"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	flags := strings.Split(strings.ReplaceAll(strings.Join(valkeyTLSArgs(), "\n"), kvTLSMountPath+"/", tlsDir+"/"), "\n")
	if got := start(); !slices.Equal(got, bracket(append(slices.Clone(server), flags...))) {
		t.Fatalf("restarted with the certificate mounted: %q, want the listener flags appended", got)
	}
}

// TestKeyValueTLSRuntimeLayoutLeavesOlderStoresAlone: the operator upgrade
// must restart no running store. Only a new StatefulSet, one already
// switching, or an older store's first enable (which restarts anyway) uses
// the runtime switch.
func TestKeyValueTLSRuntimeLayoutLeavesOlderStoresAlone(t *testing.T) {
	existing := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "7"}}
	switching := existing.DeepCopy()
	switching.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: kvTLSVolume, VolumeSource: corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{SecretName: "s", Optional: new(true)}}}}
	static := existing.DeepCopy()
	static.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: kvTLSVolume, VolumeSource: corev1.VolumeSource{
		Secret: &corev1.SecretVolumeSource{SecretName: "s"}}}}
	for name, tc := range map[string]struct {
		sts       *appsv1.StatefulSet
		spec      appv1alpha1.KeyValueSpec
		status    appv1alpha1.KeyValueStatus
		listening bool
		want      bool
	}{
		"new store":                        {&appsv1.StatefulSet{}, appv1alpha1.KeyValueSpec{}, appv1alpha1.KeyValueStatus{}, false, true},
		"already switching":                {switching, appv1alpha1.KeyValueSpec{}, appv1alpha1.KeyValueStatus{}, false, true},
		"status says switching":            {existing, appv1alpha1.KeyValueSpec{}, appv1alpha1.KeyValueStatus{TLSRuntimeSwitch: true}, false, true},
		"older private store":              {existing, appv1alpha1.KeyValueSpec{}, appv1alpha1.KeyValueStatus{}, false, false},
		"older store, static listener":     {static, appv1alpha1.KeyValueSpec{Public: true}, appv1alpha1.KeyValueStatus{TLSListener: true}, true, false},
		"older private store's 1st enable": {existing, appv1alpha1.KeyValueSpec{Public: true}, appv1alpha1.KeyValueStatus{}, false, true},
	} {
		t.Run(name, func(t *testing.T) {
			kv := &appv1alpha1.KeyValue{Spec: tc.spec, Status: tc.status}
			if got := keyValueTLSRuntime(kv, tc.sts, "kv.example.test", tc.listening); got != tc.want {
				t.Fatalf("runtime switch = %v, want %v", got, tc.want)
			}
		})
	}
	if keyValueTemplateListens(switching) || !keyValueTemplateListens(static) {
		t.Fatal("an optional mount is the runtime switch, a required one the static listener")
	}
}

// The runtime enable and the startup flags are one configuration.
func TestValkeyTLSArgsMatchTheRuntimeConfig(t *testing.T) {
	want := []string{"--tls-cert-file", "/tls/tls.crt", "--tls-key-file", "/tls/tls.key", "--tls-ca-cert-file", "/tls/tls.crt",
		"--tls-auth-clients", "no", "--tls-port", "6380"}
	if got := valkeyTLSArgs(); !slices.Equal(got, want) {
		t.Fatalf("TLS args %q", got)
	}
	if config := valkeyTLSConfig(); config[len(config)-1][0] != "tls-port" {
		t.Fatal("the port must be set last, after the certificate files it loads")
	}
}
