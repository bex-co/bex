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
	"bytes"
	"context"
	"encoding/json"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// updateStatusIfChanged avoids a status PUT when the persisted representation
// is unchanged. The caller still owns error handling and conflict retries.
func updateStatusIfChanged(ctx context.Context, c client.Client, obj client.Object) error {
	if statusUnchanged(ctx, c, obj) {
		return nil
	}
	return c.Status().Update(ctx, obj)
}

// statusUnchanged compares the complete status, including durable deploy facts.
// Compare its wire representation: an empty omitempty slice (e.g. cron Runs)
// and nil are the same persisted state. A failed read never suppresses a write.
func statusUnchanged(ctx context.Context, c client.Client, obj client.Object) bool {
	stored := obj.DeepCopyObject().(client.Object)
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), stored); err != nil {
		return false
	}
	var desired, current any
	switch typed := obj.(type) {
	case *appv1alpha1.App:
		desired, current = typed.Status, stored.(*appv1alpha1.App).Status
	case *appv1alpha1.Database:
		desired, current = typed.Status, stored.(*appv1alpha1.Database).Status
	case *appv1alpha1.KeyValue:
		desired, current = typed.Status, stored.(*appv1alpha1.KeyValue).Status
	default:
		return false
	}
	want, err := json.Marshal(desired)
	if err != nil {
		return false
	}
	have, err := json.Marshal(current)
	return err == nil && bytes.Equal(want, have)
}
