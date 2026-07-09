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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// appWithHosts builds a sample App with spec.hosts set.
func appWithHosts(name string, hosts ...string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Hosts = hosts
	return a
}

// tlsSecret creates a fake TLS Secret as cert-manager would.
func tlsSecret(namespace, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("fake-cert"),
			"tls.key": []byte("fake-key"),
		},
	}
}

// --- Service verbs ---

func TestListDomainsEmpty(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	domains, err := svc.ListDomains(context.Background(), "web")
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) != 0 {
		t.Errorf("no spec.hosts => empty list, got %v", domains)
	}
}

func TestListDomainsReturnsHosts(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("web", "www.example.com", "api.example.com"))
	domains, err := svc.ListDomains(context.Background(), "web")
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) != 2 {
		t.Fatalf("want 2 domains, got %d", len(domains))
	}
	if domains[0].Name != "www.example.com" || domains[1].Name != "api.example.com" {
		t.Errorf("domain names wrong: %v", domains)
	}
}

func TestGetDomainFoundAndNotFound(t *testing.T) {
	svc, _ := newService(nil, appWithHosts("web", "www.example.com"))

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain found: %v", err)
	}
	if d.Name != "www.example.com" {
		t.Errorf("name = %q", d.Name)
	}

	if _, err := svc.GetDomain(context.Background(), "web", "nope.example.com"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing domain => ErrNotFound, got %v", err)
	}
}

func TestAddDomainAppendsToHosts(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	d, err := svc.AddDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if d.Name != "www.example.com" {
		t.Errorf("returned name = %q", d.Name)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "www.example.com" {
		t.Errorf("spec.hosts = %v, want [www.example.com]", got)
	}
}

func TestAddDomainIdempotent(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	if _, err := svc.AddDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// Second add of the same hostname must be a no-op (still one entry).
	if _, err := svc.AddDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("second add: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 {
		t.Errorf("duplicate add must not create a second entry, got %v", got)
	}
}

func TestAddDomainEmptyHostnameIsBadRequest(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	if _, err := svc.AddDomain(context.Background(), "web", ""); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("empty hostname => ErrBadRequest, got %v", err)
	}
}

func TestDeleteDomainRemovesFromHosts(t *testing.T) {
	svc, cl := newService(nil, appWithHosts("web", "www.example.com", "api.example.com"))

	if err := svc.DeleteDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "api.example.com" {
		t.Errorf("spec.hosts = %v, want [api.example.com]", got)
	}
}

func TestDeleteDomainIdempotent(t *testing.T) {
	svc, cl := newService(nil, appWithHosts("web", "www.example.com"))

	// Remove existing host.
	if err := svc.DeleteDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("first delete: %v", err)
	}
	// Removing again must be a silent no-op.
	if err := svc.DeleteDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("second delete (no-op) should not error: %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("spec.hosts should be empty, got %v", got)
	}
}

// --- Store write-through for managed Apps ---

func TestAddDomainManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.AddDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if len(rec.domainAdds) != 1 || rec.domainAdds[0].id != "srv-1" || rec.domainAdds[0].host != "www.example.com" {
		t.Fatalf("want row write [srv-1 www.example.com], got %v", rec.domainAdds)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 1 || got[0] != "www.example.com" {
		t.Errorf("CR spec.hosts = %v, want [www.example.com]", got)
	}
}

func TestDeleteDomainManagedAppWritesRowThenCR(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, managedAppWithHosts("web", "srv-1", "www.example.com"))

	if err := svc.DeleteDomain(context.Background(), "web", "www.example.com"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	if len(rec.domainRems) != 1 || rec.domainRems[0].id != "srv-1" || rec.domainRems[0].host != "www.example.com" {
		t.Fatalf("want row remove [srv-1 www.example.com], got %v", rec.domainRems)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("CR spec.hosts should be empty, got %v", got)
	}
}

func TestAddDomainUnmanagedAppSkipsStore(t *testing.T) {
	rec := &recordingStore{}
	svc, cl := newService(rec, sampleApp("hand"))

	if _, err := svc.AddDomain(context.Background(), "hand", "www.example.com"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if len(rec.domainAdds) != 0 {
		t.Fatalf("unmanaged app must not touch the store, got %v", rec.domainAdds)
	}
	if got := getApp(t, cl, "hand").Spec.Hosts; len(got) != 1 || got[0] != "www.example.com" {
		t.Errorf("CR spec.hosts = %v", got)
	}
}

func TestAddDomainRowWriteFailureLeavesCRUntouched(t *testing.T) {
	rec := &recordingStore{err: errors.New("db down")}
	svc, cl := newService(rec, managedApp("web", "srv-1"))

	if _, err := svc.AddDomain(context.Background(), "web", "www.example.com"); err == nil {
		t.Fatal("want error when row write fails")
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("CR spec.hosts must be untouched when row write failed, got %v", got)
	}
}

// --- Verification status (TLS Secret lookup) ---

func TestDomainVerifiedWhenTLSSecretExists(t *testing.T) {
	// spec.host is set, so spec.hosts[] items are secondary → secret = "<app>-tls-<host>"
	a := appWithHosts("web", "www.example.com")
	a.Spec.Host = "web.onbex.co"
	secret := tlsSecret("default", "web-tls-www.example.com")
	cl := fakeClient(a, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.VerificationStatus != "verified" {
		t.Errorf("verificationStatus = %q, want verified (TLS secret present)", d.VerificationStatus)
	}
	if d.ServerStatus != "active" {
		t.Errorf("serverStatus = %q, want active (cert issued and not suspended)", d.ServerStatus)
	}
}

func TestDomainPendingWithoutTLSSecret(t *testing.T) {
	a := appWithHosts("web", "www.example.com")
	a.Spec.Host = "web.onbex.co"
	cl := fakeClient(a) // no TLS secret
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.VerificationStatus != "pending" {
		t.Errorf("verificationStatus = %q, want pending (no TLS secret)", d.VerificationStatus)
	}
	if d.ServerStatus != "pending" {
		t.Errorf("serverStatus = %q, want pending", d.ServerStatus)
	}
}

func TestDomainServerStatusPendingWhenSuspended(t *testing.T) {
	a := appWithHosts("web", "www.example.com")
	a.Spec.Host = "web.onbex.co"
	a.Spec.Suspended = true
	secret := tlsSecret("default", "web-tls-www.example.com")
	cl := fakeClient(a, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	// Cert issued but app is suspended → serverStatus must be pending.
	if d.VerificationStatus != "verified" {
		t.Errorf("verificationStatus = %q, want verified", d.VerificationStatus)
	}
	if d.ServerStatus != "pending" {
		t.Errorf("serverStatus = %q, want pending (app suspended)", d.ServerStatus)
	}
}

// tlsSecretNameForFirstHost: if spec.host is unset and expose is false, spec.hosts[0]
// is the first effective host and gets the "<app>-tls" secret (not "<app>-tls-<host>").
func TestTLSSecretForFirstHostWhenNoPrimaryHost(t *testing.T) {
	a := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Hosts: []string{"www.example.com"}},
	}
	secret := tlsSecret("default", "web-tls") // first effective host → "<app>-tls"
	cl := fakeClient(a, secret)
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}

	d, err := svc.GetDomain(context.Background(), "web", "www.example.com")
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if d.VerificationStatus != "verified" {
		t.Errorf("verificationStatus = %q, want verified (first host uses <app>-tls)", d.VerificationStatus)
	}
}

// --- domainType heuristic ---

func TestDomainTypeClassification(t *testing.T) {
	cases := map[string]string{
		"example.com":     "apex",
		"www.example.com": "subdomain",
		"api.example.com": "subdomain",
		"a.b.example.com": "subdomain",
	}
	for hostname, want := range cases {
		if got := domainType(hostname); got != want {
			t.Errorf("domainType(%q) = %q, want %q", hostname, got, want)
		}
	}
}

// --- REST fragment ---

func TestRESTCustomDomains(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// POST adds and returns 201.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/custom-domains",
		strings.NewReader(`{"name":"www.example.com"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST => 201, got %d: %s", rec.Code, rec.Body)
	}
	var created renderCustomDomain
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Name != "www.example.com" {
		t.Errorf("name = %q", created.Name)
	}
	if created.DomainType != "subdomain" {
		t.Errorf("domainType = %q, want subdomain", created.DomainType)
	}

	// GET list returns [{customDomain:{...}, cursor:"..."}].
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/custom-domains", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET list => 200, got %d", rec.Code)
	}
	var list []customDomainWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if list[0].CustomDomain.Name != "www.example.com" || list[0].Cursor == "" {
		t.Errorf("list item wrong: %+v", list[0])
	}

	// GET single returns the domain.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/custom-domains/www.example.com", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET single => 200, got %d", rec.Code)
	}

	// DELETE returns 204 No Content.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/v1/services/web/custom-domains/www.example.com", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE => 204, got %d", rec.Code)
	}
	if got := getApp(t, cl, "web").Spec.Hosts; len(got) != 0 {
		t.Errorf("spec.hosts should be empty after delete, got %v", got)
	}

	// GET single after delete => 404.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web/custom-domains/www.example.com", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("deleted domain => 404, got %d", rec.Code)
	}
}

func TestRESTCustomDomainsBadRequest(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Missing name field => 400.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services/web/custom-domains",
		strings.NewReader(`{"name":""}`)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty name => 400, got %d", rec.Code)
	}
}

func TestRESTCustomDomainsUnknownApp(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/nope/custom-domains", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown app => 404, got %d", rec.Code)
	}
}

// --- GraphQL fragment ---

func TestGraphQLCustomDomains(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	// addCustomDomain mutation.
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { addCustomDomain(id: "web", name: "www.example.com") { name domainType verificationStatus } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("addCustomDomain: %v", res.Errors)
	}
	added := res.Data.(map[string]any)["addCustomDomain"].(map[string]any)
	if added["name"] != "www.example.com" || added["domainType"] != "subdomain" {
		t.Errorf("addCustomDomain result wrong: %v", added)
	}

	// customDomains query.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ customDomains(id: "web") { name verificationStatus } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("customDomains: %v", res.Errors)
	}
	domains := res.Data.(map[string]any)["customDomains"].([]any)
	if len(domains) != 1 {
		t.Fatalf("want 1 domain, got %d", len(domains))
	}

	// customDomain query (single).
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ customDomain(id: "web", name: "www.example.com") { name } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("customDomain: %v", res.Errors)
	}

	// deleteCustomDomain mutation.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { deleteCustomDomain(id: "web", name: "www.example.com") }`})
	if len(res.Errors) > 0 {
		t.Fatalf("deleteCustomDomain: %v", res.Errors)
	}
	if deleted := res.Data.(map[string]any)["deleteCustomDomain"]; deleted != true {
		t.Errorf("deleteCustomDomain should return true on success, got %v", deleted)
	}
}

// --- helpers ---

func managedAppWithHosts(name, appID string, hosts ...string) *appv1alpha1.App {
	a := managedApp(name, appID)
	a.Spec.Hosts = hosts
	return a
}
