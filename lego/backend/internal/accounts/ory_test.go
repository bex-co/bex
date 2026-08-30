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

package accounts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestOryCleanerEscapesSubjectAndTreatsAbsentAsConverged(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		if len(paths)%2 == 0 {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cleaner := NewOryCleaner(server.URL, server.URL)
	cleaner.Client = server.Client()
	subject := "identity/with ? delimiters"
	if err := cleaner.CleanupSubject(context.Background(), subject); err != nil {
		t.Fatalf("Hydra cleanup: %v", err)
	}
	if err := cleaner.DeleteSessions(context.Background(), subject); err != nil {
		t.Fatalf("Kratos session cleanup: %v", err)
	}
	if err := cleaner.DeleteIdentity(context.Background(), subject); err != nil {
		t.Fatalf("Kratos identity cleanup: %v", err)
	}
	want := []string{
		"/admin/oauth2/auth/sessions/consent?subject=identity%2Fwith+%3F+delimiters&all=true",
		"/admin/oauth2/auth/sessions/login?subject=identity%2Fwith+%3F+delimiters",
		"/admin/identities/identity%2Fwith%20%3F%20delimiters/sessions",
		"/admin/identities/identity%2Fwith%20%3F%20delimiters",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths=%v want=%v", paths, want)
	}
}

func TestOryCleanerRetriesNonterminalResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "dependency unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	cleaner := NewOryCleaner(server.URL, server.URL)
	cleaner.Client = server.Client()

	if err := cleaner.DeleteIdentity(context.Background(), "identity-a"); err == nil {
		t.Fatal("503 identity deletion converged; want retryable error")
	}
}

func TestOryCleanerTreatsTimeoutAsRetryable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	cleaner := NewOryCleaner(server.URL, server.URL)
	cleaner.Client = &http.Client{Timeout: 25 * time.Millisecond}
	if err := cleaner.CleanupSubject(context.Background(), "identity-a"); err == nil {
		t.Fatal("timed-out Hydra cleanup converged; want retryable error")
	}
}
