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
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const (
	// kvStorageClass is the StorageClass for Valkey data volumes (same class the
	// Database controller uses for Postgres PVCs).
	kvStorageClass = "hcloud-volumes"
	// kvEntryPoint is the Traefik TCP entrypoint bound to :6379 (traefik values).
	kvEntryPoint = "valkey"
	// kvPort is the Valkey listen + Service port.
	kvPort = 6379
	// kvDataPath is where Valkey writes its AOF data inside the container.
	kvDataPath = "/data"
	// kvDefaultImage is the Valkey image used when spec.version is empty.
	kvDefaultImage = "valkey/valkey:8-alpine"
	// labelKeyValue marks the workload/Service a KeyValue creates, and is the
	// StatefulSet selector + Service selector so the two stay coupled.
	labelKeyValue = "app.bex.co/keyvalue"
)

// growOnlyStorage raises a requested volume size to a plan's floor — storage
// can grow past the plan, never shrink below it. Shared by the Database
// (resolvePlan) and KeyValue (resolveKVPlan) plan resolvers.
func growOnlyStorage(requested, floor int32) int32 {
	return max(requested, floor)
}

// resolveKVPlan returns the Valkey plan (defaulting to free) and the effective
// storage size in GB (never below the plan floor — storage only grows). The
// ladder is lego/types/tiers' valkey family, the KeyValue sibling of
// resolvePlan's postgres family.
func resolveKVPlan(spec appv1alpha1.KeyValueSpec) (tiers.ValkeyTier, int32) {
	plan, ok := tiers.Valkey.ByID(spec.Plan)
	if !ok {
		plan = tiers.Valkey.Default()
	}
	return plan, growOnlyStorage(spec.StorageGB, plan.StorageGB)
}

// valkeyImage resolves the Valkey image for a major version; empty => the
// operator default.
func valkeyImage(version string) string {
	if version == "" {
		return kvDefaultImage
	}
	return "valkey/valkey:" + version + "-alpine"
}

// kvResources maps a Valkey tier to a Guaranteed-QoS allocation via the shared
// guaranteedResources helper (requests == limits).
func kvResources(plan tiers.ValkeyTier) corev1.ResourceRequirements {
	return guaranteedResources(plan.CPU, plan.Memory)
}

// generatePassword returns a URL-safe random password for a Valkey instance.
// RawURLEncoding keeps it free of +, /, and = so it embeds cleanly in a
// redis://:<password>@host connection URI without escaping.
func generatePassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// KeyValueReconciler projects a KeyValue into a single-instance Valkey
// StatefulSet plus a headless Service (internal DNS) plus a credentials Secret,
// and optionally exposes an external SNI endpoint via Traefik. It is a thin
// executor — the Valkey image does the actual key-value lifecycle.
type KeyValueReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// KvDomain is the wildcard base for external endpoints, e.g. "kv.bex.co".
	// Empty => public KeyValues get no external route.
	KvDomain string
}

// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=traefik.io,resources=ingressroutetcps;middlewaretcps,verbs=get;list;watch;create;update;patch;delete
// Secrets access is namespace-scoped to the apps namespace via deploy/gitops/base/operator-apps-rbac.yaml.

func (r *KeyValueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var kv appv1alpha1.KeyValue
	if err := r.Get(ctx, req.NamespacedName, &kv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// Deletion: the StatefulSet, Service, Secret, and any route are owned by the
	// KeyValue, so they are garbage-collected automatically via owner refs.
	if !kv.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	plan, storageGB := resolveKVPlan(kv.Spec)
	internalHost := fmt.Sprintf("%s.%s.svc", kv.Name, kv.Namespace)
	// Suspended => scale to zero (the PVC, Secret, Service and route are kept, so
	// resume restores the same data, password and endpoint). Render's KV suspend.
	replicas := plan.Instances
	if kv.Spec.Suspended {
		replicas = 0
	}
	labels := map[string]string{labelKeyValue: kv.Name}
	// Build pod-template labels: selector labels plus the workspace label so
	// same-workspace NetworkPolicy selectors can reach the Valkey instance.
	podLabels := map[string]string{labelKeyValue: kv.Name}
	if ws := kv.Labels[labelWorkspace]; ws != "" {
		podLabels[labelWorkspace] = ws
	}

	// --- credentials Secret (created first so the StatefulSet's env ref resolves) ---
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sec, func() error {
		if sec.Data == nil {
			sec.Data = map[string][]byte{}
		}
		// Reuse an existing password; only generate on first creation.
		if _, ok := sec.Data["password"]; !ok {
			pw, err := generatePassword()
			if err != nil {
				return err
			}
			sec.Data["password"] = []byte(pw)
		}
		password := string(sec.Data["password"])
		sec.Data["username"] = []byte("default")
		sec.Data["host"] = []byte(internalHost)
		sec.Data["port"] = []byte(strconv.Itoa(kvPort))
		uri := fmt.Sprintf("redis://:%s@%s:%d", password, internalHost, kvPort)
		sec.Data["uri"] = []byte(uri)
		if kv.Spec.Public && r.KvDomain != "" {
			external := fmt.Sprintf("%s.%s", kv.Name, r.KvDomain)
			externalURI := fmt.Sprintf("rediss://:%s@%s:%d", password, external, kvPort)
			sec.Data["externalUri"] = []byte(externalURI)
		} else {
			delete(sec.Data, "externalUri")
		}
		return controllerutil.SetControllerReference(&kv, sec, r.Scheme)
	}); err != nil {
		return r.kvFail(ctx, &kv, "SecretFailed", err)
	}

	// --- headless Service: gives the internal DNS "<name>.<ns>.svc" and is the
	// StatefulSet's serviceName (stable pod identity). ---
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		svc.Spec.Selector = labels
		svc.Spec.Ports = []corev1.ServicePort{{Port: kvPort, TargetPort: intstr.FromInt(kvPort), Name: "valkey"}}
		return controllerutil.SetControllerReference(&kv, svc, r.Scheme)
	}); err != nil {
		return r.kvFail(ctx, &kv, "ServiceFailed", err)
	}

	// --- single-instance Valkey StatefulSet, sized to the tier ---
	storageClass := kvStorageClass
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		// selector / serviceName / volumeClaimTemplates are immutable on a
		// StatefulSet — set them once, at create. The pod template (resources,
		// image, env) is reapplied every reconcile so a plan/version bump rolls.
		if sts.CreationTimestamp.IsZero() {
			sts.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
			sts.Spec.ServiceName = kv.Name
			sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data"},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", storageGB)),
						},
					},
					StorageClassName: &storageClass,
				},
			}}
		}
		sts.Spec.Replicas = &replicas
		sts.Spec.Template.Labels = podLabels
		sts.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:  "valkey",
			Image: valkeyImage(kv.Spec.Version),
			// VALKEY_PASSWORD (env, below) expands in args — k8s substitutes
			// $(VAR) from the container env list. appendonly persists to the PVC.
			Args:  []string{"--requirepass", "$(VALKEY_PASSWORD)", "--appendonly", "yes"},
			Ports: []corev1.ContainerPort{{ContainerPort: kvPort, Name: "valkey"}},
			Env: []corev1.EnvVar{{
				Name: "VALKEY_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: kv.Name},
					Key:                  "password",
				}},
			}},
			Resources:    kvResources(plan),
			VolumeMounts: []corev1.VolumeMount{{Name: "data", MountPath: kvDataPath}},
			ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(kvPort)},
			}},
		}}
		return controllerutil.SetControllerReference(&kv, sts, r.Scheme)
	}); err != nil {
		return r.kvFail(ctx, &kv, "StatefulSetFailed", err)
	}

	// --- optional external SNI endpoint via Traefik (mirrors the Database route) ---
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(traefikIngressRouteTCPGVK)
	route.SetName(kv.Name + "-kv")
	route.SetNamespace(kv.Namespace)
	mwName := kv.Name + "-kv-allow"
	if kv.Spec.Public && r.KvDomain != "" {
		// IP allowlist: a middleware referenced by the SNI route when CIDRs are
		// set (the same gate the Database external route uses). Empty list =>
		// no middleware, open route; internal access is never gated.
		var middlewares []any
		if len(kv.Spec.IPAllowList) > 0 {
			if err := upsertOwned(ctx, r.Client, r.Scheme, &kv, traefikMiddlewareTCPGVK, mwName, ipAllowListMiddlewareSpec(kv.Spec.IPAllowList)); err != nil {
				return r.kvFail(ctx, &kv, "RouteFailed", err)
			}
			middlewares = []any{map[string]any{"name": mwName, "namespace": kv.Namespace}}
		} else if err := deleteOwned(ctx, r.Client, &kv, traefikMiddlewareTCPGVK, mwName); err != nil {
			return r.kvFail(ctx, &kv, "RouteFailed", err)
		}
		externalHost := fmt.Sprintf("%s.%s", kv.Name, r.KvDomain)
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, route, func() error {
			route.Object["spec"] = ingressRouteTCPSpec(kvEntryPoint, externalHost, kv.Name, kvPort, middlewares)
			return controllerutil.SetControllerReference(&kv, route, r.Scheme)
		}); err != nil {
			return r.kvFail(ctx, &kv, "RouteFailed", err)
		}
		kv.Status.ExternalHost = externalHost
	} else {
		// Not public (or no base domain): best-effort remove any route and
		// allowlist middleware we made.
		if err := deleteOptionalObject(ctx, r.Client, route); err != nil {
			return r.kvFail(ctx, &kv, "RouteCleanupFailed", err)
		}
		if err := deleteOwned(ctx, r.Client, &kv, traefikMiddlewareTCPGVK, mwName); err != nil {
			return r.kvFail(ctx, &kv, "RouteCleanupFailed", err)
		}
		kv.Status.ExternalHost = ""
	}

	// Surface the connection coordinates (the password stays in the Secret).
	kv.Status.Host = internalHost
	kv.Status.Port = kvPort
	kv.Status.SecretName = kv.Name
	kv.Status.ObservedGeneration = kv.Generation

	// Ready when the StatefulSet reports its desired replica count available. A
	// suspended store desires zero replicas, so it settles Ready immediately (the
	// separate spec.suspended is the client-facing suspend signal — the API
	// surfaces it via the Render "suspended" enum, distinct from this health phase).
	_ = r.Get(ctx, client.ObjectKeyFromObject(sts), sts)
	if sts.Status.AvailableReplicas >= replicas {
		reason, message := "Provisioned", "valkey ready"
		if kv.Spec.Suspended {
			reason, message = "Suspended", "valkey suspended (scaled to zero)"
		}
		kv.Status.Phase = appv1alpha1.KVPhaseReady
		meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionTrue, Reason: reason,
			Message: message, ObservedGeneration: kv.Generation,
		})
		if err := r.Status().Update(ctx, &kv); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("keyvalue reconciled", "name", kv.Name, "host", kv.Status.Host, "suspended", kv.Spec.Suspended)
		return ctrl.Result{}, nil
	}

	kv.Status.Phase = appv1alpha1.KVPhaseProvisioning
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: "Provisioning",
		Message: "waiting for valkey", ObservedGeneration: kv.Generation,
	})
	if err := r.Status().Update(ctx, &kv); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *KeyValueReconciler) kvFail(ctx context.Context, kv *appv1alpha1.KeyValue, reason string, err error) (ctrl.Result, error) {
	kv.Status.Phase = appv1alpha1.KVPhaseFailed
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionFalse, Reason: reason, Message: err.Error(),
		ObservedGeneration: kv.Generation,
	})
	_ = r.Status().Update(ctx, kv)
	return ctrl.Result{}, err
}

// SetupWithManager wires the controller. It owns the StatefulSet, Service,
// Secret, and (optional) Traefik route so deletes cascade and child changes
// re-trigger reconcile.
func (r *KeyValueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(traefikIngressRouteTCPGVK)
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.KeyValue{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		Owns(route).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Named("keyvalue").
		Complete(r)
}
