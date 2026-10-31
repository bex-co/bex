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

package build

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A transient Docker Hub answer is retried a bounded number of times before
// a native resolve gives up; a definitive refusal is not retried.
func TestImageMetadataRetriesOnlyTransientReads(t *testing.T) {
	pause := imageMetadataRetryPause
	imageMetadataRetryPause = time.Millisecond
	t.Cleanup(func() { imageMetadataRetryPause = pause })
	image := "node:24-bookworm@sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		status int
		want   int32
	}{
		{http.StatusServiceUnavailable, imageMetadataAttempts},
		{http.StatusTooManyRequests, imageMetadataAttempts},
		{http.StatusNotFound, 1},
	} {
		var reads atomic.Int32
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			reads.Add(1)
			w.WriteHeader(tc.status)
		}))
		resolver := &DockerHubResolver{Client: server.Client(), RegistryURL: server.URL}
		if _, _, err := resolver.ImageMetadata(context.Background(), image); err == nil {
			t.Fatalf("status %d: want an error", tc.status)
		}
		server.Close()
		if got := reads.Load(); got != tc.want {
			t.Fatalf("status %d: %d reads, want %d", tc.status, got, tc.want)
		}
	}
}
