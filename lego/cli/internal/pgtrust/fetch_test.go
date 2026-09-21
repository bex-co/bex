package pgtrust

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testDatabaseID = "dpg-d0000000000000000000ab"

// stubAPI stands in for bex-api's connection-info endpoint and records the path
// the launcher asked for.
func stubAPI(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	t.Setenv("RENDER_HOST", server.URL+"/v1/")
	t.Setenv("RENDER_API_KEY", "test-token")
	return server, &path
}

func TestFetcherReadsTheServerCACertificate(t *testing.T) {
	_, path := stubAPI(t, http.StatusOK, `{
		"externalConnectionString":"postgresql://u:p@db.example.test:5432/d?sslmode=verify-full",
		"serverCaCertificate":"`+strings.ReplaceAll(testCertificate, "\n", "\\n")+`"}`)

	got, err := NewFetcher()(context.Background(), testDatabaseID)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got != testCertificate {
		t.Fatalf("server CA = %q, want the endpoint's certificate", got)
	}
	if *path != "/v1/postgres/"+testDatabaseID+"/connection-info" {
		t.Fatalf("request path = %q", *path)
	}
}

func TestFetcherReportsAnUnavailableServerCA(t *testing.T) {
	_, _ = stubAPI(t, http.StatusServiceUnavailable,
		`{"error":"service unavailable: Postgres \"`+testDatabaseID+`\" server CA is not provisioned yet; retry shortly",`+
			`"message":"service unavailable: Postgres \"`+testDatabaseID+`\" server CA is not provisioned yet; retry shortly","id":"..."}`)

	_, err := NewFetcher()(context.Background(), testDatabaseID)
	if err == nil {
		t.Fatal("a 503 connection-info read was reported as success")
	}
	if !errors.Is(err, ErrCAUnavailable) {
		t.Fatalf("error is not an unavailable-CA error: %v", err)
	}
	if !strings.Contains(err.Error(), "server CA is not provisioned yet") {
		t.Fatalf("error dropped the endpoint's own explanation: %v", err)
	}
}

// An internal-only database omits the field entirely; that is not a failure.
func TestFetcherReturnsNoCertificateForAnInternalOnlyDatabase(t *testing.T) {
	_, _ = stubAPI(t, http.StatusOK,
		`{"internalConnectionString":"postgresql://u:p@db-rw.ns.svc:5432/d","password":"p"}`)

	got, err := NewFetcher()(context.Background(), testDatabaseID)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got != "" {
		t.Fatalf("server CA = %q, want none", got)
	}
}

// Any other status belongs to the delegated command's own error reporting.
func TestFetcherReportsOtherStatusesWithoutTheUnavailableMarker(t *testing.T) {
	_, _ = stubAPI(t, http.StatusNotFound, `{"message":"not found"}`)

	_, err := NewFetcher()(context.Background(), testDatabaseID)
	if err == nil {
		t.Fatal("a 404 connection-info read was reported as success")
	}
	if errors.Is(err, ErrCAUnavailable) {
		t.Fatalf("a 404 was classified as a missing CA: %v", err)
	}
}
