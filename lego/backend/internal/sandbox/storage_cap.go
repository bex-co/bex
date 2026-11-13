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
	"fmt"
	"log"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CodeSandboxStorageLimit is a suspend refused because the workspace's retained
// sandbox images are at its storage cap (ADR104 D11).
const CodeSandboxStorageLimit = "SANDBOX_STORAGE_LIMIT"

// StorageUsage reads the bytes a workspace's suspended sandboxes retain: the
// storage meter's open, sized objects.
type StorageUsage interface {
	SandboxStorageBytes(ctx context.Context, workspaceID string) (int64, error)
}

// checkStorageCap refuses the suspend of a running sandbox whose workspace
// already retains MaxStorageBytes of images: the suspend would commit one more.
// The refused sandbox keeps running, which is billed as compute, so the cap
// never costs the customer more than leaving the sandbox up. A read failure
// does not refuse: the cap bounds bex's registry, and blocking every suspend on
// a database blip would keep every idle sandbox billing.
func (s *Service) checkStorageCap(ctx context.Context, raw osSandbox) error {
	used, full := s.storageFull(ctx, raw.Metadata[metadataWorkspace])
	if !full {
		return nil
	}
	s.Metrics.observeRefusal("storage", "suspend")
	return core.NewTooManyRequestsError(CodeSandboxStorageLimit,
		fmt.Sprintf("this workspace's suspended sandboxes and sandbox images already hold %s of its %s sandbox storage limit; terminate sandboxes you no longer need, then suspend again",
			formatGB(used), formatGB(s.MaxStorageBytes)),
		map[string]any{"limitBytes": s.MaxStorageBytes, "usedBytes": used})
}

// storageFull reports whether the workspace's retained images (suspended
// sandboxes' and imported ones, w2/m358) have reached MaxStorageBytes, and
// how much they hold. A read failure is not full: the cap bounds bex's
// registry, and refusing on every outage would cost more than it saves.
func (s *Service) storageFull(ctx context.Context, workspace string) (int64, bool) {
	if s.MaxStorageBytes <= 0 || s.Storage == nil {
		return 0, false
	}
	used, err := s.Storage.SandboxStorageBytes(ctx, workspace)
	if err != nil {
		log.Printf("sandbox storage: cap check workspace=%s: %v", workspace, err)
		return 0, false
	}
	return used, used >= s.MaxStorageBytes
}

func formatGB(bytes int64) string {
	return fmt.Sprintf("%.1f GB", float64(bytes)/float64(store.BytesPerDecimalGB))
}
