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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestRESTBodyLimitKeepsBlueprintBoundIndependentOfGlobal(t *testing.T) {
	for _, global := range []int64{0, 64, 2 << 20, 8 << 20} {
		t.Run(fmt.Sprint(global), func(t *testing.T) {
			var reached int
			handler := restBodyLimit(withBodyLimit(global))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				reached = len(body)
				w.WriteHeader(http.StatusOK)
			}))
			for _, size := range []int{apps.MaxBlueprintValidationBodyBytes, apps.MaxBlueprintValidationBodyBytes + 1} {
				reached = 0
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, apps.BlueprintValidationPath, bytes.NewReader(make([]byte, size)))
				req.ContentLength = -1
				handler.ServeHTTP(rec, req)
				if size == apps.MaxBlueprintValidationBodyBytes {
					if rec.Code != http.StatusOK || reached != size {
						t.Fatalf("at-limit envelope: %d (read %d)", rec.Code, reached)
					}
				} else if rec.Code != http.StatusRequestEntityTooLarge || reached != 0 || !strings.Contains(rec.Body.String(), "Blueprint manifests are limited to 512 KiB") {
					t.Fatalf("over-limit envelope: %d %s (read %d)", rec.Code, rec.Body.String(), reached)
				}
			}
			for _, path := range []string{"/v1/services", "/v1/blueprints/deploy"} {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(make([]byte, global+1))))
				want := http.StatusRequestEntityTooLarge
				if global == 0 {
					want = http.StatusOK
				}
				if rec.Code != want {
					t.Errorf("%s = %d, want %d", path, rec.Code, want)
				}
			}
		})
	}
}

func TestBlueprintValidationLimitThroughAuthenticatedRouter(t *testing.T) {
	srv := NewServer(&core.Base{Client: fakeClient(), Namespace: "default"}, Deps{})
	srv.HydraAdminURL = fakeHydraURL(t)
	srv.MaxBodyBytes = 2 << 20
	handler := buildHandler(t, srv)
	const limit = 512 << 10
	const valid = "\nservices:\n  - type: web\n    name: web\n    runtime: image\n    image: {url: nginx:1}\n"
	for _, format := range []string{"json", "json-escaped", "multipart"} {
		for _, size := range []int{limit, limit + 1, 600 << 10, 3 << 20} {
			if format == "json-escaped" && size > limit+1 {
				continue
			}
			t.Run(fmt.Sprintf("%s/%d", format, size), func(t *testing.T) {
				manifest := "#" + strings.Repeat("x", size-len(valid)-1) + valid
				var body bytes.Buffer
				contentType := "application/json"
				if format == "multipart" {
					writer := multipart.NewWriter(&body)
					if err := writer.WriteField("ownerId", "tea-a"); err != nil {
						t.Fatal(err)
					}
					part, err := writer.CreateFormFile("file", "render.yaml")
					if err != nil {
						t.Fatal(err)
					}
					if _, err := io.WriteString(part, manifest); err != nil {
						t.Fatal(err)
					}
					if err := writer.Close(); err != nil {
						t.Fatal(err)
					}
					contentType = writer.FormDataContentType()
				} else {
					encoded, err := json.Marshal(map[string]string{"bexYaml": manifest})
					if err != nil {
						t.Fatal(err)
					}
					if format == "json-escaped" {
						encoded = bytes.ReplaceAll(encoded, []byte("x"), []byte(`\u0078`))
					}
					body.Write(encoded)
				}
				req := httptest.NewRequest(http.MethodPost, apps.BlueprintValidationPath, &body)
				req.Header.Set("Content-Type", contentType)
				req.Header.Set("Authorization", "Bearer "+testToken)
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if size == limit {
					var result apps.BlueprintValidation
					if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || !result.Valid {
						t.Fatalf("at-limit manifest: %d %s", rec.Code, rec.Body.String())
					}
				} else {
					var refusal struct{ Message string }
					if rec.Code != http.StatusRequestEntityTooLarge || json.Unmarshal(rec.Body.Bytes(), &refusal) != nil || refusal.Message != "Blueprint manifests are limited to 512 KiB" {
						t.Fatalf("over-limit manifest: %d %s", rec.Code, rec.Body.String())
					}
				}
			})
		}
	}
	for _, body := range []string{`{"bexYaml":1}`, `{"bexYaml":"services: []","typo":true}`, `{"ownerId":"tea-a"}`} {
		if rec := do(t, handler, http.MethodPost, apps.BlueprintValidationPath, testToken, body); rec.Code != http.StatusBadRequest {
			t.Errorf("malformed JSON %s = %d %s", body, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, apps.BlueprintValidationPath, strings.NewReader(valid))
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "text/yaml")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unsupported raw YAML media type = %d %s", rec.Code, rec.Body.String())
	}
	// Authentication must reject before any Blueprint body pre-read.
	body := bytes.NewReader(make([]byte, apps.MaxBlueprintValidationBodyBytes+1))
	req = httptest.NewRequest(http.MethodPost, apps.BlueprintValidationPath, body)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || body.Len() != apps.MaxBlueprintValidationBodyBytes+1 {
		t.Fatalf("unauthenticated upload: %d; %d bytes remain unread", rec.Code, body.Len())
	}
}
