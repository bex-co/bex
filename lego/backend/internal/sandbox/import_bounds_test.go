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

package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestImportCountsItsOwnSizeAgainstTheStorageCap is round 24's S-3: imports
// still copying counted for nothing, so ten groups' in-flight imports could
// store far past the workspace cap. The store checks a new import's declared
// size against the cap under a workspace lock, after returning a build the
// group already has, so a repeated import is never refused.
func TestImportCountsItsOwnSizeAgainstTheStorageCap(t *testing.T) {
	f := newImageFixture(t)
	ref := hostOf(f.source) + "/acme/app:1"
	img, _ := pushImage(t, ref, linuxAMD64, 2048)
	size, err := imageBytes(img)
	if err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"name":"app","sourceImage":%q}`, ref)

	f.svc.MaxStorageBytes = size - 1 // the workspace holds nothing yet
	code, resp := f.importImage(t, body)
	if code != http.StatusTooManyRequests || errCode(resp) != CodeSandboxStorageLimit || !strings.Contains(resp, "would take") {
		t.Fatalf("import past the cap = %d %s, want 429 %s naming the image's size", code, resp, CodeSandboxStorageLimit)
	}
	if rows, err := f.st.SandboxBuilds(context.Background(), f.ws, f.group.ID, "", nil); err != nil || len(rows) != 0 {
		t.Fatalf("a refused import left %d builds, %v", len(rows), err)
	}

	f.svc.MaxStorageBytes = size // exactly room for it
	first := f.mustImport(t, body)
	if first.Status != "pending" && first.Status != "running" && first.Status != "succeeded" {
		t.Fatalf("import within the cap = %+v", first)
	}
	// The workspace is now at its cap, counting the import still copying, so a
	// second image is refused while the same import again returns its build.
	other := hostOf(f.source) + "/acme/other:1"
	pushImage(t, other, linuxAMD64, 2048)
	if code, resp := f.importImage(t, fmt.Sprintf(`{"name":"other","sourceImage":%q}`, other)); code != http.StatusTooManyRequests {
		t.Fatalf("a second import at the cap = %d %s, want 429", code, resp)
	}
	code, resp = f.importImage(t, body)
	var again Build
	if err := json.Unmarshal([]byte(resp), &again); err != nil || code >= 300 || again.ID != first.ID {
		t.Fatalf("repeating the import at the cap = %d %s, want build %s back", code, resp, first.ID)
	}
	if got := f.waitBuild(t, first.ID); got.Status != "succeeded" {
		t.Fatalf("import = %+v, want succeeded", got)
	}
}
