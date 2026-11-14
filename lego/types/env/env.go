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

// Package env reads process environment variables at entrypoint startup. It
// is a leaf utility shared by both sides of the App CR boundary — every
// bex binary parses the same "unset or empty means default" convention — so
// none of them carries its own copy (w10/unify-duplicates).
package env

import "os"

// Or reads the environment variable name, falling back to def when it is
// unset or empty. An empty value is indistinguishable from unset on purpose:
// a manager Deployment that sets `BEX_X: ""` means "not configured", never
// "configured as empty".
func Or(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}
