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

package id

import "strings"

// EnvironmentPublicID exposes the Render-compatible alias of an environment's
// durable env- key. The suffix never changes, so old API links and existing
// memberships keep addressing the same identity. Only minted shapes are
// aliases; names and malformed inputs must not acquire ID semantics.
func EnvironmentPublicID(value string) string {
	if strings.HasPrefix(value, "env-") && WellFormed(value) {
		return "evm-" + value[4:]
	}
	return value
}

// EnvironmentStorageID resolves the public alias to the sole durable key
// namespace. The environments table reserves evm- against insertion, preventing
// an alias from ever shadowing another environment. Use this before lookup or
// persistence, then apply the ordinary authorization and project/workspace gates.
func EnvironmentStorageID(value string) string {
	if strings.HasPrefix(value, "evm-") && WellFormed(value) {
		return "env-" + value[4:]
	}
	return value
}
