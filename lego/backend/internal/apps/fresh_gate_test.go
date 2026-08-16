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

package apps

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// codex round-8 #8: deletion is irreversible (store row + CR cascade + OpenBao
// purge) — a member revoked inside PositiveTTL must not tear down one last
// service off a stale cached positive. Reuses ssh_test.go's sshAuthzRecorder
// (allow on the cached path, freshDeny on the uncached one).
func TestDeleteFailsClosedOnFreshRevocation(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	svc.Authz = &sshAuthzRecorder{allow: true, freshDeny: true}

	if err := svc.Delete(ctxAs("user-a"), "web"); !errors.Is(err, core.ErrForbidden) {
		t.Fatalf("Delete on a stale positive: %v, want ErrForbidden", err)
	}
	getApp(t, cl, "web") // the App CR must survive the refused delete
}
