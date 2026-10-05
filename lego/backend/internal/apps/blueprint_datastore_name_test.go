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

// TestBlueprintDatastoreNameRefusesResourceIDShape is w8/049's Blueprint half:
// databases[].name and a keyvalue service name shaped like a resource ID are
// refused at parse, with the same code the direct APIs return.
func TestBlueprintDatastoreNameRefusesResourceIDShape(t *testing.T) {
	var coded *core.CodedError
	if _, err := parseDatabase(bexDatabase{Name: "dpg-zzzzzzzzzzzzzzzzzzzz"}); !errors.As(err, &coded) || coded.Code != "NAME_RESOURCE_ID_RESERVED" {
		t.Errorf("parseDatabase = %v, want NAME_RESOURCE_ID_RESERVED", err)
	}
	coded = nil
	if _, err := parseKeyValue(bexService{Name: "srv-zzzzzzzzzzzzzzzzzzzz", Type: "keyvalue"}); !errors.As(err, &coded) || coded.Code != "NAME_RESOURCE_ID_RESERVED" {
		t.Errorf("parseKeyValue = %v, want NAME_RESOURCE_ID_RESERVED", err)
	}
	if _, err := parseDatabase(bexDatabase{Name: "orders"}); err != nil {
		t.Errorf("control parseDatabase(orders) = %v", err)
	}
	if _, err := parseKeyValue(bexService{Name: "cache", Type: "keyvalue"}); err != nil {
		t.Errorf("control parseKeyValue(cache) = %v", err)
	}
}
