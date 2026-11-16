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

package api

import (
	"bytes"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/pricing"
)

// GET /pricing answers without a credential: bex.co fetches it to check that
// its pricing page quotes what bex charges.
func TestPricingIsPublic(t *testing.T) {
	h, err := matrixServer(t).Handler()
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, "GET", "/pricing", "", "")
	if rec.Code != 200 {
		t.Fatalf("unauthenticated GET /pricing = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "public, max-age=300" {
		t.Errorf("Cache-Control = %q", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), pricing.Default.PublicJSON()) {
		t.Errorf("body is not the public price sheet: %s", rec.Body.String())
	}
}
