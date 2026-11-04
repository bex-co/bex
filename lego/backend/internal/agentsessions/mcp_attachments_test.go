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

package agentsessions

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// callTool runs one MCP tool as who and returns the result; a protocol error
// fails the test, a tool error is the caller's to inspect.
func callTool(t *testing.T, client *mcp.ClientSession, who, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := client.CallTool(caller(who), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func toolText(res *mcp.CallToolResult, i int) string {
	if len(res.Content) <= i {
		return ""
	}
	if text, ok := res.Content[i].(*mcp.TextContent); ok {
		return text.Text
	}
	return ""
}

func mcpUpload(t *testing.T, client *mcp.ClientSession, who, name string, body []byte) Attachment {
	t.Helper()
	res := callTool(t, client, who, "upload_agent_attachment", map[string]any{
		"ownerId": "tea-a", "name": name, "contentBase64": base64.StdEncoding.EncodeToString(body),
	})
	if res.IsError {
		t.Fatalf("upload %s: %s", name, toolText(res, 0))
	}
	var a Attachment
	if err := json.Unmarshal([]byte(toolText(res, 0)), &a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestMCPUploadThenGetReturnsTheFileInline(t *testing.T) {
	svc, _, _, objects, _, _ := attachmentFixture()
	client := newMCPClient(t, caller("alice"), svc)

	png := mcpUpload(t, client, "alice", "shot.png", pngBytes)
	if png.MediaType != "image/png" || png.Size != int64(len(pngBytes)) || !bytes.Equal(objects.data["tea-a/"+png.ID], pngBytes) {
		t.Fatalf("upload = %+v, want a stored image/png of %d bytes", png, len(pngBytes))
	}
	res := callTool(t, client, "alice", "get_agent_attachment", map[string]any{"id": png.ID})
	image, ok := res.Content[len(res.Content)-1].(*mcp.ImageContent)
	if res.IsError || len(res.Content) != 2 || !ok || image.MIMEType != "image/png" || !bytes.Equal(image.Data, pngBytes) {
		t.Fatalf("get image = %+v, want metadata then the exact image", res.Content)
	}
	var meta Attachment
	if json.Unmarshal([]byte(toolText(res, 0)), &meta) != nil || meta.ID != png.ID || meta.DownloadURL == "" {
		t.Fatalf("get metadata = %s, want the attachment with a downloadUrl", toolText(res, 0))
	}

	log := mcpUpload(t, client, "alice", "build.log", []byte("error: boom\n"))
	if res := callTool(t, client, "alice", "get_agent_attachment", map[string]any{"id": log.ID}); res.IsError || toolText(res, 1) != "error: boom\n" {
		t.Fatalf("get text = %+v, want the log's text as the second block", res.Content)
	}

	// Past the inline bound only the metadata and URL come back.
	big := mcpUpload(t, client, "alice", "big.log", bytes.Repeat([]byte("x"), maxInlineTextBytes+1))
	if res := callTool(t, client, "alice", "get_agent_attachment", map[string]any{"id": big.ID}); res.IsError || len(res.Content) != 1 {
		t.Fatalf("get big.log = %d blocks, want metadata only", len(res.Content))
	}
	// An SVG is markup: it may come back as text, never as an image a client
	// would render.
	svg := mcpUpload(t, client, "alice", "logo.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>x()</script></svg>`))
	for _, c := range callTool(t, client, "alice", "get_agent_attachment", map[string]any{"id": svg.ID}).Content {
		if _, isImage := c.(*mcp.ImageContent); isImage {
			t.Fatal("an SVG came back as an image block")
		}
	}
}

func TestMCPUploadRefusals(t *testing.T) {
	svc, _, _, _, _, _ := attachmentFixture()
	client := newMCPClient(t, caller("alice"), svc)
	cases := map[string]struct {
		content string
		code    string
	}{
		"over the MCP cap": {base64.StdEncoding.EncodeToString(make([]byte, maxMCPUploadBytes+1)), "AGENT_ATTACHMENT_TOO_LARGE"},
		"not base64":       {"%%%not-base64%%%", "AGENT_ATTACHMENT_INVALID"},
		"empty":            {"", "AGENT_ATTACHMENT_INVALID"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res := callTool(t, client, "alice", "upload_agent_attachment", map[string]any{"ownerId": "tea-a", "name": "f.bin", "contentBase64": tc.content})
			if !res.IsError || !strings.HasPrefix(toolText(res, 0), tc.code) {
				t.Fatalf("upload = %q, want %s", toolText(res, 0), tc.code)
			}
		})
	}
	// Exactly the cap is accepted.
	if a := mcpUpload(t, client, "alice", "edge.bin", make([]byte, maxMCPUploadBytes)); a.Size != maxMCPUploadBytes {
		t.Fatalf("upload at the cap = %d bytes", a.Size)
	}
}

// attachmentIds on the MCP create and follow-up tools bind exactly as REST
// does: the files reach the sandbox archive of their turn.
func TestMCPSessionToolsBindAttachments(t *testing.T) {
	for _, follow := range []string{"send_agent_session_message", "steer_agent_session"} {
		t.Run(follow, func(t *testing.T) {
			svc, _, _, _, _, copier := attachmentFixture()
			client := newMCPClient(t, caller("alice"), svc)
			first := mcpUpload(t, client, "alice", "first.log", []byte("1"))
			res := callTool(t, client, "alice", "create_agent_session", map[string]any{
				"ownerId": "tea-a", "agentConfig": map[string]any{"agent": "claude", "task": "t"}, "attachmentIds": []string{first.ID},
			})
			var view View
			if res.IsError || json.Unmarshal([]byte(toolText(res, 0)), &view) != nil {
				t.Fatalf("create = %s", toolText(res, 0))
			}
			if got := copier.calls[len(copier.calls)-1]["1/"+first.ID+"-first.log"]; got != "1" {
				t.Fatalf("turn 1 archive = %v, want first.log", copier.calls[len(copier.calls)-1])
			}
			st := svc.Store.(bindingStore).fakeStore
			row := st.rows[view.ID]
			row.Phase = PhaseCompleted
			st.rows[view.ID] = row

			second := mcpUpload(t, client, "alice", "second.log", []byte("22"))
			if res := callTool(t, client, "alice", follow, map[string]any{"id": view.ID, "prompt": "and this", "attachmentIds": []string{second.ID}}); res.IsError {
				t.Fatalf("%s = %s", follow, toolText(res, 0))
			}
			if got := copier.calls[len(copier.calls)-1]["2/"+second.ID+"-second.log"]; got != "22" {
				t.Fatalf("turn 2 archive = %v, want second.log bound by %s", copier.calls[len(copier.calls)-1], follow)
			}

			list := callTool(t, client, "alice", "list_agent_session_attachments", map[string]any{"id": view.ID, "direction": "input"})
			var page listAttachmentsResult
			if list.IsError || json.Unmarshal([]byte(toolText(list, 0)), &page) != nil || len(page.Attachments) != 2 || page.Attachments[0].ID != first.ID {
				t.Fatalf("list = %s, want both inputs oldest first", toolText(list, 0))
			}
			// A bound file goes with its session.
			if res := callTool(t, client, "alice", "delete_agent_attachment", map[string]any{"id": first.ID}); !res.IsError || !strings.HasPrefix(toolText(res, 0), "AGENT_ATTACHMENT_BOUND") {
				t.Fatalf("delete bound = %q, want AGENT_ATTACHMENT_BOUND", toolText(res, 0))
			}
		})
	}
}

func TestMCPUnboundUploadsBelongToTheirUploader(t *testing.T) {
	svc, _, _, _, _, _ := attachmentFixture()
	alice := newMCPClient(t, caller("alice"), svc)
	carol := newMCPClient(t, caller("carol"), svc)
	a := mcpUpload(t, alice, "alice", "mine.log", []byte("x"))

	for _, tool := range []string{"get_agent_attachment", "delete_agent_attachment"} {
		if res := callTool(t, carol, "carol", tool, map[string]any{"id": a.ID}); !res.IsError || !strings.HasPrefix(toolText(res, 0), "AGENT_ATTACHMENT_NOT_FOUND") {
			t.Fatalf("carol %s = %q, want AGENT_ATTACHMENT_NOT_FOUND", tool, toolText(res, 0))
		}
	}
	if res := callTool(t, alice, "alice", "delete_agent_attachment", map[string]any{"id": a.ID}); res.IsError {
		t.Fatalf("alice delete = %s", toolText(res, 0))
	}
	if res := callTool(t, alice, "alice", "get_agent_attachment", map[string]any{"id": a.ID}); !res.IsError {
		t.Fatal("a deleted upload is still readable")
	}
}

// An upload at the MCP cap, with the longest name, must fit in one /mcp
// JSON-RPC request under the default BEX_MAX_BODY_BYTES (2 MiB,
// cmd/api/config.go), so a caller at the cap reaches the tool rather than a
// transport 413.
func TestMCPUploadCapFitsTheMCPBodyLimit(t *testing.T) {
	const defaultMaxBodyBytes = 2097152
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "upload_agent_attachment", "arguments": map[string]any{
			"ownerId": "tea-" + strings.Repeat("a", 20), "name": strings.Repeat("n", 255),
			"contentBase64": base64.StdEncoding.EncodeToString(make([]byte, maxMCPUploadBytes)),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > defaultMaxBodyBytes {
		t.Fatalf("an at-cap upload request is %d bytes, over the %d-byte /mcp body limit", len(body), defaultMaxBodyBytes)
	}
}
