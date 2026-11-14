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

package rendertest

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/api/openapi"
)

// mcpresult.go checks a bex MCP answer against the result shape Render's own
// MCP server gives the same tool (openapi.RenderMCPResults, w4/m378): the
// text's form — an object, a bare array, an array plus Render's cursor suffix,
// or Render's sentence for an empty list — and, for every object in it, that
// Render's always-present keys are there and every other key is one Render's
// type declares or one ADR018 records as a bex extension.

// MCPResults is the parsed capture.
type MCPResults struct {
	Source struct {
		Commit string `json:"commit"`
	} `json:"source"`
	Tools map[string]MCPResultForm `json:"tools"`
	Types map[string]MCPResultType `json:"types"`
}

// MCPResultForm is one tool's answer shape.
type MCPResultForm struct {
	Form      string `json:"form"` // object | array | arrayCursor | text
	Type      string `json:"type,omitempty"`
	EmptyText string `json:"emptyText,omitempty"`
}

// MCPResultType is one Render Go type's JSON keys, and which fields hold
// another recorded type.
type MCPResultType struct {
	Required []string          `json:"required"`
	Optional []string          `json:"optional"`
	Fields   map[string]string `json:"fields,omitempty"`
}

// MCPResultExtensions are the keys bex adds to a Render type on MCP, as REST
// does: the additive extensions docs/ADR018-render-parity.md § "MCP parity is
// pinned and measured" records (bex fields with no Render counterpart, each
// already returned by the REST object the tool shares). A key not here and
// not in Render's type fails AssertRenderMCPResult; add one only with its
// ADR018 record.
var MCPResultExtensions = map[string][]string{
	"Deploy": {"bexTrigger", "cancelReason", "failureReason", "preDeployStatus", "rollbackOf", "serviceId", "stallReason"},
	"KeyValue": {"blueprintId", "externalHost", "internalAuth", "projectId", "public", "statusReason", "statusReasonCode",
		"suspended"},
	"KeyValueDetail": {"blueprintId", "dashboardUrl", "externalHost", "internalAuth", "projectId", "public", "statusReason",
		"statusReasonCode", "suspended"},
	"Postgres": {"backupsEnabled", "blueprintId", "externalHost", "ownerId", "poolerEnabled", "projectId", "public",
		"statusReason", "statusReasonCode"},
	"PostgresDetail": {"backupsEnabled", "blueprintId", "externalHost", "ownerId", "poolerEnabled", "projectId", "public",
		"statusReason", "statusReasonCode"},
	"Service": {"autoscaling", "blueprintId", "command", "displayName", "healthCheckPath", "idleTTLSeconds", "immutableName",
		"lastSuccessfulRunAt", "linkedEnvGroupIds", "nextRunAt", "notificationsToSend", "phase", "projectId",
		"publicRoutingNotice", "pushDeliveryMethod", "registryCredentialId", "replicas", "revision", "runs", "schedule",
		"servingImage", "undeployedChanges", "undeployedChangesApplying", "undeployedChangesReason", "urls"},
}

var (
	mcpResultsOnce sync.Once
	mcpResults     MCPResults
	mcpResultsErr  error
)

// LoadMCPResults parses the embedded capture once.
func LoadMCPResults() (MCPResults, error) {
	mcpResultsOnce.Do(func() {
		mcpResultsErr = json.Unmarshal(openapi.RenderMCPResults, &mcpResults)
		if mcpResultsErr == nil && (len(mcpResults.Tools) == 0 || mcpResults.Source.Commit == "") {
			mcpResultsErr = fmt.Errorf("render-mcp-results.json records no tools or no upstream commit")
		}
	})
	return mcpResults, mcpResultsErr
}

// AssertRenderMCPResult fails t unless res is the answer Render's MCP server
// shapes for tool. fixtureLacks names always-present keys the test's fake
// cluster cannot fill (the fake API server mints no creationTimestamp, so a
// fresh object has no createdAt); they may be absent, never renamed.
func AssertRenderMCPResult(t testing.TB, tool string, res *mcp.CallToolResult, fixtureLacks ...string) {
	t.Helper()
	if err := CheckRenderMCPResult(tool, res, fixtureLacks...); err != nil {
		t.Errorf("%s: %v", tool, err)
	}
}

// CheckRenderMCPResult is AssertRenderMCPResult's verdict as an error.
func CheckRenderMCPResult(tool string, res *mcp.CallToolResult, fixtureLacks ...string) error {
	pin, err := LoadMCPResults()
	if err != nil {
		return err
	}
	spec, ok := pin.Tools[tool]
	if !ok {
		return fmt.Errorf("not a tool Render's MCP server shares with bex (render-mcp-results.json)")
	}
	if res == nil || res.IsError || len(res.Content) == 0 {
		return fmt.Errorf("want a successful answer with content, got %+v", res)
	}
	first, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		return fmt.Errorf("first content is %T, want text", res.Content[0])
	}
	text := first.Text
	if spec.EmptyText != "" && text == spec.EmptyText {
		return nil
	}
	c := resultChecker{pin: pin, lacks: fixtureLacks}
	switch spec.Form {
	case "text":
		if text == "" || strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
			return fmt.Errorf("text %q, want Render's sentence", clip(text))
		}
		return nil
	case "object":
		return c.checkValue(spec.Type, json.RawMessage(text), "$")
	case "array":
		return c.checkArray(spec.Type, json.RawMessage(text), "$")
	case "arrayCursor":
		body, _, ok := strings.Cut(text, MCPCursorMarker)
		if !ok {
			return fmt.Errorf("text %q lacks Render's %q suffix", clip(text), MCPCursorMarker)
		}
		return c.checkArray(spec.Type, json.RawMessage(body), "$")
	}
	return fmt.Errorf("unknown recorded form %q", spec.Form)
}

// resultChecker walks one answer against the capture.
type resultChecker struct {
	pin   MCPResults
	lacks []string // always-present keys the test fixture cannot fill
}

func (c resultChecker) checkArray(typ string, raw json.RawMessage, path string) error {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil || items == nil {
		return fmt.Errorf("%s = %s, want a JSON array (Render's bare list, never an object wrapping one)", path, clip(string(raw)))
	}
	for i, item := range items {
		if err := c.checkValue(typ, item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
			return err
		}
	}
	return nil
}

func (c resultChecker) checkValue(typ string, raw json.RawMessage, path string) error {
	switch typ {
	case "":
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
			return fmt.Errorf("%s = %s, want a JSON object", path, clip(string(raw)))
		}
		return nil
	case "string":
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return fmt.Errorf("%s = %s, want a string", path, clip(string(raw)))
		}
		return nil
	}
	shape, ok := c.pin.Types[typ]
	if !ok {
		return fmt.Errorf("%s: type %s is not recorded", path, typ)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return fmt.Errorf("%s = %s, want Render's %s object", path, clip(string(raw)), typ)
	}
	for _, key := range shape.Required {
		if _, ok := obj[key]; !ok && !slices.Contains(c.lacks, key) {
			return fmt.Errorf("%s lacks %q, which Render's %s always carries (keys: %v)", path, key, typ, keysOf(obj))
		}
	}
	var unknown []string
	for _, key := range keysOf(obj) {
		if !slices.Contains(shape.Required, key) && !slices.Contains(shape.Optional, key) && !slices.Contains(MCPResultExtensions[typ], key) {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("%s carries %q, which Render's %s does not have and ADR018 records as no bex extension", path, unknown, typ)
	}
	for field, inner := range shape.Fields {
		value, ok := obj[field]
		if !ok || string(value) == "null" {
			continue
		}
		var err error
		if strings.HasPrefix(strings.TrimSpace(string(value)), "[") {
			err = c.checkArray(inner, value, path+"."+field)
		} else {
			err = c.checkValue(inner, value, path+"."+field)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func keysOf(obj map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func clip(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
