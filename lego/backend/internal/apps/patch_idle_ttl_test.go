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
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// w4/455: REST returns idleTTLSeconds at the service's top level, so PATCH must
// take it there as well as under serviceDetails; the nested value wins.
func TestPatchIdleTTLSecondsAcceptsTopLevelAndNested(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       int32
	}{
		{"top level", `{"idleTTLSeconds":120}`, 120},
		{"nested", `{"serviceDetails":{"idleTTLSeconds":300}}`, 300},
		{"both, nested wins", `{"idleTTLSeconds":120,"serviceDetails":{"idleTTLSeconds":300}}`, 300},
		{"top level zero", `{"idleTTLSeconds":0}`, 0},
		{"nested absent keeps top level", `{"idleTTLSeconds":120,"serviceDetails":{"plan":"free"}}`, 120},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req patchServiceRequest
			dec := json.NewDecoder(strings.NewReader(tc.body))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				t.Fatalf("strict decode: %v", err)
			}
			f, err := req.resolveFields(httptest.NewRequest("PATCH", "/v1/services/srv-1", nil))
			if err != nil {
				t.Fatal(err)
			}
			if f.idleTTL == nil || *f.idleTTL != tc.want {
				t.Fatalf("idleTTL = %v, want %d", f.idleTTL, tc.want)
			}
		})
	}
}
