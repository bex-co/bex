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
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestListServicesAcceptsIncludePreviews is w4/660: Render's list_services
// takes includePreviews. bex has no preview services, so the flag is accepted
// either way and the list is the same as with no arguments.
func TestListServicesAcceptsIncludePreviews(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"), sampleApp("api"))
	session := displayNameMCPSession(t, svc)
	text := func(args map[string]any) string {
		t.Helper()
		res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_services", Arguments: args})
		if err != nil || res.IsError || len(res.Content) != 1 {
			t.Fatalf("list_services %v: err=%v result=%+v", args, err, res)
		}
		return res.Content[0].(*mcp.TextContent).Text
	}
	want := text(nil)
	for _, args := range []map[string]any{{"includePreviews": true}, {"includePreviews": false}} {
		if got := text(args); got != want {
			t.Errorf("list_services %v = %s, want the list with no arguments: %s", args, got, want)
		}
	}
}
