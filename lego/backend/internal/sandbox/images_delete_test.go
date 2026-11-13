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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w3/m367: Render's delete-sandbox-build. A build a sandbox still runs from is
// kept (409 image_in_use, naming the sandbox); once that sandbox has
// terminated, the delete answers 204, the build leaves every read and the
// create-by-name, its registry image is gone, a repeat answers 204 again, and
// the same image imports as a new build.
func TestDeleteImageAsOnRender(t *testing.T) {
	f := newImageFixture(t)
	f.svc.Images.Reaper = &RegistryReaper{RegistryConfig: RegistryConfig{BaseURL: f.sink.URL, Prefix: DefaultSnapshotRepoPrefix}}
	ref := hostOf(f.source) + "/library/py:3.13"
	pushImage(t, ref, linuxAMD64, 256)
	b := f.mustImport(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref))
	waitSucceeded(t, f.st, f.ws, f.group.ID, b.ID)
	buildPath := "/v1/sandbox-groups/" + f.group.ID + "/builds/" + b.ID

	code, body := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildId":%q}`, f.ws, f.group.ID, b.ID))
	var sb Sandbox
	if err := json.Unmarshal([]byte(body), &sb); code != http.StatusCreated || err != nil {
		t.Fatalf("create from the build = %d %s", code, body)
	}
	if code, body := f.do(t, http.MethodDelete, buildPath, ""); code != http.StatusConflict || errCode(body) != CodeImageInUse || !strings.Contains(body, sb.ID) {
		t.Fatalf("delete while %s runs = %d %s, want 409 %s naming it", sb.ID, code, body, CodeImageInUse)
	}
	// The refused delete changed nothing: the build reads and starts sandboxes.
	if code, body := f.do(t, http.MethodGet, buildPath, ""); code != http.StatusOK {
		t.Fatalf("get after a refused delete = %d %s, want 200", code, body)
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildName":"py"}`, f.ws, f.group.ID)); code != http.StatusCreated {
		t.Fatalf("create by name after a refused delete = %d %s, want 201", code, body)
	} else {
		var second Sandbox
		_ = json.Unmarshal([]byte(body), &second)
		if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+second.ID+"/terminate?ownerId="+f.ws, ""); code >= 300 {
			t.Fatalf("terminate %s = %d %s", second.ID, code, body)
		}
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes/"+sb.ID+"/terminate?ownerId="+f.ws, ""); code >= 300 {
		t.Fatalf("terminate = %d %s", code, body)
	}

	if code, body := f.do(t, http.MethodDelete, buildPath, ""); code != http.StatusNoContent || body != "" {
		t.Fatalf("delete = %d %q, want 204 and no body", code, body)
	}
	if code, body := f.do(t, http.MethodGet, buildPath, ""); code != http.StatusNotFound || errCode(body) != CodeBuildNotFound {
		t.Errorf("get after delete = %d %s, want 404 %s", code, body, CodeBuildNotFound)
	}
	if code, body := f.do(t, http.MethodGet, "/v1/sandbox-groups/"+f.group.ID+"/builds", ""); code != http.StatusOK || strings.Contains(body, b.ID) {
		t.Errorf("list after delete = %d %s, want the build gone", code, body)
	}
	if code, body := f.do(t, http.MethodPost, "/v1/sandboxes", fmt.Sprintf(`{"ownerId":%q,"sandboxGroupId":%q,"buildName":"py"}`, f.ws, f.group.ID)); code != http.StatusNotFound || errCode(body) != CodeBuildNotFound {
		t.Errorf("create by name after delete = %d %s, want 404 %s", code, body, CodeBuildNotFound)
	}
	stored, err := name.ParseReference(fmt.Sprintf("%s/snapshots/%s/images/%s:image", hostOf(f.sink), store.SandboxNamespace(f.ws), b.ID), name.Insecure)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := remote.Head(stored); err == nil {
		t.Error("the deleted build's image is still in the registry")
	}
	if code, body := f.do(t, http.MethodDelete, buildPath, ""); code != http.StatusNoContent {
		t.Errorf("repeated delete = %d %s, want 204", code, body)
	}
	if code, body := f.do(t, http.MethodDelete, "/v1/sandbox-groups/"+f.group.ID+"/builds/sbb-00000000000000000000", ""); code != http.StatusNotFound || errCode(body) != CodeBuildNotFound {
		t.Errorf("delete of an unknown build = %d %s, want 404", code, body)
	}
	if again := f.mustImport(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref)); again.ID == b.ID {
		t.Errorf("re-import after delete returned the deleted build %s", b.ID)
	}
}

// A build reads Render's failureCode: null while it runs or once it has
// succeeded, and the kind of failure once it failed. The classifier names
// source-registry refusals by kind and anything bex's own side did as
// internal_error.
func TestBuildFailureCode(t *testing.T) {
	f := newImageFixture(t)
	ref := hostOf(f.source) + "/library/py:3.13"
	pushImage(t, ref, linuxAMD64, 128)
	ok := f.waitBuild(t, f.mustImport(t, fmt.Sprintf(`{"name":"py","sourceImage":%q}`, ref)).ID)
	if ok.Status != store.SandboxBuildSucceeded || ok.FailureCode != nil {
		t.Fatalf("succeeded build = %+v, want a null failureCode", ok)
	}

	for _, tc := range []struct {
		err  error
		want string
	}{
		{sourceError{&transport.Error{StatusCode: http.StatusUnauthorized}}, "image_access_denied"},
		{sourceError{&transport.Error{StatusCode: http.StatusForbidden}}, "image_access_denied"},
		{sourceError{&transport.Error{StatusCode: http.StatusNotFound}}, "image_not_found"},
		{sourceError{&transport.Error{StatusCode: http.StatusBadGateway}}, "image_pull_failed"},
		{sourceError{errors.New("connection reset")}, "image_pull_failed"},
		{fmt.Errorf("copy: %w", context.DeadlineExceeded), "deadline_exceeded"},
		{&transport.Error{StatusCode: http.StatusNotFound}, "internal_error"}, // the sandbox registry's own 404
		{errors.New("the sandbox registry credential is unavailable"), "internal_error"},
	} {
		if got := buildFailureCode(tc.err); got != tc.want {
			t.Errorf("buildFailureCode(%v) = %q, want %q", tc.err, got, tc.want)
		}
	}
	// A source layer read inside the write (the source image is lazy) is the
	// source's refusal; the sandbox registry's own error is not.
	src, _ := http.NewRequest(http.MethodGet, "https://registry.example/v2/library/py/blobs/sha256:aa", nil)
	sink, _ := http.NewRequest(http.MethodPut, "https://sink.example/v2/x/manifests/image", nil)
	if !fromOtherHost(fmt.Errorf("write: %w", &transport.Error{StatusCode: http.StatusForbidden, Request: src}), "sink.example") {
		t.Error("a source blob refusal during the write was not read as the source's")
	}
	if fromOtherHost(&transport.Error{StatusCode: http.StatusForbidden, Request: sink}, "sink.example") || fromOtherHost(errors.New("boom"), "sink.example") {
		t.Error("the sandbox registry's own error was read as the source's")
	}
	// A build that failed before codes were recorded reads internal_error.
	if got := failureCode(store.SandboxBuild{Status: store.SandboxBuildFailed}); got == nil || *got != "internal_error" {
		t.Errorf("legacy failed build failureCode = %v, want internal_error", got)
	}
}
