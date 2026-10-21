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

// Package resourcename is the one check a Postgres or Key Value name passes on
// every way in: the API's create and rename, and Blueprint parsing. It lives
// apart from core, which imports no internal package, because the check needs
// internal/id.
package resourcename

import (
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
)

// CheckDatastore refuses a name a Postgres or Key Value cannot take. The name
// is the resource's mutable label (its dpg-/red- id is the identity), which
// Render leaves free-form (w5/m153): it follows core.CheckLabel, must not be
// empty, and must not be shaped like a resource ID, which a selector could
// read as that ID (w8/049).
func CheckDatastore(name string) error {
	label, err := core.CheckLabel(name)
	if err != nil {
		return err
	}
	if label == "" || label != name {
		return core.NewBadRequestError("RESOURCE_NAME_INVALID", "name must be a non-empty label without leading or trailing spaces", map[string]any{"field": "name"})
	}
	if id.LooksLikeResourceID(name) {
		return core.NameResourceIDReservedError("name", name)
	}
	return nil
}
