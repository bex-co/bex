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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/494: under strict decoding a non-integer maxShutdownDelaySeconds used to
// answer the generic "invalid JSON request body", although the body is valid
// JSON and only the type is wrong. It now names the field.
func TestMaxShutdownDelayTypeErrorNamesTheField(t *testing.T) {
	ctx := core.WithStrictJSONDecoding(t.Context())
	for _, body := range []string{
		`{"serviceDetails":{"maxShutdownDelaySeconds":1.5}}`,
		`{"serviceDetails":{"maxShutdownDelaySeconds":"60"}}`,
		`{"serviceDetails":{"maxShutdownDelaySeconds":null}}`,
	} {
		var req patchServiceRequest
		err := core.UnmarshalJSON(ctx, []byte(body), &req)
		const want = "maxShutdownDelaySeconds must be a whole number of seconds"
		if err == nil || err.Error() != want {
			t.Errorf("%s: err = %v, want %q", body, err, want)
		}
	}
	var ok patchServiceRequest
	if err := core.UnmarshalJSON(ctx, []byte(`{"serviceDetails":{"maxShutdownDelaySeconds":60}}`), &ok); err != nil {
		t.Fatalf("a whole number: %v", err)
	}
	if err := core.UnmarshalJSON(ctx, []byte(`{"serviceDetails":{"maxShutdownDelaySeconds":}}`), &ok); err == nil || err.Error() != "invalid JSON request body" {
		t.Errorf("malformed JSON = %v, want the generic message unchanged", err)
	}
}
