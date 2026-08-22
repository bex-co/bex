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

// Package selfimage answers "which image am I running?" for the manager.
//
// The operator creates workloads that must run a bex entrypoint out of the bex
// image — today the Key Value backup CronJob's /backup-encrypt stage (w7/m85).
// It cannot hardcode that reference: production resolves it from the digest
// deploy.yml pushed, which kustomize writes into this Deployment's `image:`
// field, and no constant in the source can know it.
//
// Reading it back off the running Pod means the derived workload always tracks
// the exact image the operator itself was rolled out with — one artifact, one
// digest, no second thing to keep in sync.
package selfimage

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ErrNoPodIdentity is returned when the downward-API values that name the
// running Pod are absent — the manager is running outside a cluster (`make
// run`, envtest), so there is no Pod to read.
var ErrNoPodIdentity = errors.New("POD_NAME/POD_NAMESPACE are unset")

// Resolve returns the image of the named container in the given Pod.
//
// The image comes from `.spec`, not `.status.containerStatuses[].imageID`:
// spec is what a derived workload must ask the kubelet to pull (imageID is a
// node-local, runtime-formatted string), and in production it is already the
// digest reference kustomize wrote.
func Resolve(ctx context.Context, c client.Client, namespace, name, container string) (string, error) {
	if namespace == "" || name == "" {
		return "", ErrNoPodIdentity
	}
	var pod corev1.Pod
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &pod); err != nil {
		return "", fmt.Errorf("read own pod %s/%s: %w", namespace, name, err)
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			if c.Image == "" {
				return "", fmt.Errorf("container %q in pod %s/%s has no image", container, namespace, name)
			}
			return c.Image, nil
		}
	}
	return "", fmt.Errorf("pod %s/%s has no container named %q", namespace, name, container)
}

// ManagerContainer is the container name the manager Deployment gives the
// operator (lego/operator/config/manager/manager.yaml). Resolve looks that
// container up by name rather than taking containers[0], so adding a sidecar
// cannot silently make the backup helper run something else.
const ManagerContainer = "manager"
