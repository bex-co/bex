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

package registry

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ErrConflictRequeue is returned when a Kubernetes write conflict persists
// beyond the short in-function retry budget. The controller should requeue
// the reconcile rather than fail permanently.
var ErrConflictRequeue = errors.New("persistent write conflict: requeue required")

// secretWriteAttempts bounds the read-modify-write retry budget. Both Zot
// Secrets are cluster-global and every App reconcile touches them, so
// contention is normal; a caller that exhausts the budget requeues.
const secretWriteAttempts = 3

// secretMutator computes the next value of a Secret key from its current one.
// Reporting changed=false skips the write entirely, so a converged reconcile
// never touches the API server.
type secretMutator func(current []byte) (next []byte, changed bool, err error)

// mutateSecretKey applies mutate to one key of a Secret in the Zot namespace
// under optimistic locking, retrying conflicts before surfacing
// ErrConflictRequeue.
//
// bootstrap, when non-nil, makes a missing Secret a state to converge from
// rather than an error: the Secret is created with mutate applied to
// bootstrap's value. When nil, a missing Secret surfaces the NotFound error —
// callers for which absence simply means "nothing to change" wrap the result
// in client.IgnoreNotFound.
func (c *Creds) mutateSecretKey(
	ctx context.Context,
	name, key string,
	bootstrap func() []byte,
	mutate secretMutator,
) (changed bool, err error) {
	objKey := client.ObjectKey{Namespace: c.ZotNamespace, Name: name}
	for range secretWriteAttempts {
		var sec corev1.Secret
		switch err := c.Client.Get(ctx, objKey, &sec); {
		case apierrors.IsNotFound(err) && bootstrap != nil:
			base := bootstrap()
			value, changed, err := mutate(base)
			if err != nil {
				return false, err
			}
			if !changed {
				value = base
			}
			created := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: c.ZotNamespace,
					Labels:    map[string]string{"app.bex.co/managed-by": "bex-operator"},
				},
				Data: map[string][]byte{key: value},
			}
			if err := c.Client.Create(ctx, &created); err != nil {
				if apierrors.IsAlreadyExists(err) {
					// Another reconcile bootstrapped it first. Re-read and
					// apply the mutation to the winner's document instead of
					// dropping it on the floor.
					continue
				}
				return false, err
			}
			return true, nil
		case err != nil:
			return false, err
		}

		value, changed, err := mutate(sec.Data[key])
		if err != nil || !changed {
			return false, err
		}
		patch := sec.DeepCopy()
		if patch.Data == nil {
			patch.Data = map[string][]byte{}
		}
		patch.Data[key] = value
		if err := c.Client.Patch(ctx, patch, client.MergeFromWithOptions(&sec, client.MergeFromWithOptimisticLock{})); err != nil {
			if apierrors.IsConflict(err) {
				continue
			}
			return false, err
		}
		return true, nil
	}
	return false, ErrConflictRequeue
}
