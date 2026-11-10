package main_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// sandboxGroupsContract mirrors lego/cli/testdata/sandbox-groups-contract.json,
// the golden bex-api's group routes are proven to emit
// (lego/backend/internal/sandbox/groups_contract_test.go). Running the PINNED
// `sandbox-groups` and `sandboxes create --group` commands against it makes a
// pin bump that changes what the client sends or reads fail here (w22/m311).
type sandboxGroupsContract struct {
	Create struct {
		Method      string         `json:"method"`
		Path        string         `json:"path"`
		RequestBody map[string]any `json:"requestBody"`
		Status      int            `json:"status"`
		Response    map[string]any `json:"response"`
	} `json:"create"`
	List struct {
		Method   string           `json:"method"`
		Path     string           `json:"path"`
		Status   int              `json:"status"`
		Response []map[string]any `json:"response"`
	} `json:"list"`
	SandboxCreate struct {
		Method         string         `json:"method"`
		Path           string         `json:"path"`
		RequestBodyHas map[string]any `json:"requestBodyHas"`
		Status         int            `json:"status"`
		Response       map[string]any `json:"response"`
	} `json:"sandboxCreate"`
}

func loadSandboxGroupsContract(t *testing.T) sandboxGroupsContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sandbox-groups-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c sandboxGroupsContract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

type groupsCall struct {
	method, path, query string
	body                map[string]any
}

// groupsContractServer serves the golden and records what the client sent.
func groupsContractServer(t *testing.T, c sandboxGroupsContract) (*httptest.Server, func() []groupsCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []groupsCall
	reply := func(w http.ResponseWriter, status int, body any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		calls = append(calls, groupsCall{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, body: body})
		mu.Unlock()
		switch r.Method + " " + r.URL.Path {
		case c.Create.Method + " " + c.Create.Path:
			reply(w, c.Create.Status, c.Create.Response)
		case c.List.Method + " " + c.List.Path:
			reply(w, c.List.Status, c.List.Response)
		case c.SandboxCreate.Method + " " + c.SandboxCreate.Path:
			reply(w, c.SandboxCreate.Status, c.SandboxCreate.Response)
		case http.MethodGet + " " + c.SandboxCreate.Path:
			// Render's sandbox list is a cursor-paginated [{sandbox, cursor}] array.
			reply(w, http.StatusOK, []map[string]any{{"sandbox": c.SandboxCreate.Response, "cursor": c.SandboxCreate.Response["id"]}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []groupsCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]groupsCall(nil), calls...)
	}
}

func runBexAgainst(t *testing.T, apiURL string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	command := bexAgainst(t, apiURL, args...)
	var out, errOut bytes.Buffer
	command.Stdout, command.Stderr = &out, &errOut
	err = command.Run()
	return out.String(), errOut.String(), err
}

// The pinned `sandbox-groups create` and `list` succeed against bex-api's
// answers and read every field the schema requires.
func TestPinnedSandboxGroupsCommandsReadBexAnswers(t *testing.T) {
	c := loadSandboxGroupsContract(t)
	srv, calls := groupsContractServer(t, c)

	stdout, stderr, err := runBexAgainst(t, srv.URL, "sandbox-groups", "create", "--name", "agents", "-o", "json")
	if err != nil {
		t.Fatalf("sandbox-groups create: %v\n%s", err, stderr)
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(stdout), &created); err != nil {
		t.Fatalf("create output %q: %v", stdout, err)
	}
	for _, key := range []string{"id", "name", "isDefault", "concurrencyLimit", "region"} {
		if created[key] != c.Create.Response[key] {
			t.Errorf("create printed %s = %v, want %v (the pinned decoder no longer reads bex's group)", key, created[key], c.Create.Response[key])
		}
	}

	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "list", "-o", "json")
	if err != nil {
		t.Fatalf("sandbox-groups list: %v\n%s", err, stderr)
	}
	var listed []map[string]any
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil || len(listed) != 1 || listed[0]["name"] != "agents" || listed[0]["isDefault"] != true {
		t.Fatalf("list output %q (%v), want the one default group", stdout, err)
	}

	seen := calls()
	if len(seen) != 2 {
		t.Fatalf("the client made %d requests, want 2: %+v", len(seen), seen)
	}
	if seen[0].method != c.Create.Method || seen[0].path != c.Create.Path || seen[0].body["name"] != c.Create.RequestBody["name"] || seen[0].body["ownerId"] != "tea-bex" {
		t.Errorf("create request = %+v, want %s %s with the name and the workspace", seen[0], c.Create.Method, c.Create.Path)
	}
	if seen[1].method != c.List.Method || seen[1].path != c.List.Path || !strings.Contains(seen[1].query, "ownerId=tea-bex") {
		t.Errorf("list request = %+v, want %s %s?ownerId=", seen[1], c.List.Method, c.List.Path)
	}
}

// `sandboxes create --group` sends sandboxGroupId, and the pinned client reads a
// sandbox that carries one.
func TestPinnedSandboxCreateSendsTheGroup(t *testing.T) {
	c := loadSandboxGroupsContract(t)
	srv, calls := groupsContractServer(t, c)
	group, _ := c.SandboxCreate.RequestBodyHas["sandboxGroupId"].(string)

	stdout, stderr, err := runBexAgainst(t, srv.URL, "sandboxes", "create", "--group", group, "-o", "json")
	if err != nil {
		t.Fatalf("sandboxes create --group: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, group) {
		t.Errorf("create output %q does not carry the sandbox's group", stdout)
	}
	seen := calls()
	if len(seen) != 1 || seen[0].path != c.SandboxCreate.Path || seen[0].body["sandboxGroupId"] != group {
		t.Fatalf("requests = %+v, want one create carrying sandboxGroupId %s", seen, group)
	}
}

// `sandboxes list --group` sends the group as the sandboxGroupId query filter
// next to the workspace, and the pinned client reads the group off each sandbox.
func TestPinnedSandboxListSendsTheGroupFilter(t *testing.T) {
	c := loadSandboxGroupsContract(t)
	srv, calls := groupsContractServer(t, c)
	group, _ := c.SandboxCreate.RequestBodyHas["sandboxGroupId"].(string)

	stdout, stderr, err := runBexAgainst(t, srv.URL, "sandboxes", "list", "--group", group, "-o", "json")
	if err != nil {
		t.Fatalf("sandboxes list --group: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, group) {
		t.Errorf("list output %q does not carry the sandbox's group", stdout)
	}
	seen := calls()
	if len(seen) != 1 || seen[0].method != http.MethodGet || seen[0].path != c.SandboxCreate.Path ||
		!strings.Contains(seen[0].query, "sandboxGroupId="+group) || !strings.Contains(seen[0].query, "ownerId=tea-bex") {
		t.Fatalf("requests = %+v, want one list of %s filtered by sandboxGroupId=%s for the workspace", seen, c.SandboxCreate.Path, group)
	}
}
