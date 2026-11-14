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

package postgres

import (
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/246: the HA and read-replica refusals name the plan the request spelled
// (or, when it named none, the plan's spec-based id) and give CPU as Render
// does — never the internal id beside a Kubernetes quantity.
func TestPlanRefusalsNameThePlanAsTyped(t *testing.T) {
	message := func(t *testing.T, err error) string {
		t.Helper()
		var coded *core.CodedError
		if !errors.As(err, &coded) {
			t.Fatalf("err = %v, want a coded refusal", err)
		}
		return coded.Error()
	}
	for _, tc := range []struct {
		name     string
		err      error
		contains []string
	}{
		{"replica on create, spec-based plan", CheckDatabaseAdmission(nil, appv1alpha1.DatabaseSpec{Plan: "0.1c-256mb", ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "rr"}}}),
			[]string{`plan "0.1c-256mb" has 0.1 CPU`, "at least 0.5 CPU", "choose a plan with at least 0.5 CPU (for example 0.5c-1g) to add read replicas"}},
		{"replica on create, REST spelling", CheckDatabaseAdmission(nil, appv1alpha1.DatabaseSpec{Plan: "basic_256mb", ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "rr"}}}),
			[]string{`plan "basic_256mb" has 0.1 CPU`}},
		{"plan change with replicas", CheckDatabaseAdmission(
			&appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Plan: "1c-2g", ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "rr"}}, StorageGB: 10}},
			appv1alpha1.DatabaseSpec{Plan: "0.1c-256mb", ReadReplicas: []appv1alpha1.DatabaseReadReplica{{Name: "rr"}}, StorageGB: 10}),
			[]string{`plan "0.1c-256mb" has 0.1 CPU`, "remove read replicas before selecting this plan"}},
		{"HA on create", CheckHighAvailabilityPlan("0.1c-256mb"),
			[]string{`plan "0.1c-256mb" has 0.1 CPU`, "at least 1 CPU"}},
		// w8/311: the paid-feature remedy fits what the request did.
		{"pooling on a free create", CheckDatabaseAdmission(nil, appv1alpha1.DatabaseSpec{Plan: "free", Pooler: true}),
			[]string{"connection pooling requires a paid Postgres plan; choose a paid plan (for example 0.1c-256mb) to enable connection pooling"}},
		{"pooling turned on for a free database", CheckDatabaseAdmission(
			&appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Plan: "free"}}, appv1alpha1.DatabaseSpec{Plan: "free", Pooler: true}),
			[]string{"choose a paid plan (for example 0.1c-256mb) to enable connection pooling"}},
		{"a pooled database moved to free", CheckDatabaseAdmission(
			&appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Plan: "basic-256mb", Pooler: true}}, appv1alpha1.DatabaseSpec{Plan: "free", Pooler: true}),
			[]string{"disable connection pooling before selecting the free plan"}},
		{"autoscaling on a free create", CheckDatabaseAdmission(nil, appv1alpha1.DatabaseSpec{Plan: "free", DiskAutoscaling: true}),
			[]string{"disk autoscaling requires a paid Postgres plan; choose a paid plan (for example 0.1c-256mb) to enable disk autoscaling"}},
		{"autoscaling turned on for a free database", CheckDatabaseAdmission(
			&appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Plan: "free"}}, appv1alpha1.DatabaseSpec{Plan: "free", DiskAutoscaling: true}),
			[]string{"choose a paid plan (for example 0.1c-256mb) to enable disk autoscaling"}},
		{"an autoscaling database moved to free", CheckDatabaseAdmission(
			&appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Plan: "basic-256mb", DiskAutoscaling: true}}, appv1alpha1.DatabaseSpec{Plan: "free", DiskAutoscaling: true}),
			[]string{"disable disk autoscaling before selecting the free plan"}},
		{"HA enable on an existing database names its spec-based id", checkHighAvailabilityPatch(
			&appv1alpha1.Database{Spec: appv1alpha1.DatabaseSpec{Plan: "basic-256mb"}}, nil, ptrBool(true)),
			[]string{`plan "0.1c-256mb" has 0.1 CPU`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := message(t, tc.err)
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("message %q lacks %q", got, want)
				}
			}
			if strings.Contains(got, "100m") || strings.Contains(got, `"basic-256mb"`) {
				t.Errorf("message %q names the internal id or a Kubernetes quantity", got)
			}
		})
	}
}

func ptrBool(v bool) *bool { return &v }
