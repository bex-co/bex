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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// imagesCLIGolden is the `images` object of the contract the pinned Render CLI
// v2.37.0 runs `sandbox-groups images list|delete` against
// (lego/cli/sandbox_images_contract_test.go, w22/m444).
type imagesCLIGolden struct {
	ListQuery    string           `json:"listQuery"`
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

func loadImagesCLIGolden(t *testing.T) imagesCLIGolden {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "sandbox-groups-contract.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Images imagesCLIGolden `json:"images"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	return file.Images
}

// The build routes answer the request the pinned client sends in the shape it
// is proven to read: a [{build, cursor}] page for its list query, 409
// image_in_use with the golden's message while a sandbox runs from the image,
// and the golden's status once it does not.
func TestImageRoutesEmitTheCLIContractGolden(t *testing.T) {
	golden := loadImagesCLIGolden(t)
	f := newImageFixture(t)
	ref := hostOf(f.source) + "/library/python:3.13"
	pushImage(t, ref, linuxAMD64, 256)
	b := f.mustImport(t, fmt.Sprintf(`{"name":"python","sourceImage":%q}`, ref))
	waitSucceeded(t, f.st, f.ws, f.group.ID, b.ID)
	builds := "/v1/sandbox-groups/" + f.group.ID + "/builds"

	code, body := f.do(t, http.MethodGet, builds+"?"+golden.ListQuery, "")
	var listed []map[string]any
	if err := json.Unmarshal([]byte(body), &listed); code != http.StatusOK || err != nil || len(listed) != len(golden.ListResponse) {
		t.Fatalf("list?%s = %d %s (%v), want %d item(s)", golden.ListQuery, code, body, err, len(golden.ListResponse))
	}
	wantBuild := golden.ListResponse[0]["build"].(map[string]any)
	gotBuild, _ := listed[0]["build"].(map[string]any)
	sameShape(t, "list item", listed[0], golden.ListResponse[0])
	sameShape(t, "list item build", gotBuild, wantBuild)
	sameShape(t, "list item build source", gotBuild["source"].(map[string]any), wantBuild["source"].(map[string]any))
	if listed[0]["cursor"] != b.ID || gotBuild["id"] != b.ID || gotBuild["name"] != wantBuild["name"] || gotBuild["status"] != wantBuild["status"] {
		t.Errorf("list item = %s, want build %s named %v, %v, as its own cursor", body, b.ID, wantBuild["name"], wantBuild["status"])
	}

	// The delete's preview is this read; the client prints it as the image.
	code, body = f.do(t, http.MethodGet, builds+"/"+b.ID, "")
	var read map[string]any
	if err := json.Unmarshal([]byte(body), &read); code != http.StatusOK || err != nil {
		t.Fatalf("get = %d %s", code, body)
	}
	sameShape(t, "get", read, wantBuild)

	code, body = f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildId":%q}`, f.ws, f.group.ID, b.ID))
	var sb Sandbox
	if err := json.Unmarshal([]byte(body), &sb); code != http.StatusCreated || err != nil {
		t.Fatalf("create from the build = %d %s", code, body)
	}
	code, body = f.do(t, http.MethodDelete, builds+"/"+b.ID, "")
	var refusal struct {
		Error, Message, ID, Code string
		Params                   struct {
			Dependents []string `json:"dependents"`
		} `json:"params"`
	}
	_ = json.Unmarshal([]byte(body), &refusal)
	wantMessage := strings.ReplaceAll(golden.InUse.Message, golden.InUse.Dependent, sb.ID)
	if code != golden.InUse.Status || refusal.Code != golden.InUse.Code || refusal.ID != golden.InUse.ID ||
		refusal.Message != wantMessage || refusal.Error != wantMessage || !slices.Equal(refusal.Params.Dependents, []string{sb.ID}) {
		t.Fatalf("delete in use = %d %s, want %d %s (%s) saying %q", code, body, golden.InUse.Status, golden.InUse.Code, golden.InUse.ID, wantMessage)
	}

	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/terminate?ownerId="+f.ws, ""); code >= 300 {
		t.Fatalf("terminate = %d %s", code, body)
	}
	if code, body := f.do(t, http.MethodDelete, builds+"/"+b.ID, ""); code != golden.DeleteStatus || body != "" {
		t.Fatalf("delete = %d %q, want %d and no body", code, body, golden.DeleteStatus)
	}
}

// The list's filters as `images list --name` and `--status` use them: the name
// matches exactly, statuses arrive comma-separated or repeated, the two
// combine, an empty result is [], and an unknown status is refused even beside
// a known one.
func TestImageListFiltersByExactNameAndStatus(t *testing.T) {
	f := newImageFixture(t)
	ctx := context.Background()
	seeded := 0
	seed := func(name, status string) string {
		t.Helper()
		seeded++ // a name and digest the group already has is the same build
		b, _, err := f.st.CreateSandboxBuild(ctx, store.NewSandboxBuild{
			WorkspaceID: f.ws, SandboxGroupID: f.group.ID, Name: name,
			SourceRepository: "docker.io", SourceImage: "library/" + name, SourceReference: status,
			SourceDigest: fmt.Sprintf("sha256:%064x", seeded),
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
			if err := f.st.FailSandboxBuild(ctx, b.ID, "image_pull_failed", "pull failed"); err != nil {
				t.Fatal(err)
			}
		}
		return b.ID
	}
	pythonOK := seed("python", store.SandboxBuildSucceeded)
	pythonFailed := seed("python", store.SandboxBuildFailed)
	python3 := seed("python3", store.SandboxBuildSucceeded)
	nodePending := seed("node", store.SandboxBuildPending)
	nodeRunning := seed("node", store.SandboxBuildRunning)

	builds := "/v1/sandbox-groups/" + f.group.ID + "/builds"
	for _, tc := range []struct {
		query string
		want  []string
	}{
		{"", []string{pythonOK, pythonFailed, python3, nodePending, nodeRunning}},
		{"?name=python", []string{pythonOK, pythonFailed}},
		{"?name=pyth", nil},
		{"?name=Python", nil},
		{"?status=succeeded", []string{pythonOK, python3}},
		{"?status=succeeded,failed", []string{pythonOK, pythonFailed, python3}},
		{"?status=succeeded&status=failed", []string{pythonOK, pythonFailed, python3}},
		{"?status=pending,running", []string{nodePending, nodeRunning}},
		{"?limit=100&name=python&status=failed", []string{pythonFailed}},
		{"?name=python&status=pending", nil},
	} {
		code, body := f.do(t, http.MethodGet, builds+tc.query, "")
		var page []BuildEnvelope
		if err := json.Unmarshal([]byte(body), &page); code != http.StatusOK || err != nil {
			t.Errorf("list%s = %d %s (%v)", tc.query, code, body, err)
			continue
		}
		got := make([]string, 0, len(page))
		for _, item := range page {
			got = append(got, item.Build.ID)
		}
		want := slices.Clone(tc.want)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("list%s = %v, want %v", tc.query, got, want)
		}
		if len(tc.want) == 0 && strings.TrimSpace(body) != "[]" {
			t.Errorf("list%s with no match = %s, want []", tc.query, body)
		}
	}
	if code, body := f.do(t, http.MethodGet, builds+"?status=succeeded,done", ""); code != http.StatusBadRequest || errCode(body) != CodeInvalidBuildStatus {
		t.Errorf("list with an unknown status beside a known one = %d %s, want 400 %s", code, body, CodeInvalidBuildStatus)
	}
}

// An image more sandboxes run from than an image_in_use names lists the first
// ten and stays in place; a member of another workspace cannot delete it or
// learn that it exists.
func TestDeleteImageInUseNamesAtMostTenAndStaysInItsWorkspace(t *testing.T) {
	f := newImageFixture(t)
	ref := hostOf(f.source) + "/library/py:3.13"
	pushImage(t, ref, linuxAMD64, 256)
	b := f.mustImport(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref))
	waitSucceeded(t, f.st, f.ws, f.group.ID, b.ID)
	buildPath := "/v1/sandbox-groups/" + f.group.ID + "/builds/" + b.ID

	var started []string
	for range maxImageDependents + 2 {
		code, body := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildId":%q}`, f.ws, f.group.ID, b.ID))
		var sb Sandbox
		if err := json.Unmarshal([]byte(body), &sb); code != http.StatusCreated || err != nil {
			t.Fatalf("create from the build = %d %s", code, body)
		}
		started = append(started, sb.ID)
	}
	code, body := f.do(t, http.MethodDelete, buildPath, "")
	var refusal struct {
		Message string `json:"message"`
		Params  struct {
			Dependents []string `json:"dependents"`
		} `json:"params"`
	}
	_ = json.Unmarshal([]byte(body), &refusal)
	if code != http.StatusConflict || errCode(body) != CodeImageInUse || len(refusal.Params.Dependents) != maxImageDependents {
		t.Fatalf("delete with %d sandboxes = %d %s, want 409 %s naming %d", len(started), code, body, CodeImageInUse, maxImageDependents)
	}
	for _, id := range refusal.Params.Dependents {
		if !slices.Contains(started, id) || !strings.Contains(refusal.Message, id) {
			t.Errorf("dependent %s is not a sandbox of this image, or the message %q does not name it", id, refusal.Message)
		}
	}

	// Another workspace's member: the group is not theirs, so nothing about
	// the image leaks and nothing is deleted.
	stranger, err := f.st.CreateWorkspace(context.Background(), "sbimages-other-"+b.ID, store.PlanHobby, "sbimages-other-owner-"+b.ID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.st.DeleteTenant(context.Background(), stranger.ID) })
	f.svc.Base.Workspace = fakeWorkspace{"id-a": f.ws, "id-b": stranger.ID}
	for _, method := range []string{http.MethodDelete, http.MethodGet} {
		if code, body := f.doAs(t, "id-b", method, buildPath, ""); code != http.StatusNotFound || errCode(body) != CodeSandboxGroupNotFound || strings.Contains(body, b.ID) {
			t.Errorf("%s from another workspace = %d %s, want 404 %s without the build", method, code, body, CodeSandboxGroupNotFound)
		}
	}
	if code, body := f.doAs(t, "id-b", http.MethodGet, "/v1/sandbox-groups/"+f.group.ID+"/builds", ""); code != http.StatusNotFound || strings.Contains(body, b.ID) {
		t.Errorf("list from another workspace = %d %s, want 404", code, body)
	}
	if code, body := f.do(t, http.MethodGet, buildPath, ""); code != http.StatusOK {
		t.Errorf("the image after the refused deletes = %d %s, want it still there", code, body)
	}
}
