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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w21/m394 t012 (R25-11): a caller waiting on the 16-slot custom maintenance
// fetch semaphore blocked until a slot freed or its own request ended, with no
// stale fallback while it waited, so 16 slow tenant URLs delayed every other
// tenant's maintenance page.

// busyCache is a cache whose every fetch slot is taken and whose origin must
// not be reached.
func busyCache(t *testing.T) *customPageCache {
	t.Helper()
	c := newCustomPageCache()
	for range customPageMaxFetches {
		c.fetchSem <- struct{}{}
	}
	c.origin = func(context.Context, *appv1alpha1.App, *url.URL, func(string) bool) (*http.Response, error) {
		t.Error("a fetch reached the origin while every slot was taken")
		return nil, context.Canceled
	}
	return c
}

func TestBusyMaintenanceFetchServesTheStaleSnapshotAtOnce(t *testing.T) {
	const uri = "https://status.example.com/page"
	c := busyCache(t)
	c.store(uri, customPageEntry{
		body: []byte("last good page"), contentType: "text/html", status: http.StatusServiceUnavailable,
		fetchedAt: time.Now().Add(-(customPageTTL + time.Minute)), // past the TTL, inside the stale window
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := c.fetcher(nil)(ctx, maintenanceApp(uri), uri)
	if err != nil {
		t.Fatalf("fetch = %v, want the stale snapshot", err)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "last good page" {
		t.Fatalf("body = %q, want the stale snapshot", body)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("fetch waited %v for a free slot, want an immediate fallback", elapsed)
	}
}

func TestBusyMaintenanceFetchWithNoSnapshotAnswersTheDefaultPage(t *testing.T) {
	const uri = "https://status.example.com/never-fetched"
	c := busyCache(t)
	app := maintenanceApp(uri)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "https://web.onbex.co/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	start := time.Now()
	serveMaintenanceWithFetcher(rr, req, app, c.fetcher(nil))
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "<!DOCTYPE html>") {
		t.Fatalf("response = %d %q, want the default maintenance page", rr.Code, rr.Body.String())
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("answered after %v, want an immediate default page", elapsed)
	}
}

// A real fetch error still answers 502, not the default page: only a busy
// fetcher falls back.
func TestFailedMaintenanceFetchStillAnswersBadGateway(t *testing.T) {
	const uri = "https://status.example.com/failing"
	c := newCustomPageCache()
	c.origin = func(context.Context, *appv1alpha1.App, *url.URL, func(string) bool) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://web.onbex.co/", nil)
	serveMaintenanceWithFetcher(rr, req, maintenanceApp(uri), c.fetcher(nil))
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("failed fetch = %d, want 502", rr.Code)
	}
}
