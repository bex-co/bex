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

package api

import "github.com/bex-co/bex/lego/backend/internal/core"

// scopeClassOverrides is the hand-maintained non-default classification.
// defaultScopeClass treats GET/Query/list_/get_ as read and everything else as
// write; this map lifts mint (durable credentials) and sensitive reveals.
//
// Shell tickets are sensitive (a shell is printenv — can_view_sensitive), not
// mint (they are short-lived, not a durable credential). Deploy-hook GET is
// mint because it reveals/creates an unguessable trigger URL.
var scopeClassOverrides = map[string]string{
	// Read-only bearer preview uses a POST body to keep capabilities out of URLs.
	"REST POST /v1/invites/preview": core.OpClassRead,
	"MCP preview_workspace_invite":  core.OpClassRead,
	// Durable-credential mint. AuthorizeMintClass still runs at the verb.
	"REST POST /v1/api-keys":                             core.OpClassMint,
	"REST POST /v1/ssh-keys":                             core.OpClassMint,
	"REST POST /v1/postgres/{id}/users":                  core.OpClassMint,
	"REST GET /v1/services/{id}/deploy-hook":             core.OpClassMint,
	"REST POST /v1/services/{id}/deploy-hook/regenerate": core.OpClassMint,
	"GQL Mutation.createApiKey":                          core.OpClassMint,
	"GQL Mutation.createSSHKey":                          core.OpClassMint,
	"GQL Mutation.createDatabaseUser":                    core.OpClassMint,
	"GQL Query.deployHook":                               core.OpClassMint,
	"GQL Mutation.regenerateDeployHook":                  core.OpClassMint,
	"MCP create_api_key":                                 core.OpClassMint,
	"MCP add_ssh_key":                                    core.OpClassMint,
	"MCP create_postgres_user":                           core.OpClassMint,
	"MCP get_deploy_hook":                                core.OpClassMint,
	"MCP regenerate_deploy_hook":                         core.OpClassMint,

	// Env-var / secret-file value reveals (list responses include values).
	"REST GET /v1/services/{id}/env-vars":              core.OpClassSensitive,
	"REST GET /v1/services/{id}/env-vars/{key}":        core.OpClassSensitive,
	"REST GET /v1/services/{id}/secret-files":          core.OpClassSensitive,
	"REST GET /v1/services/{id}/secret-files/{name}":   core.OpClassSensitive,
	"REST GET /v1/env-groups/{id}/env-vars/{key}":      core.OpClassSensitive,
	"REST GET /v1/env-groups/{id}/secret-files/{name}": core.OpClassSensitive,
	"GQL Query.envVars":                                core.OpClassSensitive,
	"GQL Query.secretFiles":                            core.OpClassSensitive,
	"GQL Query.envGroupVar":                            core.OpClassSensitive,
	"GQL Query.envGroupSecretFile":                     core.OpClassSensitive,
	"MCP list_env_vars":                                core.OpClassSensitive,
	"MCP get_env_var":                                  core.OpClassSensitive,
	"MCP list_secret_files":                            core.OpClassSensitive,
	"MCP get_secret_file":                              core.OpClassSensitive,
	"MCP get_env_group_var":                            core.OpClassSensitive,
	"MCP get_env_group_secret_file":                    core.OpClassSensitive,

	// Linking materializes every environment-group value in App workload code.
	"REST POST /v1/env-groups/{id}/services/{serviceId}": core.OpClassSensitive,
	"GQL Mutation.linkEnvGroup":                          core.OpClassSensitive,
	"MCP link_env_group":                                 core.OpClassSensitive,

	// Datastore connection info, SQL text, dump URLs, live query text.
	"REST GET /v1/postgres/{id}/connection-info":  core.OpClassSensitive,
	"REST POST /v1/postgres/{id}/query":           core.OpClassSensitive,
	"REST GET /v1/postgres/{id}/export":           core.OpClassSensitive,
	"REST GET /v1/postgres/{id}/processes":        core.OpClassSensitive,
	"REST GET /v1/postgres/{id}/top-queries":      core.OpClassSensitive,
	"REST GET /v1/key-value/{id}/connection-info": core.OpClassSensitive,
	"GQL Query.databaseConnectionInfo":            core.OpClassSensitive,
	"GQL Query.databaseExports":                   core.OpClassSensitive,
	"GQL Query.databaseProcesses":                 core.OpClassSensitive,
	"GQL Query.databaseTopQueries":                core.OpClassSensitive,
	"GQL Query.keyValueConnectionInfo":            core.OpClassSensitive,
	"MCP list_postgres_exports":                   core.OpClassSensitive,
	"MCP list_postgres_processes":                 core.OpClassSensitive,
	"MCP list_postgres_top_queries":               core.OpClassSensitive,
	"MCP query_render_postgres":                   core.OpClassSensitive,

	// Blueprint preview/generate can include env values.
	"REST POST /v1/blueprints/preview":  core.OpClassSensitive,
	"REST POST /v1/blueprints/generate": core.OpClassSensitive,
	"GQL Query.blueprintPreview":        core.OpClassSensitive,
	"GQL Query.generateBlueprint":       core.OpClassSensitive,
	"MCP preview_blueprint":             core.OpClassSensitive,
	"MCP generate_blueprint":            core.OpClassSensitive,

	// Browser Web Shell ticket: can_view_sensitive, short-lived, not mint.
	"REST POST /v1/services/{id}/shell-ticket": core.OpClassSensitive,
	"GQL Mutation.createShellSession":          core.OpClassSensitive,
}
