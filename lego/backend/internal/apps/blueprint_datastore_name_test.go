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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestBlueprintDatastoreNameAcceptsResourceIDShape: databases[].name and a
// keyvalue service name are free-form labels, as on Render, even when shaped
// like a resource ID (w5/m118 reversed w8/049). A padded name is still refused
// at parse, with the code the direct APIs return.
func TestBlueprintDatastoreNameAcceptsResourceIDShape(t *testing.T) {
	for _, name := range []string{"orders", "dpg-zzzzzzzzzzzzzzzzzzzz"} {
		if _, err := parseDatabase(bexDatabase{Name: name}); err != nil {
			t.Errorf("parseDatabase(%q) = %v, want accepted", name, err)
		}
	}
	for _, name := range []string{"cache", "srv-zzzzzzzzzzzzzzzzzzzz"} {
		if _, err := parseKeyValue(bexService{Name: name, Type: "keyvalue"}); err != nil {
			t.Errorf("parseKeyValue(%q) = %v, want accepted", name, err)
		}
	}
	var coded *core.CodedError
	if _, err := parseDatabase(bexDatabase{Name: " orders"}); !errors.As(err, &coded) || coded.Code != "RESOURCE_NAME_INVALID" {
		t.Errorf("parseDatabase(padded) = %v, want RESOURCE_NAME_INVALID", err)
	}
}
