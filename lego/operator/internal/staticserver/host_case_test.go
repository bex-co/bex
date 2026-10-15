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
	"net/http"
	"net/http/httptest"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Hostnames compare case-insensitively; paths do not (w4/309). Every declared
// alias resolves under any letter case, with or without a port, while a
// case-distinct asset path still selects its own object.
func TestHostLookupIgnoresLetterCase(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	const prefix = "tea-aaaaaaaaaaaaaaaaaaaa/mysite/rev-1/"
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: appID, Namespace: "apps"},
		Spec: appv1alpha1.AppSpec{
			Type:   appv1alpha1.TypeStaticSite,
			Host:   testHost,
			Expose: true,
			// A hand-applied declaration in mixed case resolves too.
			Hosts: []string{"WWW.Example.com"},
		},
		Status: appv1alpha1.AppStatus{ActiveRevision: "rev-1", StaticPrefix: prefix},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	resolver := NewCachedResolver(cl, "apps", "onbex.co")
	if err := resolver.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	h := New(resolver, newFakeOrigin(map[string]Object{
		prefix + "index.html": {Body: []byte("home"), ContentType: "text/html"},
		prefix + "Readme.txt": {Body: []byte("upper"), ContentType: "text/plain"},
		prefix + "readme.txt": {Body: []byte("lower"), ContentType: "text/plain"},
	}), 1<<20)

	serve := func(method, host, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://"+host+path, nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	hosts := []string{
		"site.onbex.co", "SITE.ONBEX.CO", "Site.OnBex.co", "SITE.onbex.co:443",
		"mysite.onbex.co", "MySite.ONBEX.co", // computed platform alias
		"www.example.com", "WWW.EXAMPLE.COM:8080", // declared alias
	}
	for _, host := range hosts {
		for _, tc := range []struct{ path, body string }{{"/", "home"}, {"/Readme.txt", "upper"}, {"/readme.txt", "lower"}} {
			if rec := serve(http.MethodGet, host, tc.path); rec.Code != http.StatusOK || rec.Body.String() != tc.body {
				t.Errorf("GET %s%s => %d %q, want 200 %q", host, tc.path, rec.Code, rec.Body, tc.body)
			}
		}
		if rec := serve(http.MethodHead, host, "/readme.txt"); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/plain" {
			t.Errorf("HEAD %s => %d %v, want 200 text/plain", host, rec.Code, rec.Header())
		}
	}

	if rec := serve(http.MethodGet, "OTHER.onbex.co", "/"); rec.Code != http.StatusNotFound || rec.Body.String() != "no static site for host\n" {
		t.Errorf("unknown host => %d %q, want 404", rec.Code, rec.Body)
	}
	if rec := serve(http.MethodPost, "SITE.ONBEX.CO", "/"); rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST => %d Allow %q, want 405 GET, HEAD", rec.Code, rec.Header().Get("Allow"))
	}
}
