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

package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestComposedMCPQueryKeepsExactNumbers runs the captured w4/m158 SELECTs
// against a real Postgres through the composed /mcp handler — workspace
// middleware, scope gate, exact query registration and the SDK's
// streamable-HTTP transport — and reads the numbers from the raw response
// bytes. Needs BEX_TEST_DB_URI (a throwaway database); skipped otherwise.
func TestComposedMCPQueryKeepsExactNumbers(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	workspaceStore := newFakeWSStore()
	ws := mustCreate(t, workspaceStore, "exact", "hobby", "client-1")
	db := ownedDB("exact-db", ws.ID)
	db.Spec.Plan = "free"
	db.Status = appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady, SecretName: "exact-db-app"}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "exact-db-app", Namespace: "default"},
		Data:       map[string][]byte{"uri": []byte(uri)},
	}
	base := &core.Base{Client: fakeClient(db, secret), Namespace: "default", Workspace: &apiFakeResolver{store: workspaceStore}}
	handler := NewServer(base, Deps{WorkspaceStore: workspaceStore}).mcpHTTPHandler()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithIdentity(r.Context(), core.Identity{Subject: "client-1", Method: "session"})
		handler.ServeHTTP(w, r.WithContext(ctx))
	}))
	defer ts.Close()

	call := func(sql string) map[string]json.RawMessage {
		t.Helper()
		args, _ := json.Marshal(map[string]string{"workspaceId": ws.ID, "postgresId": "exact-db", "sql": sql})
		body := `{"jsonrpc":"2.0","id":40,"method":"tools/call","params":{"name":"query_render_postgres","arguments":` + string(args) + `}}`
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		raw, _ := io.ReadAll(resp.Body)
		_, data, ok := strings.Cut(string(raw), "data: ")
		if resp.StatusCode != http.StatusOK || !ok {
			t.Fatalf("tools/call => %d %s", resp.StatusCode, raw)
		}
		var env struct {
			Result map[string]json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &env); err != nil {
			t.Fatal(err)
		}
		if string(env.Result["isError"]) == "true" {
			t.Fatalf("%s => tool error %s", sql, env.Result["content"])
		}
		var content []struct{ Text string }
		if err := json.Unmarshal(env.Result["content"], &content); err != nil || len(content) != 1 || content[0].Text != string(env.Result["structuredContent"]) {
			t.Fatalf("text block %s differs from structuredContent %s", env.Result["content"], env.Result["structuredContent"])
		}
		return env.Result
	}

	scalar := call(`SELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal`)
	if got, want := string(scalar["structuredContent"]), `"rows":[[9007199254740993,12345678901234567890.123456789]]`; !strings.Contains(got, want) {
		t.Fatalf("scalar structuredContent %s, want %s", got, want)
	}
	arrays := call(`SELECT ARRAY[9007199254740993::bigint,-9007199254740993::bigint] AS exact_array, 0.12345678901234567890123456789::numeric AS fraction, 42::bigint AS small_int, 1.25::numeric AS simple_decimal, NULL::numeric AS absent, true AS flag, decode('0001ff','hex') AS binary_control, 9007199254740993::bigint::text AS exact_text`)
	if got, want := string(arrays["structuredContent"]), `"rows":[[[9007199254740993,-9007199254740993],0.12345678901234567890123456789,42,1.25,null,true,"AAH/","9007199254740993"]]`; !strings.Contains(got, want) {
		t.Fatalf("array structuredContent %s, want %s", got, want)
	}
}
