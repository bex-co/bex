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

package core

import (
	"context"
	"reflect"

	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// fanOutLimit bounds how many writes one PatchEach runs at once: enough to
// finish a workspace-wide change quickly, few enough not to crowd the API
// server.
const fanOutLimit = 8

// Pointers returns a pointer to each element of items, for handing a listed
// slice of CRs to PatchEach.
func Pointers[T any](items []T) []*T {
	out := make([]*T, len(items))
	for i := range items {
		out[i] = &items[i]
	}
	return out
}

// PatchEach merge-patches every object in objs that mutate changes, at most
// fanOutLimit at a time (w9/m388). Each patch starts from the listed object
// under an optimistic lock, so no extra read is made; one that loses to a
// concurrent write is re-read and mutated again (retry.DefaultRetry), and an
// object deleted or deleting meanwhile is skipped. mutate reports whether the
// object needs a write — false patches nothing — and must be safe to run again
// on a fresh read. The first failure stops scheduling the rest and is
// returned.
func PatchEach[T client.Object](ctx context.Context, c client.Client, objs []T, mutate func(T) bool) error {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(fanOutLimit)
	for _, obj := range objs {
		g.Go(func() error { return patchWithConflictRetry(ctx, c, obj, mutate) })
	}
	return g.Wait()
}

func patchWithConflictRetry[T client.Object](ctx context.Context, c client.Client, obj T, mutate func(T) bool) error {
	key := client.ObjectKeyFromObject(obj)
	reread := false
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if reread {
			// Decode into a zeroed object: a field the server no longer sets
			// must not survive from the failed attempt's mutation.
			reflect.ValueOf(obj).Elem().SetZero()
			if err := c.Get(ctx, key, obj); err != nil {
				return client.IgnoreNotFound(err)
			}
		}
		reread = true
		if obj.GetDeletionTimestamp() != nil {
			return nil
		}
		before, ok := obj.DeepCopyObject().(client.Object)
		if !ok || !mutate(obj) {
			return nil
		}
		return client.IgnoreNotFound(c.Patch(ctx, obj, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})))
	})
}
