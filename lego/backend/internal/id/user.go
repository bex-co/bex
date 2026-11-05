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

// UserPublicPrefix is Render's spelling of a user id (`userIdPathParam`,
// `^usr-[0-9a-v]{20}$`). Every client-read surface projects the durable own-
// key into this namespace, as Blueprints do with exs- (w5/m277).
const UserPublicPrefix = "usr"

// UserPublicID exposes the Render-compatible alias of a user's durable own-
// id. The suffix never changes; other values pass through unchanged.
func UserPublicID(value string) string {
	return respell(value, Owner.prefix, UserPublicPrefix)
}

// UserStorageID resolves the public alias to the durable own- key. Use it on
// every client-supplied user id before lookup; nothing is stored as usr-.
func UserStorageID(value string) string {
	return respell(value, UserPublicPrefix, Owner.prefix)
}
