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
	"errors"
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// quotaExceededErr reproduces the exact Kubernetes ResourceQuota admission
// rejection message (verified live against a real envtest apiserver during
// w3/m34 development: "<resource> "<name>" is forbidden: exceeded quota:
// <quota-name>, requested: <key>=<n>, used: <key>=<n>, limited: <key>=<n>").
func quotaExceededErr(resource, name, quotaName, key string, requested, used, limited int) error {
	gr := schema.GroupResource{Group: "app.bex.co", Resource: resource}
	cause := fmt.Errorf("exceeded quota: %s, requested: %s=%d, used: %s=%d, limited: %s=%d",
		quotaName, key, requested, key, used, key, limited)
	return apierrors.NewForbidden(gr, name, cause)
}

func TestQuotaCapError_MapsExceededQuotaToRenderShapedMessage(t *testing.T) {
	err := quotaExceededErr("apps", "over-cap", "tenant-quota", "count/apps.app.bex.co", 1, 25, 25)

	mapped, ok := QuotaCapError(err, "count/apps.app.bex.co", "service")
	if !ok {
		t.Fatalf("QuotaCapError: ok = false, want true for a quota-exceeded Forbidden")
	}
	if !errors.Is(mapped, ErrBadRequest) {
		t.Errorf("mapped error = %v, want it to wrap ErrBadRequest", mapped)
	}
	const want = "workspace is limited to 25 services; delete an existing service to create another"
	if got := mapped.Error(); got != fmt.Sprintf("%s: %s", ErrBadRequest, want) {
		t.Errorf("mapped message = %q, want it to end with %q", got, want)
	}
}

func TestQuotaCapError_IgnoresUnrelatedQuotaDimension(t *testing.T) {
	// Same namespace, same Forbidden, but a DIFFERENT resource's quota
	// (e.g. Postgres) was what actually exceeded — must not be misreported
	// as a service-cap rejection.
	err := quotaExceededErr("databases", "over-cap", "tenant-quota", "count/databases.app.bex.co", 1, 1, 1)

	if _, ok := QuotaCapError(err, "count/apps.app.bex.co", "service"); ok {
		t.Errorf("QuotaCapError matched the wrong count key")
	}
}

func TestQuotaCapError_IgnoresNonForbiddenAndNonQuotaErrors(t *testing.T) {
	for _, err := range []error{
		nil,
		errors.New("boom"),
		ErrNotFound,
		apierrors.NewForbidden(schema.GroupResource{Resource: "apps"}, "x", errors.New("some other admission denial")),
	} {
		if _, ok := QuotaCapError(err, "count/apps.app.bex.co", "service"); ok {
			t.Errorf("QuotaCapError(%v) = ok true, want false (not a quota-exceeded rejection)", err)
		}
	}
}
