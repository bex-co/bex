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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/types"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

// TestSwitchedImageCloneIsAnImageService drives the PINNED client's clone path
// against testdata/switched-image-clone-source.json, the read bex-api is proven
// to emit for a native Node service switched to a prebuilt image
// (lego/backend/internal/apps/native_image_switch_test.go). That read used to
// say runtime node beside the imagePath, with the native build and start
// commands, so `services create --from` refused before sending anything:
// "--build-command and --start-command are only supported for native runtimes"
// (w8/m461).
func TestSwitchedImageCloneIsAnImageService(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "switched-image-clone-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source client.Service
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatalf("the pinned client cannot decode bex's read: %v", err)
	}

	from, region, plan := source.Id, types.RegionFrankfurt, "free"
	input := servicetypes.NormalizeServiceCreateCLIInput(servicetypes.ServiceCreateInput{
		Name: "qa-switched-image-clone", From: &from, Region: &region, Plan: &plan,
	})
	service.ServiceFromAPI(&input, &source)
	input, err = servicetypes.NormalizeAndValidateCreateInput(input, false)
	if err != nil {
		t.Fatalf("the clone input does not validate: %v", err)
	}
	body, err := service.BuildCreateRequest(input, "tea-contract")
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	var sent struct {
		Type  string  `json:"type"`
		Repo  *string `json:"repo"`
		Image *struct {
			ImagePath string `json:"imagePath"`
		} `json:"image"`
		ServiceDetails struct {
			Runtime            string         `json:"runtime"`
			EnvSpecificDetails map[string]any `json:"envSpecificDetails"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(wire, &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Type != "web_service" || sent.ServiceDetails.Runtime != "image" {
		t.Errorf("type/runtime = %q/%q, want web_service/image: %s", sent.Type, sent.ServiceDetails.Runtime, wire)
	}
	if sent.Image == nil || sent.Image.ImagePath != "docker.io/jmalloc/echo-server:v0.3.7" || sent.Repo != nil {
		t.Errorf("the clone must run the same image with no repo: %s", wire)
	}
	for _, field := range []string{"buildCommand", "startCommand", "dockerCommand"} {
		if v, ok := sent.ServiceDetails.EnvSpecificDetails[field]; ok && v != "" {
			t.Errorf("the clone carries %s %v: %s", field, v, wire)
		}
	}
}
