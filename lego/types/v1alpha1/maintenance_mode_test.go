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

package v1alpha1

import (
	"encoding/json"
	"testing"
)

func TestMaintenanceModeJSONPreservesOmittedAndExplicitStates(t *testing.T) {
	states := []struct {
		name string
		spec AppSpec
	}{
		{name: "omitted", spec: AppSpec{}},
		{name: "disabled default", spec: AppSpec{MaintenanceMode: &MaintenanceModeSpec{}}},
		{name: "enabled default", spec: AppSpec{MaintenanceMode: &MaintenanceModeSpec{Enabled: true}}},
		{name: "enabled custom", spec: AppSpec{MaintenanceMode: &MaintenanceModeSpec{Enabled: true, URI: "https://status.example.com/maintenance"}}},
	}
	for _, tc := range states {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			var got AppSpec
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if tc.spec.MaintenanceMode == nil {
				if got.MaintenanceMode != nil {
					t.Fatalf("omitted maintenanceMode round-tripped as %+v", got.MaintenanceMode)
				}
				return
			}
			if got.MaintenanceMode == nil || *got.MaintenanceMode != *tc.spec.MaintenanceMode {
				t.Fatalf("maintenanceMode = %+v, want %+v", got.MaintenanceMode, tc.spec.MaintenanceMode)
			}
		})
	}
}

func TestMaintenanceModeDeepCopyDoesNotAlias(t *testing.T) {
	original := &App{Spec: AppSpec{MaintenanceMode: &MaintenanceModeSpec{Enabled: true, URI: "https://status.example.com"}}}
	copy := original.DeepCopy()
	copy.Spec.MaintenanceMode.URI = "https://other.example.com"
	if original.Spec.MaintenanceMode.URI != "https://status.example.com" {
		t.Fatalf("DeepCopy aliased maintenanceMode: %+v", original.Spec.MaintenanceMode)
	}
}
