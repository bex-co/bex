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
	"fmt"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The convergence invariant (w7/m84): a reconcile that changes nothing writes
// nothing.
//
// Every owned object in this package is authored through
// controllerutil.CreateOrUpdate, which PUTs unless the projection DeepEquals
// what it fetched. Because the API server defaults a long list of optional
// pod-template fields on write, a projection that rebuilds a container or a
// PodSpec without them can never equal the stored object, and every reconcile
// of an unchanged App/KeyValue/Database re-issues the same PUT forever — once
// per object, per pass, per tenant. w9/m57 fixed one instance of exactly this
// (a Service's Protocol) in July; nothing generalized the check, and the
// Deployment instance survived until w7/m84. This is the generalization.
//
// The oracle is the client itself, not resourceVersion. The API server
// re-defaults an incoming object, finds the result identical to what is stored,
// and skips etcd — so a redundant PUT leaves resourceVersion untouched and
// fires no watch event. An RV-only assertion would therefore pass against the
// very bug this file exists to catch. writeRecorder counts the requests instead,
// which is the OperationResultNone the definition of done asks for; the RV
// snapshot is kept alongside it as the second, independent half.
//
// Status writes are counted for the cron and datastore fixtures below. Web
// fixtures without kubelet rollout progress still exercise owned objects only.
// Ready and suspended datastores are explicitly constructed to cover both
// status convergence and real lifecycle transitions (w7/033).
//
// Known limits, so the green is read correctly:
//   - Built-in resources use real schema defaults. Third-party CRDs remain
//     stubs without webhooks; the Database case injects representative live
//     CNPG defaults and asserts zero subsequent writes. Unit request-count
//     tests cover the other shared unstructured projections and ObjectStore.
//     These test ownership preservation, not upstream admission validation.
//   - reconcileExecutionNetworkPolicy now only deletes a legacy policy; it
//     no longer has the per-pass spec projection originally noted in w7/032.
//   - Objects no shape below reaches (kpack, build Jobs, the static publish
//     path) are Create-once rather than CreateOrUpdate-per-pass, so they cannot
//     churn the way this invariant tests for.

// writeRecorder wraps a client.Client and records every mutating call made
// through it.
type writeRecorder struct {
	client.Client
	writes       []string
	recordStatus bool
}

// Status recording is opt-in for fixtures that have a stable lifecycle state.
func (w *writeRecorder) Status() client.SubResourceWriter {
	return &statusWriteRecorder{SubResourceWriter: w.Client.Status(), owner: w}
}

type statusWriteRecorder struct {
	client.SubResourceWriter
	owner *writeRecorder
}

func (w *statusWriteRecorder) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	if w.owner.recordStatus {
		w.owner.record("status update", obj)
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

func (w *statusWriteRecorder) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if w.owner.recordStatus {
		w.owner.record("status patch", obj)
	}
	return w.SubResourceWriter.Patch(ctx, obj, patch, opts...)
}

func (w *writeRecorder) record(verb string, obj client.Object) {
	kind := fmt.Sprintf("%T", obj)
	if gvk, err := apiutil.GVKForObject(obj, w.Scheme()); err == nil {
		kind = gvk.Kind
	}
	w.writes = append(w.writes, fmt.Sprintf("%s %s/%s", verb, kind, obj.GetName()))
}

func (w *writeRecorder) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	w.record("create", obj)
	return w.Client.Create(ctx, obj, opts...)
}

func (w *writeRecorder) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	w.record("update", obj)
	return w.Client.Update(ctx, obj, opts...)
}

func (w *writeRecorder) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	w.record("patch", obj)
	return w.Client.Patch(ctx, obj, patch, opts...)
}

func (w *writeRecorder) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	w.record("delete", obj)
	return w.Client.Delete(ctx, obj, opts...)
}

func (w *writeRecorder) reset() { w.writes = nil }

func (w *writeRecorder) sorted() []string {
	out := append([]string(nil), w.writes...)
	sort.Strings(out)
	return out
}

// convergenceKinds is every namespaced kind the three controllers own. Listed
// as unstructured so the CNPG/cert-manager/Traefik CRDs need no Go types, and
// so a kind a projection starts authoring is picked up by name alone.
var convergenceKinds = []schema.GroupVersionKind{
	{Group: "apps", Version: "v1", Kind: "Deployment"},
	{Group: "apps", Version: "v1", Kind: "StatefulSet"},
	{Group: "", Version: "v1", Kind: "Service"},
	{Group: "", Version: "v1", Kind: "Secret"},
	{Group: "", Version: "v1", Kind: "ConfigMap"},
	{Group: "", Version: "v1", Kind: "ServiceAccount"},
	{Group: "batch", Version: "v1", Kind: "Job"},
	{Group: "batch", Version: "v1", Kind: "CronJob"},
	{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
	{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"},
	{Group: "postgresql.cnpg.io", Version: "v1", Kind: "Cluster"},
	{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"},
	{Group: "traefik.io", Version: "v1alpha1", Kind: "Middleware"},
}

// ownedResourceVersions maps "Kind/name" to resourceVersion for every object in
// ns that owner controls.
func ownedResourceVersions(ctx context.Context, owner client.Object, ns string) map[string]string {
	out := map[string]string{}
	for _, gvk := range convergenceKinds {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk.GroupVersion().WithKind(gvk.Kind + "List"))
		if err := k8sClient.List(ctx, list, client.InNamespace(ns)); err != nil {
			continue // the CRD is absent from this envtest; nothing of that kind exists
		}
		for i := range list.Items {
			item := &list.Items[i]
			if !metav1.IsControlledBy(item, owner) {
				continue
			}
			out[gvk.Kind+"/"+item.GetName()] = item.GetResourceVersion()
		}
	}
	return out
}

// changedObjects diffs two ownedResourceVersions snapshots.
func changedObjects(before, after map[string]string) []string {
	var out []string
	for key, rv := range after {
		if prev, ok := before[key]; !ok {
			out = append(out, "appeared "+key)
		} else if prev != rv {
			out = append(out, "changed "+key)
		}
	}
	for key := range before {
		if _, ok := after[key]; !ok {
			out = append(out, "disappeared "+key)
		}
	}
	sort.Strings(out)
	return out
}

// reconcileToFixedPoint runs five passes — more than the longest settle in this
// package (finalizer → credentials → workload → status), so the pass the caller
// measures afterwards is genuinely redundant.
func reconcileToFixedPoint(run func() (ctrl.Result, error)) {
	GinkgoHelper()
	for range 5 {
		_, err := run()
		Expect(err).NotTo(HaveOccurred())
	}
}

// divergence settles the reconciler, then reports everything one further,
// entirely redundant pass did that it should not have: each write it issued,
// and each owned object whose resourceVersion moved.
//
// wantKinds names the objects the shape is supposed to have produced. Asserting
// them is what keeps the invariant from passing vacuously: an owner that owns
// nothing writes nothing on the next pass too.
func divergence(
	ctx context.Context,
	rec *writeRecorder,
	run func() (ctrl.Result, error),
	owner client.Object,
	wantKinds ...string,
) []string {
	GinkgoHelper()
	ns := owner.GetNamespace()
	reconcileToFixedPoint(run)
	before := ownedResourceVersions(ctx, owner, ns)
	owned := make([]string, 0, len(before))
	for key := range before {
		owned = append(owned, strings.SplitN(key, "/", 2)[0])
	}
	Expect(owned).To(ContainElements(wantKinds), "%s must own every kind its shape produces", owner.GetName())
	rec.reset()
	_, err := run()
	Expect(err).NotTo(HaveOccurred())
	return append(rec.sorted(), changedObjects(before, ownedResourceVersions(ctx, owner, ns))...)
}

// deleteOwner removes the CR and drives its reconciler through the deletion
// path, so the next spec starts from a clean namespace (envtest runs no GC).
// A teardown error stops the drain rather than failing the spec that already
// passed; the leftover object is named after its spec, so a later collision is
// traceable.
func deleteOwner(ctx context.Context, owner client.Object, run func() (ctrl.Result, error)) {
	GinkgoHelper()
	Expect(k8sClient.Delete(ctx, owner)).To(Succeed())
	for range 3 {
		if _, err := run(); err != nil {
			return
		}
	}
}

var _ = Describe("Reconcile convergence (w7/m84)", func() {
	const namespace = "default"
	ctx := context.Background()

	It("writes nothing on a redundant App reconcile, for every workload shape", func() {
		for _, tc := range []struct {
			name string
			spec appv1alpha1.AppSpec
			owns []string
		}{
			{name: "converge-web", owns: []string{"Deployment", "Service"}, spec: appv1alpha1.AppSpec{
				Image: "ghcr.io/example/web:v1", Port: 3000, HealthCheckPath: "/healthz",
			}},
			{name: "converge-tcp", owns: []string{"Deployment", "Service"}, spec: appv1alpha1.AppSpec{
				// No healthCheckPath: the probes are TCPSocket rather than HTTPGet,
				// which is a different set of defaulted probe fields.
				Image: "ghcr.io/example/tcp:v1", Port: 3000,
			}},
			{name: "converge-routed", owns: []string{"Deployment", "Service", "Ingress", "Middleware"}, spec: appv1alpha1.AppSpec{
				// Ingress + both ipAllowList middlewares + the websocket meter
				// middleware + a projected secret-files volume, so the Traefik
				// middleware projection and the volume defaults are covered too.
				Image: "ghcr.io/example/routed:v1", Port: 8080, Expose: true,
				Hosts:                  []string{"routed.example.test"},
				IPAllowList:            []string{"10.0.0.0/8"},
				EnvironmentIPAllowList: []string{"192.168.0.0/16"},
				FilesFromSecrets:       []string{"routed-files"},
			}},
			{name: "converge-worker", owns: []string{"Deployment"}, spec: appv1alpha1.AppSpec{
				Image: "ghcr.io/example/worker:v1", Type: appv1alpha1.TypeBackgroundWorker,
			}},
			{name: "converge-cron", owns: []string{"CronJob"}, spec: appv1alpha1.AppSpec{
				Image: "ghcr.io/example/cron:v1", Type: appv1alpha1.TypeCronJob,
				Schedule: "*/5 * * * *",
			}},
		} {
			rec := &writeRecorder{Client: k8sClient, recordStatus: tc.spec.Type == appv1alpha1.TypeCronJob}
			r := &AppReconciler{
				Client: rec, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
				BaseDomain: "example.test", ClusterIssuer: "letsencrypt-staging",
			}
			nn := types.NamespacedName{Name: tc.name, Namespace: namespace}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: namespace},
				Spec:       tc.spec,
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			run := func() (ctrl.Result, error) {
				return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			Expect(divergence(ctx, rec, run, app, tc.owns...)).To(BeEmpty(),
				"%s: a converged App reconcile must write nothing and change no owned object", tc.name)

			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			deleteOwner(ctx, app, run)
		}
	})

	It("writes nothing on a redundant KeyValue reconcile, with and without backups", func() {
		for _, tc := range []struct {
			name   string
			owns   []string
			backup BackupStore
		}{
			{name: "converge-kv", owns: []string{"StatefulSet", "Service", "Secret", "NetworkPolicy"}},
			{name: "converge-kv-backed", owns: []string{"StatefulSet", "Service", "Secret", "NetworkPolicy", "CronJob"}, backup: BackupStore{
				DestinationPath: "s3://bex-tfstate/keyvalue",
				EndpointURL:     "https://s3.example.test",
				S3Secret:        "bex-kv-backup-s3",
			}},
		} {
			rec := &writeRecorder{Client: k8sClient, recordStatus: true}
			r := &KeyValueReconciler{Client: rec, Scheme: k8sClient.Scheme(), Backup: tc.backup}
			nn := types.NamespacedName{Name: tc.name, Namespace: namespace}
			kv := &appv1alpha1.KeyValue{
				ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: namespace},
				Spec:       appv1alpha1.KeyValueSpec{Name: tc.name, Plan: "standard"},
			}
			Expect(k8sClient.Create(ctx, kv)).To(Succeed())
			run := func() (ctrl.Result, error) {
				return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
			Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
			Expect(divergence(ctx, rec, run, kv, tc.owns...)).To(BeEmpty(),
				"%s: a converged KeyValue reconcile must write nothing and change no owned object", tc.name)

			// Supply the StatefulSet-controller observation absent in envtest.
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			pvc := sts.Spec.VolumeClaimTemplates[0].DeepCopy()
			pvc.ObjectMeta = metav1.ObjectMeta{Name: keyValuePVCName(kv.Name), Namespace: namespace}
			pvc.Spec.VolumeName = "pv-" + kv.Name
			Expect(k8sClient.Create(ctx, pvc)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, pvc)).To(Succeed()) })
			pvc.Status.Phase = corev1.ClaimBound
			pvc.Status.Capacity = pvc.Spec.Resources.Requests.DeepCopy()
			Expect(k8sClient.Status().Update(ctx, pvc)).To(Succeed())
			sts.Status = appsv1.StatefulSetStatus{
				ObservedGeneration: sts.Generation, Replicas: 1, CurrentReplicas: 1,
				UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1,
				CurrentRevision: "ready", UpdateRevision: "ready",
			}
			Expect(k8sClient.Status().Update(ctx, sts)).To(Succeed())
			Expect(divergence(ctx, rec, run, kv, tc.owns...)).To(BeEmpty())
			Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
			Expect(kv.Status.Phase).To(Equal(appv1alpha1.KVPhaseReady))

			kv.Spec.Suspended = true
			Expect(k8sClient.Update(ctx, kv)).To(Succeed())
			Expect(divergence(ctx, rec, run, kv, tc.owns...)).To(BeEmpty())
			Expect(k8sClient.Get(ctx, nn, kv)).To(Succeed())
			Expect(kv.Status.Phase).To(Equal(appv1alpha1.KVPhaseReady))
			Expect(kv.Status.Conditions).To(ContainElement(HaveField("Reason", reasonSuspended)))
			deleteOwner(ctx, kv, run)
		}
	})

	It("writes nothing on a redundant Database reconcile", func() {
		const name = "dpg-converge"
		rec := &writeRecorder{Client: k8sClient, recordStatus: true}
		r := &DatabaseReconciler{Client: rec, Scheme: k8sClient.Scheme()}
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		db := &appv1alpha1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       appv1alpha1.DatabaseSpec{Name: "converge", Plan: "basic-1gb"},
		}
		Expect(k8sClient.Create(ctx, db)).To(Succeed())
		run := func() (ctrl.Result, error) {
			return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		Expect(k8sClient.Get(ctx, nn, db)).To(Succeed())
		Expect(divergence(ctx, rec, run, db, "Cluster")).To(BeEmpty(),
			"a converged Database reconcile must write nothing and change no owned object")

		cluster := &unstructured.Unstructured{}
		cluster.SetGroupVersionKind(cnpgClusterGVK)
		Expect(k8sClient.Get(ctx, nn, cluster)).To(Succeed())
		// Simulate the live CNPG webhook defaults absent from the stub CRD.
		Expect(unstructured.SetNestedField(cluster.Object, true, "spec", "enablePDB")).To(Succeed())
		Expect(unstructured.SetNestedField(cluster.Object, "logical", "spec", "postgresql", "parameters", "wal_level")).To(Succeed())
		Expect(unstructured.SetNestedField(cluster.Object, "UTF8", "spec", "bootstrap", "initdb", "encoding")).To(Succeed())
		Expect(k8sClient.Update(ctx, cluster)).To(Succeed())
		rec.reset()
		_, err := run()
		Expect(err).NotTo(HaveOccurred())
		Expect(rec.sorted()).To(BeEmpty(), "webhook-defaulted Cluster must cause zero update requests")
		Expect(unstructured.SetNestedField(cluster.Object, int64(1), "status", "readyInstances")).To(Succeed())
		Expect(k8sClient.Status().Update(ctx, cluster)).To(Succeed())
		Expect(divergence(ctx, rec, run, db, "Cluster")).To(BeEmpty())
		Expect(k8sClient.Get(ctx, nn, db)).To(Succeed())
		Expect(db.Status.Phase).To(Equal(appv1alpha1.DBPhaseReady))
		deleteOwner(ctx, db, run)
	})

	// Anti-tautology: the invariant above is only worth its green if it goes red
	// on the regression it exists to prevent. Re-run the reconciler's own
	// Deployment CreateOrUpdate with the pre-w7/m84 projection — the same mutate,
	// minus the server defaults — and prove the recorder sees the PUT.
	It("turns red when a projection stops writing the server defaults", func() {
		const name = "converge-regression"
		rec := &writeRecorder{Client: k8sClient}
		r := &AppReconciler{Client: rec, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "ghcr.io/example/regress:v1", Port: 3000, HealthCheckPath: "/healthz",
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		run := func() (ctrl.Result, error) {
			return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
		reconcileToFixedPoint(run)
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())

		params := deploymentParams{image: app.Spec.Image, port: 3000, replicas: 1}

		// The control: the shipped projection, through the very call the
		// reconciler makes. OperationResultNone in so many words.
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		op, err := controllerutil.CreateOrUpdate(ctx, rec, dep, func() error {
			applyDeploymentSpec(dep, app, params)
			return controllerutil.SetControllerReference(app, dep, r.Scheme)
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(op).To(Equal(controllerutil.OperationResultNone))

		for _, tc := range []struct {
			field   string
			regress func(*corev1.PodSpec)
		}{
			{"pod terminationGracePeriodSeconds", func(s *corev1.PodSpec) {
				s.TerminationGracePeriodSeconds = nil
			}},
			{"container terminationMessagePath/Policy", func(s *corev1.PodSpec) {
				s.Containers[0].TerminationMessagePath = ""
				s.Containers[0].TerminationMessagePolicy = ""
			}},
			{"container port protocol", func(s *corev1.PodSpec) {
				s.Containers[0].Ports[0].Protocol = ""
			}},
			{"probe successThreshold", func(s *corev1.PodSpec) {
				s.Containers[0].ReadinessProbe.SuccessThreshold = 0
			}},
			{"readiness periodSeconds/failureThreshold", func(s *corev1.PodSpec) {
				s.Containers[0].ReadinessProbe.PeriodSeconds = 0
				s.Containers[0].ReadinessProbe.FailureThreshold = 0
			}},
			{"httpGet scheme", func(s *corev1.PodSpec) {
				s.Containers[0].ReadinessProbe.HTTPGet.Scheme = ""
			}},
		} {
			rec.reset()
			regressed := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
			_, err := controllerutil.CreateOrUpdate(ctx, rec, regressed, func() error {
				applyDeploymentSpec(regressed, app, params)
				tc.regress(&regressed.Spec.Template.Spec)
				return controllerutil.SetControllerReference(app, regressed, r.Scheme)
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(rec.sorted()).To(ContainElement("update Deployment/"+name),
				"dropping the %s default must make CreateOrUpdate PUT again — otherwise the invariant proves nothing", tc.field)
		}

		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		deleteOwner(ctx, app, run)
	})
})
