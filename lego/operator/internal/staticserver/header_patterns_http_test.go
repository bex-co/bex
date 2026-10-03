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

package staticserver

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestHeaderPatternsResolvedErrors(t *testing.T) {
	for _, tc := range []struct {
		code      int
		originErr error
	}{{400, nil}, {404, nil}, {413, ErrObjectTooLarge}, {502, errors.New("origin denied")}, {503, ErrOverloaded}} {
		t.Run(strconv.Itoa(tc.code), func(t *testing.T) {
			headers := make([]appv1alpha1.StaticHeader, 1, 5)
			headers[0] = appv1alpha1.StaticHeader{Path: "/**/*.yaml", Name: "X-Selected", Value: "nested"}
			for _, name := range []string{"Content-Type", "Content-Length", "Transfer-Encoding", "Retry-After"} {
				headers = append(headers, appv1alpha1.StaticHeader{Path: "/**/*.yaml", Name: name, Value: "999"})
			}
			site := Site{Headers: headers}
			if tc.code == 400 {
				site.Routes = []appv1alpha1.StaticRoute{{Type: "redirect", Source: "/nested/missing.yaml", Destination: "//unsafe.example"}}
			}
			h, origin := newTestHandler(t, site, nil)
			if tc.originErr != nil {
				origin.errs[key("nested/missing.yaml")] = tc.originErr
			}
			server := httptest.NewServer(h)
			defer server.Close()
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				req, err := http.NewRequestWithContext(t.Context(), method, server.URL+"/nested/missing.yaml", nil)
				if err != nil {
					t.Fatal(err)
				}
				req.Host = testHost
				response, err := server.Client().Do(req)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if err != nil {
					t.Fatal(err)
				}
				if response.StatusCode != tc.code || response.Header.Get("X-Selected") != "nested" {
					t.Errorf("%s response=%d headers=%v", method, response.StatusCode, response.Header)
				}
				if response.Header.Get("Content-Type") != "text/plain; charset=utf-8" || response.Header.Get("Content-Length") == "999" || response.Header.Get("Transfer-Encoding") == "999" {
					t.Errorf("error metadata corrupted: %v", response.Header)
				}
				if tc.code == 503 && response.Header.Get("Retry-After") != "1" {
					t.Error("platform Retry-After overridden")
				}
				if method == http.MethodHead && len(body) != 0 {
					t.Error("HEAD sent body")
				}
				if method == http.MethodGet && len(body) == 0 {
					t.Error("error body missing")
				}
			}
		})
	}
}

func TestHeaderPatternsDoNotExposeNonStaticApps(t *testing.T) {
	for _, kind := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob} {
		t.Run(kind, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := appv1alpha1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: appID, Namespace: "apps"}, Spec: appv1alpha1.AppSpec{Type: kind, Host: testHost, Headers: []appv1alpha1.StaticHeader{{Path: "/*", Name: "X-All", Value: "all"}, {Path: "/*.yaml", Name: "X-Root", Value: "root"}}}, Status: appv1alpha1.AppStatus{ActiveRevision: rev}}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
			resolver := NewCachedResolver(cl, app.Namespace, "onbex.co")
			if err := resolver.Refresh(t.Context()); err != nil {
				t.Fatal(err)
			}
			origin := newFakeOrigin(map[string]Object{key("render.yaml"): {Body: []byte("private")}})
			h := New(resolver, origin, 1<<20)
			for _, method := range []string{http.MethodGet, http.MethodHead} {
				rec := do(h, method, "/render.yaml")
				if rec.Code != 404 || rec.Header().Get("X-All") != "" || rec.Header().Get("X-Root") != "" {
					t.Fatalf("nonstatic request exposed site: %d %v", rec.Code, rec.Header())
				}
			}
			if len(origin.gets) != 0 {
				t.Fatal("nonstatic request read static origin")
			}
		})
	}
}

func TestHeaderPatternsDecodeSlashExactlyOnce(t *testing.T) {
	h, _ := newTestHandler(t, Site{Headers: []appv1alpha1.StaticHeader{
		{Path: "/*.yaml", Name: "X-Root", Value: "root"},
		{Path: "/**/*.yaml", Name: "X-Nested", Value: "nested"},
	}}, map[string]Object{
		key("nested/file.yaml"):   {Body: []byte("nested object")},
		key("nested%2Ffile.yaml"): {Body: []byte("literal encoded slash")},
	})
	for _, tc := range []struct{ path, body, root, nested string }{
		{"/nested%2Ffile.yaml", "nested object", "", "nested"},
		{"/nested%252Ffile.yaml", "literal encoded slash", "root", ""},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+tc.path, func(t *testing.T) {
				rec := do(h, method, tc.path)
				if rec.Code != http.StatusOK || rec.Header().Get("X-Root") != tc.root || rec.Header().Get("X-Nested") != tc.nested {
					t.Fatalf("response=%d headers=%v", rec.Code, rec.Header())
				}
				if method == http.MethodGet && rec.Body.String() != tc.body {
					t.Fatalf("body=%q want%q", rec.Body.String(), tc.body)
				}
				if method == http.MethodHead && rec.Body.Len() != 0 {
					t.Fatal("HEAD sent body")
				}
			})
		}
	}
}
