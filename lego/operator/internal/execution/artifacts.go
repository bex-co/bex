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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

// Labels returns the common inventory labels for one cross-namespace artifact.
func (i ArtifactIdentity) Labels(component string) map[string]string {
	return PodLabels(i.Name, i.UID, component, i.Workspace, i.Namespace, false)
}

// Owns reports whether obj belongs to this exact App lifetime. Both sides must
// carry a UID: name-only ownership is ambiguous after an App is recreated.
func (i ArtifactIdentity) Owns(obj metav1.Object) bool {
	labels := obj.GetLabels()
	return i.UID != "" && labels[LabelApp] == i.Name && labels[LabelAppUID] == i.UID
}

// CheckOwner rejects deterministic-name reuse across App lifetimes.
func (i ArtifactIdentity) CheckOwner(obj metav1.Object) error {
	if i.Owns(obj) {
		return nil
	}
	return fmt.Errorf("artifact %s/%s belongs to a different App lifetime", obj.GetNamespace(), obj.GetName())
}
