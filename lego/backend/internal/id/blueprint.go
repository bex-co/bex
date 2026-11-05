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

// BlueprintPublicPrefix is Render's spelling of a Blueprint id. The pinned
// Render CLI (v2.35 pkg/validate/id.go IsBlueprintID) refuses any Blueprint id
// that is not exs- plus 20 lowercase letters or digits, so every client-read
// surface projects the durable blp- key into this namespace (w2/m269).
const BlueprintPublicPrefix = "exs"

// BlueprintPublicID exposes the Render-compatible alias of a Blueprint's
// durable blp- key. The suffix never changes, so stored rows, CR labels and
// resource claims keep addressing the same Blueprint. Only minted shapes are
// aliases; names and malformed inputs must not acquire ID semantics.
func BlueprintPublicID(value string) string {
	return respell(value, Blueprint.prefix, BlueprintPublicPrefix)
}

// BlueprintStorageID resolves the public alias to the sole durable key
// namespace (blp-). Use it on every client-supplied Blueprint id before lookup,
// comparison or persistence; nothing is ever stored as exs-.
func BlueprintStorageID(value string) string {
	return respell(value, BlueprintPublicPrefix, Blueprint.prefix)
}
