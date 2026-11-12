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

package logs

import (
	"os"
	"testing"
	"time"
)

// The package's tail tests feed pod logs dated in the past, so a tail with no
// startTime keeps no lower bound here; TestATailWithNoStartTimeBeginsWhenItOpens
// pins the production rule (w8/288).
func TestMain(m *testing.M) {
	tailOpenedAt = func() time.Time { return time.Time{} }
	os.Exit(m.Run())
}
