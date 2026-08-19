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

// IDList normalizes a patch tool's optional membership argument to the
// non-nil slice the SetX verbs expect (w1/m71). The pointer carries "was this
// membership mentioned at all"; once the caller has decided it was, an explicit
// null and an explicit [] both mean the same thing — empty this membership —
// which is the coercion each retired set_* tool did with a required argument.
func IDList(ids *[]string) []string {
	if ids == nil || *ids == nil {
		return []string{}
	}
	return *ids
}
