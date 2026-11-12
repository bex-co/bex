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

package sshgateway

import (
	"testing"
	"time"
)

// A loop that keeps retrying after exhausting its budget must stay refused: the
// refusals keep the key alive, so the idle sweep never hands it a fresh budget.
// Only a full idle day with no requests at all resets it.
func TestExchangeBudgetExhaustedKeyStaysExhaustedWhileRetried(t *testing.T) {
	const limit = 2
	budget := NewExchangeBudget(0, limit)
	t0 := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	const key = "workspace|acme"

	if !budget.charge(key, limit, t0) || !budget.charge(key, limit, t0) {
		t.Fatal("charges within the cap were refused")
	}
	for _, at := range []time.Duration{time.Hour, 23 * time.Hour, 25 * time.Hour, 47 * time.Hour} {
		if budget.charge(key, limit, t0.Add(at)) {
			t.Fatalf("exhausted key admitted a request at +%s while retries kept arriving", at)
		}
	}
	if !budget.charge(key, limit, t0.Add(47*time.Hour+exchangeBudgetIdleTTL+time.Second)) {
		t.Fatal("key idle for a full day was not reset")
	}
}

func TestExchangeBudgetSweepsIdleKeys(t *testing.T) {
	budget := NewExchangeBudget(0, 1)
	t0 := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)

	budget.charge("workspace|gone", 1, t0)
	budget.charge("workspace|live", 1, t0.Add(exchangeBudgetIdleTTL+time.Second))

	if _, ok := budget.entries["workspace|gone"]; ok || len(budget.entries) != 1 {
		t.Fatalf("entries=%v, want only the live key", budget.entries)
	}
}
