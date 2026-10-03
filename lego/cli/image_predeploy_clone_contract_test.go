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

// TestImagePreDeployCloneKeepsItsCommand drives the PINNED client's clone path
// against testdata/image-predeploy-clone-source-<type>.json, the reads bex-api
// is proven to emit (lego/backend/internal/apps/image_predeploy_clone_test.go).
// The client reads a pre-deploy command only from
// envSpecificDetails.preDeployCommand, which an image service never carried,
// so the clone deployed without its migration step (w2/042).
func TestImagePreDeployCloneKeepsItsCommand(t *testing.T) {
	for _, svcType := range []string{"web_service", "private_service", "background_worker"} {
		raw, err := os.ReadFile(filepath.Join("testdata", "image-predeploy-clone-source-"+svcType+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var source client.Service
		if err := json.Unmarshal(raw, &source); err != nil {
			t.Fatalf("%s: the pinned client cannot decode bex's read: %v", svcType, err)
		}

		from, region := source.Id, types.RegionFrankfurt
		input := servicetypes.NormalizeServiceCreateCLIInput(servicetypes.ServiceCreateInput{
			Name: "qa-image-predeploy-clone", From: &from, Region: &region,
		})
		service.ServiceFromAPI(&input, &source)
		input, err = servicetypes.NormalizeAndValidateCreateInput(input, false)
		if err != nil {
			t.Fatalf("%s: the clone input does not validate: %v", svcType, err)
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
				PreDeployCommand   string         `json:"preDeployCommand"`
				EnvSpecificDetails map[string]any `json:"envSpecificDetails"`
			} `json:"serviceDetails"`
		}
		if err := json.Unmarshal(wire, &sent); err != nil {
			t.Fatal(err)
		}
		if got := sent.ServiceDetails.PreDeployCommand; got != "echo QA_PREDEPLOY_MARKER" {
			t.Errorf("%s: the clone POST carries preDeployCommand %q, want the source's command: %s", svcType, got, wire)
		}
		if sent.Type != svcType || sent.ServiceDetails.Runtime != "image" {
			t.Errorf("%s: type/runtime = %q/%q: %s", svcType, sent.Type, sent.ServiceDetails.Runtime, wire)
		}
		if sent.Image == nil || sent.Image.ImagePath != "docker.io/mendhak/http-https-echo:35" || sent.Repo != nil {
			t.Errorf("%s: the clone must stay image-backed with no invented repo: %s", svcType, wire)
		}
		for _, field := range []string{"buildCommand", "startCommand"} {
			if v, ok := sent.ServiceDetails.EnvSpecificDetails[field]; ok && v != "" {
				t.Errorf("%s: the clone invented %s %v: %s", svcType, field, v, wire)
			}
		}
	}
}
