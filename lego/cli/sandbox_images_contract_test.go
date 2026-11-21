package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// sandboxImagesContract is the `images` object of
// testdata/sandbox-groups-contract.json: what bex-api answers on the sandbox
// build routes (lego/backend/internal/sandbox/images_cli_contract_test.go proves
// it does). The v2.37.0 `sandbox-groups images list` and `delete` commands run
// against it here, so a pin bump that changes what they send or read fails
// until the contract and bex-api move together (w22/m444).
type sandboxImagesContract struct {
	ListPath     string           `json:"listPath"`
	ListQuery    string           `json:"listQuery"`
	BuildPath    string           `json:"buildPath"`
	DeleteStatus int              `json:"deleteStatus"`
	ListResponse []map[string]any `json:"listResponse"`
	InUse        struct {
		Status    int    `json:"status"`
		Code      string `json:"code"`
		ID        string `json:"id"`
		Dependent string `json:"dependent"`
		Message   string `json:"message"`
	} `json:"inUse"`
}

func (c sandboxImagesContract) build() map[string]any {
	return c.ListResponse[0]["build"].(map[string]any)
}

func (c sandboxImagesContract) group() string { return c.build()["sandboxGroupId"].(string) }
func (c sandboxImagesContract) id() string    { return c.build()["id"].(string) }

func loadSandboxImagesContract(t *testing.T) sandboxImagesContract {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "sandbox-groups-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Images sandboxImagesContract `json:"images"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Images.ListResponse) != 1 {
		t.Fatalf("the contract lists %d images, want 1", len(file.Images.ListResponse))
	}
	return file.Images
}

// imagesContractServer answers the build routes with the golden and records
// each request as "METHOD path?query". inUse makes the delete answer bex-api's
// 409 image_in_use.
func imagesContractServer(t *testing.T, c sandboxImagesContract, inUse bool) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var calls []string
	reply := func(w http.ResponseWriter, status int, body any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" {
			call += "?" + r.URL.RawQuery
		}
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
		switch r.Method + " " + r.URL.Path {
		case "GET " + c.ListPath:
			reply(w, http.StatusOK, c.ListResponse)
		case "GET " + c.BuildPath:
			reply(w, http.StatusOK, c.build())
		case "DELETE " + c.BuildPath:
			if inUse {
				reply(w, c.InUse.Status, map[string]any{
					"error": c.InUse.Message, "message": c.InUse.Message, "id": c.InUse.ID, "code": c.InUse.Code,
					"params": map[string]any{"id": c.id(), "dependents": []string{c.InUse.Dependent}},
				})
				return
			}
			w.WriteHeader(c.DeleteStatus)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), calls...)
	}
}

// `images list --name --status --status` asks the build route for an exact
// name and both statuses in one comma-separated parameter, 100 at a time, and
// prints the builds themselves, not their cursor envelopes.
func TestPinnedSandboxImagesListSendsItsFilters(t *testing.T) {
	c := loadSandboxImagesContract(t)
	srv, calls := imagesContractServer(t, c, false)

	stdout, stderr, err := runBexAgainst(t, srv.URL, "sandbox-groups", "images", "list", "--group", c.group(),
		"--name", "python", "--status", "succeeded", "--status", "failed", "-o", "json")
	if err != nil {
		t.Fatalf("images list: %v\n%s", err, stderr)
	}
	var listed []map[string]any
	if err := json.Unmarshal([]byte(stdout), &listed); err != nil || len(listed) != 1 {
		t.Fatalf("list output %q (%v), want the one image", stdout, err)
	}
	for _, key := range []string{"id", "sandboxGroupId", "name", "status", "sourceDigest"} {
		if listed[0][key] != c.build()[key] {
			t.Errorf("list printed %s = %v, want %v (the pinned decoder no longer reads bex's build)", key, listed[0][key], c.build()[key])
		}
	}
	// One page: fewer than the limit's 100 builds ends the list.
	if seen := calls(); len(seen) != 1 || seen[0] != "GET "+c.ListPath+"?"+c.ListQuery {
		t.Fatalf("requests = %q, want one GET %s?%s", seen, c.ListPath, c.ListQuery)
	}

	// The text table is what a script without -o sees.
	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "images", "list", "--group", c.group())
	if err != nil {
		t.Fatalf("images list as text: %v\n%s", err, stderr)
	}
	for _, want := range []string{"NAME", "STATUS", c.id(), "python", "succeeded", "docker.io/library/python:3.13"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("text list lacks %q:\n%s", want, stdout)
		}
	}

	// A status outside the build vocabulary never reaches the server.
	before := len(calls())
	_, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "images", "list", "--group", c.group(), "--status", "done")
	if err == nil || !strings.Contains(stderr, `invalid status "done"`) || len(calls()) != before {
		t.Errorf("--status done: err=%v stderr=%q requests=%q, want a local refusal", err, stderr, calls()[before:])
	}
}

// Without --confirm, `images delete` reads the image and deletes nothing; with
// it, the DELETE follows the read.
func TestPinnedSandboxImagesDeletePreviewsUntilConfirmed(t *testing.T) {
	c := loadSandboxImagesContract(t)
	srv, calls := imagesContractServer(t, c, false)

	stdout, stderr, err := runBexAgainst(t, srv.URL, "sandbox-groups", "images", "delete", c.id(), "--group", c.group())
	if err != nil {
		t.Fatalf("delete preview: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "would delete") || !strings.Contains(stdout, c.id()) || !strings.Contains(stdout, "--confirm") {
		t.Errorf("preview output lacks the image or the next step:\n%s", stdout)
	}
	if seen := calls(); len(seen) != 1 || seen[0] != "GET "+c.BuildPath {
		t.Fatalf("preview requests = %q, want only GET %s", seen, c.BuildPath)
	}

	stdout, stderr, err = runBexAgainst(t, srv.URL, "sandbox-groups", "images", "delete", c.id(), "--group", c.group(), "--confirm", "-o", "json")
	if err != nil {
		t.Fatalf("delete --confirm: %v\n%s", err, stderr)
	}
	var out struct {
		Data map[string]any `json:"data"`
		Meta struct {
			Deleted bool `json:"deleted"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil || !out.Meta.Deleted || out.Data["id"] != c.id() {
		t.Fatalf("delete output %q (%v), want meta.deleted for %s", stdout, err, c.id())
	}
	if seen := calls(); len(seen) != 3 || seen[1] != "GET "+c.BuildPath || seen[2] != "DELETE "+c.BuildPath {
		t.Fatalf("requests = %q, want the read and then DELETE %s", seen, c.BuildPath)
	}
}

// bex-api's 409 image_in_use reaches the user with the sandbox it names, and
// the command fails.
func TestPinnedSandboxImagesDeleteNamesTheSandboxInUse(t *testing.T) {
	c := loadSandboxImagesContract(t)
	srv, _ := imagesContractServer(t, c, true)

	stdout, stderr, err := runBexAgainst(t, srv.URL, "sandbox-groups", "images", "delete", c.id(), "--group", c.group(), "--confirm")
	if err == nil {
		t.Fatalf("delete of an image in use succeeded:\n%s", stdout)
	}
	if !strings.Contains(stderr, c.InUse.Dependent) || !strings.Contains(stderr, "409 ("+c.InUse.Code+")") {
		t.Errorf("stderr = %q, want the 409 %s naming %s", stderr, c.InUse.Code, c.InUse.Dependent)
	}
	if strings.Contains(stdout, "Deleted") {
		t.Errorf("a refused delete printed a success:\n%s", stdout)
	}
}
