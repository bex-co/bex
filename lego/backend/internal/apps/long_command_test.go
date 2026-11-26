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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/183. Render's pg_pinecone_render template inlines its start script into
// render.yaml as base64 lines, a 4,717-byte dockerCommand. Render's API schema
// puts no length limit on a command, but the App CRD capped startCommand,
// buildCommand, preDeployCommand and initialDeployHook at 4096 bytes, so
// production's server-side dry run refused the file at validate and at the
// plan. These tests run against crdValidatingClient, which applies the CRD's
// own schema as that dry run does.

// longDockerCommand is a start script inlined as base64 lines, the way
// pg_pinecone_render's dockerCommand is, exactly n bytes long.
func longDockerCommand(t *testing.T, n int) string {
	t.Helper()
	const (
		head    = "/bin/sh -c 'set -eu\nrm -f /tmp/start.b64\n"
		tail    = "base64 -d /tmp/start.b64 > /tmp/start.sh\nexec sh /tmp/start.sh'"
		open    = `printf "%s\n" "`
		closing = "\" >> /tmp/start.b64\n"
	)
	line := open + strings.Repeat("A", 76) + closing
	room := n - len(head) - len(tail) - len(open) - len(closing)
	if room < 0 {
		t.Fatalf("a command of %d bytes cannot hold the script's frame", n)
	}
	full := room / len(line)
	command := head + strings.Repeat(line, full) + open + strings.Repeat("A", room-full*len(line)) + closing + tail
	if len(command) != n {
		t.Fatalf("built a %d-byte command, want %d", len(command), n)
	}
	return command
}

// literalBlock is s as a YAML literal block scalar (|-) indented by indent.
func literalBlock(s, indent string) string {
	return "|-\n" + indent + strings.ReplaceAll(s, "\n", "\n"+indent)
}

func TestBlueprintLongDockerCommandValidatesPlansAndApplies(t *testing.T) {
	command := longDockerCommand(t, 4717)
	manifest := "services:\n" +
		"  - type: pserv\n" +
		"    name: pg-pinecone\n" +
		"    runtime: image\n" +
		"    plan: starter\n" +
		"    image:\n" +
		"      url: docker.io/pineconeteam/pg_pinecone:pg18\n" +
		"    dockerCommand: " + literalBlock(command, "      ") + "\n"
	svc, _, ctx := newOnboardingService(t, manifest)
	svc.Client = crdValidatingClient(t)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	call := func(method, path, body string) (int, map[string]any) {
		t.Helper()
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx))
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	// Validate: the REST route `render blueprints validate` posts.
	raw, _ := json.Marshal(map[string]string{"bexYaml": manifest, "ownerId": "tea-a"})
	if code, out := call(http.MethodPost, "/v1/blueprints/validate", string(raw)); code != http.StatusOK || out["valid"] != true {
		t.Fatalf("validate = %d %v, want 200 valid", code, out)
	}
	// Plan: create the Blueprint from its repository, as Deploy to Render does.
	code, created := call(http.MethodPost, "/v1/blueprints", `{"name":"pg-pinecone","ownerId":"tea-a","source":{"repo":"https://github.com/render-examples/pg_pinecone_render","branch":"main"}}`)
	if code != http.StatusCreated || created["planningOutcome"] != PlanningPlanned {
		t.Fatalf("create = %d %v, want 201 planned", code, created)
	}
	// Apply: approve the planned sync.
	id := created["blueprint"].(map[string]any)["id"].(string)
	if code, approved := call(http.MethodPost, "/v1/blueprints/"+id+"/approve", `{}`); code != http.StatusOK || approved["state"] != store.BlueprintSyncStateSuccess {
		t.Fatalf("approve = %d %v, want 200 %s", code, approved, store.BlueprintSyncStateSuccess)
	}
	var apps appv1alpha1.AppList
	if err := svc.Client.List(ctx, &apps); err != nil || len(apps.Items) != 1 {
		t.Fatalf("applied %d Apps (err %v), want 1", len(apps.Items), err)
	}
	if got := apps.Items[0].Spec.StartCommand; got != command {
		t.Fatalf("the applied App's startCommand is %d bytes, want the file's %d-byte dockerCommand unchanged", len(got), len(command))
	}
}

// A service created or updated directly takes the same command: REST, GraphQL
// and MCP all write it to the App's startCommand.
func TestRESTCreateAndPatchTakeALongDockerCommand(t *testing.T) {
	cl := crdValidatingClient(t)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, DomainTraffic: allowDomainTraffic{}}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	send := func(method, path string, body map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		rec := httptest.NewRecorder()
		validated(t, mux).ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(string(raw))))
		if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
			t.Fatalf("%s %s = %d: %s", method, path, rec.Code, rec.Body.String())
		}
	}

	created := longDockerCommand(t, 4717)
	send(http.MethodPost, "/v1/services", map[string]any{
		"name": "pg-pinecone", "type": "private_service",
		"image": map[string]any{"imagePath": "docker.io/pineconeteam/pg_pinecone:pg18"},
		"serviceDetails": map[string]any{"plan": "starter", "runtime": "image",
			"envSpecificDetails": map[string]any{"dockerCommand": created}},
	})
	if got := getApp(t, cl, "pg-pinecone").Spec.StartCommand; got != created {
		t.Fatalf("created startCommand is %d bytes, want %d", len(got), len(created))
	}
	patched := longDockerCommand(t, 16384)
	send(http.MethodPatch, "/v1/services/pg-pinecone", map[string]any{
		"serviceDetails": map[string]any{"envSpecificDetails": map[string]any{"dockerCommand": patched}},
	})
	if got := getApp(t, cl, "pg-pinecone").Spec.StartCommand; got != patched {
		t.Fatalf("patched startCommand is %d bytes, want %d", len(got), len(patched))
	}
}

// crdValidatingClient is what makes the tests above meet the CRD: it refuses
// what the schema refuses, on a dry run too, as the API server's message.
func TestCRDValidatingClientRefusesWhatTheSchemaRefuses(t *testing.T) {
	cl := crdValidatingClient(t)
	app := sampleApp("web")
	app.Spec.Runtime = "cobol"
	app.Spec.Headers = []appv1alpha1.StaticHeader{{Path: "/*", Name: "X-Long", Value: strings.Repeat("v", 4097)}}
	err := cl.Create(context.Background(), app, client.DryRunAll)
	if !apierrors.IsInvalid(err) {
		t.Fatalf("create = %v, want the API server's Invalid refusal", err)
	}
	mapped, ok := core.InvalidFieldsError(err)
	if !ok {
		t.Fatalf("core.InvalidFieldsError did not read %v", err)
	}
	for _, want := range []string{
		`spec.runtime: Unsupported value: "cobol"`,
		"spec.headers[0].value: Too long: may not be more than 4096 bytes",
	} {
		if !strings.Contains(mapped.Error(), want) {
			t.Errorf("refusal %q does not name %q", mapped, want)
		}
	}
	app.Spec.Runtime, app.Spec.Headers = "image", nil
	if err := cl.Create(context.Background(), app); err != nil {
		t.Fatalf("a valid App was refused: %v", err)
	}
}
