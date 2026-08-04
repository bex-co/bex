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

package sshgateway

import "testing"

func TestSessionLimits(t *testing.T) {
	limits := NewSessionLimiter(2, 1)
	a, _ := limits.Acquire("a")
	aAgain, identityScope := limits.Acquire("a")
	b, _ := limits.Acquire("b")
	c, globalScope := limits.Acquire("c")
	if !a || aAgain || identityScope != "identity" || !b || c || globalScope != "global" {
		t.Fatal("global/per-identity limits not enforced")
	}
	limits.Release("a")
	c, _ = limits.Acquire("c")
	if !c {
		t.Fatal("released capacity was not reusable")
	}
}
