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
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/service"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

// switchBackRepo is the repo the switch back names, as the backend replay of
// testdata/switched-image-switch-back-request.json names it.
const switchBackRepo = "https://github.com/render-examples/express-hello-world"

// switchedImageUpdate is the PATCH body the PINNED client builds for
// `services update <id>` with in, against the read bex-api emits for a native
// Node service switched to a prebuilt image
// (testdata/switched-image-clone-source.json).
func switchedImageUpdate(t *testing.T, in servicetypes.ServiceUpdateInput) ([]byte, error) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "switched-image-clone-source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var before client.Service
	if err := json.Unmarshal(raw, &before); err != nil {
		t.Fatalf("the pinned client cannot decode bex's read: %v", err)
	}
	in.ServiceIDOrName = before.Id
	in, err = servicetypes.NormalizeAndValidateUpdateInput(in)
	if err != nil {
		return nil, err
	}
	if err := in.ValidateForServiceType(servicetypes.ServiceType(before.Type)); err != nil {
		return nil, err
	}
	body, err := service.BuildUpdateRequest(before, in)
	if err != nil {
		return nil, err
	}
	return json.Marshal(body)
}

// TestSwitchedImageSwitchBackRequest records what `bex services update <id>
// --repo … --branch main` sends for a service switched from a native runtime
// to an image: the repo and branch and nothing else, because the pinned client
// reads runtime image and refuses the native commands and a runtime change
// before any request. bex-api keeps the commands the image switch dropped and
// restores them on this exact body
// (lego/backend/internal/apps/native_commands_kept_test.go replays the golden),
// so the CLI has a route back to Git (w32/056). Regenerate with
// BEX_UPDATE_GOLDEN=1 and rerun the backend replay.
func TestSwitchedImageSwitchBackRequest(t *testing.T) {
	repo, branch := switchBackRepo, "main"
	wire, err := switchedImageUpdate(t, servicetypes.ServiceUpdateInput{Repo: &repo, Branch: &branch})
	if err != nil {
		t.Fatalf("the pinned client refuses the switch back: %v", err)
	}

	golden := filepath.Join("testdata", "switched-image-switch-back-request.json")
	if os.Getenv("BEX_UPDATE_GOLDEN") == "1" {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, wire, "", "  "); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, append(pretty.Bytes(), '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	if err := json.Unmarshal(wire, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the pinned client's switch back is\n%s\nbut %s records\n%s\nregenerate it with BEX_UPDATE_GOLDEN=1 and rerun the backend replay", wire, golden, raw)
	}
	if len(got) != 2 || got["repo"] != switchBackRepo || got["branch"] != "main" {
		t.Errorf("the switch back must carry only repo and branch: %s", wire)
	}

	// Why the server keeps the commands: the pinned client cannot send them,
	// or the runtime they build with, to a service that reads as an image. A
	// pin that starts sending either fails here, to say the kept commands may
	// no longer be needed.
	start, build, runtime := "node app.js", "yarn", servicetypes.ServiceRuntime("node")
	commands := servicetypes.ServiceUpdateInput{Repo: &repo, Branch: &branch, BuildCommand: &build, StartCommand: &start}
	if wire, err := switchedImageUpdate(t, commands); err == nil || err.Error() != "--build-command and --start-command are only supported for native runtimes" {
		t.Errorf("the switch back with both commands = %s, %v; want the pinned client's refusal before any request", wire, err)
	}
	withRuntime := servicetypes.ServiceUpdateInput{Repo: &repo, Branch: &branch, Runtime: &runtime}
	if wire, err := switchedImageUpdate(t, withRuntime); !errors.Is(err, servicetypes.ErrRuntimeUpdateNotSupported) {
		t.Errorf("the switch back with a runtime = %s, %v; want the pinned client's ErrRuntimeUpdateNotSupported", wire, err)
	}
}
