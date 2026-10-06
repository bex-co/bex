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
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// CheckDatastore refuses a name a Postgres or Key Value cannot take: one
// outside appv1alpha1.ValidResourceName, or one shaped like a resource ID,
// which a selector could read as that ID (w8/049).
func CheckDatastore(name string) error {
	if !appv1alpha1.ValidResourceName(name) {
		return fmt.Errorf("%w: name %s", core.ErrBadRequest, core.ResourceNameRule)
	}
	if id.LooksLikeResourceID(name) {
		return core.NameResourceIDReservedError("name", name)
	}
	return nil
}
