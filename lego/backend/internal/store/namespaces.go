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

package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Per-tenant namespace isolation (ADR043, w3/m31 t001). A workspace maps to a
// hosting namespace `<ws>` and, when sandboxes are enabled (pillar 5, m32), a
// `<ws>-sandbox` namespace. The NamespaceReconciler is the lifecycle mechanism:
// it is level-triggered exactly like the App Reconciler — a full resync plus a
// Kick after workspace writes — so the set of tenant namespaces is a rebuildable
// projection of the `tenants` table, never imperative drift.
//
// This is the FOUNDATION only: t001 provisions each namespace with its identity
// labels, a base ResourceQuota/LimitRange, and a default-deny NetworkPolicy.
// Placing workloads into `<ws>` (t002), per-`<ws>` RBAC (t003), plan-tier quota
// values (t004), and the same-workspace/platform NetworkPolicy allows (t005)
// build on top of what this creates. Gated off by default (BEX_TENANT_NAMESPACES
// unset in cmd/api) so production is byte-identical until a cluster opts in.
const (
	// RegimeLabel distinguishes a workspace's hosting namespace from its sandbox
	// namespace (ADR043 D2 — both untrusted, opposite default network postures).
	// RegimeLabel also rides OpenSandbox create metadata, selecting the resulting
	// BatchSandbox and Pod for sandbox-only network policy.
	RegimeLabel   = "app.bex.co/regime"
	RegimeHosting = "hosting"
	RegimeSandbox = "sandbox"

	// sandboxSuffix names the sandbox regime namespace for a workspace.
	sandboxSuffix = "-sandbox"

	defaultNamespaceResync = 60 * time.Second
)

// WorkspaceNamespace is the hosting namespace name for a workspace id. The
// workspace id (tea-<xid>) is already a DNS-safe label ≤63 chars (ADR020), so
// it is used verbatim as the namespace name — the k8s object IS the tenant.
func WorkspaceNamespace(workspaceID string) string { return workspaceID }

// SandboxNamespace is the sandbox-regime namespace name for a workspace id.
func SandboxNamespace(workspaceID string) string { return workspaceID + sandboxSuffix }

// NamespaceReconciler ensures every workspace has its per-tenant namespace(s)
// with base isolation objects, and prunes namespaces for deleted workspaces.
type NamespaceReconciler struct {
	Client client.Client
	Store  Store
	Resync time.Duration
	// Sandboxes, when true, also provisions and prunes each workspace's
	// `<ws>-sandbox` namespace (pillar 5 / m32). Off keeps hosting-only.
	Sandboxes bool

	kick chan struct{}
}

// NewNamespaceReconciler builds a reconciler with the default resync period.
func NewNamespaceReconciler(cl client.Client, store Store) *NamespaceReconciler {
	return &NamespaceReconciler{
		Client: cl,
		Store:  store,
		Resync: defaultNamespaceResync,
		kick:   make(chan struct{}, 1),
	}
}

// Kick schedules an immediate reconcile (non-blocking, coalescing) — called
// after a workspace create/delete so provisioning is not a full resync away.
func (r *NamespaceReconciler) Kick() {
	select {
	case r.kick <- struct{}{}:
	default:
	}
}

// Run reconciles until ctx is done.
func (r *NamespaceReconciler) Run(ctx context.Context) {
	ticker := time.NewTicker(r.Resync)
	defer ticker.Stop()
	for {
		if err := r.ReconcileOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("controlplane: namespace reconcile: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-r.kick:
		}
	}
}

// ReconcileOnce ensures a namespace (and its base isolation objects) for every
// workspace, then deletes managed namespaces whose workspace no longer exists.
// Per-workspace failures are collected, not fatal — one bad workspace can't
// block the rest, mirroring the App Reconciler's error discipline.
func (r *NamespaceReconciler) ReconcileOnce(ctx context.Context) error {
	tenants, err := r.Store.ListTenants(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}
	desired := make(map[string]bool, len(tenants)*2)
	var errs []error
	for _, t := range tenants {
		desired[WorkspaceNamespace(t.ID)] = true
		if err := r.ensureNamespace(ctx, t, RegimeHosting); err != nil {
			errs = append(errs, fmt.Errorf("workspace %s hosting namespace: %w", t.ID, err))
		}
		if r.Sandboxes {
			desired[SandboxNamespace(t.ID)] = true
			if err := r.ensureNamespace(ctx, t, RegimeSandbox); err != nil {
				errs = append(errs, fmt.Errorf("workspace %s sandbox namespace: %w", t.ID, err))
			}
		}
	}
	if err := r.pruneOrphans(ctx, desired); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// namespaceName returns the namespace name for a workspace under a regime.
func namespaceName(workspaceID, regime string) string {
	if regime == RegimeSandbox {
		return SandboxNamespace(workspaceID)
	}
	return WorkspaceNamespace(workspaceID)
}

// ensureNamespace creates or converges one workspace namespace and its base
// isolation objects (ResourceQuota, LimitRange, default-deny NetworkPolicy).
func (r *NamespaceReconciler) ensureNamespace(ctx context.Context, t Tenant, regime string) error {
	name := namespaceName(t.ID, regime)
	if err := r.applyObject(ctx, workspaceNamespaceObject(name, t, regime)); err != nil {
		return fmt.Errorf("namespace: %w", err)
	}
	if err := r.applyObject(ctx, baseResourceQuota(name, t)); err != nil {
		return fmt.Errorf("resourcequota: %w", err)
	}
	if err := r.applyObject(ctx, baseLimitRange(name)); err != nil {
		return fmt.Errorf("limitrange: %w", err)
	}
	if err := r.applyObject(ctx, defaultDenyNetworkPolicy(name)); err != nil {
		return fmt.Errorf("networkpolicy: %w", err)
	}
	allows := allowNetworkPolicies(name, regime)
	desiredPolicies := map[string]bool{defaultDenyNetworkPolicyName: true}
	for _, np := range allows {
		desiredPolicies[np.Name] = true
	}
	// Reconciliation must remove policies that belonged to an older regime
	// shape. In particular, merely ceasing to generate allow-same-namespace and
	// allow-dns-egress would leave those broad allows active forever in existing
	// sandbox namespaces. Prune before adding allows so a partial reconcile fails
	// closed behind default-deny.
	if err := r.pruneManagedNetworkPolicies(ctx, name, desiredPolicies); err != nil {
		return fmt.Errorf("networkpolicy prune: %w", err)
	}
	for _, np := range allows {
		if err := r.applyObject(ctx, np); err != nil {
			return fmt.Errorf("networkpolicy %s: %w", np.Name, err)
		}
	}
	desiredBindings := tenantRoleBindings(name, regime)
	if err := r.pruneManagedRoleBindings(ctx, name, desiredBindings); err != nil {
		return fmt.Errorf("rolebinding prune: %w", err)
	}
	for _, rb := range desiredBindings {
		if err := r.applyObject(ctx, rb); err != nil {
			return fmt.Errorf("rolebinding %s: %w", rb.Name, err)
		}
	}
	return nil
}

// pruneManagedRoleBindings removes obsolete control-plane projections without
// touching tenant-authored RoleBindings. Removing a binding from the desired
// regime must revoke it from already-provisioned namespaces on the next pass.
func (r *NamespaceReconciler) pruneManagedRoleBindings(ctx context.Context, namespace string, desired []*rbacv1.RoleBinding) error {
	wanted := make(map[string]bool, len(desired))
	for _, binding := range desired {
		wanted[binding.Name] = true
	}

	var list rbacv1.RoleBindingList
	if err := r.Client.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		return err
	}
	var errs []error
	for i := range list.Items {
		binding := &list.Items[i]
		if !wanted[binding.Name] {
			errs = append(errs, client.IgnoreNotFound(r.Client.Delete(ctx, binding)))
		}
	}
	return errors.Join(errs...)
}

// pruneManagedNetworkPolicies deletes obsolete projections from one tenant
// namespace. The managed-by selector is the ownership boundary: a tenant's
// hand-written policy is never touched even when its name is not desired.
func (r *NamespaceReconciler) pruneManagedNetworkPolicies(ctx context.Context, namespace string, desired map[string]bool) error {
	var list networkingv1.NetworkPolicyList
	if err := r.Client.List(ctx, &list,
		client.InNamespace(namespace),
		client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		return err
	}
	var errs []error
	for i := range list.Items {
		policy := &list.Items[i]
		if desired[policy.Name] {
			continue
		}
		if err := r.Client.Delete(ctx, policy); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete obsolete policy %s: %w", policy.Name, err))
		}
	}
	return errors.Join(errs...)
}

// applyObject creates obj, or — if it already exists — updates it to converge
// on the desired managed shape. Only bex-managed objects are touched: a Create
// conflict on a pre-existing object we don't manage would surface as an update
// error, which the caller collects rather than silently overwriting.
func (r *NamespaceReconciler) applyObject(ctx context.Context, obj client.Object) error {
	err := r.Client.Create(ctx, obj)
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return err
	}
	// Converge managed labels/spec on an existing object. Fetch-merge-update via
	// a fresh copy so ResourceVersion is current.
	existing := obj.DeepCopyObject().(client.Object)
	if getErr := r.Client.Get(ctx, client.ObjectKeyFromObject(obj), existing); getErr != nil {
		return getErr
	}
	if !isManaged(existing) {
		return fmt.Errorf("object %s exists and is not bex-managed", client.ObjectKeyFromObject(obj))
	}
	obj.SetResourceVersion(existing.GetResourceVersion())
	return r.Client.Update(ctx, obj)
}

// pruneOrphans deletes managed tenant namespaces whose workspace is gone.
// Only namespaces carrying the managed-by label and a workspace label are
// considered — platform namespaces (bex-system, bex-build, …) never match.
func (r *NamespaceReconciler) pruneOrphans(ctx context.Context, desired map[string]bool) error {
	var list corev1.NamespaceList
	if err := r.Client.List(ctx, &list,
		client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		return fmt.Errorf("list managed namespaces: %w", err)
	}
	var errs []error
	for i := range list.Items {
		ns := &list.Items[i]
		if ns.Labels[LabelWorkspace] == "" || desired[ns.Name] {
			continue
		}
		if ns.DeletionTimestamp != nil {
			continue
		}
		if err := r.Client.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete orphan namespace %s: %w", ns.Name, err))
		}
	}
	return errors.Join(errs...)
}

// isManaged reports whether an object carries bex's managed-by label — the
// guard that keeps the reconciler from ever clobbering a hand-created object.
func isManaged(obj client.Object) bool {
	return obj.GetLabels()[LabelManagedBy] == ManagedByValue
}

// managedLabels are stamped on every object the namespace reconciler owns, so
// pruneOrphans and applyObject can recognize them and List can select them.
func managedLabels(workspaceID string) map[string]string {
	return map[string]string{
		LabelManagedBy:              ManagedByValue,
		core.LabelWorkspace:         workspaceID,
		"app.kubernetes.io/part-of": "bex",
	}
}

// workspaceNamespaceObject builds the Namespace for a workspace regime, with the
// identity labels plus the Pod Security Admission baseline the shared tenant
// namespace already enforces (deploy/gitops/base/tenant-namespace.yaml).
func workspaceNamespaceObject(name string, t Tenant, regime string) *corev1.Namespace {
	labels := managedLabels(t.ID)
	labels[RegimeLabel] = regime
	// PSS baseline enforced; restricted warned/audited (matches the shared
	// tenant namespace — tenant images may legitimately run as root).
	labels["pod-security.kubernetes.io/enforce"] = "baseline"
	labels["pod-security.kubernetes.io/enforce-version"] = "latest"
	labels["pod-security.kubernetes.io/warn"] = "restricted"
	labels["pod-security.kubernetes.io/warn-version"] = "latest"
	labels["pod-security.kubernetes.io/audit"] = "restricted"
	labels["pod-security.kubernetes.io/audit-version"] = "latest"
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
	}
}

// baseResourceQuota is the per-namespace aggregate ceiling. t001 ships a
// generous plan-scaled base (the per-namespace analog of the shared
// tenant-apps-quota); t004 replaces the numbers with the exact plan-tier
// catalog values and adds the count/<resource> caps that retire BEX_MAX_*.
func baseResourceQuota(namespace string, t Tenant) *corev1.ResourceQuota {
	caps := quotaForPlan(t.Plan)
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota",
			Namespace: namespace,
			Labels:    managedLabels(t.ID),
		},
		Spec: corev1.ResourceQuotaSpec{Hard: caps},
	}
}

// quotaForPlan maps a workspace plan to aggregate resource caps: compute/pod
// ceilings plus per-resource object counts. The count/<resource> caps push the
// per-workspace service/Postgres/Key Value limits (the app-code BEX_MAX_*
// counters) down to the API server, where a create past the cap is rejected by
// admission and cannot be bypassed by an application bug (ADR043 D3, t004). The
// free tier mirrors Render's Hobby anchors (25 services, 1 Postgres, 1 Key
// Value); paid plans get a generous ceiling. Enforced only for CRs that land in
// the namespace — i.e. once t002's per-tenant projection is enabled.
func quotaForPlan(plan string) corev1.ResourceList {
	// Paid default (mirrors the retired shared tenant-apps-quota, per tenant).
	cpuReq, memReq, cpuLim, memLim, pods, jobs := "50", "100Gi", "100", "200Gi", "500", "250"
	apps, dbs, kvs := "100", "25", "25"
	switch plan {
	case "", "free":
		cpuReq, memReq, cpuLim, memLim, pods, jobs = "2", "4Gi", "4", "8Gi", "50", "25"
		apps, dbs, kvs = "25", "1", "1" // Render Hobby anchors (root CLAUDE.md)
	}
	return corev1.ResourceList{
		corev1.ResourceRequestsCPU:    resource.MustParse(cpuReq),
		corev1.ResourceRequestsMemory: resource.MustParse(memReq),
		corev1.ResourceLimitsCPU:      resource.MustParse(cpuLim),
		corev1.ResourceLimitsMemory:   resource.MustParse(memLim),
		corev1.ResourcePods:           resource.MustParse(pods),
		"count/jobs.batch":            resource.MustParse(jobs),
		"count/apps.app.bex.co":       resource.MustParse(apps),
		"count/databases.app.bex.co":  resource.MustParse(dbs),
		"count/keyvalues.app.bex.co":  resource.MustParse(kvs),
	}
}

// baseLimitRange supplies request/limit defaults for pods that omit them
// (hand-applied or legacy App CRs), mirroring the shared tenant-apps-limits.
func baseLimitRange(namespace string) *corev1.LimitRange {
	return &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-limits",
			Namespace: namespace,
			Labels:    map[string]string{LabelManagedBy: ManagedByValue},
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{{
				Type: corev1.LimitTypeContainer,
				DefaultRequest: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Default: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("4Gi"),
				},
				Max: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("8"),
					corev1.ResourceMemory: resource.MustParse("32Gi"),
				},
			}},
		},
	}
}

// allowNetworkPolicies layers the opt-in allows on top of the default-deny
// (NetworkPolicies are additive), regime-aware per ADR043 D2 (t005). Because a
//   - hosting (`<ws>`): an addressable service — allow intra-namespace, DNS,
//     Traefik ingress (external traffic enters here), and internet egress minus
//     in-cluster private ranges and the link-local cloud-metadata range.
//   - sandbox (`<ws>-sandbox`): every Pod is a separate hostile principal. No
//     native same-namespace, raw-DNS, or label-only lifecycle allow is
//     installed. Cluster-wide Cilium policy owns both exact-ServiceAccount
//     execd ingress and approved DNS/FQDN/SNI egress outside gVisor.
func allowNetworkPolicies(namespace, regime string) []*networkingv1.NetworkPolicy {
	switch regime {
	case RegimeHosting:
		return []*networkingv1.NetworkPolicy{
			sameNamespaceNetworkPolicy(namespace),
			dnsEgressNetworkPolicy(namespace),
			traefikIngressNetworkPolicy(namespace),
			internetEgressNetworkPolicy(namespace),
		}
	case RegimeSandbox:
		return nil
	}
	return nil
}

func npMeta(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: namespace,
		Labels:    map[string]string{LabelManagedBy: ManagedByValue},
	}
}

// sameNamespaceNetworkPolicy allows all traffic between pods in the namespace
// (private services between a workspace's own apps).
func sameNamespaceNetworkPolicy(namespace string) *networkingv1.NetworkPolicy {
	inNamespace := []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}}
	return &networkingv1.NetworkPolicy{
		ObjectMeta: npMeta(namespace, "allow-same-namespace"),
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			Ingress:     []networkingv1.NetworkPolicyIngressRule{{From: inNamespace}},
			Egress:      []networkingv1.NetworkPolicyEgressRule{{To: inNamespace}},
		},
	}
}

// dnsEgressNetworkPolicy allows hosting workloads to query kube-system DNS.
// Sandbox DNS is instead constrained by the cluster-wide Cilium L7 policy.
func dnsEgressNetworkPolicy(namespace string) *networkingv1.NetworkPolicy {
	udp, tcp := corev1.ProtocolUDP, corev1.ProtocolTCP
	dnsPort := intstr.FromInt32(53)
	return &networkingv1.NetworkPolicy{
		ObjectMeta: npMeta(namespace, "allow-dns-egress"),
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				Ports: []networkingv1.NetworkPolicyPort{
					{Protocol: &udp, Port: &dnsPort},
					{Protocol: &tcp, Port: &dnsPort},
				},
				To: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "kube-system"},
					},
				}},
			}},
		},
	}
}

// traefikIngressNetworkPolicy allows external traffic to enter a hosting
// namespace's pods through the Traefik ingress controller.
func traefikIngressNetworkPolicy(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: npMeta(namespace, "allow-traefik-ingress"),
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{{
				From: []networkingv1.NetworkPolicyPeer{{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"kubernetes.io/metadata.name": "traefik"},
					},
				}},
			}},
		},
	}
}

// internetEgressNetworkPolicy allows a hosting namespace's pods to reach the
// public internet, minus in-cluster private ranges (RFC1918 / CGNAT) and the
// link-local cloud-metadata range — the same "internet yes, cluster no" shape
// the shared per-App policy used (ADR022 §Per-App NetworkPolicy shape).
func internetEgressNetworkPolicy(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: npMeta(namespace, "allow-internet-egress"),
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
			Egress: []networkingv1.NetworkPolicyEgressRule{{
				To: []networkingv1.NetworkPolicyPeer{{
					IPBlock: &networkingv1.IPBlock{
						CIDR: "0.0.0.0/0",
						Except: []string{
							"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
							"100.64.0.0/10", "169.254.0.0/16",
						},
					},
				}},
			}},
		},
	}
}

// platform service accounts the tenant-namespace RoleBindings grant access to.
// All live in bex-system (a RoleBinding may reference a SA in another namespace).
const platformNamespace = "bex-system"

// openSandboxNamespace hosts the OpenSandbox lifecycle server (m32 t002); its SA
// is bound per-`<ws>-sandbox` so the server can drive BatchSandboxes there.
const openSandboxNamespace = "opensandbox-system"

var (
	operatorSA   = rbacSubject("bex-controller-manager")
	apiSA        = rbacSubject("bex-api")
	sshGatewaySA = rbacSubject("bex-ssh-gateway")
	// sandboxServerSA is the OpenSandbox server ServiceAccount (opensandbox-system).
	sandboxServerSA = rbacv1.Subject{Kind: "ServiceAccount", Name: "opensandbox-server", Namespace: openSandboxNamespace}
	// sandboxControllerSA is the vendored OpenSandbox controller. Its cluster
	// role is informer-only; reconciliation writes are bound per sandbox ns.
	sandboxControllerSA = rbacv1.Subject{Kind: "ServiceAccount", Name: "opensandbox-controller-manager", Namespace: openSandboxNamespace}
)

func rbacSubject(name string) rbacv1.Subject {
	return rbacv1.Subject{Kind: "ServiceAccount", Name: name, Namespace: platformNamespace}
}

// tenantRoleBindings binds the per-tenant-namespace ClusterRoles
// (deploy/gitops/base/tenant-namespace-clusterroles.yaml) to the platform
// service accounts, scoped to this one namespace (ADR043 D5, t003). A
// RoleBinding→ClusterRole grants the ClusterRole's rules ONLY within the
// binding's namespace, so operator/bex-api Secret access exists only inside
// hosting tenant namespaces — never cluster-wide. Stamped by the reconciler at
// namespace-create so there is no manual per-workspace RoleBinding churn.
//
// A sandbox namespace (`<ws>-sandbox`) is driven by the OpenSandbox lifecycle
// server and controller, not the operator, bex-api's pod reads, or the SSH
// gateway. The hosting namespace binds operator + bex-api + ssh-gateway.
func tenantRoleBindings(namespace, regime string) []*rbacv1.RoleBinding {
	switch regime {
	case RegimeHosting:
		return []*rbacv1.RoleBinding{
			tenantRoleBinding(namespace, "bex-tenant-operator", operatorSA),
			tenantRoleBinding(namespace, "bex-tenant-api", apiSA),
			tenantRoleBinding(namespace, "bex-tenant-ssh-gateway", sshGatewaySA),
		}
	case RegimeSandbox:
		return []*rbacv1.RoleBinding{
			tenantRoleBinding(namespace, "bex-tenant-sandbox-server", sandboxServerSA),
			tenantRoleBinding(namespace, "bex-tenant-sandbox-controller", sandboxControllerSA),
			// `render ea sandbox exec` (w3/m33): the isolated SSH gateway runs the
			// command in the sandbox pod via pods/exec — the same ClusterRole it uses
			// for running-instance SSH, now bound in the sandbox regime too so
			// pods/exec stays confined to the gateway (never bex-api, Option A).
			tenantRoleBinding(namespace, "bex-tenant-ssh-gateway", sshGatewaySA),
		}
	default:
		return nil
	}
}

func tenantRoleBinding(namespace, clusterRole string, subject rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterRole, // one binding per role; name mirrors the role
			Namespace: namespace,
			Labels:    map[string]string{LabelManagedBy: ManagedByValue},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole,
		},
		Subjects: []rbacv1.Subject{subject},
	}
}

// defaultDenyNetworkPolicy denies all ingress and egress in the namespace — the
// structural default ADR043 D3 requires. allowNetworkPolicies layers the
// same-namespace, DNS, and (hosting-only) Traefik-ingress/internet-egress allows
// on top; the cluster-wide node/metadata Cilium egressDeny (retained from
// ADR022) still applies beneath all of these.
// defaultDenyNetworkPolicyName is the structural deny policy's name and the
// single source of truth shared by the constructor and the prune desired-set
// seed, so the two cannot drift and silently prune the namespace's default-deny.
const defaultDenyNetworkPolicyName = "default-deny"

func defaultDenyNetworkPolicy(namespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      defaultDenyNetworkPolicyName,
			Namespace: namespace,
			Labels:    map[string]string{LabelManagedBy: ManagedByValue},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{}, // all pods
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			// No Ingress/Egress rules => deny all in both directions.
		},
	}
}
