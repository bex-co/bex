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

// Package hostingdomain validates the browser trust boundary used for shared
// tenant application hostnames.
package hostingdomain

import (
	"fmt"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ValidateSharedSuffix permits an empty (disabled) platform domain or a
// canonical domain listed in the Public Suffix List's PRIVATE section. Sharing
// an ordinary registrable domain lets one tenant set a Domain cookie received
// by every sibling tenant; a browser-recognized private suffix prevents that.
func ValidateSharedSuffix(domain string) error {
	if domain == "" {
		return nil
	}
	canonical := strings.ToLower(strings.TrimSpace(domain))
	if canonical != domain || strings.HasSuffix(canonical, ".") || !strings.Contains(canonical, ".") {
		return fmt.Errorf("shared tenant domain %q must be a canonical DNS Public Suffix", domain)
	}
	suffix, icann := publicsuffix.PublicSuffix(canonical)
	if suffix != canonical || icann {
		return fmt.Errorf("shared tenant domain %q is not a private Public Suffix; disable BEX_BASE_DOMAIN until the suffix is registered", domain)
	}
	return nil
}
