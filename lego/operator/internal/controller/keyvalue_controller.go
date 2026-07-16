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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
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
	// kvPort is the Valkey listen + Service port.
	kvPort = 6379
	// kvTLSPort is private to the public pass-through proxy. Keeping plaintext on
	// kvPort preserves existing in-cluster redis:// clients while external
	// rediss:// clients terminate end-to-end TLS inside Valkey.
	kvTLSPort = 6380
	// kvDataPath is where Valkey writes its AOF data inside the container.
	kvDataPath = "/data"
	// kvDefaultImage is the Valkey image used when spec.version is empty.
	kvDefaultImage = "valkey/valkey:8-alpine"
	// kvExporterPort / kvExporterImage back the redis_exporter metrics sidecar
	// (w5/011): it scrapes Valkey's INFO stats and exposes them as Prometheus
	// metrics on kvExporterPort, discovered by the valkey-instances scrape job.
	kvExporterPort  = 9121
	kvExporterImage = "oliver006/redis_exporter:alpine"
	// labelKeyValue marks the workload/Service a KeyValue creates, and is the
	// StatefulSet selector + Service selector so the two stay coupled.
	labelKeyValue = "app.bex.co/keyvalue"
)

var certManagerCertificateGVK = schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"}

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

// kvExporterResources is the fixed, tiny footprint for the redis_exporter
// sidecar — it only polls INFO, so it needs a fraction of a core and a few MiB,
// independent of the store's plan.
func kvExporterResources() corev1.ResourceRequirements {
	return guaranteedResources("10m", "32Mi")
}

// valkeyArgs builds the valkey-server flags from the KeyValue spec: the password,
// the persistence mode (Render's Persistence Mode → AOF/RDB), and — when an
// eviction policy is set — the memory budget and policy (Render's Maxmemory
// Policy). An empty PersistenceMode/MaxmemoryPolicy preserves the prior default
// (appendonly yes, no maxmemory), so a KeyValue created before these fields
// reconciles byte-identically.
func valkeyArgs(spec appv1alpha1.KeyValueSpec, plan tiers.ValkeyTier) []string {
	args := []string{"--requirepass", "$(VALKEY_PASSWORD)"}
	switch spec.PersistenceMode {
	case "off":
		// No AOF and no RDB save points — a pure in-memory cache.
		args = append(args, "--appendonly", "no", "--save", "")
	case "snapshot":
		// RDB snapshots only (the image's default save points); no AOF journal.
		args = append(args, "--appendonly", "no")
	default: // journal-snapshot and "" (the prior default): AOF + RDB.
		args = append(args, "--appendonly", "yes")
	}
	// Only bound memory when a policy is chosen; without maxmemory the policy has
	// nothing to evict against, and leaving both unset keeps legacy stores as they
	// were.
	if spec.MaxmemoryPolicy != "" {
		args = append(args, "--maxmemory", valkeyMaxmemory(plan), "--maxmemory-policy", spec.MaxmemoryPolicy)
	}
	return args
}

// valkeyMaxmemory returns the data budget (bytes) eviction triggers at: 80% of
// the plan's RAM, leaving headroom below the container memory limit so valkey's
// own eviction runs before the kernel OOM-kills the pod.
func valkeyMaxmemory(plan tiers.ValkeyTier) string {
	q := resource.MustParse(plan.Memory)
	return strconv.FormatInt(q.Value()*4/5, 10)
}

// generatePassword returns a URL-safe random password for a Valkey instance.
// RawURLEncoding keeps it free of +, /, and = so it embeds cleanly in a
// redis://default:<password>@host connection URI without escaping.
func generatePassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// KeyValueReconciler projects a KeyValue into a single-instance Valkey
// StatefulSet plus a headless Service (internal DNS) plus a credentials Secret,
// and optionally a cert-manager certificate for the metered SNI front door. It
// is a thin executor — the Valkey image does the actual key-value lifecycle.
type KeyValueReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// KvDomain is the wildcard base for external endpoints, e.g. "kv.bex.co".
	// Empty => public KeyValues get no external route.
	KvDomain string
	// ClusterIssuer signs the end-to-end Valkey server certificate used only by
	// the proxy-facing TLS port. Public endpoints fail closed when it is empty.
	ClusterIssuer string
}

// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=traefik.io,resources=ingressroutetcps;middlewaretcps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
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
	public := kv.Spec.Public && r.KvDomain != ""
	tlsSecretName := kv.Name + "-kv-tls"
	certificateName := tlsSecretName
	// Remove the old unmetered path before validating or creating the new one.
	// A missing TLS issuer must fail closed, not leave a legacy route serving.
	if err := cleanupLegacyKeyValueRoutes(ctx, r.Client, &kv); err != nil {
		return r.kvFail(ctx, &kv, "RouteCleanupFailed", err)
	}
	if public && r.ClusterIssuer == "" {
		kv.Status.ExternalHost = ""
		return r.kvFail(ctx, &kv, "TLSIssuerMissing", fmt.Errorf("BEX_CLUSTER_ISSUER is required for a public Key Value endpoint"))
	}
	if public {
		host := fmt.Sprintf("%s.%s", kv.Name, r.KvDomain)
		certificateSpec := map[string]any{
			"secretName": tlsSecretName,
			"dnsNames":   []any{host},
			"issuerRef": map[string]any{
				"name": r.ClusterIssuer, "kind": "ClusterIssuer",
			},
		}
		if err := upsertOwned(ctx, r.Client, r.Scheme, &kv, certManagerCertificateGVK, certificateName, certificateSpec); err != nil {
			return r.kvFail(ctx, &kv, "CertificateFailed", err)
		}
	} else if err := deleteOwned(ctx, r.Client, &kv, certManagerCertificateGVK, certificateName); err != nil {
		return r.kvFail(ctx, &kv, "CertificateCleanupFailed", err)
	}
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
		// Explicit "default" user, not the empty-username redis://:<password>@
		// shorthand: verified live against valkey-cli 8.1.8 (the tool the
		// dashboard's own CLI-command helper recommends) — the empty-username
		// form fails AUTH ("NOAUTH"/"WRONGPASS") against a --requirepass server
		// (which sets the ACL default user's password), while an explicit
		// default:<password> URI authenticates correctly. -a/plain AUTH
		// (single-arg) is unaffected; this is specifically a URI-parsing gap.
		uri := fmt.Sprintf("redis://default:%s@%s:%d", password, internalHost, kvPort)
		sec.Data["uri"] = []byte(uri)
		if kv.Spec.Public && r.KvDomain != "" {
			external := fmt.Sprintf("%s.%s", kv.Name, r.KvDomain)
			externalURI := fmt.Sprintf("rediss://default:%s@%s:%d", password, external, kvPort)
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
		if public {
			svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{Port: kvTLSPort, TargetPort: intstr.FromInt(kvTLSPort), Name: "valkey-tls"})
		}
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
		// PVC retention policy: delete PVCs when the StatefulSet is deleted (GA in
		// k8s 1.30, StatefulSetAutoDeletePVC feature gate) so Valkey data doesn't
		// orphan on KeyValue deletion. WhenScaled=Retain keeps PVCs on scale-down
		// (they'd re-attach on scale-up). This is mutable on an existing StatefulSet.
		sts.Spec.PersistentVolumeClaimRetentionPolicy = &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
			WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
			WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
		}
		sts.Spec.Replicas = &replicas
		sts.Spec.Template.Labels = podLabels
		// The Valkey password, shared by the server (arg expansion) and the metrics
		// exporter (authenticated INFO scrape).
		passwordEnv := corev1.EnvVar{
			Name: "VALKEY_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: kv.Name},
				Key:                  "password",
			}},
		}
		serverArgs := valkeyArgs(kv.Spec, plan)
		serverPorts := []corev1.ContainerPort{{ContainerPort: kvPort, Name: "valkey"}}
		var serverMounts []corev1.VolumeMount
		sts.Spec.Template.Spec.Volumes = nil
		if public {
			serverArgs = append(serverArgs,
				"--tls-port", strconv.Itoa(kvTLSPort),
				"--tls-cert-file", "/tls/tls.crt",
				"--tls-key-file", "/tls/tls.key",
				"--tls-ca-cert-file", "/tls/tls.crt",
				"--tls-auth-clients", "no",
			)
			serverPorts = append(serverPorts, corev1.ContainerPort{ContainerPort: kvTLSPort, Name: "valkey-tls"})
			serverMounts = append(serverMounts, corev1.VolumeMount{Name: "public-tls", MountPath: "/tls", ReadOnly: true})
			sts.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "public-tls", VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: tlsSecretName},
			}}}
		}
		sts.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:  "valkey",
			Image: valkeyImage(kv.Spec.Version),
			// VALKEY_PASSWORD (env, below) expands in args — k8s substitutes
			// $(VAR) from the container env list. appendonly persists to the PVC.
			Args:         serverArgs,
			Ports:        serverPorts,
			Env:          []corev1.EnvVar{passwordEnv},
			Resources:    kvResources(plan),
			VolumeMounts: append([]corev1.VolumeMount{{Name: "data", MountPath: kvDataPath}}, serverMounts...),
			ReadinessProbe: &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
				TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(kvPort)},
			}},
		}, {
			// redis_exporter sidecar: exposes Valkey's INFO stats as Prometheus
			// metrics (redis_memory_used_bytes, redis_connected_clients, …) on
			// :9121, scraped by the valkey-instances job (deploy/gitops/base/
			// prometheus.yaml) and surfaced as the Key Value metrics tab (w5/011).
			Name:  "metrics",
			Image: kvExporterImage,
			Env: []corev1.EnvVar{
				{Name: "REDIS_ADDR", Value: fmt.Sprintf("redis://localhost:%d", kvPort)},
				// The exporter reuses REDIS_PASSWORD; alias the shared secret key.
				{Name: "REDIS_PASSWORD", ValueFrom: passwordEnv.ValueFrom},
			},
			Ports:     []corev1.ContainerPort{{ContainerPort: kvExporterPort, Name: "metrics"}},
			Resources: kvExporterResources(),
		}}
		return controllerutil.SetControllerReference(&kv, sts, r.Scheme)
	}); err != nil {
		return r.kvFail(ctx, &kv, "StatefulSetFailed", err)
	}

	// --- optional external SNI endpoint via the metered Key Value front door ---
	// The shared kv-sni-proxy watches this KeyValue directly and owns SNI,
	// resource/environment IP allowlisting, TLS pass-through, and
	// backend→client accounting.
	if kv.Spec.Public && r.KvDomain != "" {
		externalHost := fmt.Sprintf("%s.%s", kv.Name, r.KvDomain)
		kv.Status.ExternalHost = externalHost
	} else {
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

func cleanupLegacyKeyValueRoutes(ctx context.Context, c client.Client, kv *appv1alpha1.KeyValue) error {
	route := &unstructured.Unstructured{}
	route.SetGroupVersionKind(traefikIngressRouteTCPGVK)
	route.SetName(kv.Name + "-kv")
	route.SetNamespace(kv.Namespace)
	if err := deleteOptionalObject(ctx, c, route); err != nil {
		return err
	}
	for _, name := range []string{kv.Name + "-kv-allow", kv.Name + "-kv-env-allow"} {
		if err := deleteOwned(ctx, c, kv, traefikMiddlewareTCPGVK, name); err != nil {
			return err
		}
	}
	return nil
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

// SetupWithManager wires the controller. It owns the StatefulSet, Service, and
// Secret; the unstructured Certificate also has an owner reference but needs no
// watch because cert-manager owns its issuance lifecycle.
func (r *KeyValueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.KeyValue{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Service{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Secret data updates do not increment metadata.generation. ResourceVersion
		// keeps credential drift self-healing while the manager cache scopes this
		// watch to BEX_APPS_NAMESPACE (NamespacedSecretCacheOptions).
		Owns(&corev1.Secret{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Named("keyvalue").
		Complete(r)
}
