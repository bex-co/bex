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
	"crypto/sha256"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cleanupJobName derives a stable DNS-label name for a parent's terminal Job.
// Long resource names use a wider UID hash while remaining within Kubernetes'
// 63-byte Job-name limit.
func cleanupJobName(prefix, parentName string, parentUID types.UID) string {
	sum := sha256.Sum256([]byte(parentUID))
	name := fmt.Sprintf("%s%s-%x", prefix, parentName, sum[:4])
	if len(name) <= 63 {
		return name
	}
	suffix := fmt.Sprintf("-%x", sum[:6])
	maxParentLength := 63 - len(prefix) - len(suffix)
	return prefix + parentName[:min(len(parentName), maxParentLength)] + suffix
}

// reconcileCleanupJob runs a persisted terminal Job state machine. The parent
// annotation is written only after JobComplete, then the Job and every labeled
// Pod are deleted and observed absent before done becomes true.
func reconcileCleanupJob(ctx context.Context, cl client.Client, parent client.Object, job *batchv1.Job, completionAnnotation string) (bool, error) {
	marker := string(parent.GetUID())
	if parent.GetAnnotations()[completionAnnotation] == marker {
		var current batchv1.Job
		key := client.ObjectKeyFromObject(job)
		if err := cl.Get(ctx, key, &current); err == nil {
			if current.DeletionTimestamp.IsZero() {
				if err := cl.Delete(ctx, &current, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
					return false, fmt.Errorf("delete completed cleanup Job %s: %w", current.Name, err)
				}
			}
			return false, nil
		} else if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("read completed cleanup Job %s: %w", job.Name, err)
		}
		var pods corev1.PodList
		if err := cl.List(ctx, &pods, client.InNamespace(job.Namespace), client.MatchingLabels(job.Spec.Template.Labels)); err != nil {
			return false, fmt.Errorf("list cleanup Pods for %s: %w", job.Name, err)
		}
		if len(pods.Items) > 0 {
			for idx := range pods.Items {
				if err := cl.Delete(ctx, &pods.Items[idx]); err != nil && !apierrors.IsNotFound(err) {
					return false, fmt.Errorf("delete cleanup Pod %s: %w", pods.Items[idx].Name, err)
				}
			}
			return false, nil
		}
		return true, nil
	}

	var current batchv1.Job
	key := client.ObjectKeyFromObject(job)
	if err := cl.Get(ctx, key, &current); apierrors.IsNotFound(err) {
		if err := cl.Create(ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
			return false, fmt.Errorf("create cleanup Job %s: %w", job.Name, err)
		}
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("read cleanup Job %s: %w", job.Name, err)
	}
	if !cleanupJobOwnedByParent(&current, parent) {
		return false, fmt.Errorf("cleanup Job %s belongs to a different resource lifetime", current.Name)
	}
	if cleanupJobCondition(&current, batchv1.JobFailed) {
		message := cleanupJobFailureMessage(&current)
		if current.DeletionTimestamp.IsZero() {
			if err := cl.Delete(ctx, &current, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !apierrors.IsNotFound(err) {
				return false, fmt.Errorf("cleanup Job %s failed (%s) and could not be deleted: %w", current.Name, message, err)
			}
		}
		return false, fmt.Errorf("cleanup Job %s failed: %s", current.Name, message)
	}
	if !cleanupJobCondition(&current, batchv1.JobComplete) {
		return false, nil
	}
	before := client.MergeFrom(parent.DeepCopyObject().(client.Object))
	annotations := parent.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[completionAnnotation] = marker
	parent.SetAnnotations(annotations)
	if err := cl.Patch(ctx, parent, before); err != nil {
		return false, fmt.Errorf("persist cleanup completion for %s: %w", parent.GetName(), err)
	}
	return false, nil
}

func cleanupJobOwnedByParent(job *batchv1.Job, parent client.Object) bool {
	uid := string(parent.GetUID())
	if appUID := job.Labels["app.bex.co/app-uid"]; appUID != "" {
		return appUID == uid
	}
	if databaseUID := job.Labels["app.bex.co/database-uid"]; databaseUID != "" {
		return databaseUID == uid
	}
	if keyValueUID := job.Labels["app.bex.co/keyvalue-uid"]; keyValueUID != "" {
		return keyValueUID == uid
	}
	return false
}

func cleanupJobCondition(job *batchv1.Job, conditionType batchv1.JobConditionType) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == conditionType && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func cleanupJobFailureMessage(job *batchv1.Job) string {
	for _, condition := range job.Status.Conditions {
		if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
			if condition.Message != "" {
				return condition.Reason + ": " + condition.Message
			}
			return condition.Reason
		}
	}
	return "unknown failure"
}

// deleteAndWait deletes obj idempotently and reports done only after a later
// read observes NotFound. This is the primitive used by finalizers for named
// resources whose deletion may be delayed by Kubernetes finalizers or GC.
func deleteAndWait(ctx context.Context, cl client.Client, obj client.Object) (bool, error) {
	key := client.ObjectKeyFromObject(obj)
	if err := cl.Get(ctx, key, obj); apierrors.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	if obj.GetDeletionTimestamp().IsZero() {
		if err := cl.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
	}
	return false, nil
}
