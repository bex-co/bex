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

package controller

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// w4/m363: a build's merged secret files are packed so no Secret passes the
// apiserver's limit, whatever the service and its groups hold together.
func TestPackSecretFilesKeepsEverySecretWithinTheLimit(t *testing.T) {
	check := func(t *testing.T, data map[string][]byte, wantShards int) {
		t.Helper()
		shards, err := packSecretFiles(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(shards) != wantShards {
			t.Fatalf("%d shards, want %d", len(shards), wantShards)
		}
		seen := 0
		for i, shard := range shards {
			size := 0
			for name, value := range shard {
				size += len(value)
				if string(data[name]) != string(value) {
					t.Fatalf("shard %d changed %s", i, name)
				}
			}
			if size > corev1.MaxSecretSize || len(shard) > maxFilesPerShard {
				t.Fatalf("shard %d holds %d bytes in %d keys, over a Secret's limit", i, size, len(shard))
			}
			seen += len(shard)
		}
		if seen != len(data) {
			t.Fatalf("packed %d files, want %d", seen, len(data))
		}
	}

	t.Run("one full map fits one Secret", func(t *testing.T) {
		check(t, map[string][]byte{"a": make([]byte, corev1.MaxSecretSize)}, 1)
	})
	t.Run("a service and two groups at the bound need three", func(t *testing.T) {
		check(t, map[string][]byte{
			"own":    make([]byte, corev1.MaxSecretSize),
			"group1": make([]byte, corev1.MaxSecretSize),
			"group2": make([]byte, corev1.MaxSecretSize),
		}, 3)
	})
	t.Run("one byte past a Secret starts the next", func(t *testing.T) {
		check(t, map[string][]byte{"a": make([]byte, corev1.MaxSecretSize-10), "b": make([]byte, 11)}, 2)
	})
	t.Run("past 500 files starts the next", func(t *testing.T) {
		many := map[string][]byte{}
		for i := range maxFilesPerShard + 1 {
			many[fmt.Sprintf("f%04d", i)] = []byte("x")
		}
		check(t, many, 2)
	})
	t.Run("no files, no Secrets", func(t *testing.T) {
		check(t, map[string][]byte{}, 0)
	})
	t.Run("a file no Secret can hold fails by name", func(t *testing.T) {
		_, err := packSecretFiles(map[string][]byte{"huge.bin": make([]byte, corev1.MaxSecretSize+1)})
		if err == nil || !strings.Contains(err.Error(), "huge.bin") {
			t.Fatalf("an over-limit file = %v, want a refusal naming it", err)
		}
	})
}
