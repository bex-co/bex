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

package v1alpha1

import (
	"net/textproto"
	"strings"
)

// reservedStaticHeaders are body-framing and hop-by-hop names a header rule
// must never set: the server owns them, and a rule such as Content-Length: 1
// made every matching response abort mid-body, taking the site down (w4/204).
var reservedStaticHeaders = map[string]struct{}{
	"Content-Length": {}, "Transfer-Encoding": {}, "Connection": {}, "Keep-Alive": {},
	"Upgrade": {}, "Te": {}, "Trailer": {}, "Proxy-Connection": {},
}

// ReservedStaticHeader reports whether a static-site header rule may not set
// name. The API refuses such rules and the static server skips them.
func ReservedStaticHeader(name string) bool {
	_, reserved := reservedStaticHeaders[textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(name))]
	return reserved
}
