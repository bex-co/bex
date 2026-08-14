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

package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type helperBody struct {
	Name string `json:"name"`
}

func TestDecodeBodyDecodesAValidBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/things", strings.NewReader(`{"name":"web"}`))
	body, err := DecodeBody[helperBody](r)
	if err != nil {
		t.Fatalf("DecodeBody: %v", err)
	}
	if body.Name != "web" {
		t.Fatalf("Name = %q, want %q", body.Name, "web")
	}
}

func TestDecodeBodyMapsAMalformedBodyToErrBadRequest(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/things", strings.NewReader(`not-json`))
	body, err := DecodeBody[helperBody](r)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("err = %v, want ErrBadRequest", err)
	}
	if body != (helperBody{}) {
		t.Fatalf("body = %+v, want zero value", body)
	}

	// Through the standard handler shape the sentinel becomes the exact
	// Render-dialect 400 envelope.
	handler := HandleJSON(http.StatusOK, func(r *http.Request) (any, error) {
		decoded, err := DecodeBody[helperBody](r)
		if err != nil {
			return nil, err
		}
		return decoded, nil
	})
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, "/v1/things", strings.NewReader(`{"name":`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	want := `{"error":"bad request","id":"bad_request","message":"bad request"}` + "\n"
	if got := w.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestDryRunRequestedFoldsBodyAndQuery(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		body  bool
		want  bool
	}{
		{name: "neither", query: "/v1/things", body: false, want: false},
		{name: "body only", query: "/v1/things", body: true, want: true},
		{name: "query only", query: "/v1/things?dryRun=true", body: false, want: true},
		{name: "both", query: "/v1/things?dryRun=true", body: true, want: true},
		{name: "query not the literal true", query: "/v1/things?dryRun=1", body: false, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, tc.query, nil)
			if got := DryRunRequested(r, tc.body); got != tc.want {
				t.Fatalf("DryRunRequested(%q, body=%v) = %v, want %v", tc.query, tc.body, got, tc.want)
			}
		})
	}
}

func TestHandleByIDAnswers200WithTheServiceResult(t *testing.T) {
	handler := HandleByID(func(_ context.Context, id string) (helperBody, error) {
		return helperBody{Name: id}, nil
	})
	r := httptest.NewRequest(http.MethodGet, "/v1/things/srv-1", nil)
	r.SetPathValue("id", "srv-1")
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if want := `{"name":"srv-1"}` + "\n"; w.Body.String() != want {
		t.Fatalf("body = %q, want %q", w.Body.String(), want)
	}
}

func TestHandleByIDMapsAServiceError(t *testing.T) {
	handler := HandleByID(func(_ context.Context, _ string) (helperBody, error) {
		return helperBody{}, ErrNotFound
	})
	r := httptest.NewRequest(http.MethodGet, "/v1/things/missing", nil)
	r.SetPathValue("id", "missing")
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	want := `{"error":"app not found","id":"not_found","message":"app not found"}` + "\n"
	if got := w.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHandleMappedProjectsTheResultThroughTheView(t *testing.T) {
	handler := HandleMapped(http.StatusCreated, func(r *http.Request) (string, error) {
		return "web", nil
	}, func(name string) helperBody {
		return helperBody{Name: name + "-rendered"}
	})
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodPost, "/v1/things", nil))
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", w.Code)
	}
	if want := `{"name":"web-rendered"}` + "\n"; w.Body.String() != want {
		t.Fatalf("body = %q, want %q", w.Body.String(), want)
	}
}

func TestHandleMappedMapsAServiceErrorWithoutViewing(t *testing.T) {
	handler := HandleMapped(http.StatusOK, func(r *http.Request) (string, error) {
		return "", ErrForbidden
	}, func(string) helperBody {
		t.Fatal("view must not run on a service error")
		return helperBody{}
	})
	w := httptest.NewRecorder()
	handler(w, httptest.NewRequest(http.MethodGet, "/v1/things", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	want := `{"error":"forbidden","id":"forbidden","message":"forbidden"}` + "\n"
	if got := w.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
