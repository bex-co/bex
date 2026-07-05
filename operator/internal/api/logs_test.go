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
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/operator/api/v1alpha1"
)

// reqLine is a sample request-log message reused across log assertions.
const reqLine = "GET / 200"

// staticLogStream is the PodLogStream sibling of staticLogs (mcp_test.go): it
// serves each pod's canned lines then EOF, so FollowLogs ends and the SSE
// recorder completes.
func staticLogStream(lines map[string][]string) PodLogStream {
	return func(_ context.Context, _, pod, _ string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(strings.Join(lines[pod], "\n"))), nil
	}
}

// logServer wires a Server whose Core reads canned pod logs (query + follow).
func logServer(t *testing.T, logs map[string][]string, objs ...client.Object) http.Handler {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	srv := &Server{
		Core: &Core{
			Client:        cl,
			Namespace:     "default",
			PodLogs:       staticLogs(logs),
			PodLogsFollow: staticLogStream(logs),
		},
		Token: testToken,
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h
}

func TestREST_LogsEnvelopeAndFilters(t *testing.T) {
	h := logServer(t, map[string][]string{
		"web-1": {"2026-07-05T00:00:01Z first boot ok", "2026-07-05T00:00:03Z " + reqLine},
		"web-2": {"2026-07-05T00:00:02Z second boot ok"},
	}, sampleApp("web"), podFor("web", "web-1"), podFor("web", "web-2"))

	// Render {hasMore, next*Time, logs} envelope, sorted oldest-first.
	var env renderLogList
	decode(t, do(t, h, "GET", "/v1/logs?resource=web", testToken, ""), &env)
	if len(env.Logs) != 3 {
		t.Fatalf("want 3 lines, got %d: %+v", len(env.Logs), env)
	}
	if env.Logs[0].Message != "first boot ok" || env.Logs[2].Message != reqLine {
		t.Errorf("lines should be sorted oldest-first: %+v", env.Logs)
	}
	// Render-required id + type=app label; cursors always present.
	if env.Logs[0].ID == "" {
		t.Errorf("each log must carry an id: %+v", env.Logs[0])
	}
	if env.Logs[0].Labels[0].Name != "type" || env.Logs[0].Labels[0].Value != renderLogTypeApp {
		t.Errorf("first label should be type=app: %+v", env.Logs[0].Labels)
	}
	if env.NextStartTime == "" || env.NextEndTime == "" {
		t.Errorf("envelope must carry next{Start,End}Time: %+v", env)
	}

	// text search (case-insensitive).
	decode(t, do(t, h, "GET", "/v1/logs?resource=web&text=get", testToken, ""), &env)
	if len(env.Logs) != 1 || env.Logs[0].Message != reqLine {
		t.Errorf("text=get should match one line, got %+v", env.Logs)
	}

	// type=application (and its `app` alias) return application logs.
	for _, ty := range []string{"application", "app"} {
		decode(t, do(t, h, "GET", "/v1/logs?resource=web&type="+ty, testToken, ""), &env)
		if len(env.Logs) != 3 {
			t.Errorf("type=%s should return app logs, got %d", ty, len(env.Logs))
		}
	}
	// limit keeps the newest N.
	decode(t, do(t, h, "GET", "/v1/logs?resource=web&limit=1", testToken, ""), &env)
	if len(env.Logs) != 1 || env.Logs[0].Message != reqLine || !env.HasMore {
		t.Errorf("limit=1 should keep newest line + hasMore, got %+v", env)
	}
}

func TestREST_LogsRequestAndBuildAreEmpty(t *testing.T) {
	h := logServer(t, map[string][]string{"web-1": {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", "web-1"))

	// bex has no request/build source: 200 + empty (not 400/404).
	for _, ty := range []string{"request", "build"} {
		var env renderLogList
		w := do(t, h, "GET", "/v1/logs?resource=web&type="+ty, testToken, "")
		if w.Code != 200 {
			t.Fatalf("type=%s => 200, got %d", ty, w.Code)
		}
		decode(t, w, &env)
		if len(env.Logs) != 0 {
			t.Errorf("type=%s should be empty, got %d", ty, len(env.Logs))
		}
	}
}

func TestREST_LogsErrors(t *testing.T) {
	h := logServer(t, map[string][]string{"web-1": {"2026-07-05T00:00:01Z hi"}},
		sampleApp("web"), podFor("web", "web-1"))

	if code := do(t, h, "GET", "/v1/logs", testToken, "").Code; code != 400 {
		t.Errorf("missing resource => 400, got %d", code)
	}
	if code := do(t, h, "GET", "/v1/logs?resource=nope", testToken, "").Code; code != 404 {
		t.Errorf("unknown app => 404, got %d", code)
	}
	if code := do(t, h, "GET", "/v1/logs?resource=web&type=bogus", testToken, "").Code; code != 400 {
		t.Errorf("bad type => 400, got %d", code)
	}
	if code := do(t, h, "GET", "/v1/logs?resource=web", "", "").Code; code != 401 {
		t.Errorf("no token => 401, got %d", code)
	}
}

func TestREST_LogsUnavailableWithoutSource(t *testing.T) {
	// A Core with no PodLogs wired => 503, not a 500/404.
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(sampleApp("web")).Build()
	srv := &Server{Core: &Core{Client: cl, Namespace: "default"}, Token: testToken}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if code := do(t, h, "GET", "/v1/logs?resource=web", testToken, "").Code; code != 503 {
		t.Errorf("no source => 503, got %d", code)
	}
}

func TestREST_LogsSubscribeStreamsSSE(t *testing.T) {
	h := logServer(t, map[string][]string{
		"web-1": {"2026-07-05T00:00:01Z live one", "2026-07-05T00:00:02Z live two"},
	}, sampleApp("web"), podFor("web", "web-1"))

	w := do(t, h, "GET", "/v1/logs/subscribe?resource=web", testToken, "")
	if w.Code != 200 {
		t.Fatalf("subscribe => 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected SSE content-type, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "data: ") || !strings.Contains(body, "live one") || !strings.Contains(body, "live two") {
		t.Errorf("SSE body missing streamed lines: %q", body)
	}
}

func TestGraphQL_Logs(t *testing.T) {
	h := logServer(t, map[string][]string{"web-1": {"2026-07-05T00:00:01Z hello"}},
		sampleApp("web"), podFor("web", "web-1"))

	data := gql(t, h, `{ logs(resource:"web") { message type instance } }`)
	logs := data["logs"].([]any)
	if len(logs) != 1 {
		t.Fatalf("want 1 log, got %d", len(logs))
	}
	first := logs[0].(map[string]any)
	if first["message"] != "hello" || first["type"] != renderLogTypeApp || first["instance"] != "web-1" {
		t.Fatalf("unexpected log shape: %+v", first)
	}
}
