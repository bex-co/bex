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

const envNamePattern = /^[A-Z_][A-Z0-9_]*$/;

function jsonArray(value, name) {
  if (!value) return [];
  let parsed;
  try {
    parsed = JSON.parse(value);
  } catch (error) {
    throw new Error(`${name} must be a JSON array: ${error.message}`);
  }
  if (
    !Array.isArray(parsed) ||
    parsed.some((item) => typeof item !== "string")
  ) {
    throw new Error(`${name} must be a JSON array of strings`);
  }
  return parsed;
}

function jsonObject(value, name) {
  if (!value) return {};
  let parsed;
  try {
    parsed = JSON.parse(value);
  } catch (error) {
    throw new Error(`${name} must be a JSON object: ${error.message}`);
  }
  if (
    parsed === null ||
    Array.isArray(parsed) ||
    typeof parsed !== "object" ||
    Object.values(parsed).some((item) => typeof item !== "string")
  ) {
    throw new Error(`${name} must be a JSON object with string values`);
  }
  return parsed;
}

function positivePort(value) {
  const port = Number(value || 8787);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    throw new Error("BEX_AGENT_LISTEN_PORT must be an integer from 1 to 65535");
  }
  return port;
}

function positiveMilliseconds(value) {
  const milliseconds = Number(value || 4 * 60 * 60 * 1000);
  if (!Number.isInteger(milliseconds) || milliseconds < 1) {
    throw new Error("BEX_AGENT_TURN_TIMEOUT_MS must be a positive integer");
  }
  return milliseconds;
}

export function loadConfig(env = process.env) {
  const cwd = path.resolve(env.BEX_AGENT_CWD || "/workspace");
  const sessionLogPath =
    env.BEX_AGENT_SESSION_LOG || "/var/log/bex-agent/session.jsonl";
  const statusPath =
    env.BEX_AGENT_STATUS_FILE || "/var/run/bex-agent/status.json";
  const credentialEnvName =
    env.BEX_AGENT_MODEL_API_KEY_ENV || "ANTHROPIC_API_KEY";
  if (!envNamePattern.test(credentialEnvName)) {
    throw new Error(
      "BEX_AGENT_MODEL_API_KEY_ENV must be a valid environment variable name",
    );
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

  return {
    command: env.BEX_AGENT_COMMAND || "claude-code-acp",
    args: jsonArray(env.BEX_AGENT_ARGS, "BEX_AGENT_ARGS"),
    cwd,
    prompt: env.BEX_AGENT_PROMPT || "",
    existingSessionId: env.BEX_AGENT_EXISTING_SESSION_ID || "",
    persistSession: env.BEX_AGENT_PERSIST_SESSION !== "0",
    listenHost: env.BEX_AGENT_LISTEN_HOST || "0.0.0.0",
    listenPort: positivePort(env.BEX_AGENT_LISTEN_PORT),
    sessionLogPath,
    statusPath,
    exitAfterTurn: env.BEX_AGENT_EXIT_AFTER_TURN === "1",
    turnTimeoutMs: positiveMilliseconds(env.BEX_AGENT_TURN_TIMEOUT_MS),
    credentialEnvName,
    modelCredential: env.BEX_AGENT_MODEL_API_KEY || "",
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
