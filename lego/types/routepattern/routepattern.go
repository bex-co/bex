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

// Package routepattern holds the static-site rewrite grammar both sides of the
// App CR boundary must agree on. bex-api validates a rewrite's placeholders
// when the owner saves it; the operator's static server expands those same
// placeholders when it serves a request. A second, hand-copied grammar lets
// the control plane accept a pattern the data plane cannot match (or reject
// one it can), so the rule lives here rather than in either side.
package routepattern

// PlaceholderName returns the capture name of a whole-segment ":name"
// placeholder: [A-Za-z_][A-Za-z0-9_]*. Anything else is not a placeholder.
func PlaceholderName(segment string) (string, bool) {
	if len(segment) < 2 || segment[0] != ':' {
		return "", false
	}
	name := segment[1:]
	for i, c := range name {
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return "", false
	}
	return name, true
}
