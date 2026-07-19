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

package execution

import (
	"context"
	"fmt"
	"maps"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ArtifactIdentity is the durable owner identity for resources that cannot use
// an App ownerReference because they live in a different namespace. Name alone
// is insufficient: Kubernetes permits the same name after deletion, while UID
// identifies exactly one lifetime of that App.
type ArtifactIdentity struct {
	Name      string
	UID       string
	Workspace string
	Namespace string
	CreatedAt time.Time
}

// Labels returns the common inventory labels for one cross-namespace artifact.
func (i ArtifactIdentity) Labels(component string) map[string]string {
	return PodLabels(i.Name, i.UID, component, i.Workspace, i.Namespace, false)
}

// Owns reports whether obj belongs to this exact App lifetime. Objects created
// by an older operator may lack LabelAppUID; those are adopted only when their
// creation timestamp proves they were created after this App. That migrates a
// live pre-m61 workload without letting a newly recreated App adopt older
// same-name residue.
func (i ArtifactIdentity) Owns(obj metav1.Object) bool {
	labels := obj.GetLabels()
	if labels[LabelApp] != i.Name {
		return false
	}
	if got := labels[LabelAppUID]; got != "" || i.UID == "" {
		return i.UID == "" || got == i.UID
	}
	created := obj.GetCreationTimestamp().Time
	return !i.CreatedAt.IsZero() && !created.IsZero() && !created.Before(i.CreatedAt)
}

// CheckOwner rejects deterministic-name adoption across App lifetimes.
func (i ArtifactIdentity) CheckOwner(obj metav1.Object) error {
	if i.Owns(obj) {
		return nil
	}
	return fmt.Errorf("artifact %s/%s belongs to a different App lifetime", obj.GetNamespace(), obj.GetName())
}

// Adopt verifies ownership and backfills the UID label on a provably current
// legacy artifact. Only metadata is updated; immutable Job/Pod specs stay
// untouched.
func (i ArtifactIdentity) Adopt(ctx context.Context, cl client.Client, obj client.Object, desired map[string]string) error {
	if err := i.CheckOwner(obj); err != nil {
		return err
	}
	if i.UID == "" || obj.GetLabels()[LabelAppUID] == i.UID {
		return nil
	}
	labels := maps.Clone(obj.GetLabels())
	if labels == nil {
		labels = map[string]string{}
	}
	maps.Copy(labels, desired)
	obj.SetLabels(labels)
	return cl.Update(ctx, obj)
}
