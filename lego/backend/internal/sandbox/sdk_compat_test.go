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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// renderSandboxSchemas loads Render's pinned sandbox schema (the create, get
// and list item body) and its list item, which the SDK's generated models
// decode every sandbox read against. The id keeps bex's 20-character sbx- ids
// beside Render's 25, as the request gate widens it (renderSandboxIDPattern,
// ADR018 round 5); neither SDK checks the id's shape.
func renderSandboxSchemas(t *testing.T) (sandbox, listItem *openapi3.Schema) {
	t.Helper()
	spec, err := openapi3.NewLoader().LoadFromFile("../api/openapi/render-public-api-1.json")
	if err != nil {
		t.Fatal(err)
	}
	idSchema := openapi3.NewStringSchema().WithPattern(`^sbx-[0-9a-v]{20}(?:[0-9a-v]{5})?$`).NewRef()
	widenID := func(sc *openapi3.Schema) {
		if sc == nil {
			return
		}
		if _, ok := sc.Properties["id"]; ok {
			sc.Properties["id"] = idSchema
		}
		for _, sub := range sc.AllOf {
			if sub != nil && sub.Value != nil {
				if _, ok := sub.Value.Properties["id"]; ok {
					sub.Value.Properties["id"] = idSchema
				}
			}
		}
	}
	sandbox = spec.Paths.Find("/sandboxes").Post.Responses.Status(http.StatusCreated).Value.Content.Get("application/json").Schema.Value
	widenID(sandbox)
	item := spec.Components.Schemas["sandboxWithCursor"].Value
	if s := item.Properties["sandbox"]; s != nil {
		widenID(s.Value)
	}
	return sandbox, item
}

// The Render SDK v1.8 Python client validates every sandbox it reads against
// Render's schema, whose required fields include region and createdAt, and
// cursor on each list item (w32/m284). A response missing one made create, get
// and list raise even though the call succeeded. The production walk of
// scripts/render-sdk-compat.sh (t002) decoded every read; this pins it against
// the pinned schema, so a field the next pin makes required fails here.
func TestSandboxReadsCarryRenderRequiredFields(t *testing.T) {
	up := &policyUpstream{}
	svc := stubServer(t, up.handler(t))
	svc.Region = "frankfurt"
	svc.Names = &memNames{}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	sandboxSchema, itemSchema := renderSandboxSchemas(t)
	conforms := func(step string, schema *openapi3.Schema, doc any) {
		t.Helper()
		if err := schema.VisitJSON(doc); err != nil {
			t.Errorf("%s is not Render's shape: %v", step, err)
		}
	}
	read := func(method, path, body string, status int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(callerCtx()))
		if rec.Code != status {
			t.Fatalf("%s %s = %d %s", method, path, rec.Code, rec.Body)
		}
		return rec.Body.Bytes()
	}

	var created map[string]any
	_ = json.Unmarshal(read(http.MethodPost, "/v1/sandboxes", `{"template":"node","name":"sdk","networkPolicy":{"type":"deny-all"}}`, http.StatusCreated), &created)
	if created["region"] != "frankfurt" || created["createdAt"] == nil {
		t.Fatalf("create body lacks region/createdAt: %v", created)
	}
	conforms("create", sandboxSchema, created)
	id, _ := created["id"].(string)

	var got map[string]any
	_ = json.Unmarshal(read(http.MethodGet, "/v1/sandboxes/"+id, "", http.StatusOK), &got)
	conforms("get", sandboxSchema, got)

	var items []map[string]any
	if err := json.Unmarshal(read(http.MethodGet, "/v1/sandboxes", "", http.StatusOK), &items); err != nil || len(items) != 1 {
		t.Fatalf("list = %v (%v)", items, err)
	}
	conforms("list item", itemSchema, items[0])
	sb, _ := items[0]["sandbox"].(map[string]any)
	if items[0]["cursor"] != id || sb["region"] != "frankfurt" {
		t.Fatalf("list item = %v, want cursor = id and the platform region", items[0])
	}

	// The check is real: a read without a required field is refused.
	delete(got, "createdAt")
	if sandboxSchema.VisitJSON(got) == nil {
		t.Fatal("a sandbox without createdAt validated; the schema check is vacuous")
	}
}

// A page is cut only when asked for; the cursor resumes after that sandbox in
// the newest-first order (equal creation times fall back to the id).
func TestSandboxListPagesOnlyWhenAsked(t *testing.T) {
	var rows []string
	for i := 0; i < 3; i++ {
		rows = append(rows, fmt.Sprintf(`{"id":"os-%d","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox","bex.co/sandbox-id":"sbx-%d"},"status":{"state":"Running"}}`, i, i))
	}
	svc := stubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
	})
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	ids := func(query string) []string {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sandboxes"+query, nil).WithContext(callerCtx()))
		var items []SandboxEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
			t.Fatalf("%s: %d %s", query, rec.Code, rec.Body)
		}
		var out []string
		for _, it := range items {
			out = append(out, it.Cursor)
		}
		return out
	}
	if got := ids(""); len(got) != 3 {
		t.Fatalf("unpaged list = %v, want all three", got)
	}
	if got := ids("?limit=2"); strings.Join(got, ",") != "sbx-2,sbx-1" {
		t.Fatalf("first page = %v", got)
	}
	if got := ids("?limit=2&cursor=sbx-1"); strings.Join(got, ",") != "sbx-0" {
		t.Fatalf("second page = %v", got)
	}
}

// memNames is an in-memory NameStore.
type memNames struct {
	mu    sync.Mutex
	names map[string]string
}

func (m *memNames) SetSandboxName(_ context.Context, _, id, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.names == nil {
		m.names = map[string]string{}
	}
	m.names[id] = name
	return nil
}

func (m *memNames) DeleteSandboxName(_ context.Context, _, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.names, id)
	return nil
}

func (m *memNames) SandboxNames(context.Context, string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]string{}
	for k, v := range m.names {
		out[k] = v
	}
	return out, nil
}

func (m *memNames) PruneSandboxNames(context.Context, string, []string) error { return nil }

// w5/m287: Render CLI v2.36 `sandboxes create --name` and `list --name` —
// a named create answers its name, a list shows names (null when unnamed)
// and filters on an exact one, and Render's rule refuses a bad name before
// anything is created.
func TestSandboxNames(t *testing.T) {
	var mu sync.Mutex
	var rows []string
	failCreate := false
	svc := stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPost {
			if failCreate {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			var body createRequest
			_ = json.NewDecoder(r.Body).Decode(&body)
			id := body.Metadata[metadataPublicID]
			rows = append(rows, fmt.Sprintf(`{"id":"os-%d","metadata":{"bex.co/owner":"id-a","bex.co/workspace":"tea-a","bex.co/network-policy":"deny-all","app.bex.co/regime":"sandbox","bex.co/sandbox-id":%q},"status":{"state":"Running"}}`, len(rows), id))
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"os-x","status":{"state":"Running"}}`))
			return
		}
		_, _ = w.Write([]byte("[" + strings.Join(rows, ",") + "]"))
	})
	svc.DefaultTemplate = "node"
	names := &memNames{}
	svc.Names = names
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	do := func(method, path, body string) (int, string) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(callerCtx()))
		return rec.Code, rec.Body.String()
	}

	for _, bad := range []string{strings.Repeat("a", 65), "sbx-mine"} {
		if code, body := do(http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"name":%q}`, bad)); code != http.StatusBadRequest || !strings.Contains(body, CodeInvalidSandboxName) {
			t.Errorf("name %q: %d %s, want invalid_sandbox_name", bad, code, body)
		}
	}
	if len(rows) != 0 {
		t.Fatal("a refused name reached OpenSandbox")
	}
	code, body := do(http.MethodPost, "/v1/sandboxes", `{"name":"my agent ✨"}`)
	var created Sandbox
	if err := json.Unmarshal([]byte(body), &created); code != http.StatusCreated || err != nil || created.Name == nil || *created.Name != "my agent ✨" {
		t.Fatalf("named create = %d %s", code, body)
	}
	if code, body = do(http.MethodPost, "/v1/sandboxes", `{}`); code != http.StatusCreated || !strings.Contains(body, `"name":null`) {
		t.Fatalf("unnamed create = %d %s, want name null", code, body)
	}
	if _, body = do(http.MethodPost, "/v1/sandboxes", `{"name":"my agent ✨"}`); !strings.Contains(body, `"name":"my agent ✨"`) {
		t.Fatalf("second same-named create = %s, want accepted (names are not unique)", body)
	}

	list := func(query string) []SandboxEnvelope {
		t.Helper()
		code, body := do(http.MethodGet, "/v1/sandboxes"+query, "")
		var items []SandboxEnvelope
		if err := json.Unmarshal([]byte(body), &items); code != http.StatusOK || err != nil {
			t.Fatalf("list %s = %d %s", query, code, body)
		}
		return items
	}
	if got := list(""); len(got) != 3 {
		t.Fatalf("list = %d sandboxes, want 3", len(got))
	}
	named := list("?name=" + url.QueryEscape("my agent ✨"))
	if len(named) != 2 || named[0].Sandbox.Name == nil || *named[0].Sandbox.Name != "my agent ✨" {
		t.Fatalf("list --name = %+v, want the two named sandboxes", named)
	}
	if got := list("?name=other"); len(got) != 0 {
		t.Fatalf("list --name other = %d, want none", len(got))
	}
	if code, body := do(http.MethodGet, "/v1/sandboxes/"+created.ID, ""); code != http.StatusOK || !strings.Contains(body, `"name":"my agent ✨"`) {
		t.Fatalf("get = %d %s, want its name", code, body)
	}

	// A create that fails leaves no name behind.
	mu.Lock()
	failCreate = true
	mu.Unlock()
	before := len(names.names)
	if code, _ := do(http.MethodPost, "/v1/sandboxes", `{"name":"doomed"}`); code < 500 {
		t.Fatalf("failing create = %d, want a server error", code)
	}
	if len(names.names) != before {
		t.Errorf("a failed create left its name: %v", names.names)
	}
}

// ---- SDK v1.10.0's sandbox contract (w22/m445) ----

// sdkContract is testdata/sdk-v1.10.0-contract.json: what Render's Python and
// TypeScript SDK v1.10.0 turn into a named error class, how they read an ended
// sandbox, and the requests of the calls v1.9.0 and v1.10.0 added, read from
// their source.
type sdkContract struct {
	SDKs map[string]struct {
		Tag    string `json:"tag"`
		Commit string `json:"commit"`
	} `json:"sdks"`
	Codes []struct {
		Code       string  `json:"code"`
		Status     int     `json:"status"`
		TypeScript *string `json:"typescript"`
		Python     *string `json:"python"`
		AnsweredBy string  `json:"answeredBy"`
	} `json:"codes"`
	Statuses []struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
	} `json:"statuses"`
	Ended struct {
		Mints   []sdkRequest `json:"mints"`
		Refusal struct {
			Status int    `json:"status"`
			Code   string `json:"code"`
		} `json:"refusal"`
		Read struct {
			sdkRequest
			EndedStatuses []string `json:"endedStatuses"`
			EndedAtField  string   `json:"endedAtField"`
		} `json:"read"`
	} `json:"ended"`
	Image struct {
		FailureCode struct {
			Field  string   `json:"field"`
			Values []string `json:"values"`
		} `json:"failureCode"`
	} `json:"image"`
	Requests         map[string]json.RawMessage `json:"requests"`
	CannotOccurOnBex map[string]string          `json:"cannotOccurOnBex"`
}

// sdkRequest is one request an SDK sends and the statuses it accepts.
type sdkRequest struct {
	Method  string          `json:"method"`
	Path    string          `json:"path"`
	Body    json.RawMessage `json:"body"`
	Success []int           `json:"success"`
}

func loadSDKContract(t *testing.T) sdkContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sdk-v1.10.0-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c sdkContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	for _, sdk := range []string{"typescript", "python"} {
		if got := c.SDKs[sdk]; !strings.HasSuffix(got.Tag, "/v1.10.0") || len(got.Commit) != 40 {
			t.Fatalf("the contract's %s source is %+v, want the v1.10.0 tag and its commit", sdk, got)
		}
	}
	return c
}

// request is one of the contract's named requests.
func (c sdkContract) request(t *testing.T, name string) sdkRequest {
	t.Helper()
	var r sdkRequest
	if err := json.Unmarshal(c.Requests[name], &r); err != nil || r.Method == "" {
		t.Fatalf("the contract has no %q request (%v)", name, err)
	}
	return r
}

// bindVars puts a test's ids in place of a contract's placeholders.
func bindVars(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, k, v)
	}
	return s
}

// bindSDK is a contract path with the test's ids, under bex's /v1.
func bindSDK(path string, vars map[string]string) string {
	return "/v1" + bindVars(path, vars)
}

// defaultGroupID is the workspace's default sandbox group.
func (f *groupFixture) defaultGroupID(t *testing.T) string {
	t.Helper()
	for _, g := range f.listGroups(t) {
		if g.IsDefault {
			return g.ID
		}
	}
	t.Fatal("the workspace has no default group")
	return ""
}

// terminate ends a sandbox through the API, as an SDK's terminate does.
func (f *groupFixture) terminate(t *testing.T, id string) {
	t.Helper()
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+id+"/terminate?ownerId="+f.ws, ""); code != http.StatusNoContent {
		t.Fatalf("terminate %s = %d %s", id, code, body)
	}
}

// sdkScenario drives bex into the situation behind one SDK code, in the shared
// fixture, and returns the answer the SDK reads.
type sdkScenario func(t *testing.T, f *snapshotFixture) (int, string)

// sdkCodeScenarios has one scenario per code the SDKs map, keyed by the code.
// A code in the contract with neither a scenario here nor a reason in its
// cannotOccurOnBex fails TestEverySDKErrorCodeHasABexAnswer.
var sdkCodeScenarios = map[string]sdkScenario{
	"snapshot_creating": func(t *testing.T, f *snapshotFixture) (int, string) {
		snap := f.mustCapture(t, f.sandbox(t, "").ID, `{"kind":"filesystem"}`)
		return f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+f.group.ID+"/snapshots/"+snap.ID, "")
	},
	"snapshot_not_available": func(t *testing.T, f *snapshotFixture) (int, string) {
		snap := f.mustCapture(t, f.sandbox(t, "").ID, `{"kind":"filesystem"}`)
		return f.restore(t, fmt.Sprintf(`"snapshotId":%q`, snap.ID))
	},
	"snapshot_not_found": func(t *testing.T, f *snapshotFixture) (int, string) {
		return f.restore(t, `"snapshotName":"sdk-missing"`)
	},
	"default_sandbox_group_not_deletable": func(t *testing.T, f *snapshotFixture) (int, string) {
		return f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+f.defaultGroupID(t), "")
	},
	"build_not_succeeded": func(t *testing.T, f *snapshotFixture) (int, string) {
		b := f.seedBuild(t, "sdk-pending", store.SandboxBuildPending)
		return f.restore(t, fmt.Sprintf(`"buildId":%q`, b.ID))
	},
	"build_not_found": func(t *testing.T, f *snapshotFixture) (int, string) {
		return f.restore(t, `"buildName":"sdk-missing"`)
	},
	"image_in_use": func(t *testing.T, f *snapshotFixture) (int, string) {
		b := f.seedBuild(t, "sdk-used", store.SandboxBuildSucceeded)
		f.sandbox(t, fmt.Sprintf(`"buildId":%q`, b.ID))
		return f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+f.group.ID+"/builds/"+b.ID, "")
	},
	"sandbox_not_running": func(t *testing.T, f *snapshotFixture) (int, string) {
		sb := f.sandbox(t, "")
		f.terminate(t, sb.ID)
		return f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/runs/stream/token?ownerId="+f.ws, "")
	},
}

// sdkStatusScenarios answer the contract's statuses, which the SDKs map to a
// sandbox class by path and status whatever the code.
var sdkStatusScenarios = map[string]sdkScenario{
	"snapshotPath404": func(t *testing.T, f *snapshotFixture) (int, string) {
		return f.do(t, http.MethodGet, "/v1/sandbox-groups/"+f.group.ID+"/snapshots/"+ids.SandboxSnapshot.Prefix()+"-"+strings.Repeat("a", 20), "")
	},
	"credentialPath404": func(t *testing.T, f *snapshotFixture) (int, string) {
		return f.do(t, http.MethodGet, "/v1/sandbox-groups/"+f.group.ID+"/credentials/sdk-missing", "")
	},
	"sandboxPath404": func(t *testing.T, f *snapshotFixture) (int, string) {
		return f.do(t, http.MethodGet, "/v1/sandboxes/"+ids.Sandbox.Prefix()+"-"+strings.Repeat("a", 20)+"?ownerId="+f.ws, "")
	},
}

// seededBuilds numbers seedBuild's digests: a name and digest a group already
// has is the same build.
var seededBuilds atomic.Int64

// seedBuild records an image in the fixture's group in the given status, as
// an import would leave it.
func (f *imageFixture) seedBuild(t *testing.T, name, status string) store.SandboxBuild {
	t.Helper()
	ctx := context.Background()
	b, _, err := f.st.CreateSandboxBuild(ctx, store.NewSandboxBuild{
		WorkspaceID: f.ws, SandboxGroupID: f.group.ID, Name: name,
		SourceRepository: "docker.io", SourceImage: "library/" + name, SourceReference: status,
		SourceDigest: fmt.Sprintf("sha256:%064x", seededBuilds.Add(1)),
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if status != store.SandboxBuildPending {
		if ok, err := f.st.StartSandboxBuild(ctx, b.ID); err != nil || !ok {
			t.Fatalf("start %s: %v %v", b.ID, ok, err)
		}
	}
	switch status {
	case store.SandboxBuildSucceeded:
		if ok, err := f.st.SucceedSandboxBuild(ctx, b.ID, b.SourceDigest, 1); err != nil || !ok {
			t.Fatalf("succeed %s: %v %v", b.ID, ok, err)
		}
	case store.SandboxBuildFailed:
		if err := f.st.FailSandboxBuild(ctx, b.ID, buildFailedPull, "pull failed"); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

// The contract and bex's answers cannot drift apart: every code the SDKs map
// has a scenario here, a reason it cannot occur on bex, or is the file
// gateway's (pinned in internal/sshgateway/sandboxfiles), every code the API
// answers survives its Render-code filter, and nothing here names a code the
// SDKs no longer map.
func TestEverySDKErrorCodeHasABexAnswer(t *testing.T) {
	c := loadSDKContract(t)
	// bex-api strips from a Render operation's error every code outside the
	// pinned errorCode vocabulary (w8/411), so a code the SDKs map must be in
	// it, or production would answer it without and the SDKs raise ClientError.
	raw, err := os.ReadFile(filepath.Join("..", "api", "openapi", "render-public-api-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Components struct {
			Schemas struct {
				ErrorCode struct {
					Enum []string `json:"enum"`
				} `json:"errorCode"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil || len(spec.Components.Schemas.ErrorCode.Enum) == 0 {
		t.Fatalf("the pinned errorCode vocabulary: %v", err)
	}
	inContract := map[string]bool{}
	for _, row := range c.Codes {
		inContract[row.Code] = true
		_, scenario := sdkCodeScenarios[row.Code]
		reason, cannot := c.CannotOccurOnBex[row.Code]
		if row.AnsweredBy != "file-gateway" && !slices.Contains(spec.Components.Schemas.ErrorCode.Enum, row.Code) {
			t.Errorf("%s is not in the pinned errorCode vocabulary, so bex-api would strip it from a Render operation's error", row.Code)
		}
		switch {
		case row.TypeScript == nil && row.Python == nil:
			t.Errorf("%s maps to no class in either SDK", row.Code)
		case row.Status < 400 || row.Status > 499:
			t.Errorf("%s answers %d, want the 4xx the SDKs map", row.Code, row.Status)
		case row.AnsweredBy == "file-gateway":
			if scenario || cannot {
				t.Errorf("%s is the file gateway's answer, not the API's", row.Code)
			}
		case scenario && cannot:
			t.Errorf("%s has a scenario and a reason it cannot occur", row.Code)
		case !scenario && !cannot:
			t.Errorf("%s has no scenario and no reason it cannot occur on bex: drive bex into it in sdkCodeScenarios, or say why not in cannotOccurOnBex", row.Code)
		case cannot && strings.TrimSpace(reason) == "":
			t.Errorf("%s cannot occur on bex, but the contract gives no reason", row.Code)
		}
	}
	for code := range sdkCodeScenarios {
		if !inContract[code] {
			t.Errorf("sdkCodeScenarios has %s, which the contract does not map", code)
		}
	}
	for code := range c.CannotOccurOnBex {
		if !inContract[code] {
			t.Errorf("cannotOccurOnBex has %s, which the contract does not map", code)
		}
	}
	statuses := map[string]bool{}
	for _, row := range c.Statuses {
		statuses[row.ID] = true
		if _, ok := sdkStatusScenarios[row.ID]; !ok {
			t.Errorf("status mapping %s has no scenario", row.ID)
		}
	}
	for id := range sdkStatusScenarios {
		if !statuses[id] {
			t.Errorf("sdkStatusScenarios has %s, which the contract does not map", id)
		}
	}
}

// Each situation answers the status and the code the SDKs turn into the named
// class, so a user's `except SnapshotNotReadyError` or `instanceof
// SandboxImageInUseError` branch runs on bex as on Render.
func TestBexAnswersEverySDKErrorCode(t *testing.T) {
	c := loadSDKContract(t)
	f := newSnapshotFixture(t)
	f.svc.Records, f.svc.Names, f.svc.Executions = f.st, f.st, f.st
	f.svc.Credentials, f.svc.CredentialVault = f.st, NewCredentialVault(newMemCredentialKV())
	for _, row := range c.Codes {
		scenario, ok := sdkCodeScenarios[row.Code]
		if !ok {
			continue // TestEverySDKErrorCodeHasABexAnswer accounts for it
		}
		t.Run(row.Code, func(t *testing.T) {
			if code, body := scenario(t, f); code != row.Status || errCode(body) != row.Code {
				t.Errorf("= %d %s, want %d %s, which the SDKs raise as %s / %s", code, body, row.Status, row.Code, className(row.TypeScript), className(row.Python))
			}
		})
	}
	for _, row := range c.Statuses {
		t.Run(row.ID, func(t *testing.T) {
			if code, body := sdkStatusScenarios[row.ID](t, f); code != row.Status {
				t.Errorf("= %d %s, want %d", code, body, row.Status)
			}
		})
	}
}

// className is an SDK's class for a code, ClientError where it names none.
func className(s *string) string {
	if s == nil {
		return "ClientError"
	}
	return *s
}

// SandboxEndedError, as both SDKs raise it: every connect-token mint of a
// terminated or errored sandbox answers the contract's refusal, and the
// client's follow-up read finds the sandbox ended, with the time a terminated
// one ended.
func TestTheSDKsReadAnEndedSandboxAsEnded(t *testing.T) {
	c := loadSDKContract(t)
	f := recordsFixture(t)
	terminated := f.mustCreate(t, "")
	f.terminate(t, terminated.ID)
	f.addBox("os-errored", "Failed")
	for _, ended := range []struct {
		id, status string
		at         *time.Time
	}{
		{terminated.ID, string(StatusTerminated), &f.now},
		{"os-errored", string(StatusErrored), nil},
	} {
		vars := map[string]string{"$SANDBOX": ended.id, "$OWNER": f.ws}
		for _, mint := range c.Ended.Mints {
			code, body := f.do(t, mint.Method, bindSDK(mint.Path, vars), "")
			if code != c.Ended.Refusal.Status || errCode(body) != c.Ended.Refusal.Code {
				t.Errorf("%s %s of a %s sandbox = %d %s, want %d %s", mint.Method, mint.Path, ended.status, code, body, c.Ended.Refusal.Status, c.Ended.Refusal.Code)
			}
		}
		code, body := f.do(t, c.Ended.Read.Method, bindSDK(c.Ended.Read.Path, vars), "")
		var read map[string]any
		if err := json.Unmarshal([]byte(body), &read); !slices.Contains(c.Ended.Read.Success, code) || err != nil {
			t.Fatalf("the SDK's read of a %s sandbox = %d %s", ended.status, code, body)
		}
		if read["status"] != ended.status || !slices.Contains(c.Ended.Read.EndedStatuses, ended.status) {
			t.Errorf("the SDK reads status %v, want %s, one of %v", read["status"], ended.status, c.Ended.Read.EndedStatuses)
		}
		at, _ := read[c.Ended.Read.EndedAtField].(string)
		switch {
		case ended.at != nil && at != ended.at.Format(time.RFC3339):
			t.Errorf("a terminated sandbox reads %s %q, want %s, the time the SDK names", c.Ended.Read.EndedAtField, at, ended.at.Format(time.RFC3339))
		case ended.at == nil && at != "":
			t.Errorf("an errored sandbox reads %s %q, want none", c.Ended.Read.EndedAtField, at)
		}
	}
}

// images.list as both SDKs send it: newest first, filtered by name and
// status, and paged from the last item's cursor (Python's next_cursor). A
// failed image reads a failureCode from the SDKs' vocabulary, a succeeded one
// none. images.delete answers 204, again for one already deleted, and 404
// build_not_found for one the group never had.
func TestTheSDKImageListAndDeleteRequests(t *testing.T) {
	c := loadSDKContract(t)
	list, del := c.request(t, "images.list"), c.request(t, "images.delete")
	f := newImageFixture(t)
	first := f.seedBuild(t, "py", store.SandboxBuildSucceeded)
	failed := f.seedBuild(t, "py", store.SandboxBuildFailed)
	newest := f.seedBuild(t, "node", store.SandboxBuildSucceeded)
	vars := map[string]string{"$GROUP": f.group.ID}
	page := func(query string) []BuildEnvelope {
		t.Helper()
		code, body := f.do(t, list.Method, bindSDK(list.Path, vars)+query, "")
		var items []BuildEnvelope
		if err := json.Unmarshal([]byte(body), &items); !slices.Contains(list.Success, code) || err != nil {
			t.Fatalf("images.list%s = %d %s", query, code, body)
		}
		return items
	}
	idsOf := func(items []BuildEnvelope) []string {
		var out []string
		for _, it := range items {
			if it.Cursor != it.Build.ID {
				t.Errorf("item %s carries cursor %s", it.Build.ID, it.Cursor)
			}
			out = append(out, it.Build.ID)
		}
		return out
	}
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"", []string{newest.ID, failed.ID, first.ID}},
		{"?name=py", []string{failed.ID, first.ID}},
		{"?status=succeeded", []string{newest.ID, first.ID}},
		{"?status=succeeded&status=failed&name=py", []string{failed.ID, first.ID}},
	} {
		if got := idsOf(page(tc.query)); !slices.Equal(got, tc.want) {
			t.Errorf("images.list%s = %v, want %v newest first", tc.query, got, tc.want)
		}
	}
	one := page("?limit=2")
	rest := page("?limit=2&cursor=" + one[len(one)-1].Cursor)
	if got := idsOf(append(one, rest...)); !slices.Equal(got, []string{newest.ID, failed.ID, first.ID}) {
		t.Errorf("images.list paged by 2 = %v, want every image once, newest first", got)
	}

	_, body := f.do(t, list.Method, bindSDK(list.Path, vars), "")
	var raw []struct {
		Build map[string]any `json:"build"`
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatal(err)
	}
	codes := map[string]any{}
	for _, it := range raw {
		value, present := it.Build[c.Image.FailureCode.Field]
		if !present {
			t.Fatalf("image %v has no %s field", it.Build["id"], c.Image.FailureCode.Field)
		}
		codes[it.Build["id"].(string)] = value
	}
	if got, _ := codes[failed.ID].(string); !slices.Contains(c.Image.FailureCode.Values, got) {
		t.Errorf("a failed image reads %s %v, want one of the SDKs' %v", c.Image.FailureCode.Field, codes[failed.ID], c.Image.FailureCode.Values)
	}
	if codes[first.ID] != nil || codes[newest.ID] != nil {
		t.Errorf("succeeded images read %s %v and %v, want null", c.Image.FailureCode.Field, codes[first.ID], codes[newest.ID])
	}
	for _, recorded := range []string{buildFailedDeadline, buildFailedAccessDenied, buildFailedNotFound, buildFailedPull, buildFailedInternal} {
		if !slices.Contains(c.Image.FailureCode.Values, recorded) {
			t.Errorf("bex records failureCode %q, which the SDKs' vocabulary does not have", recorded)
		}
	}

	path := bindSDK(del.Path, map[string]string{"$GROUP": f.group.ID, "$IMAGE": newest.ID})
	for _, attempt := range []string{"delete", "repeat"} {
		if code, body := f.do(t, del.Method, path, ""); !slices.Contains(del.Success, code) || body != "" {
			t.Errorf("images.delete (%s) = %d %q, want %v and no body", attempt, code, body, del.Success)
		}
	}
	if got := idsOf(page("")); slices.Contains(got, newest.ID) {
		t.Errorf("images.list after the delete = %v, want %s gone", got, newest.ID)
	}
	unknown := bindSDK(del.Path, map[string]string{"$GROUP": f.group.ID, "$IMAGE": ids.SandboxBuild.Prefix() + "-" + strings.Repeat("a", 20)})
	if code, body := f.do(t, del.Method, unknown, ""); code != http.StatusNotFound || errCode(body) != CodeBuildNotFound {
		t.Errorf("images.delete of an image the group never had = %d %s, want 404 %s", code, body, CodeBuildNotFound)
	}
}

// snapshots.create with a TTL, as v1.9.0 changed it: ttlSeconds instead of
// an expiry time, answered 202 with the fields the SDKs read.
func TestTheSDKSnapshotCreateWithATTL(t *testing.T) {
	c := loadSDKContract(t)
	req := c.request(t, "snapshots.create")
	f := newSnapshotFixture(t)
	sb := f.sandbox(t, "")
	code, body := f.do(t, req.Method, bindSDK(req.Path, map[string]string{"$SANDBOX": sb.ID, "$OWNER": f.ws}), string(req.Body))
	var sent struct {
		TTLSeconds int64 `json:"ttlSeconds"`
	}
	_ = json.Unmarshal(req.Body, &sent)
	var snap Snapshot
	if err := json.Unmarshal([]byte(body), &snap); !slices.Contains(req.Success, code) || err != nil {
		t.Fatalf("snapshots.create = %d %s, want %v", code, body, req.Success)
	}
	if snap.ID == "" || snap.SandboxGroupID != f.group.ID || snap.Status != store.SandboxSnapshotCreating || snap.TTLSeconds != sent.TTLSeconds {
		t.Errorf("snapshots.create = %+v, want an id, group %s, status creating and ttlSeconds %d", snap, f.group.ID, sent.TTLSeconds)
	}
}

// Python SDK v1.10.0's create(wait=True), canceled while the sandbox is still
// creating, terminates it with the contract's request. bex accepts that
// terminate of a creating sandbox: the sandbox ends, its meter stops and its
// slot in the group is free again. A sandbox id that never existed is the 404
// Python raises SandboxNotFoundError on.
func TestACanceledSDKCreateEndsTheCreatingSandbox(t *testing.T) {
	c := loadSDKContract(t)
	req := c.request(t, "create(wait) canceled")
	f := recordsFixture(t)
	g := f.setLimit(t, f.createGroup(t, "one-slot"), 1)
	sb := f.mustCreate(t, g.ID)
	f.os.set(f.os.idOf(sb.ID), func(b *fakeBox) { b.state = "Pending" })
	if code, got, body := f.getSandbox(t, "id-a", sb.ID); code != http.StatusOK || got.Status != StatusCreating {
		t.Fatalf("the wait's read = %d %s, want 200 creating", code, body)
	}

	if code, body := f.do(t, req.Method, bindSDK(req.Path, map[string]string{"$SANDBOX": sb.ID, "$OWNER": f.ws}), ""); !slices.Contains(req.Success, code) {
		t.Fatalf("terminate of a creating sandbox = %d %s, want %v", code, body, req.Success)
	}
	if code, got, body := f.getSandbox(t, "id-a", sb.ID); code != http.StatusOK || got.Status != StatusTerminated || got.TerminatedAt == nil {
		t.Fatalf("after the terminate = %d %s, want 200 terminated with terminatedAt", code, body)
	}
	if f.os.idOf(sb.ID) != "" {
		t.Error("the sandbox is still in OpenSandbox")
	}
	if phases := f.phases(sb.ID); len(phases) == 0 || phases[len(phases)-1] != string(StatusTerminated) {
		t.Errorf("meter phases = %v, want them to end terminated", phases)
	}
	if code, body := f.create(t, g.ID); code != http.StatusCreated {
		t.Errorf("a create in the one-slot group after the terminate = %d %s, want 201", code, body)
	}

	never := ids.Sandbox.Prefix() + "-" + strings.Repeat("b", 20)
	if code, body := f.do(t, req.Method, bindSDK(req.Path, map[string]string{"$SANDBOX": never, "$OWNER": f.ws}), ""); code != http.StatusNotFound {
		t.Errorf("terminate of a sandbox that never existed = %d %s, want 404", code, body)
	}
}

// The codes the SDKs map are bex's answer on every surface, not only REST:
// GraphQL's extensions.code and MCP's tool error carry the same string for
// the same situation (GraphQL has no exec, so the ended sandbox is MCP's).
func TestTheSDKCodesAgreeOnGraphQLAndMCP(t *testing.T) {
	c := loadSDKContract(t)
	mapped := map[string]bool{}
	for _, row := range c.Codes {
		mapped[row.Code] = true
	}
	f := newSnapshotFixture(t)
	f.svc.Records, f.svc.Names, f.svc.Executions = f.st, f.st, f.st
	gql := newGQLDriver(t, f.svc)
	mcpd := mcpDriver{t: t, client: mcpClient(t, f.svc)}
	defaultGroup := f.defaultGroupID(t)
	pending := f.seedBuild(t, "sdk-pending", store.SandboxBuildPending)
	used := f.seedBuild(t, "sdk-used", store.SandboxBuildSucceeded)
	f.sandbox(t, fmt.Sprintf(`"buildId":%q`, used.ID))
	creating := f.mustCapture(t, f.sandbox(t, "").ID, `{"kind":"filesystem"}`)
	ended := f.sandbox(t, "")
	f.terminate(t, ended.ID)
	create := func(arg, value string) outcome {
		return gql.run("createSandbox", `mutation($g: String, $v: String) { createSandbox(sandboxGroupId: $g, `+arg+`: $v) { id } }`,
			map[string]any{"g": f.group.ID, "v": value})
	}
	spawn := func(arg, value string) outcome {
		return mcpd.call("spawn_sandbox", map[string]any{"sandboxGroupId": f.group.ID, arg: value})
	}
	for _, tc := range []struct {
		code     string
		gql, mcp func() outcome
	}{
		{CodeDefaultSandboxGroupNotDeletable,
			func() outcome {
				return gql.run("deleteSandboxGroup", `mutation($id: String!) { deleteSandboxGroup(id: $id) }`, map[string]any{"id": defaultGroup})
			},
			func() outcome { return mcpd.call("delete_sandbox_group", map[string]any{"id": defaultGroup}) }},
		{CodeImageInUse,
			func() outcome {
				return gql.run("deleteSandboxImage", `mutation($g: String!, $id: String!) { deleteSandboxImage(sandboxGroupId: $g, id: $id) }`,
					map[string]any{"g": f.group.ID, "id": used.ID})
			},
			func() outcome {
				return mcpd.call("delete_sandbox_image", map[string]any{"sandboxGroupId": f.group.ID, "id": used.ID})
			}},
		{CodeBuildNotFound, func() outcome { return create("buildName", "sdk-missing") }, func() outcome { return spawn("buildName", "sdk-missing") }},
		{CodeBuildNotSucceeded, func() outcome { return create("buildId", pending.ID) }, func() outcome { return spawn("buildId", pending.ID) }},
		{CodeSnapshotNotFound, func() outcome { return create("snapshotName", "sdk-missing") }, func() outcome { return spawn("snapshotName", "sdk-missing") }},
		{CodeSnapshotNotAvailable, func() outcome { return create("snapshotId", creating.ID) }, func() outcome { return spawn("snapshotId", creating.ID) }},
		{CodeSandboxNotRunning, nil, func() outcome { return mcpd.execOn(ended.ID) }},
	} {
		if !mapped[tc.code] {
			t.Errorf("%s is not a code the SDKs map", tc.code)
		}
		if tc.gql != nil {
			if out := tc.gql(); out.Code != tc.code {
				t.Errorf("GraphQL answers %+v where REST answers %s", out, tc.code)
			}
		}
		if out := tc.mcp(); out.Code != tc.code {
			t.Errorf("MCP answers %+v where REST answers %s", out, tc.code)
		}
	}
}
