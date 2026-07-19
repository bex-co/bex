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

package httpclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testDeadline = 80 * time.Millisecond

func TestStalledConnectReleasesCaller(t *testing.T) {
	client := New(testDeadline)
	transport := client.Transport.(*http.Transport)
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	started := time.Now()
	_, err := client.Get("http://stalled.invalid")
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("stalled connect err=%v elapsed=%s", err, time.Since(started))
	}
}

func TestStalledTLSHandshakeReleasesCaller(t *testing.T) {
	client := New(testDeadline)
	transport := client.Transport.(*http.Transport)
	transport.Proxy = nil
	transport.DialTLSContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	started := time.Now()
	_, err := client.Get("https://stalled.invalid")
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("stalled TLS handshake err=%v elapsed=%s", err, time.Since(started))
	}
}

func TestStalledResponseHeaderReleasesCaller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client := New(testDeadline)
	started := time.Now()
	_, err := client.Get(server.URL)
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("stalled header err=%v elapsed=%s", err, time.Since(started))
	}
}

func TestStalledBodyAndCancellationReleaseCaller(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	client := New(testDeadline)
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	started := time.Now()
	_, err = ReadAll(resp.Body)
	if err == nil || time.Since(started) > time.Second {
		t.Fatalf("stalled body err=%v elapsed=%s", err, time.Since(started))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	_, err = Shared.Do(req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled request = %v, want context.Canceled", err)
	}
}

func TestReadAllBoundsResponseBody(t *testing.T) {
	_, err := ReadAll(strings.NewReader(strings.Repeat("x", MaxBodyBytes+1)))
	if err == nil {
		t.Fatal("oversized body was accepted")
	}
}
