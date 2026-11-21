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

package deploys

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w8/378: Render's Trigger deploy answers 201 with the deploy it started and
// 202 Queued, with no body, for one that waits behind the deploy in progress.
// The pinned CLI then finds the queued deploy with ?status=queued&limit=1.
func TestRESTTriggerAnswersRenders202WhenItsDeployWaits(t *testing.T) {
	ds, svc, _ := imageServiceRollingOut(t)
	do := newRESTHarness(t, svc)

	rec := do("POST", "/v1/services/web/deploys")
	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Fatalf("trigger behind a deploy in progress: code=%d body=%q, want 202 with no body", rec.Code, rec.Body.String())
	}
	queued := decodeList(t, do("GET", "/v1/services/web/deploys?status=queued&limit=1"))
	if len(queued) != 1 || queued[0].Deploy.Status != store.DeployQueued || queued[0].Deploy.Trigger != store.TriggerAPI {
		t.Fatalf("queued deploys = %+v, want the trigger's deploy", queued)
	}

	settle(t, ds, svc, "dep-flight")
	settle(t, ds, svc, queued[0].Deploy.ID)
	rec = do("POST", "/v1/services/web/deploys")
	var started renderDeploy
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil || rec.Code != http.StatusCreated || started.ID == "" || started.Status == store.DeployQueued {
		t.Fatalf("trigger on an idle service: code=%d body=%s, want 201 with the started deploy", rec.Code, rec.Body.String())
	}
}

// Render's MCP trigger_deploy answers a sentence when the API queued the
// deploy (render-mcp-server pkg/deploy/tools.go); structuredContent keeps the
// queued row.
func TestMCPTriggerAnswersRendersSentenceWhenItsDeployWaits(t *testing.T) {
	_, svc, _ := imageServiceRollingOut(t)
	res, err := newMCPSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{
		Name: "trigger_deploy", Arguments: map[string]any{"serviceId": "web", "imageUrl": "web:v3"},
	})
	if err != nil || res.IsError {
		t.Fatalf("trigger_deploy: err=%v result=%+v", err, res)
	}
	if got := rendertest.MCPText(t, res); got != queuedTriggerText {
		t.Fatalf("text = %q, want Render's queued sentence", got)
	}
	rendertest.AssertRenderMCPResult(t, "trigger_deploy", res)
	var row renderDeploy
	if err := decodeStructured(res.StructuredContent, &row); err != nil || row.ID == "" || row.Status != store.DeployQueued {
		t.Fatalf("structuredContent = %+v (err %v), want the queued deploy", res.StructuredContent, err)
	}
}
