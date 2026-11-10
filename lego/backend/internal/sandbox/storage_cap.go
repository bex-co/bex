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
	if s.MaxStorageBytes <= 0 || s.Storage == nil {
		return nil
	}
	workspace := raw.Metadata[metadataWorkspace]
	used, err := s.Storage.SandboxStorageBytes(ctx, workspace)
	if err != nil {
		log.Printf("sandbox storage: cap check workspace=%s: %v", workspace, err)
		return nil
	}
	if used < s.MaxStorageBytes {
		return nil
	}
	s.Metrics.observeRefusal("storage", "suspend")
	return core.NewTooManyRequestsError(CodeSandboxStorageLimit,
		fmt.Sprintf("this workspace's suspended sandboxes already hold %s of its %s sandbox storage limit; terminate sandboxes you no longer need, then suspend again",
			formatGB(used), formatGB(s.MaxStorageBytes)),
		map[string]any{"limitBytes": s.MaxStorageBytes, "usedBytes": used})
}

func formatGB(bytes int64) string {
	return fmt.Sprintf("%.1f GB", float64(bytes)/float64(store.BytesPerDecimalGB))
}
