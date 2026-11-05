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

package registrycreds

import (
	"errors"
	"fmt"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// w4/504: a malformed value the database refused is a 400, an unknown credential
// a 404, and a real failure still passes through for a 5xx.
func TestMapStoreErrClassifiesMalformedAndMissing(t *testing.T) {
	if err := mapStoreErr(fmt.Errorf("registry credential: %w", store.ErrNotFound)); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("not found = %v, want core.ErrNotFound", err)
	}
	if err := mapStoreErr(fmt.Errorf("registry credential: %w", store.ErrInvalid)); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("invalid = %v, want core.ErrBadRequest", err)
	}
	outage := errors.New("connection refused")
	if err := mapStoreErr(outage); !errors.Is(err, outage) || errors.Is(err, core.ErrBadRequest) {
		t.Errorf("outage = %v, want it passed through", err)
	}
	if mapStoreErr(nil) != nil {
		t.Error("nil must stay nil")
	}
}
