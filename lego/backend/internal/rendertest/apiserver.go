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

package rendertest

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// CreationEpoch is the creationTimestamp the fake API server stamps.
var CreationEpoch = time.Unix(1_700_000_000, 0).UTC()

// Stamped gives every obj without one a creationTimestamp, as a real API
// server guarantees for every stored object, and returns objs. It stamps in
// place, as the fake client builder already sets resourceVersion on the
// fixtures it is given, so a test updating its own fixture keeps working.
// controller-runtime's fake client leaves the timestamp zero, and bex then
// omits the createdAt Render requires, so a response from an unstamped
// fixture fails validation for a reason production never sees.
func Stamped(objs ...client.Object) []client.Object {
	for _, obj := range objs {
		stamp(obj)
	}
	return objs
}

// StampOnCreate is the interceptor half of Stamped: an object a test creates
// through the fake client gets its creationTimestamp too.
var StampOnCreate = interceptor.Funcs{
	Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
		stamp(obj)
		return c.Create(ctx, obj, opts...)
	},
}

func stamp(obj client.Object) {
	if created := obj.GetCreationTimestamp(); created.IsZero() {
		obj.SetCreationTimestamp(metav1.NewTime(CreationEpoch))
	}
}
