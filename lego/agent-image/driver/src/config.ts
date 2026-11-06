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

import {
  parseAttachmentManifest,
  type AttachmentManifest,
} from "./attachments.js";
import path from "node:path";
import { describeError } from "./errors.js";
import { loadAgentProfiles, lookupAgentProfile } from "./profiles.js";
import { isDeepStrictEqual } from "node:util";
import { secretValuesFromEnv } from "./redact.js";
import { parseSkillManifest, type SkillManifest } from "./skills.js";

export interface AgentDriverConfig {
  // Files the user attached to the session (ADR095): what setup waits for and
  // the turn presents to the agent. Undefined when none.
  attachments?: AttachmentManifest;
  // The workspace's skills (ADR099): what setup waits for, links into the
  // engines' skills folders and lists in the overlay. Undefined when none.
  skills?: SkillManifest;
  command: string;
  args: string[];
  // Release-profile id (agent-profiles.json): selects the overlay channel for
  // adapters without the ACP systemPrompt capability (Codex reads
  // developer_instructions from CODEX_CONFIG).
  profile: string;
  cwd: string;
  prompt: string;
  // Repo session (ADR090): the bound bex-agent/* branch the agent pushes, the
  // Pod-bound Git proxy URL the setup phase clones, the PR base branch, and the
  // repo-session toggle (setup clone + post-turn delivery check + PR tool).
  // The driver itself never commits or pushes.
  branch: string;
  repoUrl: string;
  baseBranch: string;
  // owner/name of the bound repository (display only, for the overlay).
  repository: string;
  // Captured from the control-plane Git proxy during setup, before any tenant
  // agent process runs. The delivery check compares the remote head to baseOid.
  deliveryBaseline?: {
    baseBranch: string;
    baseOid: string;
    remoteBranchOid: string;
  };
  deliver: boolean;
  gitName: string;
  gitEmail: string;
  existingSessionId: string;
  persistSession: boolean;
  // Continuity priming material for a fresh agent generation (ADR047 D3
  // ladder, w5/m84). bex-api injects at most one: contextJson is the prior
  // conversation extract (rung 2), originalTask the session task to re-deliver
  // when the agent never ran before (rung 3). Both are ignored when ACP
  // session/load succeeds (rung 1).
  contextJson: string;
  originalTask: string;
  listenHost: string;
  listenPort: number;
  sessionLogPath: string;
  statusPath: string;
  // Driver-owned file (outside the repo) where the bex MCP server records the
  // last successful open_or_update_draft_pr result.
  prRecordPath: string;
  // The session's Pod-bound share URL (ADR095 D6) the bex MCP server's
  // share_file posts to; empty when the deployment has no attachments store.
  shareUrl: string;
  // Structured output (ADR094 D5): the session's Pod-bound result URL, the
  // turn's JSON Schema text and whether the turn must end with a final result.
  // Empty when the turn is free-form (no submit_result tool, no reminder).
  resultUrl: string;
  outputSchema: string;
  outputRequired: boolean;
  // Driver-owned file where the bex MCP server records whether a final result
  // was accepted, which drives the one reminder.
  resultRecordPath: string;
  // The draft PR earlier turns of this session opened (empty on turn 1), so a
  // re-dispatched sandbox updates it rather than describing only its own turn.
  existingPrUrl: string;
  existingPrNumber: number | null;
  exitAfterTurn: boolean;
  turnTimeoutMs: number;
  credentialEnvName: string;
  // Only the session-bound model-proxy placeholder (ADR062); never a real key.
  modelCredential: string;
  sessionID: string;
  /** Control-plane turn number stamped into every durable log record. */
  turn: number;
  grantPublicKey: string;
  agentEnv: Record<string, string>;
  // ADR062 model proxy (mandatory): the provider base URL points at the
  // gateway (per-agent env name + composed URL), and the credential env holds
  // only the placeholder the gateway swaps for the real key upstream.
  modelBaseUrl: string;
  modelBaseUrlEnvName: string;
  // The workspace's agent network tier (BEX_AGENT_NETWORK_TIER: registries,
  // custom or full), only for the overlay's wording. "" when absent or
  // unknown (an older bex-api); the network itself is enforced outside.
  networkTier: string;
  // Session secrets (ADR098): name → value of each env var bex-api named in
  // BEX_AGENT_SECRET_NAMES. Used only to scrub published parts; the agent
  // reads the values from its own inherited env.
  secretValues: Map<string, string>;
}

// modelProxyPlaceholder mirrors agentsession.ModelKeyPlaceholder (Go). The
// placeholder is useless off-pod: the proxy authenticates the source Pod.
export function modelProxyPlaceholder(sessionID: string): string {
  return `bex-model-proxy-placeholder-${sessionID}`;
}

// outputContract reads the turn's structured-output env. The schema is checked
// here so a malformed one fails the turn at start rather than at the first
// submit_result; a turn without a schema or a result URL gets neither field
// (the agent cannot submit without the URL, so the tool is not offered).
function outputContract(env: NodeJS.ProcessEnv): {
  resultUrl: string;
  outputSchema: string;
  outputRequired: boolean;
} {
  const schema = env.BEX_AGENT_OUTPUT_SCHEMA || "";
  const url = env.BEX_AGENT_RESULT_URL || "";
  if (!schema || !url) {
    return { resultUrl: "", outputSchema: "", outputRequired: false };
  }
  if (Buffer.byteLength(schema, "utf8") > 64 * 1024) {
    throw new Error("BEX_AGENT_OUTPUT_SCHEMA exceeds 64 KiB");
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(schema);
  } catch (error) {
    throw new Error(
      `BEX_AGENT_OUTPUT_SCHEMA must be a JSON object: ${describeError(error)}`,
    );
  }
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("BEX_AGENT_OUTPUT_SCHEMA must be a JSON object");
  }
  return {
    resultUrl: url,
    outputSchema: schema,
    outputRequired: env.BEX_AGENT_OUTPUT_REQUIRED !== "0",
  };
}

const envNamePattern = /^[A-Z_][A-Z0-9_]*$/;

function jsonArray(value: string | undefined, name: string): string[] {
  if (!value) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch (error) {
    throw new Error(`${name} must be a JSON array: ${describeError(error)}`);
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
    throw new Error(`${name} must be a JSON object: ${describeError(error)}`);
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
  // bex-api always injects BEX_AGENT_TURN_TIMEOUT_MS. This fallback is the
  // safety net for a directly-launched driver: 30m — a sane bound, not the
  // former 4h, so a hung turn converges in tens of minutes rather than hours.
  const milliseconds = Number(value || 30 * 60 * 1000);
  if (!Number.isInteger(milliseconds) || milliseconds < 1) {
    throw new Error("BEX_AGENT_TURN_TIMEOUT_MS must be a positive integer");
  }
  return milliseconds;
}

function optionalNonNegativeInt(
  value: string | undefined,
  name: string,
): number {
  if (!value) return 0;
  const n = Number(value);
  if (!Number.isInteger(n) || n < 0) {
    throw new Error(`${name} must be a non-negative integer`);
  }
  return n;
}

const networkTiers = new Set(["registries", "custom", "full"]);

function networkTier(value: string | undefined): string {
  return value && networkTiers.has(value) ? value : "";
}

// model proxy routing is release-locked in agent-profiles.json (w5/m77).

export function loadConfig(
  env: NodeJS.ProcessEnv = process.env,
): AgentDriverConfig {
  const cwd = path.resolve(env.BEX_AGENT_CWD || "/workspace");
  const sessionLogPath =
    env.BEX_AGENT_SESSION_LOG || "/var/log/bex-agent/session.jsonl";
  const statusPath =
    env.BEX_AGENT_STATUS_FILE || "/var/run/bex-agent/status.json";
  const modelCredential = env.BEX_AGENT_MODEL_API_KEY || "";

  const manifest = loadAgentProfiles();
  const profile = env.BEX_AGENT_PROFILE
    ? lookupAgentProfile(env.BEX_AGENT_PROFILE, manifest)
    : env.BEX_AGENT_COMMAND
      ? manifest.profiles.find(
          (item) => item.executable === env.BEX_AGENT_COMMAND,
        )
      : lookupAgentProfile("claude", manifest);
  if (
    !profile ||
    (env.BEX_AGENT_COMMAND && env.BEX_AGENT_COMMAND !== profile.executable)
  ) {
    throw new Error(
      "BEX_AGENT_COMMAND must be an installed agent adapter path",
    );
  }

  const command = profile.executable;

  // ADR062 model proxy (mandatory): route the selected adapter's provider SDK
  // at the gateway and land the placeholder in the var that adapter reads.
  // There is no direct-to-vendor mode: a missing proxy URL or anything other
  // than the exact session-bound placeholder refuses to start.
  const modelProxyUrl = env.BEX_AGENT_MODEL_PROXY_URL || "";
  if (!modelProxyUrl) {
    throw new Error(
      "BEX_AGENT_MODEL_PROXY_URL is required: the driver only runs behind the bex model proxy",
    );
  }
  const sessionID = env.BEX_AGENT_SESSION_ID || "";
  if (!sessionID) throw new Error("BEX_AGENT_SESSION_ID is required");
  if (modelCredential !== modelProxyPlaceholder(sessionID)) {
    throw new Error(
      "BEX_AGENT_MODEL_API_KEY must be the session's model-proxy placeholder",
    );
  }
  const route = profile.modelProxy;
  const modelBaseUrlEnvName = route.baseUrlEnv;
  const modelBaseUrl = modelProxyUrl.replace(/\/+$/, "") + route.baseUrlSuffix;
  const credentialEnvName = route.credentialEnv;
  if (!envNamePattern.test(credentialEnvName)) {
    throw new Error("model proxy credential environment name is invalid");
  }
  if (env.BEX_AGENT_DELIVER === "1" && !(env.BEX_AGENT_BRANCH || "")) {
    throw new Error("BEX_AGENT_DELIVER=1 requires BEX_AGENT_BRANCH");
  }
  const agentEnv = env.BEX_AGENT_ENV_JSON
    ? jsonObject(env.BEX_AGENT_ENV_JSON, "BEX_AGENT_ENV_JSON")
    : { ...profile.env };
  if (
    Object.hasOwn(agentEnv, "BEX_AGENT_MODEL_API_KEY") ||
    Object.hasOwn(agentEnv, credentialEnvName) ||
    Object.hasOwn(agentEnv, modelBaseUrlEnvName)
  ) {
    throw new Error(
      "model credentials must use BEX_AGENT_MODEL_API_KEY, not BEX_AGENT_ENV_JSON",
    );
  }

  const args = env.BEX_AGENT_ARGS
    ? jsonArray(env.BEX_AGENT_ARGS, "BEX_AGENT_ARGS")
    : [...profile.args];
  if (
    !isDeepStrictEqual(args, profile.args) ||
    !isDeepStrictEqual(agentEnv, profile.env)
  ) {
    throw new Error(
      "BEX_AGENT_ARGS and BEX_AGENT_ENV_JSON must match the selected release profile",
    );
  }
  return {
    command,
    args,
    profile: profile.id,
    cwd,
    prompt: env.BEX_AGENT_PROMPT || "",
    branch: env.BEX_AGENT_BRANCH || "",
    repoUrl: env.BEX_AGENT_REPO_URL || "",
    baseBranch: env.BEX_AGENT_BASE_BRANCH || "",
    repository: env.BEX_AGENT_REPOSITORY || "",
    deliver: env.BEX_AGENT_DELIVER === "1",
    gitName: env.BEX_AGENT_GIT_NAME || "bex agent",
    gitEmail: env.BEX_AGENT_GIT_EMAIL || "agent@bex.co",
    existingSessionId: env.BEX_AGENT_EXISTING_SESSION_ID || "",
    persistSession: env.BEX_AGENT_PERSIST_SESSION !== "0",
    contextJson: env.BEX_AGENT_CONTEXT_JSON || "",
    originalTask: env.BEX_AGENT_ORIGINAL_TASK || "",
    listenHost: env.BEX_AGENT_LISTEN_HOST || "0.0.0.0",
    listenPort: positivePort(env.BEX_AGENT_LISTEN_PORT),
    sessionLogPath,
    statusPath,
    prRecordPath: path.join(path.dirname(statusPath), "pull-request.json"),
    shareUrl: env.BEX_AGENT_SHARE_URL || "",
    ...outputContract(env),
    resultRecordPath: path.join(path.dirname(statusPath), "result.json"),
    existingPrUrl: env.BEX_AGENT_PR_URL || "",
    existingPrNumber:
      optionalNonNegativeInt(env.BEX_AGENT_PR_NUMBER, "BEX_AGENT_PR_NUMBER") ||
      null,
    exitAfterTurn: env.BEX_AGENT_EXIT_AFTER_TURN === "1",
    turnTimeoutMs: positiveMilliseconds(env.BEX_AGENT_TURN_TIMEOUT_MS),
    credentialEnvName,
    modelCredential,
    sessionID,
    turn: Math.max(1, Number.parseInt(env.BEX_AGENT_TURN || "1", 10) || 1),
    grantPublicKey: env.BEX_AGENT_GRANT_PUBLIC_KEY || "",
    agentEnv,
    modelBaseUrl,
    modelBaseUrlEnvName,
    networkTier: networkTier(env.BEX_AGENT_NETWORK_TIER),
    attachments: parseAttachmentManifest(env.BEX_AGENT_ATTACHMENTS),
    skills: parseSkillManifest(env.BEX_AGENT_SKILLS),
    secretValues: secretValuesFromEnv(env),
  };
}
