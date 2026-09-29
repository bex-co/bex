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

// TestImageCronCloneKeepsItsCommand drives the PINNED client's clone path —
// the same calls `services create --from` makes (cmd/servicecreate.go:137-158)
// — against testdata/image-cron-clone-source.json, the read bex-api is proven
// to emit (lego/backend/internal/apps/cron_image_command_test.go). An image
// cron's command used to vanish here: the client reads it only from
// envSpecificDetails.startCommand, which an image service never carried, so
// the clone ran its image entrypoint on every tick (w1/m167).
func TestImageCronCloneKeepsItsCommand(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "image-cron-clone-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source client.Service
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatalf("the pinned client cannot decode bex's read: %v", err)
	}

	from, region := source.Id, types.RegionFrankfurt
	input := servicetypes.NormalizeServiceCreateCLIInput(servicetypes.ServiceCreateInput{
		Name: "qa-image-cron-clone", From: &from, Region: &region,
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
			Runtime            string `json:"runtime"`
			Schedule           string `json:"schedule"`
			EnvSpecificDetails struct {
				StartCommand string `json:"startCommand"`
			} `json:"envSpecificDetails"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(wire, &sent); err != nil {
		t.Fatal(err)
	}
	if got := sent.ServiceDetails.EnvSpecificDetails.StartCommand; got != "echo QA_CLONE_MARKER" {
		t.Errorf("the clone POST carries startCommand %q, want the source's command: %s", got, wire)
	}
	if sent.Type != "cron_job" || sent.ServiceDetails.Runtime != "image" || sent.ServiceDetails.Schedule != "*/2 * * * *" {
		t.Errorf("type/runtime/schedule = %q/%q/%q: %s", sent.Type, sent.ServiceDetails.Runtime, sent.ServiceDetails.Schedule, wire)
	}
	if sent.Image == nil || sent.Image.ImagePath != "docker.io/library/busybox:1.37" || sent.Repo != nil {
		t.Errorf("the clone must stay image-backed with no invented repo: %s", wire)
	}
}
