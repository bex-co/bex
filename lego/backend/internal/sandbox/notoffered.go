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
	imagesNotOffered      = "bex does not support Render's imported sandbox images (sandbox-groups images import|get, sandboxes create --image): every sandbox starts from its base image"
	snapshotsNotOffered   = "bex does not support Render's sandbox snapshots (sandboxes snapshots create|list|get|delete, sandboxes create --from)"
	groupOpNotOffered     = "bex does not support deleting a sandbox group or changing the workspace's default group (sandbox-groups delete, sandbox-groups default set)"
	credentialsNotOffered = "bex does not support Render's brokered sandbox credentials (sandbox-groups credentials), an early-access feature"
)

// CodeSandboxGroupOperationUnsupported names the group operations CLI v2.36
// added that bex's served groups (w22/m311) do not answer yet: delete and the
// workspace default (w3/m303).
const CodeSandboxGroupOperationUnsupported = "SANDBOX_GROUP_OPERATION_UNSUPPORTED"

// CodeSandboxCredentialsUnsupported names Render's early-access brokered
// sandbox credentials (CLI v2.36 `sandbox-groups credentials`, w3/m303).
const CodeSandboxCredentialsUnsupported = "SANDBOX_CREDENTIALS_UNSUPPORTED"

// NotOfferedRoutes are the Render sandbox image, snapshot and group-management
// operations the pinned Render CLI calls (`sandbox-groups images import|get`,
// `sandboxes snapshots …`; v2.36's `sandbox-groups delete|default set|credentials`)
// and bex does not serve. Each answers a 404 naming the
// unsupported feature instead of the router's bare one, which the CLI prints
// verbatim (ADR018 § Render operation omissions, w2/m263). The group routes
// themselves (`sandbox-groups create|list`, retrieve, update) are served by the
// Service since w22/m311. The composition root mounts these independently of
// the sandbox Service: they answer the same whether or not OpenSandbox is
// configured, and they reach no workspace data.
var NotOfferedRoutes = map[string]core.NotOffered{
	// CLI v2.36's `sandbox-groups delete`, `default set` and `credentials`
	// (w3/m303), absent from the public OpenAPI and not served by the group
	// routes w22/m311 added.
	"DELETE /v1/sandbox-groups/{sandboxGroupId}":                                  {Code: CodeSandboxGroupOperationUnsupported, Message: groupOpNotOffered},
	"POST /v1/sandbox-groups/{sandboxGroupId}/set-default":                        {Code: CodeSandboxGroupOperationUnsupported, Message: groupOpNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/credentials":                         {Code: CodeSandboxCredentialsUnsupported, Message: credentialsNotOffered},
	"POST /v1/sandbox-groups/{sandboxGroupId}/credentials":                        {Code: CodeSandboxCredentialsUnsupported, Message: credentialsNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}":          {Code: CodeSandboxCredentialsUnsupported, Message: credentialsNotOffered},
	"PATCH /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}":        {Code: CodeSandboxCredentialsUnsupported, Message: credentialsNotOffered},
	"DELETE /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}":       {Code: CodeSandboxCredentialsUnsupported, Message: credentialsNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}/versions": {Code: CodeSandboxCredentialsUnsupported, Message: credentialsNotOffered},
	"POST /v1/sandbox-groups/{sandboxGroupId}/credentials/{credentialId}/restore": {Code: CodeSandboxCredentialsUnsupported, Message: credentialsNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/builds":                              {Code: CodeSandboxImagesUnsupported, Message: imagesNotOffered},
	"POST /v1/sandbox-groups/{sandboxGroupId}/builds":                             {Code: CodeSandboxImagesUnsupported, Message: imagesNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/builds/{buildId}":                    {Code: CodeSandboxImagesUnsupported, Message: imagesNotOffered},
	"POST /v1/sandboxes/{id}/snapshots":                                           {Code: codeSandboxSnapshotsUnsupported, Message: snapshotsNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/snapshots":                           {Code: codeSandboxSnapshotsUnsupported, Message: snapshotsNotOffered},
	"GET /v1/sandbox-groups/{sandboxGroupId}/snapshots/{snapshotId}":              {Code: codeSandboxSnapshotsUnsupported, Message: snapshotsNotOffered},
	"DELETE /v1/sandbox-groups/{sandboxGroupId}/snapshots/{snapshotId}": {
		Code: codeSandboxSnapshotsUnsupported, Message: snapshotsNotOffered,
	},
}
