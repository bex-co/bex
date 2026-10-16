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
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CodePlacementCleanupConflict is the public conflict a conditional
// project/environment clear answers when the resource kept changing (w4/m214).
const CodePlacementCleanupConflict = "PLACEMENT_CLEANUP_CONFLICT"

var errPlacementReplaced = errors.New("resource replaced during placement cleanup")

// RetryPlacementClear runs one conditional project/environment clear on a
// datastore, re-deciding on every attempt (w4/m214). Each attempt fetches the
// resource afresh — the caller's fetch authorizes it — and hands it to clear,
// which rechecks the expected placement and patches under that read's
// resourceVersion. The optimistic lock stays: an unrelated update between the
// read and the patch (status, annotations) loses the patch, which is ordinary
// and is retried on a fresh read, while a concurrent reassignment is seen by
// the recheck and left intact. A resource replaced under the same name is
// never cleared.
//
// Only conflicts retry, a bounded number of times and never past ctx. When
// they run out, or the resource was replaced, the caller gets a public
// conflict saying what is true: earlier cleanup steps may already have been
// applied, and current placement must be reloaded before retrying. Every other
// error is returned as it was.
func RetryPlacementClear[T client.Object](ctx context.Context, fetch func() (T, error), clear func(T) error) error {
	var (
		uid  types.UID
		seen bool
	)
	err := retry.OnError(retry.DefaultBackoff, apierrors.IsConflict, func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		obj, err := fetch()
		if err != nil {
			return err
		}
		if !seen {
			uid, seen = obj.GetUID(), true
		} else if obj.GetUID() != uid {
			return errPlacementReplaced
		}
		return clear(obj)
	})
	// retry.OnError reports a done context as its last conflict; the request's
	// own cancellation or deadline is the truthful answer.
	if ctxErr := ctx.Err(); err != nil && ctxErr != nil {
		return ctxErr
	}
	if apierrors.IsConflict(err) || errors.Is(err, errPlacementReplaced) {
		return NewConflictError(CodePlacementCleanupConflict,
			"the resource kept changing while its project or environment placement was being cleared; "+
				"earlier cleanup steps may already have been applied — reload the current placement and retry", nil)
	}
	return err
}
