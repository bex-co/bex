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

package environments

import "testing"

func TestToRenderEnvironmentUsesOfficialCLIFields(t *testing.T) {
	got := toRenderEnvironment(EnvironmentView{
		ID: "env-1", ProjectID: "prj-1", Name: "staging", ServiceIDs: []string{"web"},
		DatabaseIDs: []string{"db"}, KeyValueIDs: []string{"kv"}, IPAllowList: []string{"10.0.0.0/8"},
	})
	if got.ID != "env-1" || len(got.ServiceIDs) != 1 || len(got.DatabasesIDs) != 1 || len(got.RedisIDs) != 1 {
		t.Fatalf("Render environment = %+v", got)
	}
	if len(got.IPAllowList) != 1 || got.IPAllowList[0].CIDRBlock != "10.0.0.0/8" {
		t.Fatalf("Render ipAllowList = %+v", got.IPAllowList)
	}
}
