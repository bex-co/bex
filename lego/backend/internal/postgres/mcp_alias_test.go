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

package postgres

import (
	"testing"
)

// TestCreatePostgresDiskSizeGbAlias covers w2/m91: Render's diskSizeGb wins
// over the legacy diskSizeGB alias when both are set; either alone applies.
func TestCreatePostgresDiskSizeGbAlias(t *testing.T) {
	gb := func(v int32) *int32 { return &v }

	tests := []struct {
		name string
		args createPostgresArgs
		want int32
	}{
		{"render spelling", createPostgresArgs{DiskSizeGb: gb(40)}, 40},
		{"legacy spelling", createPostgresArgs{DiskSizeGB: gb(25)}, 25},
		{"render wins when both set", createPostgresArgs{DiskSizeGb: gb(40), DiskSizeGB: gb(25)}, 40},
		{"neither", createPostgresArgs{}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.args.diskSizeGBValue(); got != tc.want {
				t.Errorf("diskSizeGBValue() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestMCPCreatePostgresDiskSizeGb(t *testing.T) {
	svc, cl := newService()
	call, cleanup := pgMCPClient(t, svc)
	defer cleanup()

	got := call("create_postgres", map[string]any{
		"name":       "disk-render",
		"plan":       "basic-1gb",
		"diskSizeGb": 40,
		"dryRun":     true,
	})
	if got["diskSizeGB"] != float64(40) {
		t.Fatalf("diskSizeGb dry-run preview diskSizeGB = %v, want 40", got["diskSizeGB"])
	}

	got = call("create_postgres", map[string]any{
		"name":       "disk-legacy",
		"plan":       "basic-1gb",
		"diskSizeGB": 25,
		"dryRun":     true,
	})
	if got["diskSizeGB"] != float64(25) {
		t.Fatalf("diskSizeGB dry-run preview = %v, want 25", got["diskSizeGB"])
	}

	got = call("create_postgres", map[string]any{
		"name":       "disk-both",
		"plan":       "basic-1gb",
		"diskSizeGb": 40,
		"diskSizeGB": 25,
		"dryRun":     true,
	})
	if got["diskSizeGB"] != float64(40) {
		t.Fatalf("both-set precedence preview = %v, want 40 (Render wins)", got["diskSizeGB"])
	}

	if n := countDatabases(t, cl); n != 0 {
		t.Fatalf("dry-run must not create a CR, got %d", n)
	}
}
