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
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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
	// The uid/gid baked into the official valkey image. Running directly as this
	// account avoids its root entrypoint's CAP_CHOWN requirement on fresh PVCs.
	valkeyRunAsUser  = int64(999)
	valkeyRunAsGroup = int64(1000)
	// kvDefaultImage is the Valkey image used when spec.version is empty.
	kvDefaultImage = "valkey/valkey:8-alpine"
	// kvExporterPort / kvExporterImage back the redis_exporter metrics sidecar
	// (w5/011): it scrapes Valkey's INFO stats and exposes them as Prometheus
	// metrics on kvExporterPort, discovered by the valkey-instances scrape job.
	kvExporterPort  = 9121
	kvExporterImage = "oliver006/redis_exporter:alpine"
	// labelKeyValue marks the workload/Service a KeyValue creates, and is the
	// StatefulSet selector + Service selector so the two stay coupled.
	labelKeyValue           = "app.bex.co/keyvalue"
	kvStorageRequeue        = 10 * time.Second
	kvStorageFailureRequeue = 5 * time.Minute
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

func valkeySecCtx() *corev1.SecurityContext {
	security := tenantSecCtx()
	security.RunAsNonRoot = ptr(true)
	security.RunAsUser = ptr(valkeyRunAsUser)
	security.RunAsGroup = ptr(valkeyRunAsGroup)
	return security
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

func keyValueConnectionSecretData(password, internalHost string, public bool, name, domain string) map[string][]byte {
	data := map[string][]byte{
		"username": []byte("default"),
		"password": []byte(password),
		"host":     []byte(internalHost),
		"port":     []byte(strconv.Itoa(kvPort)),
		// Explicit default user is required by valkey-cli URI authentication.
		"uri": fmt.Appendf(nil, "redis://default:%s@%s:%d", password, internalHost, kvPort),
	}
	if public {
		external := fmt.Sprintf("%s.%s", name, domain)
		data["externalUri"] = fmt.Appendf(nil, "rediss://default:%s@%s:%d", password, external, kvPort)
	}
	return data
}

func secretDataEqual(left, right map[string][]byte) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if !bytes.Equal(value, right[key]) {
			return false
		}
	}
	return true
}

// KeyValueReconciler projects a KeyValue into a single-instance Valkey
// StatefulSet plus a headless Service (internal DNS) plus a credentials Secret,
// and optionally a cert-manager certificate for the metered SNI front door. It
// is a thin executor — the Valkey image does the actual key-value lifecycle.
type KeyValueReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Backup is the non-secret object-store contract for paid-plan RDB
	// snapshots. All fields are required; an incomplete contract is disabled.
	Backup BackupStore
	// KvDomain is the wildcard base for external endpoints, e.g. "kv.bex.co".
	// Empty => public KeyValues get no external route.
	KvDomain string
	// ClusterIssuer signs the end-to-end Valkey server certificate used only by
	// the proxy-facing TLS port. Public endpoints fail closed when it is empty.
	ClusterIssuer string
	// SecretClient reads and writes this KeyValue's credential Secrets. It must
	// be an UNCACHED client: the manager's Secret informer covers exactly one
	// namespace (NamespacedSecretCacheOptions), while under ADR043 D8 a KeyValue
	// lives in its workspace's own `<ws>` namespace. Widening that informer is
	// not an option — the operator's ClusterRole deliberately omits cluster-wide
	// Secrets (w7/m7), so a cluster-wide Secret list would fail and stop the
	// entire shared cache, App controller included, from ever starting.
	// Nil falls back to the cached client, which keeps tests and embedders that
	// do not wire one working exactly as before.
	SecretClient client.Client
	// BackupSourceNamespace holds the GitOps-installed S3 credential that tenant
	// namespaces are projected from (ADR043 D8.4) — the shared apps namespace.
	// Empty disables projection, correct for a single-namespace deployment.
	BackupSourceNamespace string
}

// secretClient returns the uncached client for tenant-namespace Secret access —
// the same escape hatch the App path uses (AppReconciler.buildPlaneClient).
func (r *KeyValueReconciler) secretClient() client.Client {
	if r.SecretClient != nil {
		return r.SecretClient
	}
	return r.Client
}

// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=app.bex.co,resources=keyvalues/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=storage.k8s.io,resources=storageclasses,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=traefik.io,resources=ingressroutetcps;middlewaretcps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete
// Secrets access is namespace-scoped to the apps namespace via deploy/gitops/base/operator-apps-rbac.yaml.

func (r *KeyValueReconciler) keyValueStorageIntent(
	ctx context.Context,
	kv *appv1alpha1.KeyValue,
	desiredGB int32,
) (*appsv1.StatefulSet, int32, ctrl.Result, bool, error) {
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
	if err := r.Get(ctx, client.ObjectKeyFromObject(sts), sts); err != nil && !apierrors.IsNotFound(err) {
		result, failErr := r.kvFail(ctx, kv, "StatefulSetReadFailed", err)
		return nil, 0, result, true, failErr
	}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: keyValuePVCName(kv.Name), Namespace: kv.Namespace,
	}}
	if err := r.Get(ctx, client.ObjectKeyFromObject(pvc), pvc); err != nil && !apierrors.IsNotFound(err) {
		result, failErr := r.kvFail(ctx, kv, "PVCReadFailed", err)
		return nil, 0, result, true, failErr
	}
	currentGB := max(kv.Status.AllocatedStorageGB, statefulSetStorageGB(sts), pvcRequestedStorageGB(pvc), pvcCapacityStorageGB(pvc))
	intentChanged := kv.Status.AllocatedStorageGB > 0 && kv.Status.ObservedStorageGB != kv.Spec.StorageGB
	if intentChanged && kv.Spec.StorageGB > 0 && currentGB > kv.Spec.StorageGB {
		return nil, 0, ctrl.Result{}, true, r.rejectKeyValueStorageShrink(ctx, kv, currentGB, kv.Spec.StorageGB)
	}
	return sts, max(desiredGB, currentGB), ctrl.Result{}, false, nil
}

func (r *KeyValueReconciler) reconcileKeyValueTLS(
	ctx context.Context,
	kv *appv1alpha1.KeyValue,
	public bool,
	certificateName string,
	tlsSecretName string,
) (string, error) {
	if public && r.ClusterIssuer == "" {
		kv.Status.ExternalHost = ""
		return "TLSIssuerMissing", fmt.Errorf("BEX_CLUSTER_ISSUER is required for a public Key Value endpoint")
	}
	if !public {
		return "CertificateCleanupFailed", deleteOwned(ctx, r.Client, kv, certManagerCertificateGVK, certificateName)
	}
	host := fmt.Sprintf("%s.%s", kv.Name, r.KvDomain)
	spec := map[string]any{
		"secretName": tlsSecretName,
		"dnsNames":   []any{host},
		"issuerRef": map[string]any{
			"name": r.ClusterIssuer, "kind": "ClusterIssuer",
		},
	}
	return "CertificateFailed", upsertOwned(ctx, r.Client, r.Scheme, kv, certManagerCertificateGVK, certificateName, spec)
}

func (r *KeyValueReconciler) reconcileKeyValueCredentials(
	ctx context.Context,
	kv *appv1alpha1.KeyValue,
	internalHost string,
) (*corev1.Secret, ctrl.Result, bool, error) {
	// The auth Secret is the only password authority and is immutable. Seed it
	// from the legacy connection Secret during migration so an upgrade never
	// rotates an existing store.
	connection := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
	if err := r.secretClient().Get(ctx, client.ObjectKeyFromObject(connection), connection); err != nil && !apierrors.IsNotFound(err) {
		result, failErr := r.kvFail(ctx, kv, "SecretFailed", err)
		return nil, result, true, failErr
	}
	seedPassword := connection.Data["password"]
	auth := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-auth", Namespace: kv.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.secretClient(), auth, func() error {
		if auth.Data == nil {
			auth.Data = map[string][]byte{}
		}
		if len(auth.Data["password"]) == 0 {
			if len(seedPassword) > 0 {
				auth.Data["password"] = bytes.Clone(seedPassword)
			} else {
				password, err := generatePassword()
				if err != nil {
					return err
				}
				auth.Data["password"] = []byte(password)
			}
		}
		auth.Data["username"] = []byte("default")
		auth.Immutable = ptr(true)
		return controllerutil.SetControllerReference(kv, auth, r.Scheme)
	}); err != nil {
		result, failErr := r.kvFail(ctx, kv, "CredentialSecretFailed", err)
		return nil, result, true, failErr
	}
	desiredData := keyValueConnectionSecretData(string(auth.Data["password"]), internalHost,
		kv.Spec.Public && r.KvDomain != "", kv.Name, r.KvDomain)
	if connection.Immutable != nil && *connection.Immutable && !secretDataEqual(connection.Data, desiredData) {
		if err := r.Delete(ctx, connection); err != nil && !apierrors.IsNotFound(err) {
			result, failErr := r.kvFail(ctx, kv, "SecretRecreateFailed", err)
			return nil, result, true, failErr
		}
		kv.Status.Phase = appv1alpha1.KVPhaseProvisioning
		meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
			Reason: "ConnectionSecretRebuilding", Message: "rebuilding immutable connection information", ObservedGeneration: kv.Generation})
		if err := r.Status().Update(ctx, kv); err != nil {
			return nil, ctrl.Result{}, true, err
		}
		return nil, ctrl.Result{RequeueAfter: time.Second}, true, nil
	}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.secretClient(), connection, func() error {
		connection.Data = desiredData
		connection.Immutable = ptr(true)
		return controllerutil.SetControllerReference(kv, connection, r.Scheme)
	}); err != nil {
		result, failErr := r.kvFail(ctx, kv, "SecretFailed", err)
		return nil, result, true, failErr
	}
	return auth, ctrl.Result{}, false, nil
}

// reconcileKeyValueBackupNetworkPolicy supplies the egress the backup and purge
// Jobs need on top of the platform-wide Cilium node/metadata egress deny
// (deploy/gitops/base/tenant-node-egress.yaml). That CiliumNetworkPolicy selects
// every app.bex.co/workspace-labelled pod into egress default-deny and relies on
// each tenant workload's own operator-managed allow to restore DNS, in-namespace,
// and internet egress — Apps get theirs from their hosting namespace's
// namespace-wide policies. A KeyValue's Jobs carry the workspace label yet run in
// the shared apps namespace with no such allow, so the snapshot step cannot
// resolve the Valkey Service (DNS "Try again") and no backup ever uploads
// (w5/039). This k8s NetworkPolicy is additive — the Cilium deny still wins for
// the node/metadata ranges — and is a harmless no-op where a namespace-wide allow
// already covers these pods. It is scoped to the Job pods (app.bex.co/component in
// {keyvalue-backup, keyvalue-backup-purge}); the Valkey server pods, which never
// dial out, keep their default-deny egress untouched.
func (r *KeyValueReconciler) reconcileKeyValueBackupNetworkPolicy(ctx context.Context, kv *appv1alpha1.KeyValue) error {
	udp, tcp := corev1.ProtocolUDP, corev1.ProtocolTCP
	dnsPort := intstr.FromInt(53)
	valkeyPort := intstr.FromInt(kvPort)
	valkeyTLSPort := intstr.FromInt(kvTLSPort)
	np := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: kv.Name + "-backup-egress", Namespace: kv.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, np, func() error {
		np.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{labelKeyValue: kv.Name},
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "app.bex.co/component",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{keyValueBackupComponent, keyValueBackupPurgeComponent},
				}},
			},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					// DNS to kube-system CoreDNS — resolve the Valkey Service + S3 host.
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: &udp, Port: &dnsPort},
						{Protocol: &tcp, Port: &dnsPort},
					},
					To: []networkingv1.NetworkPolicyPeer{{
						NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "kube-system",
						}},
					}},
				},
				{
					// The snapshot step dials the Valkey Service (6379, or 6380 when
					// public TLS is on) — a private pod IP the internet rule excludes,
					// so it needs its own in-namespace allow to the KeyValue's pods.
					Ports: []networkingv1.NetworkPolicyPort{
						{Protocol: &tcp, Port: &valkeyPort},
						{Protocol: &tcp, Port: &valkeyTLSPort},
					},
					To: []networkingv1.NetworkPolicyPeer{{
						PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyValue: kv.Name}},
					}},
				},
				{
					// Public internet minus in-cluster/private + metadata ranges: the
					// upload/purge step reaches the object store. Mirrors the hosting-
					// namespace internet-egress shape (ADR022/ADR043).
					To: []networkingv1.NetworkPolicyPeer{{
						IPBlock: &networkingv1.IPBlock{
							CIDR: "0.0.0.0/0",
							Except: []string{
								"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
								"100.64.0.0/10", "169.254.0.0/16",
							},
						},
					}},
				},
			},
		}
		return controllerutil.SetControllerReference(kv, np, r.Scheme)
	})
	return err
}

func (r *KeyValueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var kv appv1alpha1.KeyValue
	if err := r.Get(ctx, req.NamespacedName, &kv); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !kv.DeletionTimestamp.IsZero() {
		return r.handleKeyValueDeletion(ctx, &kv)
	}
	// codex #11: refuse a KeyValue outside its canonical tenant namespace before
	// adding a finalizer or creating dependent resources. Same confused-deputy
	// threat as the Database guard above.
	if !canonicalNamespace(&kv.ObjectMeta) {
		logf.FromContext(ctx).Info("refusing KeyValue outside a canonical tenant namespace (codex #11)",
			"namespace", kv.Namespace, "name", kv.Name)
		return ctrl.Result{}, nil
	}

	plan, requestedStorageGB := resolveKVPlan(kv.Spec)
	backupEnabled := keyValueBackupsEnabled(plan, r.Backup)
	// Only a KeyValue that can write backups needs the purge finalizer. This
	// keeps the fully-disabled path byte-identical while retaining cleanup after
	// a paid store is later downgraded to Free.
	if backupEnabled && controllerutil.AddFinalizer(&kv, kvFinalizer) {
		if err := r.Update(ctx, &kv); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	sts, storageGB, result, done, err := r.keyValueStorageIntent(ctx, &kv, requestedStorageGB)
	if done || err != nil {
		return result, err
	}
	internalHost := fmt.Sprintf("%s.%s.svc", kv.Name, kv.Namespace)
	public := kv.Spec.Public && r.KvDomain != ""
	tlsSecretName := kv.Name + "-kv-tls"
	certificateName := tlsSecretName
	// Validate public TLS before updating the workload. A missing issuer must
	// fail closed without publishing a public host.
	if reason, err := r.reconcileKeyValueTLS(ctx, &kv, public, certificateName, tlsSecretName); err != nil {
		return r.kvFail(ctx, &kv, reason, err)
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

	// --- credentials Secrets (created first so the StatefulSet's env ref resolves) ---
	auth, result, done, err := r.reconcileKeyValueCredentials(ctx, &kv, internalHost)
	if done || err != nil {
		return result, err
	}
	authSecretName := auth.Name

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
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		// selector / serviceName / volumeClaimTemplates are immutable on a
		// StatefulSet — set them once, at create. The pod template (resources,
		// image, env) is reapplied every reconcile so a plan/version bump rolls.
		if sts.CreationTimestamp.IsZero() {
			sts.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
			sts.Spec.ServiceName = kv.Name
			sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "data", Labels: labels},
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
		// A managed key-value store is a single-replica stateful service: an
		// eviction is downtime, so the cluster-autoscaler must never bin-pack
		// its node away. Manual/CAPI drains (node upgrades) still evict it —
		// the StatefulSet reschedules it elsewhere.
		sts.Spec.Template.Annotations = map[string]string{
			"cluster-autoscaler.kubernetes.io/safe-to-evict": "false",
			"app.bex.co/credential-revision":                 appv1alpha1.KeyValueCredentialRevision(auth.Data["password"]),
		}
		// The Valkey password, shared by the server (arg expansion) and the metrics
		// exporter (authenticated INFO scrape).
		passwordEnv := corev1.EnvVar{
			Name: "VALKEY_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: authSecretName},
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
		// Harden the managed Valkey pod the same way tenant Deployments are (w1/m53):
		// drop ALL caps, no privilege escalation, RuntimeDefault seccomp, and no
		// ServiceAccount token mounted (Valkey never talks to the apiserver).
		sts.Spec.Template.Spec.AutomountServiceAccountToken = ptr(false)
		sts.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			FSGroup:             ptr(valkeyRunAsGroup),
			FSGroupChangePolicy: ptr(corev1.FSGroupChangeOnRootMismatch),
		}
		sts.Spec.Template.Spec.Containers = []corev1.Container{{
			Name:  "valkey",
			Image: valkeyImage(kv.Spec.Version),
			// VALKEY_PASSWORD (env, below) expands in args — k8s substitutes
			// $(VAR) from the container env list. appendonly persists to the PVC.
			Args:            serverArgs,
			Ports:           serverPorts,
			Env:             []corev1.EnvVar{passwordEnv},
			Resources:       kvResources(plan),
			SecurityContext: valkeySecCtx(),
			VolumeMounts:    append([]corev1.VolumeMount{{Name: "data", MountPath: kvDataPath}}, serverMounts...),
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
			Ports:           []corev1.ContainerPort{{ContainerPort: kvExporterPort, Name: "metrics"}},
			Resources:       kvExporterResources(),
			SecurityContext: tenantSecCtx(),
		}}
		return controllerutil.SetControllerReference(&kv, sts, r.Scheme)
	}); err != nil {
		return r.kvFail(ctx, &kv, "StatefulSetFailed", err)
	}

	// The backup/purge Jobs need egress the platform-wide Cilium node/metadata
	// deny withholds by default — install their allow before the CronJob fires.
	if err := r.reconcileKeyValueBackupNetworkPolicy(ctx, &kv); err != nil {
		return r.kvFail(ctx, &kv, "BackupNetworkPolicyFailed", err)
	}

	if err := r.reconcileKeyValueBackup(ctx, &kv, plan, authSecretName); err != nil {
		return r.kvFail(ctx, &kv, "BackupCronJobFailed", err)
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
	kv.Status.CredentialSecretName = authSecretName
	kv.Status.ObservedGeneration = kv.Generation

	storageState, err := r.reconcileKeyValueStorage(ctx, &kv, sts, storageGB)
	if err != nil {
		return r.kvFail(ctx, &kv, "PVCResizeFailed", err)
	}
	if result, done, err := r.applyKeyValueStorageState(ctx, &kv, storageState); done || err != nil {
		return result, err
	}

	return r.updateKeyValueReadiness(ctx, &kv, sts, replicas, appv1alpha1.KeyValueCredentialRevision(auth.Data["password"]))
}

type keyValueStorageState struct {
	ready   bool
	failed  bool
	reason  string
	message string
	requeue time.Duration
}

func (r *KeyValueReconciler) applyKeyValueStorageState(
	ctx context.Context,
	kv *appv1alpha1.KeyValue,
	state keyValueStorageState,
) (ctrl.Result, bool, error) {
	if state.ready || kv.Spec.Suspended {
		return ctrl.Result{}, false, nil
	}
	kv.Status.Phase = appv1alpha1.KVPhaseProvisioning
	if state.failed {
		kv.Status.Phase = appv1alpha1.KVPhaseFailed
	}
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: state.reason,
		Message: state.message, ObservedGeneration: kv.Generation,
	})
	if err := r.Status().Update(ctx, kv); err != nil {
		return ctrl.Result{}, true, err
	}
	return ctrl.Result{RequeueAfter: state.requeue}, true, nil
}

// updateKeyValueReadiness keeps storage convergence and workload rollout as
// separate gates. A suspended store desires zero replicas and is healthy by
// that contract; a running store must converge on the current StatefulSet
// revision and current-revision Pod readiness.
func (r *KeyValueReconciler) updateKeyValueReadiness(
	ctx context.Context,
	kv *appv1alpha1.KeyValue,
	sts *appsv1.StatefulSet,
	replicas int32,
	credentialRevision string,
) (ctrl.Result, error) {
	_ = r.Get(ctx, client.ObjectKeyFromObject(sts), sts)
	rolloutReady := statefulSetRolloutReady(sts, replicas)
	podsReady := r.keyValuePodsReady(ctx, kv, sts, replicas)
	if kv.Spec.Suspended || (rolloutReady && podsReady) {
		kv.Status.CredentialRevision = credentialRevision
		reason, message := "Provisioned", "valkey ready"
		if kv.Spec.Suspended {
			reason, message = "Suspended", "valkey suspended (scaled to zero)"
		}
		kv.Status.Phase = appv1alpha1.KVPhaseReady
		meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: reason,
			Message: message, ObservedGeneration: kv.Generation,
		})
		if err := r.Status().Update(ctx, kv); err != nil {
			return ctrl.Result{}, err
		}
		logf.FromContext(ctx).Info("keyvalue reconciled", "name", kv.Name, "host", kv.Status.Host, "suspended", kv.Spec.Suspended)
		return ctrl.Result{RequeueAfter: childHealthRequeue}, nil
	}

	kv.Status.Phase = appv1alpha1.KVPhaseProvisioning
	reason, message := "Provisioning", "waiting for the current Valkey StatefulSet revision"
	if rolloutReady && !podsReady {
		reason, message = "PodUnready", "a current Valkey pod is not Ready"
	}
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason,
		Message: message, ObservedGeneration: kv.Generation,
	})
	if err := r.Status().Update(ctx, kv); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: kvStorageRequeue}, nil
}

func keyValuePVCName(name string) string {
	return "data-" + name + "-0"
}

func statefulSetStorageGB(sts *appsv1.StatefulSet) int32 {
	if sts == nil || len(sts.Spec.VolumeClaimTemplates) == 0 {
		return 0
	}
	return quantityGiCeil(sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage])
}

func pvcRequestedStorageGB(pvc *corev1.PersistentVolumeClaim) int32 {
	if pvc == nil {
		return 0
	}
	return quantityGiCeil(pvc.Spec.Resources.Requests[corev1.ResourceStorage])
}

func pvcCapacityStorageGB(pvc *corev1.PersistentVolumeClaim) int32 {
	if pvc == nil {
		return 0
	}
	return quantityGiCeil(pvc.Status.Capacity[corev1.ResourceStorage])
}

func statefulSetRolloutReady(sts *appsv1.StatefulSet, replicas int32) bool {
	return sts.Status.ObservedGeneration >= sts.Generation &&
		sts.Status.CurrentRevision != "" &&
		sts.Status.CurrentRevision == sts.Status.UpdateRevision &&
		sts.Status.CurrentReplicas == replicas &&
		sts.Status.UpdatedReplicas == replicas &&
		sts.Status.ReadyReplicas >= replicas &&
		sts.Status.AvailableReplicas >= replicas
}

func (r *KeyValueReconciler) keyValuePodsReady(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, replicas int32) bool {
	if replicas == 0 {
		return true
	}
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(kv.Namespace), client.MatchingLabels{labelKeyValue: kv.Name}); err != nil {
		return true
	}
	var current, ready int32
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !pod.DeletionTimestamp.IsZero() ||
			(sts.Status.UpdateRevision != "" && pod.Labels[appsv1.StatefulSetRevisionLabel] != sts.Status.UpdateRevision) {
			continue
		}
		current++
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				ready++
				break
			}
		}
	}
	return current == 0 || ready >= replicas
}

func (r *KeyValueReconciler) rejectKeyValueStorageShrink(ctx context.Context, kv *appv1alpha1.KeyValue, current, requested int32) error {
	kv.Status.Phase = appv1alpha1.KVPhaseFailed
	kv.Status.AllocatedStorageGB = current
	message := fmt.Sprintf("Valkey storage is grow-only: requested %d GB is below the allocated %d GB", requested, current)
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: "StorageReady", Status: metav1.ConditionFalse, Reason: "StorageShrinkRejected",
		Message: message, ObservedGeneration: kv.Generation,
	})
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "StorageShrinkRejected",
		Message: message, ObservedGeneration: kv.Generation,
	})
	return r.Status().Update(ctx, kv)
}

func (r *KeyValueReconciler) reconcileKeyValueStorage(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, desiredGB int32) (keyValueStorageState, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	key := client.ObjectKey{Namespace: kv.Namespace, Name: keyValuePVCName(kv.Name)}
	if err := r.Get(ctx, key, pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return keyValueStorageState{}, err
		}
		kv.Status.AllocatedStorageGB = max(desiredGB, statefulSetStorageGB(sts))
		kv.Status.ObservedStorageGB = kv.Spec.StorageGB
		kv.Status.StorageCapacityGB = 0
		state := keyValueStorageState{reason: "WaitingForPVC", message: "waiting for the Valkey PVC to be created", requeue: kvStorageRequeue}
		setKeyValueStorageCondition(kv, state)
		return state, nil
	}

	requestedGB := pvcRequestedStorageGB(pvc)
	capacityGB := pvcCapacityStorageGB(pvc)
	desiredGB = max(desiredGB, requestedGB, capacityGB)
	if desiredGB > requestedGB {
		if pvc.Status.Phase != corev1.ClaimBound {
			state := keyValueStorageState{reason: "WaitingForPVCBinding", message: "waiting for the Valkey PVC to bind before requesting expansion", requeue: kvStorageRequeue}
			setKeyValueStorageCondition(kv, state)
			return state, nil
		}
		if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName == "" {
			state := keyValueStorageState{failed: true, reason: "StorageClassMissing", message: "Valkey PVC has no StorageClass; online expansion is unavailable", requeue: kvStorageFailureRequeue}
			setKeyValueStorageCondition(kv, state)
			return state, nil
		}
		var storageClass storagev1.StorageClass
		if err := r.Get(ctx, client.ObjectKey{Name: *pvc.Spec.StorageClassName}, &storageClass); err != nil {
			if apierrors.IsNotFound(err) {
				state := keyValueStorageState{failed: true, reason: "StorageClassNotFound", message: fmt.Sprintf("StorageClass %q was not found; cannot expand Valkey PVC", *pvc.Spec.StorageClassName), requeue: kvStorageFailureRequeue}
				setKeyValueStorageCondition(kv, state)
				return state, nil
			}
			return keyValueStorageState{}, err
		}
		if storageClass.AllowVolumeExpansion == nil || !*storageClass.AllowVolumeExpansion {
			state := keyValueStorageState{failed: true, reason: "StorageClassNotExpandable", message: fmt.Sprintf("StorageClass %q does not allow volume expansion", storageClass.Name), requeue: kvStorageFailureRequeue}
			setKeyValueStorageCondition(kv, state)
			return state, nil
		}
		before := pvc.DeepCopy()
		if pvc.Spec.Resources.Requests == nil {
			pvc.Spec.Resources.Requests = corev1.ResourceList{}
		}
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(fmt.Sprintf("%dGi", desiredGB))
		if err := r.Patch(ctx, pvc, client.MergeFrom(before)); err != nil {
			// Unlike Postgres (whose PVC CNPG owns, so the operator pre-flights the
			// quota), the KeyValue reconciler patches the PVC itself — a namespace
			// requests.storage quota rejection therefore reaches us directly as a
			// Forbidden. Surface it and back off instead of hot-looping a bare error
			// (ADR045 Finding 4, w7/m59); the resize resumes once quota headroom
			// returns. This grow seam is user-initiated (a plan/storage change), not
			// an autoscale loop, so there is no periodic-retry storm to debounce.
			if isQuotaExceeded(err) {
				state := keyValueStorageState{
					failed:  true,
					reason:  "StorageBlockedByQuota",
					message: fmt.Sprintf("Valkey PVC expansion to %d GB is blocked by the namespace storage quota; growth resumes when quota headroom is available", desiredGB),
					requeue: kvStorageFailureRequeue,
				}
				setKeyValueStorageCondition(kv, state)
				return state, nil
			}
			return keyValueStorageState{}, err
		}
		requestedGB = desiredGB
	}

	kv.Status.AllocatedStorageGB = max(desiredGB, requestedGB)
	kv.Status.ObservedStorageGB = kv.Spec.StorageGB
	kv.Status.StorageCapacityGB = capacityGB
	if capacityGB < desiredGB {
		reason := "PVCResizePending"
		message := fmt.Sprintf("Valkey PVC expansion is pending: requested %d GB, observed capacity %d GB", desiredGB, capacityGB)
		for _, condition := range pvc.Status.Conditions {
			if condition.Type == corev1.PersistentVolumeClaimFileSystemResizePending && condition.Status == corev1.ConditionTrue {
				reason = "FileSystemResizePending"
				message = fmt.Sprintf("Valkey filesystem resize is pending: requested %d GB, observed capacity %d GB", desiredGB, capacityGB)
				break
			}
		}
		state := keyValueStorageState{reason: reason, message: message, requeue: kvStorageRequeue}
		setKeyValueStorageCondition(kv, state)
		return state, nil
	}
	state := keyValueStorageState{ready: true, reason: "StorageProvisioned", message: fmt.Sprintf("Valkey PVC capacity is %d GB", capacityGB)}
	setKeyValueStorageCondition(kv, state)
	return state, nil
}

// isQuotaExceeded reports whether err is an API-server ResourceQuota rejection —
// a Forbidden carrying "exceeded quota". Used by the KeyValue storage grow path
// (which patches its own PVC) to surface a quota block as a backed-off status
// instead of a hot-looping reconcile error (ADR045 Finding 4, w7/m59).
func isQuotaExceeded(err error) bool {
	return err != nil && apierrors.IsForbidden(err) && strings.Contains(err.Error(), "exceeded quota")
}

func setKeyValueStorageCondition(kv *appv1alpha1.KeyValue, state keyValueStorageState) {
	status := metav1.ConditionFalse
	if state.ready {
		status = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: "StorageReady", Status: status, Reason: state.reason,
		Message: state.message, ObservedGeneration: kv.Generation,
	})
}

func (r *KeyValueReconciler) kvFail(ctx context.Context, kv *appv1alpha1.KeyValue, reason string, err error) (ctrl.Result, error) {
	kv.Status.Phase = appv1alpha1.KVPhaseFailed
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: err.Error(),
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
		For(&appv1alpha1.KeyValue{}, builder.WithPredicates(generationDeletionOrFinalizerPredicate{})).
		Owns(&appsv1.StatefulSet{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Owns(&batchv1.CronJob{}, builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.PersistentVolumeClaim{}, handler.EnqueueRequestsFromMapFunc(keyValuePVCRequests),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.Pod{}, handler.EnqueueRequestsFromMapFunc(keyValuePodRequests),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Owns(&corev1.Service{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&networkingv1.NetworkPolicy{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// No Owns(&corev1.Secret{}) watch: it is cache-backed, and the manager's
		// Secret informer covers only BEX_APPS_NAMESPACE, so it could never fire
		// for a KeyValue in a tenant namespace (ADR043 D8). Keeping it would make
		// drift healing work in one namespace and silently not in the others,
		// which is worse than not having it. Nothing is lost that matters: the
		// auth Secret is immutable (the API server rejects data edits outright),
		// the connection Secret is derived and re-converged on every pass, and a
		// Ready KeyValue re-reconciles every childHealthRequeue — so drift heals
		// within ~30s uniformly instead of instantly in one namespace only.
		Named("keyvalue").
		Complete(r)
}

func keyValuePVCRequests(_ context.Context, object client.Object) []reconcile.Request {
	name := object.GetLabels()[labelKeyValue]
	if name == "" {
		pvcName := object.GetName()
		if strings.HasPrefix(pvcName, "data-") && strings.HasSuffix(pvcName, "-0") {
			name = strings.TrimSuffix(strings.TrimPrefix(pvcName, "data-"), "-0")
		}
	}
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: object.GetNamespace(), Name: name}}}
}

func keyValuePodRequests(_ context.Context, object client.Object) []reconcile.Request {
	name := object.GetLabels()[labelKeyValue]
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: object.GetNamespace(), Name: name}}}
}
