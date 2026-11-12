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

package sandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w32/m297: Render's create-time env reaches the sandbox and no response
// carries a value back, even when OpenSandbox echoes it.
func TestCreateInjectsEnvAndNeverEchoesIt(t *testing.T) {
	const secret = "s3cr3t-value"
	var got createRequest
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode create body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = fmt.Fprintf(w, `{"id":"os-1","status":{"state":"Running"},"env":{"API_KEY":%q}}`, secret)
	})
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	body := fmt.Sprintf(`{"plan":"starter","template":"node","env":{"API_KEY":%q,"lower.case-name":"x","EMPTY":""}}`, secret)
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(body)).WithContext(callerCtx()))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	want := map[string]string{"API_KEY": secret, "lower.case-name": "x", "EMPTY": ""}
	if len(got.Env) != len(want) {
		t.Fatalf("OpenSandbox env = %v, want %v", got.Env, want)
	}
	for k, v := range want {
		if got.Env[k] != v {
			t.Errorf("OpenSandbox env[%q] = %q, want %q", k, got.Env[k], v)
		}
	}
	if strings.Contains(rec.Body.String(), secret) || strings.Contains(rec.Body.String(), `"env"`) {
		t.Errorf("create response carries the env: %s", rec.Body.String())
	}
}

func TestValidateSandboxEnv(t *testing.T) {
	many := func(n int) map[string]string {
		env := make(map[string]string, n)
		for i := range n {
			env[fmt.Sprintf("V%d", i)] = "1"
		}
		return env
	}
	// spread fills total bytes across names and values in entries well under
	// execve's per-entry limit.
	spread := func(total int) map[string]string {
		env := map[string]string{}
		for i := 0; total > 0; i++ {
			name := fmt.Sprintf("V%02d", i)
			n := min(total-len(name), 100_000)
			env[name] = strings.Repeat("a", n)
			total -= len(name) + n
		}
		return env
	}
	for _, tc := range []struct {
		name    string
		env     map[string]string
		wantErr string // a substring of the message; empty means accepted
	}{
		{"none", nil, ""},
		{"empty map (the SDK sends env:{} for env={})", map[string]string{}, ""},
		{"Render's 100 variables", many(100), ""},
		{"101 variables", many(101), "at most 100"},
		{"256 KB across names and values", spread(256 * 1024), ""},
		{"one byte over 256 KB", spread(256*1024 + 1), "the limit is 262144"},
		{"empty name", map[string]string{"": "1"}, "invalid environment variable name"},
		{"equals sign", map[string]string{"A=B": "1"}, `invalid environment variable name "A=B"`},
		{"dotted name, as Render's templates use", map[string]string{"discovery.type": "single-node"}, ""},
		{"NUL in a value", map[string]string{"BIN": "a\x00b"}, "NUL byte"},
		{"one entry over execve's limit", map[string]string{"HUGE": strings.Repeat("a", 131072)}, "too large"},
		{"bex prefix", map[string]string{"BEX_AGENT_MODEL_API_KEY": "x"}, "reserved"},
		{"bex prefix, any case", map[string]string{"bex_x": "x"}, "reserved"},
		{"OpenSandbox exec daemon", map[string]string{"EXECD_TOKEN": "x"}, "reserved"},
		{"OpenSandbox", map[string]string{"OPENSANDBOX_X": "x"}, "reserved"},
		{"a name merely containing BEX", map[string]string{"MY_BEX_URL": "x"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSandboxEnv(tc.env)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want one naming %q", err, tc.wantErr)
			}
			if !errors.Is(err, core.ErrBadRequest) {
				t.Errorf("err = %v is not a 400", err)
			}
			for _, v := range tc.env {
				if len(v) > 1 && strings.Contains(err.Error(), v) {
					t.Errorf("refusal leaks a value: %v", err)
				}
			}
		})
	}
}

// GraphQL's `[{key, value}]` form adds two refusals of its own: a generated
// value and a repeated key.
func TestGraphQLEnvArgs(t *testing.T) {
	env, err := envFromArgs([]any{
		map[string]any{"key": "A", "value": "1"},
		map[string]any{"key": "B"},
	})
	if err != nil || env["A"] != "1" || env["B"] != "" || len(env) != 2 {
		t.Fatalf("env = %v, %v", env, err)
	}
	if env, err := envFromArgs(nil); env != nil || err != nil {
		t.Fatalf("omitted env = %v, %v", env, err)
	}
	for name, raw := range map[string][]any{
		"generateValue": {map[string]any{"key": "A", "generateValue": true}},
		"repeated key":  {map[string]any{"key": "A", "value": "1"}, map[string]any{"key": "A", "value": "2"}},
	} {
		if _, err := envFromArgs(raw); err == nil || !strings.Contains(err.Error(), `"A"`) {
			t.Errorf("%s: err = %v, want a refusal naming A", name, err)
		}
	}
}

// w32/019: a template's own environment (the base template's HOME, Render's
// /home/render) reaches the sandbox under the caller's env, and a caller's
// variable of the same name wins.
func TestCreateMergesTheTemplateEnvUnderTheCallers(t *testing.T) {
	var got createRequest
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode create body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"os-1","status":{"state":"Running"}}`))
	})
	tmpl := svc.Templates["node"]
	tmpl.Env = map[string]string{"HOME": "/home/render", "LANG": "C.UTF-8"}
	svc.Templates["node"] = tmpl
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	for body, want := range map[string]map[string]string{
		`{"template":"node"}`:                               {"HOME": "/home/render", "LANG": "C.UTF-8"},
		`{"template":"node","env":{"HOME":"/srv","X":"1"}}`: {"HOME": "/srv", "LANG": "C.UTF-8", "X": "1"},
	} {
		got = createRequest{}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/sandboxes", strings.NewReader(body)).WithContext(callerCtx()))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s = %d: %s", body, rec.Code, rec.Body.String())
		}
		if !maps.Equal(got.Env, want) {
			t.Errorf("create %s: OpenSandbox env = %v, want %v", body, got.Env, want)
		}
	}
	if svc.Templates["node"].Env["X"] != "" {
		t.Error("a caller's env leaked into the shared template")
	}
}
