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
	"cmp"
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
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
	// kvDefaultUser is Valkey's built-in default ACL user; required by
	// valkey-cli URI authentication.
	kvDefaultUser = "default"
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
	// kvDefaultImage is the Valkey image used when spec.version is empty — the
	// path almost every KeyValue takes. Digest-pinned (round-14 #5): this image
	// is both the serving datastore and the backup pipeline's snapshot stage,
	// where it receives the instance password and mounts the plaintext backup
	// volume, so a retagged `8-alpine` must not silently become that code. The
	// tag is retained ahead of the digest for human legibility; the digest is
	// what resolves. Bumping it is a deliberate code change that rolls the
	// StatefulSets (see docs/ADR069-security-review-round14.md #5).
	kvDefaultImage = "valkey/valkey:8-alpine@sha256:a038175878d66b9d274fbf8be73c0305e93798b83917647f167e18cef3c71eec"
	// kvExporterPort / kvExporterImage back the redis_exporter metrics sidecar
	// (w5/011): it scrapes Valkey's INFO stats and exposes them as Prometheus
	// metrics on kvExporterPort, discovered by the valkey-instances scrape job.
	kvExporterPort = 9121
	// Digest-pinned like every other image bex chooses (w1/m73): the exporter is
	// a sidecar in the tenant's own pod and receives the instance password to
	// poll INFO, so a retagged `alpine` would become code running beside the
	// datastore with its credential. Bumping it is a deliberate change that
	// rolls the StatefulSets.
	kvExporterImage = "oliver006/redis_exporter:alpine@sha256:d4e0a0ad55fa4f968eea4519b518b16dac887debd06c790a6be2f60940538d82"
	// labelKeyValue marks the workload/Service a KeyValue creates, and is the
	// StatefulSet selector + Service selector so the two stay coupled.
	labelKeyValue    = "app.bex.co/keyvalue"
	kvStorageRequeue = 10 * time.Second
	// conditionStorageReady is the KeyValue status condition tracking PVC
	// provisioning and growth, alongside appv1alpha1.ConditionReady.
	conditionStorageReady   = "StorageReady"
	kvStorageFailureRequeue = 5 * time.Minute
)

var certManagerCertificateGVK = schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"}

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

// kvVersionImages pins every Valkey major a tenant may request. The set is
// closed and known at compile time — KeyValueSpec.Version carries
// +kubebuilder:validation:Enum="7";"8" — which is what retires round-14 #5's
// residual: the deferral rested on "bex cannot pre-resolve a digest for a major
// it has not seen", but the CRD guarantees it never sees one. Adding a major to
// that enum without adding its digest here fails
// TestValkeyImagesArePinnedForEveryPermittedVersion.
var kvVersionImages = map[string]string{
	"7":                                "valkey/valkey:7-alpine@sha256:211d9cb02395987d3740b11fdbb7be0cb66c5f36a065640ce5753c933700d6cc",
	appv1alpha1.DefaultKeyValueVersion: kvDefaultImage,
}

// valkeyImage resolves the Valkey image for a major version; empty => the
// operator default (kvDefaultImage). Every result is digest-pinned.
//
// An unrecognized version falls back to the pinned default rather than
// composing a mutable tag: the CRD enum should make that unreachable, and if it
// ever is reached, running a known image is safer than running whatever
// `valkey:<something>-alpine` resolves to today. The guard test is what keeps
// the fallback from becoming the quiet normal path.
func valkeyImage(version string) string {
	if image, ok := kvVersionImages[version]; ok {
		return image
	}
	return kvDefaultImage
}

// kvResources maps a Valkey tier to a Guaranteed-QoS allocation via the shared
// guaranteedResources helper (requests == limits).
func kvResources(plan tiers.ValkeyTier) corev1.ResourceRequirements {
	return guaranteedResources(plan.CPU, plan.Memory)
}

func valkeySecCtx() *corev1.SecurityContext {
	security := tenantSecCtx()
	security.RunAsNonRoot = ptr.To(true)
	security.RunAsUser = ptr.To(valkeyRunAsUser)
	security.RunAsGroup = ptr.To(valkeyRunAsGroup)
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
		"username": []byte(kvDefaultUser),
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
	// BackupHelperImage is the image the backup CronJob's encrypt stage runs
	// (its /backup-encrypt entrypoint). It is the OPERATOR'S OWN image — see
	// selfimage.Resolve — so the stage that handles the plaintext RDB runs
	// first-party code from an artifact bex builds, signs and digest-pins,
	// rather than fetching age at run time (w7/m85, ADR068 #9). Required only
	// when Backup.AgePublicKey is set; that combination fails closed.
	BackupHelperImage string
}

// secretClient prefers the uncached reader for tenant-namespace Secret access,
// falling back to the cached client (see DatabaseReconciler.secretClient).
func (r *KeyValueReconciler) secretClient() client.Client {
	return cmp.Or(r.SecretClient, r.Client)
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
	currentGB, effectiveGB, shrink := growOnlyIntent(
		kv.Status.AllocatedStorageGB, kv.Status.ObservedStorageGB, kv.Spec.StorageGB, desiredGB,
		statefulSetStorageGB(sts), pvcRequestedStorageGB(pvc), pvcCapacityStorageGB(pvc))
	if shrink {
		return nil, 0, ctrl.Result{}, true, r.rejectKeyValueStorageShrink(ctx, kv, currentGB, kv.Spec.StorageGB)
	}
	return sts, effectiveGB, ctrl.Result{}, false, nil
}

// reconcileKeyValueTLS manages the public front door's Certificate. The
// Certificate and the Secret it writes share one name — tlsSecretName is both.
func (r *KeyValueReconciler) reconcileKeyValueTLS(
	ctx context.Context,
	kv *appv1alpha1.KeyValue,
	public bool,
	tlsSecretName string,
) (string, error) {
	if public && r.ClusterIssuer == "" {
		kv.Status.ExternalHost = ""
		return "TLSIssuerMissing", fmt.Errorf("BEX_CLUSTER_ISSUER is required for a public Key Value endpoint")
	}
	if !public {
		return "CertificateCleanupFailed", deleteOwned(ctx, r.Client, kv, certManagerCertificateGVK, tlsSecretName)
	}
	host := fmt.Sprintf("%s.%s", kv.Name, r.KvDomain)
	spec := map[string]any{
		"secretName": tlsSecretName,
		"dnsNames":   []any{host},
		"issuerRef": map[string]any{
			"name": r.ClusterIssuer, "kind": "ClusterIssuer",
		},
	}
	return "CertificateFailed", upsertOwned(ctx, r.Client, r.Scheme, kv, certManagerCertificateGVK, tlsSecretName, spec)
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
		auth.Data["username"] = []byte(kvDefaultUser)
		auth.Immutable = ptr.To(true)
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
		connection.Immutable = ptr.To(true)
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

// keyValueIntent is the derived shape one reconcile pass authors: everything
// computed from the KeyValue spec + plan that the Service and the StatefulSet
// both need, resolved once instead of threaded as loose locals. Mirrors
// clusterParams/databaseBackupIntent in database_controller.go.
type keyValueIntent struct {
	plan          tiers.ValkeyTier
	storageGB     int32
	internalHost  string
	public        bool
	tlsSecretName string
	// replicas is 0 while suspended — the PVC, Secret, Service and route are
	// kept, so resume restores the same data, password and endpoint (Render's
	// KV suspend).
	replicas  int32
	labels    map[string]string
	podLabels map[string]string
	// authSecretName and credentialRevision are filled in after the credential
	// Secrets reconcile, which is the only step that must precede the workload.
	authSecretName     string
	credentialRevision string
}

func keyValueIntentFor(kv *appv1alpha1.KeyValue, plan tiers.ValkeyTier, storageGB int32, kvDomain string) keyValueIntent {
	replicas := plan.Instances
	if kv.Spec.Suspended {
		replicas = 0
	}
	// Pod-template labels are the selector labels plus the workspace label, so
	// same-workspace NetworkPolicy selectors can reach the Valkey instance.
	podLabels := map[string]string{labelKeyValue: kv.Name}
	if ws := kv.Labels[labelWorkspace]; ws != "" {
		podLabels[labelWorkspace] = ws
	}
	if env := kv.Labels[labelEnvironment]; env != "" {
		podLabels[labelEnvironment] = env
	}
	return keyValueIntent{
		plan:          plan,
		storageGB:     storageGB,
		internalHost:  fmt.Sprintf("%s.%s.svc", kv.Name, kv.Namespace),
		public:        kv.Spec.Public && kvDomain != "",
		tlsSecretName: kv.Name + "-kv-tls",
		replicas:      replicas,
		labels:        map[string]string{labelKeyValue: kv.Name},
		podLabels:     podLabels,
	}
}

// reconcileKeyValueService authors the headless Service, which gives the
// internal DNS "<name>.<ns>.svc" and is the StatefulSet's serviceName (stable
// pod identity).
func (r *KeyValueReconciler) reconcileKeyValueService(ctx context.Context, kv *appv1alpha1.KeyValue, intent keyValueIntent) error {
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: kv.Name, Namespace: kv.Namespace}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		svc.Spec.Selector = intent.labels
		svc.Spec.Ports = []corev1.ServicePort{{Port: kvPort, TargetPort: intstr.FromInt(kvPort), Name: "valkey"}}
		if intent.public {
			svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{Port: kvTLSPort, TargetPort: intstr.FromInt(kvTLSPort), Name: "valkey-tls"})
		}
		applyServicePortServerDefaults(svc.Spec.Ports)
		return controllerutil.SetControllerReference(kv, svc, r.Scheme)
	})
	return err
}

// reconcileKeyValueWorkload authors the single-instance Valkey StatefulSet,
// sized to the tier.
func (r *KeyValueReconciler) reconcileKeyValueWorkload(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, intent keyValueIntent) error {
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, sts, func() error {
		applyKeyValueStatefulSet(sts, kv, intent)
		return controllerutil.SetControllerReference(kv, sts, r.Scheme)
	})
	return err
}

// applyKeyValueStatefulSet writes the desired StatefulSet shape. It is a pure
// function of (sts, kv, intent) so the workload shape is testable without a
// client.
func applyKeyValueStatefulSet(sts *appsv1.StatefulSet, kv *appv1alpha1.KeyValue, intent keyValueIntent) {
	// selector / serviceName / volumeClaimTemplates are immutable on a
	// StatefulSet — set them once, at create. The pod template (resources,
	// image, env) is reapplied every reconcile so a plan/version bump rolls.
	if sts.CreationTimestamp.IsZero() {
		sts.Spec.Selector = &metav1.LabelSelector{MatchLabels: intent.labels}
		sts.Spec.ServiceName = kv.Name
		sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: "data", Labels: intent.labels},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse(fmt.Sprintf("%dGi", intent.storageGB)),
					},
				},
				StorageClassName: ptr.To(kvStorageClass),
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
	sts.Spec.Replicas = &intent.replicas
	sts.Spec.Template.Labels = intent.podLabels
	// A managed key-value store is a single-replica stateful service: an
	// eviction is downtime, so the cluster-autoscaler must never bin-pack
	// its node away. Manual/CAPI drains (node upgrades) still evict it —
	// the StatefulSet reschedules it elsewhere.
	sts.Spec.Template.Annotations = map[string]string{
		"cluster-autoscaler.kubernetes.io/safe-to-evict": "false",
		"app.bex.co/credential-revision":                 intent.credentialRevision,
	}
	applyValkeyPodSpec(&sts.Spec.Template.Spec, kv, intent)
}

// applyValkeyPodSpec writes the Valkey server + metrics-exporter template onto
// spec. It assigns only the fields this controller owns — never the whole
// PodSpec — so anything it does not own (nodeSelector, tolerations,
// serviceAccountName, …) survives an update instead of being blanked and
// re-defaulted into a spurious rollout every reconcile.
func applyValkeyPodSpec(spec *corev1.PodSpec, kv *appv1alpha1.KeyValue, intent keyValueIntent) {
	// The Valkey password, shared by the server (arg expansion) and the metrics
	// exporter (authenticated INFO scrape).
	passwordEnv := corev1.EnvVar{
		Name: "VALKEY_PASSWORD",
		ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: intent.authSecretName},
			Key:                  "password",
		}},
	}
	serverArgs := valkeyArgs(kv.Spec, intent.plan)
	serverPorts := []corev1.ContainerPort{{ContainerPort: kvPort, Name: "valkey"}}
	var serverMounts []corev1.VolumeMount
	var volumes []corev1.Volume
	if intent.public {
		serverArgs = append(serverArgs,
			"--tls-port", strconv.Itoa(kvTLSPort),
			"--tls-cert-file", "/tls/tls.crt",
			"--tls-key-file", "/tls/tls.key",
			"--tls-ca-cert-file", "/tls/tls.crt",
			"--tls-auth-clients", "no",
		)
		serverPorts = append(serverPorts, corev1.ContainerPort{ContainerPort: kvTLSPort, Name: "valkey-tls"})
		serverMounts = append(serverMounts, corev1.VolumeMount{Name: "public-tls", MountPath: "/tls", ReadOnly: true})
		volumes = []corev1.Volume{{Name: "public-tls", VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: intent.tlsSecretName},
		}}}
	}
	spec.Volumes = volumes
	// Harden the managed Valkey pod the same way tenant Deployments are (w1/m53):
	// drop ALL caps, no privilege escalation, RuntimeDefault seccomp, and no
	// ServiceAccount token mounted (Valkey never talks to the apiserver).
	spec.AutomountServiceAccountToken = ptr.To(false)
	spec.SecurityContext = &corev1.PodSecurityContext{
		FSGroup:             ptr.To(valkeyRunAsGroup),
		FSGroupChangePolicy: ptr.To(corev1.FSGroupChangeOnRootMismatch),
	}
	spec.Containers = []corev1.Container{{
		Name:  "valkey",
		Image: valkeyImage(kv.Spec.Version),
		// VALKEY_PASSWORD (env, below) expands in args — k8s substitutes
		// $(VAR) from the container env list. appendonly persists to the PVC.
		Args:            serverArgs,
		Ports:           serverPorts,
		Env:             []corev1.EnvVar{passwordEnv},
		Resources:       kvResources(intent.plan),
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
	// Last, always — both containers are rebuilt whole above. Fill-if-empty, so
	// the pod-level fields this function leaves to the fetched object stay put.
	// See server_defaults.go.
	applyPodSpecServerDefaults(spec)
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
	if backupEnabled {
		if res, done, err := stampFinalizer(ctx, r.Client, &kv, kvFinalizer); done {
			return res, err
		}
	}
	sts, storageGB, result, done, err := r.keyValueStorageIntent(ctx, &kv, requestedStorageGB)
	if done || err != nil {
		return result, err
	}
	intent := keyValueIntentFor(&kv, plan, storageGB, r.KvDomain)
	// Validate public TLS before updating the workload. A missing issuer must
	// fail closed without publishing a public host.
	if reason, err := r.reconcileKeyValueTLS(ctx, &kv, intent.public, intent.tlsSecretName); err != nil {
		return r.kvFail(ctx, &kv, reason, err)
	}

	// --- credentials Secrets (created first so the StatefulSet's env ref resolves) ---
	auth, result, done, err := r.reconcileKeyValueCredentials(ctx, &kv, intent.internalHost)
	if done || err != nil {
		return result, err
	}
	intent.authSecretName = auth.Name
	intent.credentialRevision = appv1alpha1.KeyValueCredentialRevision(auth.Data["password"])
	authSecretName := intent.authSecretName

	if err := r.reconcileKeyValueService(ctx, &kv, intent); err != nil {
		return r.kvFail(ctx, &kv, "ServiceFailed", err)
	}
	if err := r.reconcileKeyValueWorkload(ctx, &kv, sts, intent); err != nil {
		return r.kvFail(ctx, &kv, "StatefulSetFailed", err)
	}
	if err := reconcileEnvironmentPeerPolicy(ctx, r.Client, r.Scheme, &kv, kv.Labels[labelEnvironment], "-environment-ingress", map[string]string{labelKeyValue: kv.Name}); err != nil {
		return r.kvFail(ctx, &kv, "NetworkPolicyFailed", err)
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
	if intent.public {
		kv.Status.ExternalHost = fmt.Sprintf("%s.%s", kv.Name, r.KvDomain)
	} else {
		kv.Status.ExternalHost = ""
	}

	// Surface the connection coordinates (the password stays in the Secret).
	kv.Status.Host = intent.internalHost
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

	return r.updateKeyValueReadiness(ctx, &kv, sts, intent.replicas, intent.credentialRevision)
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
			reason, message = reasonSuspended, "valkey suspended (scaled to zero)"
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

// keyValuePodsReady is currentRevisionPodsReady over the KeyValue label and
// the StatefulSet controller's own revision label (see that helper for the
// inconclusive-true semantics it shares with deploymentPodsReady).
func (r *KeyValueReconciler) keyValuePodsReady(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, replicas int32) bool {
	if replicas == 0 {
		return true
	}
	return currentRevisionPodsReady(ctx, r.Client, kv.Namespace, map[string]string{labelKeyValue: kv.Name},
		appsv1.StatefulSetRevisionLabel, sts.Status.UpdateRevision, replicas)
}

func (r *KeyValueReconciler) rejectKeyValueStorageShrink(ctx context.Context, kv *appv1alpha1.KeyValue, current, requested int32) error {
	kv.Status.Phase = appv1alpha1.KVPhaseFailed
	kv.Status.AllocatedStorageGB = current
	message := fmt.Sprintf("Valkey storage is grow-only: requested %d GB is below the allocated %d GB", requested, current)
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: conditionStorageReady, Status: metav1.ConditionFalse, Reason: "StorageShrinkRejected",
		Message: message, ObservedGeneration: kv.Generation,
	})
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: "StorageShrinkRejected",
		Message: message, ObservedGeneration: kv.Generation,
	})
	return r.Status().Update(ctx, kv)
}

// reconcileKeyValueStorage converges the Valkey PVC toward desiredGB in three
// phases: await the PVC's creation, grow it if the request is behind, then
// report the observed capacity.
func (r *KeyValueReconciler) reconcileKeyValueStorage(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, desiredGB int32) (keyValueStorageState, error) {
	pvc := &corev1.PersistentVolumeClaim{}
	key := client.ObjectKey{Namespace: kv.Namespace, Name: keyValuePVCName(kv.Name)}
	if err := r.Get(ctx, key, pvc); err != nil {
		if !apierrors.IsNotFound(err) {
			return keyValueStorageState{}, err
		}
		return awaitKeyValuePVC(kv, sts, desiredGB), nil
	}

	requestedGB := pvcRequestedStorageGB(pvc)
	capacityGB := pvcCapacityStorageGB(pvc)
	desiredGB = max(desiredGB, requestedGB, capacityGB)
	if desiredGB > requestedGB {
		blocked, err := r.expandKeyValuePVC(ctx, kv, pvc, desiredGB)
		if err != nil {
			return keyValueStorageState{}, err
		}
		if blocked != nil {
			return *blocked, nil
		}
		requestedGB = desiredGB
	}
	return finalizeKeyValueStorage(kv, pvc, desiredGB, requestedGB, capacityGB), nil
}

// awaitKeyValuePVC records the pre-creation state: the StatefulSet's own
// template size is the best allocation estimate until the PVC exists.
func awaitKeyValuePVC(kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, desiredGB int32) keyValueStorageState {
	kv.Status.AllocatedStorageGB = max(desiredGB, statefulSetStorageGB(sts))
	kv.Status.ObservedStorageGB = kv.Spec.StorageGB
	kv.Status.StorageCapacityGB = 0
	state := keyValueStorageState{reason: "WaitingForPVC", message: "waiting for the Valkey PVC to be created", requeue: kvStorageRequeue}
	setKeyValueStorageCondition(kv, state)
	return state
}

// expandKeyValuePVC raises the PVC's storage request to desiredGB. A non-nil
// returned state means an eligibility gate blocked the grow (already recorded on
// the CR) and the caller should return it; nil means the patch landed.
func (r *KeyValueReconciler) expandKeyValuePVC(ctx context.Context, kv *appv1alpha1.KeyValue, pvc *corev1.PersistentVolumeClaim, desiredGB int32) (*keyValueStorageState, error) {
	block := func(state keyValueStorageState) (*keyValueStorageState, error) {
		setKeyValueStorageCondition(kv, state)
		return &state, nil
	}
	className := ""
	if pvc.Spec.StorageClassName != nil {
		className = *pvc.Spec.StorageClassName
	}
	outcome, err := expandPVCTo(ctx, r.Client, pvc, desiredGB)
	if err != nil {
		return nil, err
	}
	switch outcome {
	case pvcExpanded:
		return nil, nil
	case pvcExpandWaitingForBinding:
		return block(keyValueStorageState{reason: "WaitingForPVCBinding", message: "waiting for the Valkey PVC to bind before requesting expansion", requeue: kvStorageRequeue})
	case pvcExpandStorageClassMissing:
		return block(keyValueStorageState{failed: true, reason: "StorageClassMissing", message: "Valkey PVC has no StorageClass; online expansion is unavailable", requeue: kvStorageFailureRequeue})
	case pvcExpandStorageClassNotFound:
		return block(keyValueStorageState{failed: true, reason: "StorageClassNotFound", message: fmt.Sprintf("StorageClass %q was not found; cannot expand Valkey PVC", className), requeue: kvStorageFailureRequeue})
	case pvcExpandNotExpandable:
		return block(keyValueStorageState{failed: true, reason: "StorageClassNotExpandable", message: fmt.Sprintf("StorageClass %q does not allow volume expansion", className), requeue: kvStorageFailureRequeue})
	case pvcExpandQuotaBlocked:
		return block(keyValueStorageState{
			failed:  true,
			reason:  "StorageBlockedByQuota",
			message: fmt.Sprintf("Valkey PVC expansion to %d GB is blocked by the namespace storage quota; growth resumes when quota headroom is available", desiredGB),
			requeue: kvStorageFailureRequeue,
		})
	}
	return nil, nil
}

// finalizeKeyValueStorage records the observed allocation and reports whether
// the volume (and its filesystem) has caught up with the request.
func finalizeKeyValueStorage(kv *appv1alpha1.KeyValue, pvc *corev1.PersistentVolumeClaim, desiredGB, requestedGB, capacityGB int32) keyValueStorageState {
	kv.Status.AllocatedStorageGB = max(desiredGB, requestedGB)
	kv.Status.ObservedStorageGB = kv.Spec.StorageGB
	kv.Status.StorageCapacityGB = capacityGB
	state := keyValueStorageState{ready: true, reason: "StorageProvisioned", message: fmt.Sprintf("Valkey PVC capacity is %d GB", capacityGB)}
	if capacityGB < desiredGB {
		reason, subject := "PVCResizePending", "Valkey PVC expansion"
		if pvcFileSystemResizePending(pvc) {
			reason, subject = "FileSystemResizePending", "Valkey filesystem resize"
		}
		state = keyValueStorageState{
			reason:  reason,
			message: fmt.Sprintf("%s is pending: requested %d GB, observed capacity %d GB", subject, desiredGB, capacityGB),
			requeue: kvStorageRequeue,
		}
	}
	setKeyValueStorageCondition(kv, state)
	return state
}

// pvcFileSystemResizePending reports whether the volume itself has grown and
// only the filesystem expansion is outstanding.
func pvcFileSystemResizePending(pvc *corev1.PersistentVolumeClaim) bool {
	for _, condition := range pvc.Status.Conditions {
		if condition.Type == corev1.PersistentVolumeClaimFileSystemResizePending && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
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
		Type: conditionStorageReady, Status: status, Reason: state.reason,
		Message: state.message, ObservedGeneration: kv.Generation,
	})
}

func (r *KeyValueReconciler) kvFail(ctx context.Context, kv *appv1alpha1.KeyValue, reason string, err error) (ctrl.Result, error) {
	kv.Status.Phase = appv1alpha1.KVPhaseFailed
	setNotReadyCondition(ctx, r.Client, kv, &kv.Status.Conditions, reason, err)
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
