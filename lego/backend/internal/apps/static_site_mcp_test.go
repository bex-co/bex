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

package apps

import (
	"testing"
)

// TestMCPCreateStaticSiteAutoDeployBuildCommand covers w2/m91: Render's
// autoDeploy + buildCommand reach the created App spec.
func TestMCPCreateStaticSiteAutoDeployBuildCommand(t *testing.T) {
	svc, cl := newService(nil)
	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()

	got := call("create_static_site", map[string]any{
		"name":         "site-mcp",
		"repo":         "https://github.com/acme/site",
		"publishPath":  "dist",
		"buildCommand": "npm run build",
		"autoDeploy":   "no",
		"dryRun":       true,
	})
	if got["autoDeploy"] != "no" {
		t.Fatalf("autoDeploy = %v, want no", got["autoDeploy"])
	}
	details, _ := got["serviceDetails"].(map[string]any)
	if details["buildCommand"] != "npm run build" {
		t.Fatalf("serviceDetails.buildCommand = %v, want npm run build (details=%v)", details["buildCommand"], details)
	}
	if n := countApps(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d", n)
	}

	got = call("create_static_site", map[string]any{
		"name":         "site-live",
		"repo":         "https://github.com/acme/site",
		"publishPath":  "dist",
		"buildCommand": "yarn build",
		"autoDeploy":   "yes",
	})
	crName, _ := got["immutableName"].(string)
	if crName == "" {
		crName = "site-live"
	}
	a := getApp(t, cl, crName)
	if a.Spec.BuildCommand != "yarn build" {
		t.Errorf("spec.BuildCommand = %q, want yarn build", a.Spec.BuildCommand)
	}
	if !a.Spec.AutoDeploy {
		t.Errorf("spec.AutoDeploy = false, want true")
	}
}
