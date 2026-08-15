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

import path from "node:path";

export interface AgentDriverConfig {
  command: string;
  args: string[];
  cwd: string;
  prompt: string;
  // Delivery (ADR047 D4): the branch the driver commits + pushes, the repo it
  // clones when the workspace is empty, the PR base branch, and the delivery
  // toggle. Deliver defaults off so a bare `run one turn` stays unchanged.
  branch: string;
  repoUrl: string;
  baseBranch: string;
  deliver: boolean;
  gitName: string;
  gitEmail: string;
  existingSessionId: string;
  persistSession: boolean;
  listenHost: string;
  listenPort: number;
  sessionLogPath: string;
  statusPath: string;
  exitAfterTurn: boolean;
  turnTimeoutMs: number;
  credentialEnvName: string;
  modelCredential: string;
  sessionID: string;
  grantPublicKey: string;
  agentEnv: Record<string, string>;
  scrubRoots: string[];
}

const envNamePattern = /^[A-Z_][A-Z0-9_]*$/;

function jsonArray(value: string | undefined, name: string): string[] {
  if (!value) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch (error) {
    throw new Error(
      `${name} must be a JSON array: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
  if (
    !Array.isArray(parsed) ||
    parsed.some((item) => typeof item !== "string")
  ) {
    throw new Error(`${name} must be a JSON array of strings`);
  }
  return parsed;
}

function jsonObject(
  value: string | undefined,
  name: string,
): Record<string, string> {
  if (!value) return {};
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch (error) {
    throw new Error(
      `${name} must be a JSON object: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
  if (
    parsed === null ||
    Array.isArray(parsed) ||
    typeof parsed !== "object" ||
    Object.values(parsed).some((item) => typeof item !== "string")
  ) {
    throw new Error(`${name} must be a JSON object with string values`);
  }
  return parsed as Record<string, string>;
}

function positivePort(value: string | undefined): number {
  const port = Number(value || 8787);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error("BEX_AGENT_LISTEN_PORT must be an integer from 1 to 65535");
  }
  return port;
}

function positiveMilliseconds(value: string | undefined): number {
  const milliseconds = Number(value || 4 * 60 * 60 * 1000);
  if (!Number.isInteger(milliseconds) || milliseconds < 1) {
    throw new Error("BEX_AGENT_TURN_TIMEOUT_MS must be a positive integer");
  }
  return milliseconds;
}

// Anthropic OAuth tokens (minted by `claude setup-token`, prefix `sk-ant-oat`)
// authenticate claude-code through CLAUDE_CODE_OAUTH_TOKEN (Bearer + the oauth
// beta header), not the x-api-key ANTHROPIC_API_KEY path — which rejects them as
// "Invalid API key". Route a BYO credential to the agent-native variable by its
// shape when BEX_AGENT_MODEL_API_KEY_ENV isn't pinned explicitly.
function defaultCredentialEnvName(credential: string): string {
  return credential.startsWith("sk-ant-oat")
    ? "CLAUDE_CODE_OAUTH_TOKEN"
    : "ANTHROPIC_API_KEY";
}

export function loadConfig(
  env: NodeJS.ProcessEnv = process.env,
): AgentDriverConfig {
  const cwd = path.resolve(env.BEX_AGENT_CWD || "/workspace");
  const sessionLogPath =
    env.BEX_AGENT_SESSION_LOG || "/var/log/bex-agent/session.jsonl";
  const statusPath =
    env.BEX_AGENT_STATUS_FILE || "/var/run/bex-agent/status.json";
  const modelCredential = env.BEX_AGENT_MODEL_API_KEY || "";
  const credentialEnvName =
    env.BEX_AGENT_MODEL_API_KEY_ENV ||
    defaultCredentialEnvName(modelCredential);
  if (!envNamePattern.test(credentialEnvName)) {
    throw new Error(
      "BEX_AGENT_MODEL_API_KEY_ENV must be a valid environment variable name",
    );
  }
  if (env.BEX_AGENT_DELIVER === "1" && !(env.BEX_AGENT_BRANCH || "")) {
    throw new Error("BEX_AGENT_DELIVER=1 requires BEX_AGENT_BRANCH");
  }
  const agentEnv = jsonObject(env.BEX_AGENT_ENV_JSON, "BEX_AGENT_ENV_JSON");
  if (
    Object.hasOwn(agentEnv, "BEX_AGENT_MODEL_API_KEY") ||
    Object.hasOwn(agentEnv, credentialEnvName)
  ) {
    throw new Error(
      "model credentials must use BEX_AGENT_MODEL_API_KEY, not BEX_AGENT_ENV_JSON",
    );
  }

  const command = env.BEX_AGENT_COMMAND || "/usr/local/bin/claude-code-acp";
  const allowedCommands = new Set([
    "/usr/local/bin/claude-code-acp",
    "/usr/local/bin/codex-acp",
    "/usr/local/bin/gemini",
  ]);
  if (!path.isAbsolute(command) || !allowedCommands.has(command)) {
    throw new Error("BEX_AGENT_COMMAND must be an installed agent adapter path");
  }

  return {
    command,
    args: jsonArray(env.BEX_AGENT_ARGS, "BEX_AGENT_ARGS"),
    cwd,
    prompt: env.BEX_AGENT_PROMPT || "",
    branch: env.BEX_AGENT_BRANCH || "",
    repoUrl: env.BEX_AGENT_REPO_URL || "",
    baseBranch: env.BEX_AGENT_BASE_BRANCH || "",
    deliver: env.BEX_AGENT_DELIVER === "1",
    gitName: env.BEX_AGENT_GIT_NAME || "bex agent",
    gitEmail: env.BEX_AGENT_GIT_EMAIL || "agent@bex.co",
    existingSessionId: env.BEX_AGENT_EXISTING_SESSION_ID || "",
    persistSession: env.BEX_AGENT_PERSIST_SESSION !== "0",
    listenHost: env.BEX_AGENT_LISTEN_HOST || "0.0.0.0",
    listenPort: positivePort(env.BEX_AGENT_LISTEN_PORT),
    sessionLogPath,
    statusPath,
    exitAfterTurn: env.BEX_AGENT_EXIT_AFTER_TURN === "1",
    turnTimeoutMs: positiveMilliseconds(env.BEX_AGENT_TURN_TIMEOUT_MS),
    credentialEnvName,
    modelCredential,
    sessionID: env.BEX_AGENT_SESSION_ID || "",
    grantPublicKey: env.BEX_AGENT_GRANT_PUBLIC_KEY || "",
    agentEnv,
    scrubRoots: [
      ...new Set(
        (
          env.BEX_AGENT_SCRUB_ROOTS ||
          [
            cwd,
            env.HOME || "/home/bex",
            "/tmp",
            "/var/tmp",
            path.dirname(sessionLogPath),
            path.dirname(statusPath),
          ].join(",")
        )
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean)
          .map((item) => path.resolve(item)),
      ),
    ],
  };
}
