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
	"net"
)

// ValidateCIDRs rejects any entry that is not a valid CIDR with ErrBadRequest —
// the shared gate for every ipAllowList write path (postgres + keyvalue, create
// and set alike), so a bad entry is a 400 before anything lands in a CR spec.
func ValidateCIDRs(cidrs []string) error {
	for _, c := range cidrs {
		if _, _, err := net.ParseCIDR(c); err != nil {
			return fmt.Errorf("%w: %q is not a valid CIDR", ErrBadRequest, c)
		}
	}
	return nil
}
