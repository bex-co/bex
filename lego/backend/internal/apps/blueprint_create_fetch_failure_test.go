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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/github"
)

// Connecting a Blueprint whose file cannot be fetched answers the classified
// reason and sentence the preview shows, at the status it means: the caller's
// repo, branch, path or GitHub access is a 400, GitHub being rate-limited or
// unreachable a 503. It used to be a bare `500 internal error` for every one
// (w8/134).
func TestCreateBlueprintAnswersItsFetchFailureReason(t *testing.T) {
	for _, tc := range []struct {
		name     string
		path     string
		err      error
		sentinel error
		reason   BlueprintPreviewReason
		wantIn   string
	}{
		{"missing repo", "", fmt.Errorf("%w: %w", github.ErrRepoNotFoundOrNoAccess, &github.APIError{Status: 404}), core.ErrBadRequest, BlueprintFetchRepoNotFound, "GitHub app"},
		{"missing branch", "", fmt.Errorf("%w: %w", github.ErrBranchNotFound, &github.APIError{Status: 404}), core.ErrBadRequest, BlueprintFetchBranchNotFound, `"main"`},
		{"missing file", "", fmt.Errorf("github: file not found: %w", &github.APIError{Status: 404}), core.ErrBadRequest, BlueprintFetchFileNotFound, `"render.yaml"`},
		{"invalid path", "../render.yaml", nil, core.ErrBadRequest, BlueprintFetchInvalidPath, "clean repository-relative path"},
		{"access denied", "", fmt.Errorf("github: denied: %w", &github.APIError{Status: 403}), core.ErrBadRequest, BlueprintFetchAccessDenied, "not authorized"},
		{"ambiguous filename", "", ErrBlueprintFilenameAmbiguous, core.ErrBadRequest, BlueprintFetchAmbiguousFilename, LegacyBlueprintFilename},
		{"rate limited", "", fmt.Errorf("github: limited: %w", &github.APIError{Status: 429}), core.ErrUnavailable, BlueprintFetchRateLimited, "rate-limiting"},
		{"github unreachable", "", fmt.Errorf("github: boom: %w", &github.APIError{Status: 500}), core.ErrUnavailable, BlueprintFetchUnavailable, "Could not reach GitHub"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := newBlueprintService(newFakeBlueprintStore(), fakeWorkspace{"user-a": "tea-a"})
			svc.GitFetcher = fakeBlueprintFetcher{contents: stackManifest, err: tc.err}
			ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
			_, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: "https://github.com/a/app", Branch: "main", Path: tc.path})
			var coded *core.CodedError
			if !errors.Is(err, tc.sentinel) || !errors.As(err, &coded) || coded.Code != string(tc.reason) || !strings.Contains(err.Error(), tc.wantIn) {
				t.Fatalf("CreateBlueprint = %v, want %v coded %q naming %q", err, tc.sentinel, tc.reason, tc.wantIn)
			}
		})
	}
}

// The REST create answers that reason on the wire, never `internal error`.
func TestCreateBlueprintRESTFetchFailureIsNotA500(t *testing.T) {
	svc := newBlueprintService(newFakeBlueprintStore(), fakeWorkspace{"user-a": "tea-a"})
	svc.GitFetcher = fakeBlueprintFetcher{err: fmt.Errorf("%w: %w", github.ErrRepoNotFoundOrNoAccess, &github.APIError{Status: 404})}
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	req := httptest.NewRequest(http.MethodPost, "/v1/blueprints", strings.NewReader(`{"repo":"https://github.com/a/nope","branch":"main","ownerId":"tea-a"}`))
	req = req.WithContext(core.WithIdentity(req.Context(), core.Identity{Subject: "user-a", Method: "oauth2"}))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var body struct{ Code, Message string }
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusBadRequest || body.Code != string(BlueprintFetchRepoNotFound) || !strings.Contains(body.Message, "GitHub app") {
		t.Fatalf("POST /v1/blueprints = %d %s, want 400 %s with its sentence", rec.Code, rec.Body.String(), BlueprintFetchRepoNotFound)
	}
}
