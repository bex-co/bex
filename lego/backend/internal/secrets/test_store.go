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
	"net/http"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// NewOpenBaoTestStore is the OpenBao store for a test against a dev OpenBao
// (backend CI, scripts/backend-test-deps.sh): it authenticates with a static
// token instead of the pod's service-account JWT and mounts a fresh KV v2
// engine, so tests never share keys. remove unmounts it. Production code uses
// NewOpenBaoStore.
func NewOpenBaoTestStore(ctx context.Context, addr, token string) (core.SecretKV, func(), error) {
	return newTokenOpenBaoStore(ctx, addr, token, &http.Client{Timeout: 10 * time.Second})
}

func newTokenOpenBaoStore(ctx context.Context, addr, token string, client *http.Client) (*openBaoStore, func(), error) {
	addr = strings.TrimRight(addr, "/")
	mount := fmt.Sprintf("test-%d", time.Now().UnixNano())
	bao := &openBaoStore{addr: addr, mount: mount, client: client, token: token, tokenExp: time.Now().Add(time.Hour)}
	mountURL := addr + "/v1/sys/mounts/" + mount
	if err := bao.do(ctx, http.MethodPost, mountURL, token, []byte(`{"type":"kv","options":{"version":"2"}}`), nil); err != nil {
		return nil, nil, fmt.Errorf("mount test KV engine: %w", err)
	}
	remove := func() { _ = bao.do(context.Background(), http.MethodDelete, mountURL, token, nil, nil) }
	// Enabling KV v2 returns before its initial version upgrade finishes; the
	// read-only config endpoint answers 400 until it has. Never retry a tested
	// write instead, which would hide a real revision conflict.
	ready, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	for {
		err := bao.do(ready, http.MethodGet, addr+"/v1/"+mount+"/config", token, nil, nil)
		if err == nil {
			return bao, remove, nil
		}
		var status *core.HTTPStatusError
		if !errors.As(err, &status) || status.Code != http.StatusBadRequest {
			remove()
			return nil, nil, fmt.Errorf("test KV engine readiness: %w", err)
		}
		select {
		case <-ready.Done():
			remove()
			return nil, nil, errors.New("test KV engine upgrade did not finish")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
