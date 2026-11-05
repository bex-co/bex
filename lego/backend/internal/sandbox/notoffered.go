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

import "github.com/bex-co/bex/lego/backend/internal/core"

const (
	groupsNotOffered    = "bex does not support Render's sandbox groups: every sandbox belongs to its workspace directly, so there are no groups to create, list or update"
	imagesNotOffered    = "bex does not support Render's imported sandbox images (sandbox-groups images import|get, sandboxes create --image): every sandbox starts from its base image"
	snapshotsNotOffered = "bex does not support Render's sandbox snapshots (sandboxes snapshots create|list|get|delete, sandboxes create --from)"
)

// NotOfferedRoutes are the Render sandbox-group, image and snapshot operations
// the pinned Render CLI v2.35 calls (`sandbox-groups create|list`,
// `sandbox-groups images import|get`, `sandboxes snapshots …`) plus the group
// read/update Render documents beside them. Each answers a 404 naming the
// unsupported feature instead of the router's bare one, which the CLI prints
// verbatim (ADR018 § Render operation omissions, w2/m263). `images import`
// and `snapshots list` without --group first list groups, so the list refusal
// is what those commands show. The composition root mounts them independently
// of the sandbox Service: they answer the same whether or not OpenSandbox is
// configured, and they reach no workspace data.
var NotOfferedRoutes = map[string]core.NotOffered{
	"GET /v1/sandbox-groups":                                         {Code: CodeSandboxGroupsUnsupported, Message: groupsNotOffered},
	"POST /v1/sandbox-groups":                                        {Code: CodeSandboxGroupsUnsupported, Message: groupsNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}":                        {Code: CodeSandboxGroupsUnsupported, Message: groupsNotOffered},
	"PATCH /v1/sandbox-groups/{sandboxGroupId}":                      {Code: CodeSandboxGroupsUnsupported, Message: groupsNotOffered},
	"POST /v1/sandbox-groups/{sandboxGroupId}/builds":                {Code: CodeSandboxImagesUnsupported, Message: imagesNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/builds/{buildId}":       {Code: CodeSandboxImagesUnsupported, Message: imagesNotOffered},
	"POST /v1/sandboxes/{id}/snapshots":                              {Code: codeSandboxSnapshotsUnsupported, Message: snapshotsNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/snapshots":              {Code: codeSandboxSnapshotsUnsupported, Message: snapshotsNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/snapshots/{snapshotId}": {Code: codeSandboxSnapshotsUnsupported, Message: snapshotsNotOffered},
	"DELETE /v1/sandbox-groups/{sandboxGroupId}/snapshots/{snapshotId}": {
		Code: codeSandboxSnapshotsUnsupported, Message: snapshotsNotOffered,
	},
}
