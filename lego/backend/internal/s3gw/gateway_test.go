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

package s3gw

import (
	"encoding/xml"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

type errorDoc struct {
	XMLName   xml.Name `xml:"Error"`
	Code      string   `xml:"Code"`
	Message   string   `xml:"Message"`
	Resource  string   `xml:"Resource"`
	RequestID string   `xml:"RequestId"`
}

func doReq(t *testing.T, h http.Handler, method, target string) (*httptest.ResponseRecorder, errorDoc) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	var doc errorDoc
	if rec.Body.Len() > 0 {
		if err := xml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
			t.Fatalf("%s %s: the body is not an S3 error document: %v\n%s", method, target, err, rec.Body.String())
		}
	}
	return rec, doc
}

// nonS3Paths are routes of the other bex surfaces (REST, GraphQL, MCP, OAuth,
// probes, metrics). On the S3 host every one of them is a bucket or a key.
var nonS3Paths = []string{
	"/graphql", "/mcp", "/mcp/cloud-agents", "/v1/services", "/v1/objects/tea-x/oregon", "/healthz", "/readyz", "/metrics",
	"/.well-known/oauth-authorization-server", "/oauth/token", "/internal/object-gateway/egress", "/",
}

func TestADegradedGatewayAnswersS3sServiceUnavailableToEveryPath(t *testing.T) {
	g := NewDegraded()
	for _, p := range nonS3Paths {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete} {
			rec, doc := doReq(t, g, method, p)
			if rec.Code != http.StatusServiceUnavailable || doc.Code != "ServiceUnavailable" {
				t.Errorf("%s %s = %d %q, want 503 ServiceUnavailable", method, p, rec.Code, doc.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/xml" {
				t.Errorf("%s %s Content-Type = %q, want application/xml", method, p, ct)
			}
		}
	}
	rec, doc := doReq(t, g, http.MethodGet, "/photos/cat.jpg")
	id := rec.Header().Get("x-amz-request-id")
	if !regexp.MustCompile(`^[0-9A-F]{16}$`).MatchString(id) || doc.RequestID != id {
		t.Errorf("request id header %q and body %q must be one 16-hex id", id, doc.RequestID)
	}
	if doc.Resource != "/photos/cat.jpg" {
		t.Errorf("Resource = %q, want the request path", doc.Resource)
	}
	// No answer names Amazon (ADR112 Consequences).
	if strings.Contains(strings.ToLower(rec.Header().Get("Server")+rec.Body.String()), "amazon") {
		t.Error("an answer names Amazon")
	}
}

func TestAHeadAnswerCarriesTheStatusAndNoBody(t *testing.T) {
	rec, _ := doReq(t, NewDegraded(), http.MethodHead, "/photos")
	if rec.Code != http.StatusServiceUnavailable || rec.Body.Len() != 0 {
		t.Errorf("HEAD = %d with %d body bytes, want 503 and none", rec.Code, rec.Body.Len())
	}
}

func TestAGatewayWithNoKeyServiceOrObjectStoreIsDegraded(t *testing.T) {
	// A Service with no stores and Keys with no secret store are the two ways a
	// deployment lacks Wasabi or OpenBao.
	g := New(Config{Service: objectstorage.NewService(nil, nil), Keys: objectstorage.NewAccessKeys(nil, nil)})
	rec, doc := doReq(t, g, http.MethodGet, "/photos")
	if rec.Code != http.StatusServiceUnavailable || doc.Code != "ServiceUnavailable" {
		t.Errorf("an unconfigured gateway answered %d %q, want 503 ServiceUnavailable", rec.Code, doc.Code)
	}
}

// The package must not be able to mount another surface: its imports are the
// guard, because a route cannot be mounted without the package that serves it.
func TestThePackageImportsNoRESTGraphQLMCPOrOAuthSurface(t *testing.T) {
	forbidden := []string{
		"/internal/api", "graphql-go", "/internal/mcputil", "/internal/gqlutil", "/internal/oauthgrant", "/internal/cliauth", "/internal/apikeys",
		"modelcontextprotocol", "/internal/objectgw",
	}
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if strings.Contains(path, bad) {
					t.Errorf("%s imports %s: bex-s3 serves S3 and no other surface", f, path)
				}
			}
		}
	}
}

func TestTheServerSetsNoOverallTimeoutAndBoundsOnlyTheHeaderRead(t *testing.T) {
	srv := NewServer(Addr, NewDegraded())
	if srv.ReadTimeout != 0 || srv.WriteTimeout != 0 {
		t.Errorf("Read/WriteTimeout = %v/%v: an upload of 5 GiB takes longer, so each request sets its own", srv.ReadTimeout, srv.WriteTimeout)
	}
	if srv.ReadHeaderTimeout <= 0 {
		t.Error("the header read is unbounded")
	}
	if _, err := os.Stat("gateway.go"); err != nil {
		t.Fatal(err)
	}
}

func xmlUnmarshal(b []byte, v any) error { return xml.Unmarshal(b, v) }
