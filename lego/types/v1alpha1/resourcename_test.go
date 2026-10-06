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

package v1alpha1

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestResourceNameVectors pins ValidResourceName to the shared table the
// dashboard's isValidDnsLabel is tested against too
// (dashboard/src/common/lib/utils/__tests__/dns-label.test.ts), w5/m118.
func TestResourceNameVectors(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/resource-names.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Name   string `json:"name"`
		Repeat int    `json:"repeat"`
		Valid  bool   `json:"valid"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		name := row.Name
		if row.Repeat > 0 {
			name = strings.Repeat(name, row.Repeat)
		}
		if got := ValidResourceName(name); got != row.Valid {
			t.Errorf("ValidResourceName(%q) = %v, the table says %v", name, got, row.Valid)
		}
	}
	if len(rows) < 15 {
		t.Fatalf("vector table too small: %d rows", len(rows))
	}
}
