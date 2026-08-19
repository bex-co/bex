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

package build

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Toolchain freshness inventory: committed digest pins stay in native.go /
// build.go / GitOps. This sidecar records the upstream tag and last-reviewed
// resolution time the ClusterBuilder age metric and the scheduled drift
// workflow both need. Updating a digest is a reviewed commit that must bump
// resolved_at for that entry.

//go:embed toolchain-freshness.json
var toolchainFreshnessJSON []byte

const (
	toolchainFreshnessSchema = "bex.build-toolchain-freshness/v1"
	// ClusterBuilderImageID is the inventory id whose resolved_at feeds the
	// live ClusterBuilder age metric.
	ClusterBuilderImageID = "cnb-builder"
)

var toolchainDigestRE = regexp.MustCompile(`sha256:[a-f0-9]{64}`)

// ToolchainInventory is the checked-in freshness metadata for every builder,
// native-base, and helper image the build plane pins.
type ToolchainInventory struct {
	Schema string           `json:"schema"`
	Images []ToolchainImage `json:"images"`
}

// ToolchainImage ties one logical pin to its upstream tag, committed digest,
// affected files, and last-reviewed resolution time.
type ToolchainImage struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Upstream   string   `json:"upstream"`
	Committed  string   `json:"committed"`
	ResolvedAt string   `json:"resolved_at"`
	Source     string   `json:"source"`
	Files      []string `json:"files"`
}

// LoadToolchainInventory parses the embedded inventory. Malformed metadata
// fails closed rather than returning a partial set.
func LoadToolchainInventory() (ToolchainInventory, error) {
	return ParseToolchainInventory(toolchainFreshnessJSON)
}

// ParseToolchainInventory validates machine-readable freshness metadata.
func ParseToolchainInventory(raw []byte) (ToolchainInventory, error) {
	var inv ToolchainInventory
	if err := json.Unmarshal(raw, &inv); err != nil {
		return ToolchainInventory{}, fmt.Errorf("toolchain inventory: %w", err)
	}
	if inv.Schema != toolchainFreshnessSchema {
		return ToolchainInventory{}, fmt.Errorf("toolchain inventory: schema %q, want %q", inv.Schema, toolchainFreshnessSchema)
	}
	if len(inv.Images) == 0 {
		return ToolchainInventory{}, fmt.Errorf("toolchain inventory: images is empty")
	}
	seen := map[string]struct{}{}
	builder := false
	for i, img := range inv.Images {
		if err := validateToolchainImage(i, img); err != nil {
			return ToolchainInventory{}, err
		}
		if _, dup := seen[img.ID]; dup {
			return ToolchainInventory{}, fmt.Errorf("toolchain inventory: duplicate id %q", img.ID)
		}
		seen[img.ID] = struct{}{}
		if img.ID == ClusterBuilderImageID {
			builder = true
		}
	}
	if !builder {
		return ToolchainInventory{}, fmt.Errorf("toolchain inventory: missing %s", ClusterBuilderImageID)
	}
	return inv, nil
}

func validateToolchainImage(i int, img ToolchainImage) error {
	prefix := fmt.Sprintf("toolchain inventory: images[%d]", i)
	if img.ID == "" {
		return fmt.Errorf("%s: missing id", prefix)
	}
	switch img.Kind {
	case "builder", "stack", "native-base", "helper":
	default:
		return fmt.Errorf("%s: unknown kind %q", prefix, img.Kind)
	}
	if img.Upstream == "" || strings.Contains(img.Upstream, "@") || !strings.Contains(img.Upstream, ":") {
		return fmt.Errorf("%s: upstream must be a tag reference", prefix)
	}
	if toolchainDigestRE.FindString(img.Committed) == "" {
		return fmt.Errorf("%s: committed must contain sha256:<64 hex>", prefix)
	}
	if _, err := time.Parse(time.RFC3339, img.ResolvedAt); err != nil {
		return fmt.Errorf("%s: resolved_at: %w", prefix, err)
	}
	if strings.TrimSpace(img.Source) == "" {
		return fmt.Errorf("%s: missing source", prefix)
	}
	if len(img.Files) == 0 {
		return fmt.Errorf("%s: files is empty", prefix)
	}
	return nil
}

// ClusterBuilderResolvedAt returns the last-reviewed resolution time of the
// platform ClusterBuilder image. A missing or unparseable entry fails closed.
func (inv ToolchainInventory) ClusterBuilderResolvedAt() (time.Time, error) {
	for _, img := range inv.Images {
		if img.ID != ClusterBuilderImageID {
			continue
		}
		t, err := time.Parse(time.RFC3339, img.ResolvedAt)
		if err != nil {
			return time.Time{}, fmt.Errorf("toolchain inventory: %s resolved_at: %w", ClusterBuilderImageID, err)
		}
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("toolchain inventory: missing %s", ClusterBuilderImageID)
}

// Digest extracts the sha256:… identity from a committed image reference.
func (img ToolchainImage) Digest() string {
	return toolchainDigestRE.FindString(img.Committed)
}
