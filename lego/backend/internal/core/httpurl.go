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

package core

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidAbsoluteHTTPURL reports whether raw parses as an absolute http(s) URL
// with a non-empty host and NO embedded userinfo — the shape every bex-api verb
// that stores a tenant-supplied destination URL requires (the outbound-webhook
// endpoint, the maintenance-mode custom page). Shared so the callers don't
// drift on what counts as a valid destination.
//
// Userinfo (`https://user:pass@host/…`) is rejected outright (codex-security
// target #7): a credential embedded in the URL would be persisted verbatim and
// then either replayed to the fetch target or surfaced to workspace viewers,
// and it lets an allowlist that only inspects the host be confused by
// `https://trusted-host@evil-host/`. No legitimate destination carries it.
func ValidAbsoluteHTTPURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") &&
		u.Host != "" && u.User == nil
}

// ErrNotAbsoluteHTTPURL is ValidAbsoluteHTTPURL's paired error, wrapping
// ErrBadRequest with a caller-supplied field name.
func ErrNotAbsoluteHTTPURL(field string) error {
	return fmt.Errorf("%w: %s must be an absolute http(s) URL", ErrBadRequest, field)
}
