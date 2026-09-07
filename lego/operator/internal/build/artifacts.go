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

package build

import (
	"context"
	"errors"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/operator/internal/execution"
)

// ArtifactInventory is the complete cross-namespace execution inventory owned
// by one App lifetime. Every count must reach zero before the App finalizer can
// be released.
type ArtifactInventory struct {
	Jobs            int
	Pods            int
	Secrets         int
	ServiceAccounts int
	NetworkPolicies int
	KpackImages     int
	KpackBuilds     int
}

func (i ArtifactInventory) Empty() bool {
	return i == (ArtifactInventory{})
}

type artifactObjects struct {
	inventory ArtifactInventory
	jobs      []*batchv1.Job
	pods      []*corev1.Pod
	secrets   []*corev1.Secret
	accounts  []*corev1.ServiceAccount
	policies  []*networkingv1.NetworkPolicy
}

// ReclaimAppArtifacts deletes and then verifies all cross-namespace artifacts
// for identity. done is true only when a fresh inventory observes none. API
// errors are joined so one broken kind cannot hide cleanup progress in others.
func ReclaimAppArtifacts(ctx context.Context, identity execution.ArtifactIdentity, namespace string, cl client.Client) (done bool, inventory ArtifactInventory, err error) {
	if identity.UID == "" {
		return false, inventory, fmt.Errorf("reclaim build artifacts: empty App UID")
	}
	objects, kpackImages, kpackBuilds, inspectErr := inspectAppArtifacts(ctx, identity, namespace, cl)
	inventory = objects.inventory
	inventory.KpackImages = len(kpackImages)
	inventory.KpackBuilds = len(kpackBuilds)
	if inventory.Empty() && inspectErr == nil {
		return true, inventory, nil
	}

	var errs []error
	if inspectErr != nil {
		errs = append(errs, inspectErr)
	}
	deleteObject := func(kind string, obj client.Object, options ...client.DeleteOption) {
		if !obj.GetDeletionTimestamp().IsZero() {
			return
		}
		if deleteErr := cl.Delete(ctx, obj, options...); deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
			errs = append(errs, fmt.Errorf("delete %s %s: %w", kind, obj.GetName(), deleteErr))
		}
	}
	for _, obj := range objects.jobs {
		deleteObject("Job", obj, client.PropagationPolicy(metav1.DeletePropagationBackground))
	}
	for _, obj := range objects.pods {
		deleteObject("Pod", obj)
	}
	for _, obj := range objects.secrets {
		deleteObject("Secret", obj)
	}
	for _, obj := range objects.accounts {
		deleteObject("ServiceAccount", obj)
	}
	for _, obj := range objects.policies {
		deleteObject("NetworkPolicy", obj)
	}
	for _, obj := range kpackBuilds {
		deleteObject("kpack Build", obj)
	}
	for _, obj := range kpackImages {
		deleteObject("kpack Image", obj)
	}
	return false, inventory, errors.Join(errs...)
}

func inspectAppArtifacts(ctx context.Context, identity execution.ArtifactIdentity, namespace string, cl client.Client) (artifactObjects, []client.Object, []client.Object, error) {
	owned := func(obj client.Object) bool { return appArtifactOwned(identity, obj) }
	out, errs := inspectNamespacedArtifacts(ctx, namespace, cl, owned)
	imageObjects, buildObjects, kpackErrs := inspectKpackArtifacts(ctx, namespace, cl, owned)
	errs = append(errs, kpackErrs...)
	return out, imageObjects, buildObjects, errors.Join(errs...)
}

func appArtifactOwned(identity execution.ArtifactIdentity, obj client.Object) bool {
	if !identity.Owns(obj) {
		return false
	}
	switch obj.GetLabels()[execution.LabelComponent] {
	case "build", "predeploy", "publish", "copied-secret", "clone-secret", "build-registry-secret", "native-env-secret", "execution-network-policy":
		return true
	default:
		return false
	}
}

func inspectNamespacedArtifacts(ctx context.Context, namespace string, cl client.Client, owned func(client.Object) bool) (artifactObjects, []error) {
	var out artifactObjects
	var errs []error
	listOpts := []client.ListOption{client.InNamespace(namespace)}
	var jobs batchv1.JobList
	if listErr := cl.List(ctx, &jobs, listOpts...); listErr != nil {
		errs = append(errs, fmt.Errorf("list Jobs: %w", listErr))
	} else {
		for idx := range jobs.Items {
			if owned(&jobs.Items[idx]) {
				out.jobs = append(out.jobs, &jobs.Items[idx])
			}
		}
	}
	var pods corev1.PodList
	if listErr := cl.List(ctx, &pods, listOpts...); listErr != nil {
		errs = append(errs, fmt.Errorf("list Pods: %w", listErr))
	} else {
		for idx := range pods.Items {
			if owned(&pods.Items[idx]) {
				out.pods = append(out.pods, &pods.Items[idx])
			}
		}
	}
	var secrets corev1.SecretList
	if listErr := cl.List(ctx, &secrets, listOpts...); listErr != nil {
		errs = append(errs, fmt.Errorf("list Secrets: %w", listErr))
	} else {
		for idx := range secrets.Items {
			if owned(&secrets.Items[idx]) {
				out.secrets = append(out.secrets, &secrets.Items[idx])
			}
		}
	}
	var accounts corev1.ServiceAccountList
	if listErr := cl.List(ctx, &accounts, listOpts...); listErr != nil {
		errs = append(errs, fmt.Errorf("list ServiceAccounts: %w", listErr))
	} else {
		for idx := range accounts.Items {
			if owned(&accounts.Items[idx]) {
				out.accounts = append(out.accounts, &accounts.Items[idx])
			}
		}
	}
	var policies networkingv1.NetworkPolicyList
	if listErr := cl.List(ctx, &policies, listOpts...); listErr != nil {
		errs = append(errs, fmt.Errorf("list NetworkPolicies: %w", listErr))
	} else {
		for idx := range policies.Items {
			if owned(&policies.Items[idx]) {
				out.policies = append(out.policies, &policies.Items[idx])
			}
		}
	}
	out.inventory = ArtifactInventory{Jobs: len(out.jobs), Pods: len(out.pods), Secrets: len(out.secrets), ServiceAccounts: len(out.accounts), NetworkPolicies: len(out.policies)}
	return out, errs
}

func inspectKpackArtifacts(ctx context.Context, namespace string, cl client.Client, owned func(client.Object) bool) ([]client.Object, []client.Object, []error) {
	var errs []error
	listOpts := []client.ListOption{client.InNamespace(namespace)}
	var imageObjects []client.Object
	images := newKpackImageList()
	if listErr := cl.List(ctx, images, listOpts...); listErr != nil {
		if !missingKpack(listErr) {
			errs = append(errs, fmt.Errorf("list kpack Images: %w", listErr))
		}
	} else {
		for idx := range images.Items {
			if owned(&images.Items[idx]) {
				imageObjects = append(imageObjects, &images.Items[idx])
			}
		}
	}
	var buildObjects []client.Object
	builds := newKpackBuildList()
	if listErr := cl.List(ctx, builds, listOpts...); listErr != nil {
		if !missingKpack(listErr) {
			errs = append(errs, fmt.Errorf("list kpack Builds: %w", listErr))
		}
	} else {
		for idx := range builds.Items {
			if owned(&builds.Items[idx]) {
				buildObjects = append(buildObjects, &builds.Items[idx])
			}
		}
	}
	return imageObjects, buildObjects, errs
}

func missingKpack(err error) bool {
	return apierrors.IsNotFound(err) || strings.Contains(err.Error(), "no matches for kind")
}
