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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestParseStackRejectsProjectWithNoResources(t *testing.T) {
	// A project environment alone is not a deployable stack: the guard must
	// fire even though the manifest is not literally empty.
	manifest := `
projects:
  - name: p
    environments:
      - name: staging
`
	_, err := parseStack(DeployRequest{Manifest: manifest})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "must define at least one") {
		t.Errorf("resource-less project manifest => named ErrBadRequest, got %v", err)
	}
}

func TestParseStackDatabaseAllowListRejectsBadCIDR(t *testing.T) {
	manifest := `
databases:
  - name: db
    ipAllowList:
      - source: not-a-cidr
`
	_, err := parseStack(DeployRequest{Manifest: manifest})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), `database "db" ipAllowList`) {
		t.Errorf("database allowlist with bad CIDR => named ErrBadRequest, got %v", err)
	}
}

func TestManifestAllowEntriesValidatesAndMaps(t *testing.T) {
	entries, err := manifestAllowEntries([]bexIPEntry{{Source: "192.0.2.0/24", Description: "office"}}, "database", "db")
	if err != nil || len(entries) != 1 || entries[0].CIDRBlock != "192.0.2.0/24" || entries[0].Description != "office" {
		t.Errorf("manifestAllowEntries = %+v, %v", entries, err)
	}
	if entries, err := manifestAllowEntries(nil, "database", "db"); err != nil || entries != nil {
		t.Errorf("empty input => nil, nil; got %+v, %v", entries, err)
	}
}

func TestManifestAllowEntriesRejectsMissingSourceAndBadCIDR(t *testing.T) {
	for _, tt := range []struct {
		name     string
		entries  []bexIPEntry
		noun     string
		resource string
		message  string
	}{
		{name: "missing source", entries: []bexIPEntry{{Description: "office"}}, noun: "key-value", resource: "cache", message: `key-value "cache" has an ipAllowList entry without a source`},
		{name: "bad CIDR", entries: []bexIPEntry{{Source: "not-a-cidr"}}, noun: "database", resource: "db", message: `database "db" ipAllowList`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manifestAllowEntries(tt.entries, tt.noun, tt.resource)
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tt.message) {
				t.Errorf("manifestAllowEntries => named ErrBadRequest containing %q, got %v", tt.message, err)
			}
		})
	}
}
