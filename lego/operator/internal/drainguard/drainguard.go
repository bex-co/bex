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

// Package drainguard keeps single-instance services up through node rolls
// (w1/m186). A CAPI drain evicts a one-instance App's only pod before its
// replacement exists, so every roll of the tenant pool cost those services
// their start time, or hours when the replacement could not schedule (the
// 2026-10-24 beancount.io outage, w1/138).
//
// drain-guard puts a CAPI pre-drain hook on every guarded Machine. CAPI neither
// cordons nor drains a deleting Machine while a pre-drain hook remains, so on
// deletion drain-guard cordons the node itself, rolls each bex App Deployment
// whose only Ready pods are there (RollingUpdate starts the replacement on
// another node before it retires the old pod), and releases the hook once none
// is left at risk. The hold is bounded so a replacement that never becomes
// Ready cannot pin the node.
//
// Cluster Autoscaler removes a node differently: it taints the node, evicts its
// pods itself, and deletes the Machine only once it is empty, so the hook never
// sees them (w1/m187). drain-guard therefore also watches Nodes, and evacuates a
// guarded node as soon as CA marks it, inside the delay CA sleeps between that
// taint and its drain (--node-delete-delay-after-taint).
//
// It runs as its own ServiceAccount, never the operator's; an admission policy
// confines its writes to the hook, the cordon and the evacuation annotation
// (deploy/gitops/base/drain-guard-admission.yaml).
package drainguard

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// HookAnnotation is drain-guard's CAPI pre-drain hook; the value names its
	// owner, as CAPI's hook contract asks.
	HookAnnotation = "pre-drain.delete.hook.machine.cluster.x-k8s.io/" + hookOwner
	hookOwner      = "bex-drain-guard"

	// EvacuatedFromAnnotation rolls an App's pod template off a node. Its value
	// is the node, so applying it again for the same drain changes nothing.
	EvacuatedFromAnnotation = "drain-guard.bex.co/evacuated-from"

	// DefaultMaxHold bounds a hold from the Machine's deletion: long enough for
	// a replacement to pull and start (a new node included), short enough that
	// a release that can never become Ready only delays the roll.
	DefaultMaxHold = 10 * time.Minute

	// ScaleDownTaint is Cluster Autoscaler's NoSchedule taint on a node it has
	// decided to remove (cluster-autoscaler utils/taints ToBeDeletedTaint).
	ScaleDownTaint = "ToBeDeletedByClusterAutoscaler"

	// appLabel selects an App's pods; its Deployment carries the App's name.
	appLabel = "app.bex.co/app"
	// nodeNameField is the pod field selector for the node a pod runs on.
	nodeNameField = "spec.nodeName"
	// recheck paces a held Machine, or a node CA has marked, while its
	// services hand over.
	recheck = 5 * time.Second
)

// MachineGVK is the CAPI Machine drain-guard watches. v1beta2 is the storage
// version from CAPI v1.11; status.nodeRef.name reads the same in v1beta1.
var MachineGVK = schema.GroupVersionKind{Group: "cluster.x-k8s.io", Version: "v1beta2", Kind: "Machine"}

// NewMachine is an empty Machine to read into.
func NewMachine() *unstructured.Unstructured {
	m := &unstructured.Unstructured{}
	m.SetGroupVersionKind(MachineGVK)
	return m
}

// nodeOf is the node a Machine runs, or "" before it has one.
func nodeOf(m *unstructured.Unstructured) string {
	name, _, _ := unstructured.NestedString(m.Object, "status", "nodeRef", "name")
	return name
}

// Reconciler guards Machines and the nodes CA marks for removal. Machines and
// Workload are the same cluster in production (CAPI manages its own cluster)
// and two in the local mock.
type Reconciler struct {
	Machines client.Client
	Workload client.Client
	// Selector picks the Machines that get the hook (production: the tenant
	// serving pool). A deleting Machine that already carries the hook is held
	// whether or not it still matches.
	Selector labels.Selector
	MaxHold  time.Duration
	Now      func() time.Time
}

// SetupWithManager watches Machines through mgr, whose cache must be scoped to
// the Machines' namespace (drain-guard's Role covers only that namespace), and
// the Nodes CA marks for removal through nodes, a cache of the workload cluster.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager, nodes cache.Cache) error {
	if err := ctrl.NewControllerManagedBy(mgr).Named("drain-guard").For(NewMachine()).Complete(r); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).Named("drain-guard-scale-down").
		WatchesRawSource(source.Kind(nodes, &corev1.Node{}, &handler.TypedEnqueueRequestForObject[*corev1.Node]{},
			predicate.NewTypedPredicateFuncs(markedForScaleDown))).
		Complete(reconcile.Func(r.ReconcileNode))
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	m := NewMachine()
	if err := r.Machines.Get(ctx, req.NamespacedName, m); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	_, hooked := m.GetAnnotations()[HookAnnotation]
	if m.GetDeletionTimestamp() != nil {
		if !hooked {
			return ctrl.Result{}, nil
		}
		res, err := r.hold(ctx, m)
		return paced(logf.FromContext(ctx).WithValues("machine", m.GetName()), "holding the drain", res, err)
	}
	if guarded := r.Selector.Matches(labels.Set(m.GetLabels())); guarded != hooked {
		return ctrl.Result{}, r.setHook(ctx, m, guarded)
	}
	return ctrl.Result{}, nil
}

// hold keeps a deleting Machine's hook until no App on its node is at risk,
// then releases it.
func (r *Reconciler) hold(ctx context.Context, m *unstructured.Unstructured) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("machine", m.GetName())
	release := func(reason string, keysAndValues ...any) (ctrl.Result, error) {
		log.Info("releasing the pre-drain hook: "+reason, keysAndValues...)
		return ctrl.Result{}, r.setHook(ctx, m, false)
	}

	nodeName := nodeOf(m)
	if nodeName == "" {
		return release("the Machine has no node")
	}
	log = log.WithValues("node", nodeName)
	if held := r.now().Sub(m.GetDeletionTimestamp().Time); held >= r.MaxHold {
		return release("the hold reached its bound", "held", held.Round(time.Second).String())
	}
	var node corev1.Node
	if err := r.Workload.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
		if apierrors.IsNotFound(err) {
			return release("the node is gone")
		}
		return ctrl.Result{}, err
	}
	if !nodeReady(&node) {
		return release("the node is not Ready, so nothing on it is serving")
	}
	if !node.Spec.Unschedulable {
		// Cordon first, so a replacement never lands on the node it leaves.
		// CAPI's own drain would cordon it next anyway.
		before := node.DeepCopy()
		node.Spec.Unschedulable = true
		if err := r.Workload.Patch(ctx, &node, client.MergeFrom(before)); err != nil {
			return ctrl.Result{}, fmt.Errorf("cordon node %s: %w", nodeName, err)
		}
		log.Info("cordoned the node")
	}

	atRisk, err := r.atRisk(ctx, nodeName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(atRisk) == 0 {
		return release("every App on the node has a Ready instance elsewhere")
	}
	if err := r.evacuate(logf.IntoContext(ctx, log), nodeName, atRisk); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: recheck}, nil
}

// ReconcileNode hands the at-risk Apps off a guarded node CA has marked for
// removal. There is no hook to hold: CA drains on its own clock, so drain-guard
// only starts the replacements and rechecks until none is left at risk. CA's
// taint is NoSchedule, so no cordon is needed and no replacement lands here.
func (r *Reconciler) ReconcileNode(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("node", req.Name)
	res, err := r.handOver(logf.IntoContext(ctx, log), req.Name)
	return paced(log, "handing over before the scale-down", res, err)
}

// paced turns a hand-over error into a retry at recheck's steady pace instead
// of controller-runtime's growing backoff: a failing API must never carry a
// hold past MaxHold, nor a hand-over past CA's delay.
func paced(log logr.Logger, msg string, res ctrl.Result, err error) (ctrl.Result, error) {
	if err != nil {
		log.Error(err, msg)
		return ctrl.Result{RequeueAfter: recheck}, nil
	}
	return res, nil
}

func (r *Reconciler) handOver(ctx context.Context, nodeName string) (ctrl.Result, error) {
	var node corev1.Node
	if err := r.Workload.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !markedForScaleDown(&node) || !nodeReady(&node) {
		return ctrl.Result{}, nil
	}
	if guarded, err := r.guardsNode(ctx, nodeName); err != nil || !guarded {
		return ctrl.Result{}, err
	}
	atRisk, err := r.atRisk(ctx, nodeName)
	if err != nil || len(atRisk) == 0 {
		return ctrl.Result{}, err
	}
	if err := r.evacuate(ctx, nodeName, atRisk); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: recheck}, nil
}

// guardsNode reports whether node belongs to a guarded Machine, so the
// Machine selector stays the pool's only definition.
func (r *Reconciler) guardsNode(ctx context.Context, node string) (bool, error) {
	machines := &unstructured.UnstructuredList{}
	machines.SetGroupVersionKind(MachineGVK.GroupVersion().WithKind("MachineList"))
	if err := r.Machines.List(ctx, machines, client.MatchingLabelsSelector{Selector: r.Selector}); err != nil {
		return false, fmt.Errorf("list guarded Machines: %w", err)
	}
	return slices.ContainsFunc(machines.Items, func(m unstructured.Unstructured) bool { return nodeOf(&m) == node }), nil
}

// evacuate rolls each at-risk App whose rollout has finished off node. One
// already handing over, or mid-rollout, is left alone: a rollout's new pods
// avoid the unschedulable node by themselves. A pod placed there before the node
// became unschedulable is not moved; the bound (or CA's delay) ends that wait.
func (r *Reconciler) evacuate(ctx context.Context, node string, atRisk []appsv1.Deployment) error {
	log := logf.FromContext(ctx)
	waiting := make([]string, 0, len(atRisk))
	for i := range atRisk {
		dep := &atRisk[i]
		waiting = append(waiting, dep.Namespace+"/"+dep.Name)
		if dep.Spec.Template.Annotations[EvacuatedFromAnnotation] == node || !rolledOut(dep) {
			continue
		}
		before := dep.DeepCopy()
		metav1.SetMetaDataAnnotation(&dep.Spec.Template.ObjectMeta, EvacuatedFromAnnotation, node)
		if err := r.Workload.Patch(ctx, dep, client.MergeFrom(before)); err != nil {
			return fmt.Errorf("evacuate %s/%s: %w", dep.Namespace, dep.Name, err)
		}
		log.Info("starting a replacement off the node", "deployment", dep.Namespace+"/"+dep.Name)
	}
	log.V(1).Info("waiting for replacements", "apps", waiting)
	return nil
}

// atRisk returns the App Deployments with a Ready pod on node and no Ready pod
// on any other node: the services a drain of node would take down. A Recreate
// Deployment (a persistent disk, ADR082) cannot overlap two pods, so holding
// for it would only delay the roll; it is left to the drain. Only an App's
// ReplicaSet pods serve it: its pre-deploy and backup Job pods carry the same
// label and count neither on the node nor elsewhere.
func (r *Reconciler) atRisk(ctx context.Context, node string) ([]appsv1.Deployment, error) {
	var onNode corev1.PodList
	if err := r.Workload.List(ctx, &onNode, client.MatchingFields{nodeNameField: node}, client.HasLabels{appLabel}); err != nil {
		return nil, fmt.Errorf("list pods on node %s: %w", node, err)
	}
	seen := map[types.NamespacedName]bool{}
	var out []appsv1.Deployment
	for i := range onNode.Items {
		pod := &onNode.Items[i]
		key := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Labels[appLabel]}
		if seen[key] || !serving(pod) {
			continue
		}
		seen[key] = true
		var dep appsv1.Deployment
		if err := r.Workload.Get(ctx, key, &dep); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		if dep.Spec.Selector == nil || dep.Spec.Selector.MatchLabels[appLabel] != key.Name ||
			dep.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType {
			continue
		}
		var pods corev1.PodList
		if err := r.Workload.List(ctx, &pods, client.InNamespace(key.Namespace), client.MatchingLabels{appLabel: key.Name}); err != nil {
			return nil, fmt.Errorf("list pods of %s: %w", key, err)
		}
		if !slices.ContainsFunc(pods.Items, func(p corev1.Pod) bool { return p.Spec.NodeName != node && serving(&p) }) {
			out = append(out, dep)
		}
	}
	return out, nil
}

// setHook adds or removes the hook annotation. The patch is raw: a merge patch
// computed from an unstructured Machine whose last annotation goes would send
// annotations: null rather than removing just this key.
func (r *Reconciler) setHook(ctx context.Context, m *unstructured.Unstructured, on bool) error {
	value := "null"
	if on {
		value = `"` + hookOwner + `"`
	}
	patch := []byte(`{"metadata":{"annotations":{"` + HookAnnotation + `":` + value + `}}}`)
	if err := r.Machines.Patch(ctx, m, client.RawPatch(types.MergePatchType, patch)); err != nil {
		return client.IgnoreNotFound(err)
	}
	return nil
}

func (r *Reconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// rolledOut reports a Deployment whose every replica runs its current template.
func rolledOut(dep *appsv1.Deployment) bool {
	want := int32(1)
	if dep.Spec.Replicas != nil {
		want = *dep.Spec.Replicas
	}
	return dep.Status.ObservedGeneration >= dep.Generation && dep.Status.UpdatedReplicas == want && dep.Status.Replicas == want
}

// serving is an App instance that answers traffic and is not on its way out.
func serving(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil || !ownedByReplicaSet(pod) {
		return false
	}
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func ownedByReplicaSet(pod *corev1.Pod) bool {
	owner := metav1.GetControllerOf(pod)
	return owner != nil && owner.Kind == "ReplicaSet"
}

func markedForScaleDown(node *corev1.Node) bool {
	return slices.ContainsFunc(node.Spec.Taints, func(t corev1.Taint) bool { return t.Key == ScaleDownTaint })
}

func nodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
