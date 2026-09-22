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

// image_create_test.go is the core half of w4/114. MCP's create schema required
// buildCommand and startCommand — inherited verbatim from Render's git-only
// tool — while the handler refuses them for a prebuilt image, so no value
// satisfied both and every image-backed service type was unreachable over MCP.
//
// Relaxing the schema is safe precisely because the rule is not the schema's:
// resolveBuildStrategy owns it, states it per runtime, and answers for REST,
// GraphQL and MCP alike. These cases pin that, so a future "the schema used to
// catch this" argument has an answer.

import (
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestM114_ImageRuntimeNeedsNoCommands(t *testing.T) {
	// The pass-119 table: every image-backed type, none of them declaring a
	// build or start command.
	for _, svcType := range []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
	} {
		t.Run(svcType, func(t *testing.T) {
			req := CreateRequest{
				Name:    "qa-image",
				Type:    svcType,
				Runtime: "image",
				Image:   "docker.io/traefik/whoami:latest",
				Plan:    "starter",
			}
			if svcType == appv1alpha1.TypeCronJob {
				req.Schedule = "0 * * * *"
			}
			spec, err := specFromCreate(req)
			if err != nil {
				t.Fatalf("image-backed %s with no commands: %v", svcType, err)
			}
			if spec.Image != req.Image {
				t.Fatalf("spec.image = %q, want %q", spec.Image, req.Image)
			}
		})
	}
}

func TestM114_ImageRuntimeStillRefusesCommands(t *testing.T) {
	// The other half of the deadlock is unchanged: supplying them is still an
	// error, which is why relaxing the schema could not simply be "send them
	// anyway".
	_, err := specFromCreate(CreateRequest{
		Name: "qa-image", Runtime: "image", Image: "nginx:1",
		BuildCommand: "IGNORED-BY-BEX", StartCommand: "IGNORED-BY-BEX",
	})
	if err == nil {
		t.Fatal("a prebuilt image declaring build/start commands must still be refused")
	}
}

// TestM114_NativeRuntimeStillFailsClosed is the control the note insisted on:
// the git path must not be weakened by the schema relaxation. It is not,
// because the requirement was never the schema's to enforce.
func TestM114_NativeRuntimeStillFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  CreateRequest
	}{
		{"no commands at all", CreateRequest{Name: "qa-git", Runtime: "node", Repo: "https://github.com/a/b"}},
		{"build only", CreateRequest{Name: "qa-git", Runtime: "node", Repo: "https://github.com/a/b", BuildCommand: "npm ci"}},
		{"start only", CreateRequest{Name: "qa-git", Runtime: "node", Repo: "https://github.com/a/b", StartCommand: "npm start"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := specFromCreate(tc.req)
			if !errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("native runtime %s = %v, want ErrBadRequest", tc.name, err)
			}
			if !strings.Contains(err.Error(), "requires buildCommand and startCommand") {
				t.Fatalf("the refusal must say what is missing, got %v", err)
			}
		})
	}

	// And a complete git create still succeeds — the guard is not "refuse git".
	if _, err := specFromCreate(CreateRequest{
		Name: "qa-git", Runtime: "node", Repo: "https://github.com/a/b",
		BuildCommand: "npm ci", StartCommand: "npm start",
	}); err != nil {
		t.Fatalf("a complete native create: %v", err)
	}
}
