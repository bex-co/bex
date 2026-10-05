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
package keyvalue

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestKeyValueNameRefusesResourceIDShape is w8/049: a Key Value name that looks
// like another resource's ID is refused on create and rename (both run
// validateKeyValueName), so a selector can't mean two things.
func TestKeyValueNameRefusesResourceIDShape(t *testing.T) {
	for _, name := range []string{"srv-db1e4vnhb1uc73ebifi0", "dpg-zzzzzzzzzzzzzzzzzzzz", "red-aaaaaaaaaaaaaaaaaaaa"} {
		var coded *core.CodedError
		err := validateKeyValueName(name)
		if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != "NAME_RESOURCE_ID_RESERVED" {
			t.Errorf("validateKeyValueName(%q) = %v, want NAME_RESOURCE_ID_RESERVED", name, err)
		}
	}
	for _, name := range []string{"orders", "srv-web", "dpg-db1e4vnhb1uc73ebifi"} {
		if err := validateKeyValueName(name); err != nil {
			t.Errorf("validateKeyValueName(%q) = %v, want accepted", name, err)
		}
	}
	svc, _ := newService()
	if _, err := svc.CreateKeyValue(ctxAs("user-a"), CreateKeyValueRequest{Name: "srv-aaaaaaaaaaaaaaaaaaaa", Plan: "free"}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("create with an ID-shaped name = %v, want 400", err)
	}
}
