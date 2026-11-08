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
	"errors"
	"net/http"
	"testing"
)

// w4/534: a definitive refusal is typed, so the deploy names the image and the
// fix; 429 and 5xx stay transient and retried.
func TestStatusFailureTypesARefusal(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnauthorized, http.StatusForbidden} {
		var refused *RefusedError
		if err := statusFailure("image metadata request", status, http.Header{}); !errors.As(err, &refused) || refused.Status != status {
			t.Errorf("%d = %v, want a RefusedError", status, err)
		}
	}
	for _, status := range []int{http.StatusTooManyRequests, http.StatusBadGateway} {
		var transient *TransientError
		var refused *RefusedError
		err := statusFailure("image metadata request", status, http.Header{})
		if !errors.As(err, &transient) || errors.As(err, &refused) {
			t.Errorf("%d = %v, want transient only", status, err)
		}
	}
}
