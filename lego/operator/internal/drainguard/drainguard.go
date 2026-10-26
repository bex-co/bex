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
// It runs as its own ServiceAccount, never the operator's; an admission policy
// confines its writes to the hook, the cordon and the evacuation annotation
// (deploy/gitops/base/drain-guard-admission.yaml).
package drainguard

import (
	"context"
	"fmt"
	"slices"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

	// appLabel selects an App's pods; its Deployment carries the App's name.
	appLabel = "app.bex.co/app"
	// nodeNameField is the pod field selector for the node a pod runs on.
	nodeNameField = "spec.nodeName"
	// recheck paces a held Machine while its services hand over.
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

// Reconciler guards Machines. Machines and Workload are the same cluster in
// production (CAPI manages its own cluster) and two in the local mock.
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
// the Machines' namespace (drain-guard's Role covers only that namespace).
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).Named("drain-guard").For(NewMachine()).Complete(r)
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
		if err != nil {
			// Retry at the hold's own pace, not controller-runtime's growing
			// backoff, so a failing API can never carry the hold past MaxHold.
			logf.FromContext(ctx).Error(err, "holding the drain", "machine", m.GetName())
			return ctrl.Result{RequeueAfter: recheck}, nil
		}
		return res, nil
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

	nodeName, _, _ := unstructured.NestedString(m.Object, "status", "nodeRef", "name")
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
	waiting := make([]string, 0, len(atRisk))
	for i := range atRisk {
		dep := &atRisk[i]
		waiting = append(waiting, dep.Namespace+"/"+dep.Name)
		// Already handing over, or mid-rollout: a rollout's new pods avoid the
		// cordoned node by themselves. One already placed there before the
		// cordon is not moved; the bound then ends that hold.
		if dep.Spec.Template.Annotations[EvacuatedFromAnnotation] == nodeName || !rolledOut(dep) {
			continue
		}
		before := dep.DeepCopy()
		metav1.SetMetaDataAnnotation(&dep.Spec.Template.ObjectMeta, EvacuatedFromAnnotation, nodeName)
		if err := r.Workload.Patch(ctx, dep, client.MergeFrom(before)); err != nil {
			return ctrl.Result{}, fmt.Errorf("evacuate %s/%s: %w", dep.Namespace, dep.Name, err)
		}
		log.Info("starting a replacement off the node", "deployment", dep.Namespace+"/"+dep.Name)
	}
	log.V(1).Info("holding the drain", "waiting", waiting)
	return ctrl.Result{RequeueAfter: recheck}, nil
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

func nodeReady(node *corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
