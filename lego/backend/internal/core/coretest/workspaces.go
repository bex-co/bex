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

// Package coretest holds test doubles for core's seams that every feature's
// tests share (the sshgateway/gatewaytest precedent).
package coretest

import (
	"context"
	"slices"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Members is a core.WorkspaceResolver keyed by subject: each subject belongs to
// the listed workspaces, the first being their default.
type Members map[string][]string

func (m Members) Tenant(ctx context.Context, id core.Identity) (string, bool) {
	return Workspaces(m[id.Subject]).Tenant(ctx, id)
}

func (m Members) IsMember(ctx context.Context, id core.Identity, workspaceID string) (bool, error) {
	return Workspaces(m[id.Subject]).IsMember(ctx, id, workspaceID)
}

// Workspaces is Members for tests where every caller belongs to the same
// workspaces, the first being the default.
type Workspaces []string

func (w Workspaces) Tenant(context.Context, core.Identity) (string, bool) {
	if len(w) == 0 {
		return "", false
	}
	return w[0], true
}

func (w Workspaces) IsMember(_ context.Context, _ core.Identity, workspaceID string) (bool, error) {
	return slices.Contains(w, workspaceID), nil
}
