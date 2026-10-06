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
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
)

func TestDisplayNameValidationRefusesAcrossSurfacesWithoutMutation(t *testing.T) {
	const control = "DISPLAY_NAME_CONTROL_CHARACTER"
	const tooLong = "DISPLAY_NAME_TOO_LONG"
	const reserved = "NAME_RESOURCE_ID_RESERVED" // shared with datastore names (w5/m118)
	otherID := id.New(id.Service)
	messages := map[string]string{
		control: "display name must not contain control characters or line separators",
		tooLong: "display name must be at most 100 Unicode code points",
	}
	for _, surface := range []string{"service", "REST name", "REST displayName", "GraphQL", "MCP"} {
		t.Run(surface, func(t *testing.T) {
			rec := &recordingStore{}
			sink := &captureAuditSink{}
			svc, cl := newService(rec, manage(displayNameApp("web"), id.New(id.Service)), manage(displayNameApp("other"), otherID))
			svc.Audit = sink
			before := getApp(t, cl, "web")
			call := displayNameRejectionCall(t, svc, surface)
			for _, tc := range []struct{ name, value, code string }{
				{"interior newline", "first\nsecond", control},
				{"edge newline", "\nlabel\n", control},
				{"edge carriage return", "\rlabel\r", control},
				{"edge tab", "\tlabel\t", control},
				{"control only cannot clear", "\n\t\r", control},
				{"nul", "label\x00", control},
				{"escape", "\x1b[31mlabel", control},
				{"delete", "label\x7f", control},
				{"C1 control", "label\u0085", control},
				{"line separator", "\u2028label", control},
				{"paragraph separator", "label\u2029", control},
				{"101 ASCII code points", strings.Repeat("a", 101), tooLong},
				{"101 multibyte code points", strings.Repeat("界", 101), tooLong},
				{"101 combining code points", strings.Repeat("e\u0301", 50) + "e", tooLong},
				{"another service ID", otherID, reserved},
				{"trimmed ID", "  " + otherID + "  ", reserved},
				{"wide suffix lookalike", "srv-" + strings.Repeat("z", 20), reserved},
				{"public environment ID", id.New(id.Environment), reserved},
				{"legacy environment ID", id.EnvironmentStorageID(id.New(id.Environment)), reserved},
				{"legacy environment lookalike", "env-" + strings.Repeat("z", 20), reserved},
			} {
				t.Run(tc.name, func(t *testing.T) {
					code, message := call(t, tc.value)
					want := messages[tc.code]
					if tc.code == reserved {
						want = fmt.Sprintf("%q must not look like a resource ID", strings.TrimSpace(tc.value))
					}
					if code != tc.code || message != want {
						t.Fatalf("error = %q/%q, want %q/%q", code, message, tc.code, want)
					}
					if !reflect.DeepEqual(before, getApp(t, cl, "web")) {
						t.Fatal("refused display name changed the App")
					}
					if !reflect.DeepEqual(*rec, recordingStore{}) || len(sink.events) != 0 {
						t.Fatalf("refused display name changed store, deploy history, or events: store=%+v events=%+v", rec, sink.events)
					}
				})
			}
		})
	}
}

func displayNameRejectionCall(t *testing.T, svc *Service, surface string) func(*testing.T, string) (string, string) {
	t.Helper()
	ctx := context.Background()
	switch surface {
	case "service":
		return func(t *testing.T, value string) (string, string) {
			t.Helper()
			_, err := svc.SetDisplayName(ctx, "web", value)
			var coded *core.CodedError
			if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) {
				t.Fatalf("rename error = %v, want coded bad request", err)
			}
			return coded.Code, coded.Error()
		}
	case "REST name", "REST displayName":
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		return func(t *testing.T, value string) (string, string) {
			t.Helper()
			// A later runtime setting must stay untouched when the label fails.
			body, err := json.Marshal(map[string]any{strings.TrimPrefix(surface, "REST "): value, "startCommand": "new-command"})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPatch, "/v1/services/web", strings.NewReader(string(body)))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			var out struct{ Code, Message, Error string }
			if rec.Code != http.StatusBadRequest || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Message != out.Error {
				t.Fatalf("REST refusal = %d %s", rec.Code, rec.Body)
			}
			return out.Code, out.Message
		}
	case "GraphQL":
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		return func(t *testing.T, value string) (string, string) {
			t.Helper()
			res := graphql.Do(graphql.Params{
				Schema: schema, Context: ctx,
				RequestString:  `mutation($value: String!) { setDisplayName(id: "web", displayName: $value) { id } }`,
				VariableValues: map[string]any{"value": value},
			})
			if len(res.Errors) != 1 {
				t.Fatalf("GraphQL refusal = %+v", res)
			}
			code, _ := res.Errors[0].Extensions["code"].(string)
			return code, res.Errors[0].Message
		}
	case "MCP":
		session := displayNameMCPSession(t, svc)
		return func(t *testing.T, value string) (string, string) {
			t.Helper()
			res, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name: "update_service", Arguments: map[string]any{"serviceId": "web", "displayName": value, "startCommand": "new-command"},
			})
			if err != nil || res == nil || !res.IsError || len(res.Content) != 1 {
				t.Fatalf("MCP refusal = %+v, err=%v", res, err)
			}
			content, ok := res.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("MCP error content = %+v", res.Content)
			}
			code, message, _ := strings.Cut(content.Text, ": ")
			return code, message
		}
	default:
		t.Fatalf("unknown surface %q", surface)
		return nil
	}
}

func displayNameMCPSession(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "display-name-test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatal(err)
	}
	session, err := mcp.NewClient(&mcp.Implementation{Name: "display-name-test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestSetDisplayNameValidationAllowsUnicodeTrimClearAndOrdinaryNames(t *testing.T) {
	for _, tc := range []struct{ name, value, want string }{
		{"100 ASCII code points", strings.Repeat("a", 100), strings.Repeat("a", 100)},
		{"100 multibyte code points", strings.Repeat("界", 100), strings.Repeat("界", 100)},
		{"100 combining code points", strings.Repeat("e\u0301", 50), strings.Repeat("e\u0301", 50)},
		{"trim before length limit", "  " + strings.Repeat("界", 100) + "  ", strings.Repeat("界", 100)},
		{"Unicode name", "Café 東京 👩‍💻", "Café 東京 👩‍💻"},
		{"trim spaces", "  Friendly API  ", "Friendly API"},
		{"trim Unicode spaces", "\u00a0\u2003Friendly API\u2003\u00a0", "Friendly API"},
		{"clear", "", ""},
		{"spaces clear", "   ", ""},
		{"unknown prefix", "zzz-" + strings.Repeat("a", 20), "zzz-" + strings.Repeat("a", 20)},
		{"known prefix ordinary name", "srv-customer-api", "srv-customer-api"},
		{"short suffix", "srv-" + strings.Repeat("a", 19), "srv-" + strings.Repeat("a", 19)},
		{"long suffix", "srv-" + strings.Repeat("a", 21), "srv-" + strings.Repeat("a", 21)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appID := id.New(id.Service)
			app := manage(displayNameApp("web"), appID)
			app.Generation = 7
			app.Status.ReleaseGeneration = 5
			app.Status.ActiveRevision = "rev-5"
			rec := &recordingStore{}
			sink := &captureAuditSink{}
			svc, cl := newService(rec, app)
			svc.Audit = sink
			before := getApp(t, cl, "web")
			view, err := svc.SetDisplayName(context.Background(), "web", tc.value)
			if err != nil || view.DisplayName != tc.want || view.ID != appID {
				t.Fatalf("rename = %+v, err=%v", view, err)
			}
			after := getApp(t, cl, "web")
			if after.Spec.DisplayName != tc.want || len(rec.displayNameCalls) != 1 || rec.displayNameCalls[0].displayName != tc.want || rec.displayNameCalls[0].id != appID {
				t.Fatalf("stored display name = %q, row writes=%+v", after.Spec.DisplayName, rec.displayNameCalls)
			}
			after.Spec.DisplayName = before.Spec.DisplayName
			if after.Name != before.Name || !reflect.DeepEqual(after.Spec, before.Spec) || !reflect.DeepEqual(after.Status, before.Status) ||
				!reflect.DeepEqual(after.Labels, before.Labels) || len(rec.deployCalls) != 0 {
				t.Fatal("rename changed immutable identity, routing, release, or deploy history")
			}
			if len(sink.events) != 1 || sink.countVerb(core.AuditVerbSetDisplayName) != 1 {
				t.Fatalf("successful rename events = %+v", sink.events)
			}
		})
	}
}
