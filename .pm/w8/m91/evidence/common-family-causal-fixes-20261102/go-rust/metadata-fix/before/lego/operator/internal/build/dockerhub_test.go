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
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
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

type metadataCacheFixture struct {
	image, configPath string
	routes            map[string][]byte
}

func metadataCacheImage(t *testing.T, id string, env []string) metadataCacheFixture {
	t.Helper()
	encode := func(value any) ([]byte, string) {
		data, err := json.Marshal(value)
		must(t, err)
		return data, fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	}
	config, configDigest := encode(map[string]any{
		"os": "linux", "architecture": "amd64", "config": map[string]any{"Env": env},
		"history": []map[string]string{{"created_by": "RUN fixture-" + id}},
	})
	manifest, manifestDigest := encode(map[string]any{
		"schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json",
		"config": map[string]any{"digest": configDigest, "size": len(config)},
	})
	index, indexDigest := encode(map[string]any{
		"schemaVersion": 2, "mediaType": "application/vnd.oci.image.index.v1+json",
		"manifests": []map[string]any{{"digest": manifestDigest, "size": len(manifest),
			"platform": map[string]string{"os": "linux", "architecture": "amd64"}}},
	})
	base := "/v2/library/node/"
	return metadataCacheFixture{"node:fixture@" + indexDigest, base + "blobs/" + configDigest, map[string][]byte{
		base + "manifests/" + indexDigest:    index,
		base + "manifests/" + manifestDigest: manifest,
		base + "blobs/" + configDigest:       config,
	}}
}

func metadataCacheServer(t *testing.T, fixtures ...metadataCacheFixture) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	routes := map[string][]byte{}
	for _, fixture := range fixtures {
		maps.Copy(routes, fixture.routes)
	}
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		data, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(server.Close)
	return server, &reads
}

func TestImageMetadataCacheReusesVerifiedReadsAndIsolatesCallers(t *testing.T) {
	fixture := metadataCacheImage(t, "first", []string{"NODE_VERSION=24.16.0"})
	other := metadataCacheImage(t, "second", []string{"NODE_VERSION=24.17.0"})
	server, reads := metadataCacheServer(t, fixture, other)
	resolver := &DockerHubResolver{Client: server.Client(), RegistryURL: server.URL}
	env, history, err := resolver.ImageMetadata(t.Context(), fixture.image)
	must(t, err)
	if reads.Load() != 3 || !slices.Equal(env, []string{"NODE_VERSION=24.16.0"}) || !slices.Equal(history, []string{"RUN fixture-first"}) {
		t.Fatalf("first read did not traverse verified index/manifest/config: %d %v %v", reads.Load(), env, history)
	}
	env[0], history[0] = "caller mutation", "caller mutation"
	for range 2 {
		env, history, err = resolver.ImageMetadata(t.Context(), strings.Replace(fixture.image, ":fixture@", ":alias@", 1))
		must(t, err)
		if reads.Load() != 3 || env[0] != "NODE_VERSION=24.16.0" || history[0] != "RUN fixture-first" {
			t.Fatalf("immutable hit changed or read the registry again: %d %v %v", reads.Load(), env, history)
		}
		env[0], history[0] = "another caller", "another caller"
	}
	_, _, err = resolver.ImageMetadata(t.Context(), other.image)
	must(t, err)
	if reads.Load() != 6 {
		t.Fatalf("different digest did not fetch its own metadata: %d", reads.Load())
	}
	if _, _, err := resolver.ImageMetadata(t.Context(), strings.Replace(fixture.image, "node:", "python:", 1)); err == nil {
		t.Fatal("another repository borrowed the cached node metadata")
	}
}

func TestImageMetadataCacheDoesNotCrossRegistryOrResolver(t *testing.T) {
	fixture := metadataCacheImage(t, "same", []string{"NODE_VERSION=24.16.0"})
	first, firstReads := metadataCacheServer(t, fixture)
	second, secondReads := metadataCacheServer(t, fixture)
	resolver := &DockerHubResolver{Client: first.Client(), RegistryURL: first.URL}
	for _, endpoint := range []string{first.URL, second.URL, first.URL} {
		resolver.RegistryURL = endpoint
		_, _, err := resolver.ImageMetadata(t.Context(), fixture.image)
		must(t, err)
	}
	if firstReads.Load() != 3 || secondReads.Load() != 3 {
		t.Fatalf("endpoint cache scopes leaked: %d %d", firstReads.Load(), secondReads.Load())
	}
	independent := &DockerHubResolver{Client: first.Client(), RegistryURL: first.URL}
	_, _, err := independent.ImageMetadata(t.Context(), fixture.image)
	must(t, err)
	if firstReads.Load() != 6 {
		t.Fatal("another resolver reused shared metadata")
	}
}

func TestImageMetadataCacheRefusesErrorsEmptyEnvAndTampering(t *testing.T) {
	for _, failure := range []string{"404", "empty", "tampered config"} {
		t.Run(failure, func(t *testing.T) {
			env := []string{"NODE_VERSION=24.16.0"}
			if failure == "empty" {
				env = nil
			}
			fixture := metadataCacheImage(t, failure, env)
			switch failure {
			case "404":
				fixture.routes = nil
			case "tampered config":
				fixture.routes[fixture.configPath] = []byte(`{"os":"linux","architecture":"amd64","config":{"Env":["NODE_VERSION=wrong"]}}`)
			}
			server, reads := metadataCacheServer(t, fixture)
			resolver := &DockerHubResolver{Client: server.Client(), RegistryURL: server.URL}
			for range 2 {
				if env, _, err := resolver.ImageMetadata(t.Context(), fixture.image); err == nil || env != nil {
					t.Fatalf("invalid metadata was reused: %v %v", env, err)
				}
			}
			want := int32(6)
			if failure == "404" {
				want = 2
			}
			if reads.Load() != want {
				t.Fatalf("failure was cached: %d reads, want %d", reads.Load(), want)
			}
		})
	}
}

func TestImageMetadataCacheCancellationDoesNotBlockOtherReadsOrPersist(t *testing.T) {
	held := metadataCacheImage(t, "held", []string{"NODE_VERSION=24.16.0"})
	other := metadataCacheImage(t, "other", []string{"NODE_VERSION=24.17.0"})
	var block atomic.Bool
	block.Store(true)
	started := make(chan struct{}, 1)
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if strings.HasSuffix(r.URL.Path, strings.Split(held.image, "@")[1]) && block.Load() {
			started <- struct{}{}
			<-r.Context().Done()
			return
		}
		if data, ok := held.routes[r.URL.Path]; ok {
			_, _ = w.Write(data)
		} else if data, ok := other.routes[r.URL.Path]; ok {
			_, _ = w.Write(data)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	resolver := &DockerHubResolver{Client: server.Client(), RegistryURL: server.URL}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() { _, _, err := resolver.ImageMetadata(ctx, held.image); finished <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("first metadata request did not start")
	}
	otherCtx, stopOther := context.WithTimeout(t.Context(), time.Second)
	defer stopOther()
	otherFinished := make(chan error, 1)
	go func() { _, _, err := resolver.ImageMetadata(otherCtx, other.image); otherFinished <- err }()
	select {
	case err := <-otherFinished:
		must(t, err) // The cache lock must not cover the held network request.
	case <-otherCtx.Done():
		t.Fatal("another metadata read waited for the held request")
	}
	cancel()
	if err := <-finished; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled metadata read: %v", err)
	}
	block.Store(false)
	_, _, err := resolver.ImageMetadata(t.Context(), held.image)
	must(t, err)
	if reads.Load() != 7 {
		t.Fatalf("cancellation was cached or retried: %d reads", reads.Load())
	}
	if _, _, err := resolver.ImageMetadata(ctx, held.image); !errors.Is(err, context.Canceled) || reads.Load() != 7 {
		t.Fatalf("canceled caller received cached success: %v, reads %d", err, reads.Load())
	}
}

func TestImageMetadataCacheIsBounded(t *testing.T) {
	fixtures := make([]metadataCacheFixture, 65) // One more than the resolver's 64-image budget.
	for i := range fixtures {
		fixtures[i] = metadataCacheImage(t, fmt.Sprint(i), []string{"NODE_VERSION=24.16.0"})
	}
	server, reads := metadataCacheServer(t, fixtures...)
	resolver := &DockerHubResolver{Client: server.Client(), RegistryURL: server.URL}
	for _, fixture := range fixtures {
		_, _, err := resolver.ImageMetadata(t.Context(), fixture.image)
		must(t, err)
	}
	_, _, err := resolver.ImageMetadata(t.Context(), fixtures[0].image)
	must(t, err)
	if reads.Load() != int32(3*(len(fixtures)+1)) {
		t.Fatal("metadata cache retained more than its bounded immutable set")
	}
}

func TestImageMetadataCacheConcurrentReadsHaveIndependentResults(t *testing.T) {
	fixtures := make([]metadataCacheFixture, 8)
	for i := range fixtures {
		fixtures[i] = metadataCacheImage(t, fmt.Sprint(i), []string{"NODE_VERSION=24.16.0"})
	}
	server, reads := metadataCacheServer(t, fixtures...)
	resolver := &DockerHubResolver{Client: server.Client(), RegistryURL: server.URL}
	var group sync.WaitGroup
	for _, fixture := range fixtures {
		group.Go(func() {
			env, history, err := resolver.ImageMetadata(t.Context(), fixture.image)
			if err != nil {
				t.Error(err)
				return
			}
			env[0], history[0] = "caller mutation", "caller mutation"
		})
	}
	group.Wait()
	for range 32 {
		group.Go(func() {
			env, history, err := resolver.ImageMetadata(t.Context(), fixtures[0].image)
			if err != nil || env[0] != "NODE_VERSION=24.16.0" || history[0] != "RUN fixture-0" {
				t.Errorf("shared mutable metadata: %v %v %v", env, history, err)
				return
			}
			env[0], history[0] = "another caller", "another caller"
		})
	}
	group.Wait()
	if reads.Load() != 3*int32(len(fixtures)) {
		t.Fatalf("warm concurrent reads fetched metadata again: %d", reads.Load())
	}
}
