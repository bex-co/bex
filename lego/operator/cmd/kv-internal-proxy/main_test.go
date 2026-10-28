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
	"strings"
	"testing"
)

func TestRunRequiresConfigurationBeforeListening(t *testing.T) {
	for _, env := range []map[string]string{
		{},
		{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1"},
		{"VALKEY_PASSWORD": "secret"},
		{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:1", "VALKEY_PASSWORD": "secret", "BEX_KV_INTERNAL_ADDR": "127.0.0.1:0", "BEX_KV_TLS_ADDR": "127.0.0.1:0"},
	} {
		err := run(context.Background(), func(key string) string { return env[key] }, false)
		if err == nil || !strings.Contains(err.Error(), "required") {
			t.Fatalf("missing config: %v", err)
		}
		if strings.Contains(err.Error(), "secret") {
			t.Fatal("configuration error exposed password")
		}
	}
}

func TestRunCheckReportsUnreachableUpstream(t *testing.T) {
	env := map[string]string{"BEX_KV_UPSTREAM_ADDR": "127.0.0.1:0", "VALKEY_PASSWORD": "secret"}
	if err := run(t.Context(), func(key string) string { return env[key] }, true); err == nil || !strings.Contains(err.Error(), "upstream connection failed") {
		t.Fatalf("check=%v", err)
	}
}
