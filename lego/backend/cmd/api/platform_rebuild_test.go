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

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePlatformRebuildArgs(t *testing.T) {
	req, err := parsePlatformRebuildArgs([]string{"--since", "2026-10-23T09:33:41Z", "--apply", "--limit", "5"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !req.Apply || req.Limit != 5 || req.Since.Format("2006-01-02T15:04:05Z07:00") != "2026-10-23T09:33:41Z" {
		t.Errorf("request = %+v", req)
	}
	req, err = parsePlatformRebuildArgs([]string{"--since", "2026-10-23T09:33:41Z"})
	if err != nil || req.Apply || req.Limit != 10 {
		t.Errorf("defaults = %+v, %v; want a dry run with limit 10", req, err)
	}
	for _, bad := range [][]string{
		nil,
		{"--since", "yesterday"},
		{"--since", "2026-10-23T09:33:41Z", "--limit", "0"},
		{"--since", "2026-10-23T09:33:41Z", "srv-1"},
	} {
		if _, err := parsePlatformRebuildArgs(bad); err == nil {
			t.Errorf("parse %q: want a usage error", bad)
		}
	}
}

func TestLoopbackURL(t *testing.T) {
	for addr, want := range map[string]string{":8091": "http://127.0.0.1:8091", "0.0.0.0:9000": "http://127.0.0.1:9000"} {
		if got, err := loopbackURL(addr); err != nil || got != want {
			t.Errorf("loopbackURL(%q) = %q, %v; want %q", addr, got, err, want)
		}
	}
}

// The :8091 listener serves before the deploy service exists; until the verb
// is bound it must refuse rather than 404 or panic.
func TestLateHandlerServesUnavailableUntilSet(t *testing.T) {
	var l lateHandler
	rec := httptest.NewRecorder()
	l.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unset: status %d, want 503", rec.Code)
	}
	l.set(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	rec = httptest.NewRecorder()
	l.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("set: status %d, want the bound handler's", rec.Code)
	}
}
