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

package hostingdomain

import "testing"

func TestValidateSharedSuffix(t *testing.T) {
	for _, allowed := range []string{"", "onrender.com", "github.io"} {
		if err := ValidateSharedSuffix(allowed); err != nil {
			t.Errorf("ValidateSharedSuffix(%q) = %v, want nil", allowed, err)
		}
	}
	for _, rejected := range []string{"onbex.co", "example.com", "com", "LOCALHOST", "onrender.com."} {
		if err := ValidateSharedSuffix(rejected); err == nil {
			t.Errorf("ValidateSharedSuffix(%q) = nil, want rejection", rejected)
		}
	}
}
