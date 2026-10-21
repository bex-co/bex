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

// Command edge-cache is the per-pod caching proxy the operator places in front
// of a web service whose serviceDetails.cache.profile asks for Render's edge
// caching (w5/m151; internal/edgecache).
package main

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/edgecache"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	upstream, err := url.Parse(os.Getenv("BEX_EDGE_CACHE_UPSTREAM"))
	if err != nil || upstream.Host == "" {
		log.Fatalf("edge-cache: BEX_EDGE_CACHE_UPSTREAM must be an http URL, got %q", os.Getenv("BEX_EDGE_CACHE_UPSTREAM"))
	}
	maxBytes, err := strconv.ParseInt(os.Getenv("BEX_EDGE_CACHE_MAX_BYTES"), 10, 64)
	if err != nil || maxBytes <= 0 {
		log.Fatalf("edge-cache: BEX_EDGE_CACHE_MAX_BYTES must be a positive byte count")
	}
	handler := edgecache.New(upstream, edgecache.Config{
		Profile:        os.Getenv("BEX_EDGE_CACHE_PROFILE"),
		MaxBytes:       maxBytes,
		MaxObjectBytes: maxBytes / 8,
	})
	server := &http.Server{Addr: os.Getenv("BEX_EDGE_CACHE_LISTEN"), Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	drain, _ := strconv.Atoi(os.Getenv("BEX_EDGE_CACHE_DRAIN_SECONDS"))
	go func() {
		<-ctx.Done()
		// Keep serving through the pod's drain window, while the endpoints
		// still route here, then finish in-flight requests.
		time.Sleep(time.Duration(drain) * time.Second)
		shutdown, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("edge-cache: %v", err)
	}
}
