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

package apps

import (
	"context"
	"sync"
)

// requestMemo caches request-invariant answers for the life of ONE API call.
//
// It exists because a service patch asks the same question many times.
// ApplyServicePatch validates the patch twice — once as the preflight, once as
// each verb runs (w9/m166) — and within a single pass up to seven rows
// independently ask "is this service in a protected environment?", which is one
// Postgres round trip each. A patch carrying maintenanceMode likewise sweeps
// every App CR on the platform once per pass to check the URI's host. None of
// those answers can change mid-request: the whole patch is evaluated against
// one snapshot of one service.
//
// Only successes are cached. A failed lookup is left to the next caller so a
// transient store error cannot pin itself to the rest of the request; every
// consumer here fails closed on error anyway.
type requestMemo struct {
	mu   sync.Mutex
	vals map[string]any
}

type requestMemoKey struct{}

// withRequestMemo arms memoization for ctx. Callers that fan out over the same
// resource arm it once, at the top of the request; a context without it simply
// computes every time, so nothing depends on it being armed.
func withRequestMemo(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestMemoKey{}, &requestMemo{vals: map[string]any{}})
}

// memoized returns the answer for key, computing it at most once per armed
// request. key must name the question AND its subject (e.g. the app id, the
// host), or two different questions would share an answer.
func memoized[T any](ctx context.Context, key string, compute func() (T, error)) (T, error) {
	m, ok := ctx.Value(requestMemoKey{}).(*requestMemo)
	if !ok {
		return compute()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cached, hit := m.vals[key]; hit {
		return cached.(T), nil
	}
	val, err := compute()
	if err != nil {
		return val, err
	}
	m.vals[key] = val
	return val, nil
}
