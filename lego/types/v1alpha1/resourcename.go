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

import "k8s.io/apimachinery/pkg/util/validation"

// MaxResourceNameLength is the longest name a service, Postgres or Key Value
// takes: a DNS-1123 label cut to 30 characters, so "<workspace>-<name>" always
// fits the 63-character object-name limit.
const MaxResourceNameLength = 30

// ValidResourceName reports whether name is a valid user-facing name for a
// service, Postgres or Key Value: a DNS-1123 label of at most
// MaxResourceNameLength characters. The API, the store and Blueprint parsing
// share it, and the CRDs' validation markers mirror it, so no entry point
// accepts a name another refuses (w5/m118: three copies had drifted, and
// Blueprint Key Values checked none).
func ValidResourceName(name string) bool {
	return len(name) <= MaxResourceNameLength && len(validation.IsDNS1123Label(name)) == 0
}
