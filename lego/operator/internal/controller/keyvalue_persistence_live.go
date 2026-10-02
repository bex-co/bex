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
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"github.com/redis/go-redis/v9"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errKeyValuePersistenceSourceUnknown = errors.New("legacy persistence source is unknown")

// prepareKeyValuePersistence leaves the current serving pod untouched until its
// entire live snapshot dataset is journaled. The UID/generation token tells the
// offline initializer that this AOF, rather than an older RDB marker, is now
// authoritative. An initializer consumes a token once, so later restarts and
// mode reversals cannot replay an obsolete handoff.
func (r *KeyValueReconciler) prepareKeyValuePersistence(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, auth *corev1.Secret, intent *keyValueIntent) (ctrl.Result, bool, error) {
	return r.prepareKeyValuePersistenceWith(ctx, kv, sts, auth, intent, prepareKeyValueJournalAtPod)
}

func prepareKeyValueJournalAtPod(ctx context.Context, pod *corev1.Pod, auth *corev1.Secret) (bool, error) {
	timeout, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	c := redis.NewClient(&redis.Options{Addr: net.JoinHostPort(pod.Status.PodIP, "6379"), Password: string(auth.Data["password"]),
		DialTimeout: 2 * time.Second, ReadTimeout: 2 * time.Second, WriteTimeout: 2 * time.Second, MaxRetries: -1, Protocol: 2, ContextTimeoutEnabled: true})
	defer func() { _ = c.Close() }()
	return prepareValkeyJournal(timeout, c)
}

func (r *KeyValueReconciler) prepareKeyValuePersistenceWith(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, auth *corev1.Secret, intent *keyValueIntent, prepare func(context.Context, *corev1.Pod, *corev1.Secret) (bool, error)) (ctrl.Result, bool, error) {
	reader := cmp.Or(r.APIReader, client.Reader(r.Client))
	if legacyKeyValueWorkloadUnchanged(sts, kv, *intent) {
		intent.skipPersistenceInit = true
		return ctrl.Result{}, false, nil
	}

	if result, done, err := r.bootstrapKeyValuePersistence(ctx, kv, sts, intent, reader); done || err != nil {
		return result, done, err
	}
	if result, done, err := r.recoverKeyValuePersistenceToken(ctx, kv, sts, intent, reader); done || err != nil {
		return result, done, err
	}
	// Read the actual old template, not the frozen legacy PVC fallback.
	if sts.UID == "" || cmp.Or(intent.persistenceSource, keyValueTemplatePersistenceMode(sts, kv.Spec.PersistenceMode)) != keyValueSnapshotMode ||
		keyValuePersistenceMode(kv.Spec.PersistenceMode) != "journal-snapshot" ||
		(sts.Spec.Replicas != nil && *sts.Spec.Replicas == 0) {
		return ctrl.Result{}, false, nil
	}
	pod := &corev1.Pod{}
	err := reader.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, pod)
	if err == nil && metav1.IsControlledBy(pod, sts) && pod.DeletionTimestamp.IsZero() && pod.Status.PodIP != "" && keyValueHandoffSourceReady(pod, sts, intent) {
		var ready bool
		ready, err = prepare(ctx, pod, auth)
		if ready && err == nil {
			// Re-check the exact process, not just the pod UID: a container can
			// restart in the same pod while the rewrite is being observed.
			currentPod := &corev1.Pod{}
			if err := reader.Get(ctx, client.ObjectKeyFromObject(pod), currentPod); err != nil {
				return ctrl.Result{RequeueAfter: time.Second}, true, nil
			}
			if currentPod.UID != pod.UID || !currentPod.DeletionTimestamp.IsZero() || !keyValueHandoffSourceReady(currentPod, sts, intent) ||
				persistenceProcess(currentPod) != persistenceProcess(pod) {
				return ctrl.Result{RequeueAfter: time.Second}, true, nil
			}
			token := fmt.Sprintf("%s-%s-%d", pod.UID, persistenceProcess(pod), kv.Generation)
			// Metadata does not restart the pod. Save authority before considering a
			// newer user intent, so reversal still consumes the completed journal.
			before := sts.DeepCopy()
			if sts.Annotations == nil {
				sts.Annotations = map[string]string{}
			}
			sts.Annotations[keyValuePersistenceTokenAnnotation] = token
			if err := r.Patch(ctx, sts, client.MergeFrom(before)); err != nil {
				return ctrl.Result{}, true, err
			}
			intent.persistenceToken = token
			// A newer desired generation is handled before any workload mutation.
			latest := &appv1alpha1.KeyValue{}
			if err := reader.Get(ctx, client.ObjectKeyFromObject(kv), latest); err != nil {
				return ctrl.Result{}, true, err
			}
			if latest.Generation != kv.Generation {
				return ctrl.Result{RequeueAfter: time.Millisecond}, true, nil
			}
			return ctrl.Result{}, false, nil
		}
	}
	return r.deferKeyValuePersistence(ctx, kv, err)
}

func (r *KeyValueReconciler) deferKeyValuePersistence(ctx context.Context, kv *appv1alpha1.KeyValue, err error) (ctrl.Result, bool, error) {
	reason, message := "PersistenceTransition", "preparing the current keyspace journal before restarting Valkey"
	delay := 2 * time.Second
	kv.Status.Phase = appv1alpha1.KVPhaseProvisioning
	if err != nil {
		// Do not copy a Redis/network error into status: it may contain connection
		// details. The previous pod keeps serving and the next reconcile retries.
		reason, message = "PersistenceTransitionFailed", "could not prepare the journal; the previous Valkey workload is retained and preparation will retry"
		if errors.Is(err, errKeyValuePersistenceSourceUnknown) {
			reason = "PersistenceSourceUnknown"
			message = "a platform operator must establish this legacy volume's last applied persistence mode before migration or resume; no data files were changed"
		}
		kv.Status.Phase = appv1alpha1.KVPhaseFailed
		delay = 10 * time.Second
	}
	meta.SetStatusCondition(&kv.Status.Conditions, metav1.Condition{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason,
		Message: message, ObservedGeneration: kv.Generation,
	})
	if err := updateStatusIfChanged(ctx, r.Client, kv); err != nil {
		return ctrl.Result{}, true, err
	}
	return ctrl.Result{RequeueAfter: delay}, true, nil
}

func prepareValkeyJournal(ctx context.Context, c *redis.Client) (bool, error) {
	settings, err := c.ConfigGet(ctx, "appendonly").Result()
	if err != nil {
		return false, err
	}
	if settings["appendonly"] != "yes" {
		if err := c.ConfigSet(ctx, "appendonly", "yes").Err(); err != nil {
			return false, err
		}
	}
	info, err := c.Info(ctx, "persistence").Result()
	if err != nil {
		return false, err
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(info, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok {
			fields[key] = value
		}
	}
	rewrites, _ := strconv.Atoi(fields["aof_rewrites"])
	if rewrites < 1 {
		return false, nil
	}
	if fields["aof_enabled"] != "1" || fields["aof_rewrite_in_progress"] != "0" || fields["aof_rewrite_scheduled"] != "0" {
		return false, nil
	}
	if fields["aof_last_bgrewrite_status"] != "ok" || fields["aof_last_write_status"] != "ok" {
		// A failed conversion is retried in place. Never turn AOF off, which could
		// discard acknowledged writes; never roll onto the stale/failed journal.
		_ = c.BgRewriteAOF(ctx).Err()
		return false, fmt.Errorf("journal rewrite has not completed successfully")
	}
	return true, nil
}

func persistenceSourcePodReady(pod *corev1.Pod, sts *appsv1.StatefulSet) bool {
	if sts.Status.ObservedGeneration < sts.Generation || sts.Status.UpdateRevision == "" ||
		pod.Labels[appsv1.StatefulSetRevisionLabel] != sts.Status.UpdateRevision {
		return false
	}
	if !podReady(pod) || persistenceProcess(pod) == "" {
		return false
	}
	for _, actual := range pod.Spec.Containers {
		if actual.Name != keyValueContainerName {
			continue
		}
		for _, desired := range sts.Spec.Template.Spec.Containers {
			if desired.Name == keyValueContainerName {
				return reflect.DeepEqual(actual.Args, desired.Args) && actual.Image == desired.Image
			}
		}
	}
	return false
}

func persistenceProcess(pod *corev1.Pod) string {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == keyValueContainerName && status.State.Running != nil && status.ContainerID != "" {
			return fmt.Sprintf("%s-%d", status.ContainerID, status.RestartCount)
		}
	}
	return ""
}

func (r *KeyValueReconciler) recoverKeyValuePersistenceToken(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, intent *keyValueIntent, reader client.Reader) (ctrl.Result, bool, error) {
	// A completed runtime handoff survives a superseding desired generation.
	intent.persistenceToken = sts.Annotations[keyValuePersistenceTokenAnnotation]
	if intent.persistenceToken != "" && intent.persistenceToken != sts.Spec.Template.Annotations[keyValuePersistenceTokenAnnotation] {
		current := &corev1.Pod{}
		readErr := reader.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, current)
		stopped := apierrors.IsNotFound(readErr) && sts.Spec.Replicas != nil && *sts.Spec.Replicas == 0
		if kv.Spec.PersistenceMode != keyValueOffMode && !stopped && (readErr != nil || !metav1.IsControlledBy(current, sts) ||
			!current.DeletionTimestamp.IsZero() || persistenceProcess(current) == "") {
			return r.deferKeyValuePersistence(ctx, kv, nil)
		}
		if readErr == nil &&
			metav1.IsControlledBy(current, sts) && current.DeletionTimestamp.IsZero() && persistenceProcess(current) != "" && !strings.HasPrefix(intent.persistenceToken, fmt.Sprintf("%s-%s-", current.UID, persistenceProcess(current))) {
			// A snapshot process restarted before rollout: its current keyspace,
			// not the previous process's journal, is authoritative again.
			before := sts.DeepCopy()
			delete(sts.Annotations, keyValuePersistenceTokenAnnotation)
			if err := r.Patch(ctx, sts, client.MergeFrom(before)); err != nil {
				return ctrl.Result{}, true, err
			}
			intent.persistenceToken = ""
		}
	}

	return ctrl.Result{}, false, nil
}

// Bootstrap from the applied process, never an ahead-of-runtime legacy desired
// template. Once authored, the frozen fallback and PVC marker own this choice.
func (r *KeyValueReconciler) bootstrapKeyValuePersistence(ctx context.Context, kv *appv1alpha1.KeyValue, sts *appsv1.StatefulSet, intent *keyValueIntent, reader client.Reader) (ctrl.Result, bool, error) {
	if sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation] != "" {
		return ctrl.Result{}, false, nil
	}
	if kv.Spec.PersistenceMode == keyValueOffMode {
		intent.persistenceSource = keyValueOffMode
		return ctrl.Result{}, false, nil
	}
	if sts.UID == "" {
		pvc := &corev1.PersistentVolumeClaim{}
		err := reader.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: keyValuePVCName(kv.Name)}, pvc)
		if apierrors.IsNotFound(err) {
			intent.persistenceSource = "new:" + keyValuePersistenceMode(kv.Spec.PersistenceMode)
			return ctrl.Result{}, false, nil
		}
		if err == nil {
			err = errKeyValuePersistenceSourceUnknown
		}
		return r.deferKeyValuePersistence(ctx, kv, err)
	}
	pod := &corev1.Pod{}
	err := reader.Get(ctx, client.ObjectKey{Namespace: kv.Namespace, Name: kv.Name + "-0"}, pod)
	if apierrors.IsNotFound(err) {
		// Zero replicas can converge revisions without ever loading those flags.
		// They cannot establish the last authoritative on-disk persistence mode.
		return r.deferKeyValuePersistence(ctx, kv, errKeyValuePersistenceSourceUnknown)
	}
	if err != nil || !metav1.IsControlledBy(pod, sts) || !pod.DeletionTimestamp.IsZero() || !podReady(pod) || persistenceProcess(pod) == "" {
		return r.deferKeyValuePersistence(ctx, kv, err)
	}
	if !slices.ContainsFunc(pod.Spec.Containers, func(c corev1.Container) bool { return c.Name == keyValueContainerName }) {
		return r.deferKeyValuePersistence(ctx, kv, fmt.Errorf("legacy Valkey container missing"))
	}
	intent.persistenceSource = keyValueContainersPersistenceMode(pod.Spec.Containers, kv.Spec.PersistenceMode)
	intent.persistenceLegacyPod = pod
	return ctrl.Result{}, false, nil
}

func keyValueHandoffSourceReady(pod *corev1.Pod, sts *appsv1.StatefulSet, intent *keyValueIntent) bool {
	if legacy := intent.persistenceLegacyPod; legacy != nil {
		return pod.UID == legacy.UID && persistenceProcess(pod) == persistenceProcess(legacy) &&
			podReady(pod) && reflect.DeepEqual(pod.Spec.Containers, legacy.Spec.Containers)
	}
	return persistenceSourcePodReady(pod, sts)
}

// Adopting the new initializer is lazy: an operator upgrade must not restart
// every unchanged legacy database merely to add its migration machinery.
func legacyKeyValueWorkloadUnchanged(sts *appsv1.StatefulSet, kv *appv1alpha1.KeyValue, intent keyValueIntent) bool {
	if sts.UID == "" || sts.Spec.Template.Annotations[keyValuePersistenceSourceAnnotation] != "" {
		return false
	}
	if sts.Spec.Replicas == nil || !statefulSetRolloutReady(sts, *sts.Spec.Replicas) {
		return false
	}
	projected := sts.DeepCopy()
	intent.skipPersistenceInit = true
	applyKeyValueStatefulSet(projected, kv, intent)
	return apiequality.Semantic.DeepEqual(projected.Spec.Template, sts.Spec.Template) &&
		apiequality.Semantic.DeepEqual(projected.Spec.Replicas, sts.Spec.Replicas)
}
