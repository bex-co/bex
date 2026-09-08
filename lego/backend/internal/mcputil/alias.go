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

package mcputil

import (
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Alias helpers implement the w2/m91 MCP compat policy: keep legacy bex
// spellings as optional aliases, and prefer the Render spelling when both are set.

// PreferString returns primary when non-empty, else legacy.
func PreferString(primary, legacy string) string {
	if primary != "" {
		return primary
	}
	return legacy
}

// PreferPtrOrZero returns *primary when non-nil, else *legacy, else the zero value.
func PreferPtrOrZero[T ~int32 | ~int64 | ~float64](primary, legacy *T) T {
	if primary != nil {
		return *primary
	}
	if legacy != nil {
		return *legacy
	}
	var zero T
	return zero
}

// RequireAliasString returns primary when non-empty, else legacy when non-empty,
// else ErrBadRequest naming both spellings.
func RequireAliasString(primary, legacy, renderName, legacyName string) (string, error) {
	if primary != "" {
		return primary, nil
	}
	if legacy != "" {
		return legacy, nil
	}
	return "", fmt.Errorf("%w: %s (or legacy %s) is required", core.ErrBadRequest, renderName, legacyName)
}

// ResourceIDs returns []string{resourceID} when resourceID is set, else the
// legacy list when non-empty, else the same required error shape as RequireAliasString.
func ResourceIDs(resourceID string, legacy []string, renderName, legacyName string) ([]string, error) {
	if resourceID != "" {
		return []string{resourceID}, nil
	}
	if len(legacy) > 0 {
		return legacy, nil
	}
	return nil, fmt.Errorf("%w: %s (or legacy %s) is required", core.ErrBadRequest, renderName, legacyName)
}
