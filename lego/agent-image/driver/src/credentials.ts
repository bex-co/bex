/*
 * Copyright 2026.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import type { AgentDriverConfig } from "./config.js";

// agentEnvironment builds the ACP child's environment (ADR062). The model
// credential is the session-bound placeholder the gateway model proxy swaps on
// its upstream hop. BEX_AGENT_MODEL_API_KEY and the adapter's credential name
// are removed from the driver's own inherited env, so only the child sees the
// placeholder, under the one name its adapter reads.
export function agentEnvironment(
  config: Pick<
    AgentDriverConfig,
    | "agentEnv"
    | "credentialEnvName"
    | "modelCredential"
    | "modelBaseUrl"
    | "modelBaseUrlEnvName"
  >,
  env: NodeJS.ProcessEnv = process.env,
): Record<string, string> {
  delete env.BEX_AGENT_MODEL_API_KEY;
  delete env[config.credentialEnvName];
  return {
    ...config.agentEnv,
    [config.modelBaseUrlEnvName]: config.modelBaseUrl,
    [config.credentialEnvName]: config.modelCredential,
  };
}
