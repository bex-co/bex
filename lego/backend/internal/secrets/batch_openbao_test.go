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

package secrets

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// This opt-in suite uses real OpenBao KV v2 with the production store adapter.
// Kubernetes remains the fake client: this is not a live rollout test.
func realBatchBao(t *testing.T) *openBaoStore {
	t.Helper()
	addr, token := os.Getenv("BEX_TEST_OPENBAO_KV_URL"), os.Getenv("BEX_TEST_OPENBAO_KV_TOKEN")
	if addr == "" || token == "" {
		t.Skip("requires BEX_TEST_OPENBAO_KV_URL and BEX_TEST_OPENBAO_KV_TOKEN")
	}
	mount := fmt.Sprintf("batch-test-%d", time.Now().UnixNano())
	httpClient := &http.Client{Timeout: 10 * time.Second, Transport: batchBaoTrace{t: t}}
	mountURL := strings.TrimRight(addr, "/") + "/v1/sys/mounts/" + mount
	bao := &openBaoStore{addr: strings.TrimRight(addr, "/"), mount: mount, client: httpClient, token: token, tokenExp: time.Now().Add(time.Hour)}
	requestMount := func(method string, body []byte) {
		t.Helper()
		if err := bao.do(context.Background(), method, mountURL, token, body, nil); err != nil {
			t.Fatal("test KV mount request failed")
		}
	}
	requestMount(http.MethodPost, []byte(`{"type":"kv","options":{"version":"2"}}`))
	t.Cleanup(func() { requestMount(http.MethodDelete, nil) })
	// Enabling KV v2 returns before its initial version upgrade finishes.
	// Check the read-only config endpoint before the first fixture CAS; never
	// retry a tested write, which would hide real revision conflicts.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := bao.do(ctx, http.MethodGet, bao.addr+"/v1/"+mount+"/config", token, nil, nil)
		if err == nil {
			break
		}
		var status *core.HTTPStatusError
		if !errors.As(err, &status) || status.Code != http.StatusBadRequest {
			t.Fatal("test KV mount readiness check failed")
		}
		select {
		case <-ctx.Done():
			t.Fatal("test KV mount upgrade did not finish")
		case <-ticker.C:
		}
	}
	return bao
}

type batchBaoTrace struct{ t *testing.T }

func (tr batchBaoTrace) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err == nil {
		tr.t.Logf("OpenBao method=%s status=%d", req.Method, resp.StatusCode)
	}
	return resp, err
}

type advanceAfterBatchCAS struct {
	*openBaoStore
	once sync.Once
}

func (s *advanceAfterBatchCAS) PutCAS(ctx context.Context, path string, data map[string]string, expected uint64) (uint64, error) {
	version, err := s.openBaoStore.PutCAS(ctx, path, data, expected)
	if err != nil {
		return version, err
	}
	s.once.Do(func() {
		next := maps.Clone(data)
		next["OTHER"] = "concurrent-update"
		_, err = s.openBaoStore.PutCAS(ctx, path, next, version)
	})
	return version, err
}

func TestPatchEnvironmentRealOpenBaoPostWriteConflict(t *testing.T) {
	bao := realBatchBao(t)
	ctx := context.Background()
	initial, err := bao.PutCAS(ctx, envPath("web"), map[string]string{"TOKEN": "before", "OTHER": "before"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	revision := encodeEnvRevision(initial)
	svc := newService(&advanceAfterBatchCAS{openBaoStore: bao}, sampleApp("web"))
	_, err = svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "requested"}}})
	var coded *core.CodedError
	if !errors.As(err, &coded) {
		t.Fatalf("expected coded failure: %v", err)
	}
	after, readErr := bao.GetVersioned(ctx, envPath("web"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	t.Logf("source versions initial=%d after=%d code=%s", initial, after.Version, coded.Code)
	if after.Version != initial+2 || after.Data["TOKEN"] != "requested" || after.Data["OTHER"] != "concurrent-update" {
		t.Fatal("later writer or requested value was overwritten")
	}
	if coded.Code != "ENVIRONMENT_RESTORATION_FAILED" {
		t.Fatalf("committed write was mislabeled %s", coded.Code)
	}
}

func TestPatchEnvironmentRealOpenBaoSequentialAndStale(t *testing.T) {
	ctx := context.Background()
	for _, surface := range []string{"REST", "GraphQL", "MCP"} {
		for _, mode := range []SaveMode{SaveModeOnly, SaveModeDeploy} {
			t.Run(surface+"/"+string(mode), func(t *testing.T) {
				bao := realBatchBao(t)
				version, err := bao.PutCAS(ctx, envPath("web"), map[string]string{"TOKEN": "initial"}, 0)
				if err != nil {
					t.Fatal(err)
				}
				svc := newService(bao, sampleApp("web"))
				call := environmentPatchSurfaceCall(t, svc, surface)
				for i := range 20 {
					svc.Clock = func() time.Time { return fixedNow().Add(time.Duration(i) * time.Second) }
					revision := encodeEnvRevision(version)
					value := fmt.Sprintf("value-%d", i/2)
					result := call(map[string]any{"saveMode": string(mode), "expectedEnvRevision": revision, "envVars": []map[string]any{{"key": "TOKEN", "value": value}}})
					if result["revision"] != encodeEnvRevision(version+1) || result["rolledOut"] != (mode == SaveModeDeploy && i%2 == 0) {
						t.Fatal("incorrect success metadata")
					}
					before, err := bao.GetVersioned(ctx, envPath("web"))
					if err != nil {
						t.Fatal(err)
					}
					if before.Version != version+1 || before.Data["TOKEN"] != value {
						t.Fatal("successful write state mismatch")
					}
					_, err = svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: mode, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: "stale"}}})
					var coded *core.CodedError
					if !errors.As(err, &coded) || coded.Code != "ENVIRONMENT_REVISION_CONFLICT" {
						t.Fatalf("stale write error: %v", err)
					}
					after, err := bao.GetVersioned(ctx, envPath("web"))
					if err != nil {
						t.Fatal(err)
					}
					if after.Version != before.Version || !maps.Equal(after.Data, before.Data) {
						t.Fatal("stale rejection changed source")
					}
					version = after.Version
				}
			})
		}
	}
}

func TestPatchEnvironmentRealOpenBaoConcurrentWriters(t *testing.T) {
	bao := realBatchBao(t)
	ctx := context.Background()
	version, err := bao.PutCAS(ctx, envPath("web"), map[string]string{"TOKEN": "initial"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	revision := encodeEnvRevision(version)
	svc := newService(bao, sampleApp("web"))
	type outcome struct {
		value string
		err   error
	}
	results := make(chan outcome, 2)
	start := make(chan struct{})
	for _, value := range []string{"first", "second"} {
		go func() {
			<-start
			_, err := svc.PatchEnvironment(ctx, "web", EnvironmentPatch{SaveMode: SaveModeOnly, ExpectedEnvRevision: &revision, EnvVars: []EnvVarPatch{{Key: "TOKEN", Value: value}}})
			results <- outcome{value, err}
		}()
	}
	close(start)
	// Join both writers before an assertion can trigger mount cleanup.
	outcomes := []outcome{<-results, <-results}
	successes := 0
	winner := ""
	for _, result := range outcomes {
		if result.err == nil {
			successes++
			winner = result.value
			continue
		}
		var coded *core.CodedError
		if !errors.As(result.err, &coded) || coded.Code != "ENVIRONMENT_REVISION_CONFLICT" {
			t.Fatalf("loser error: %v", result.err)
		}
	}
	after, err := bao.GetVersioned(ctx, envPath("web"))
	if err != nil {
		t.Fatal(err)
	}
	if successes != 1 || after.Version != version+1 || after.Data["TOKEN"] != winner {
		t.Fatal("concurrent writes did not retain exactly one winner")
	}
}
