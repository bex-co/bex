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

import { appendFile, mkdir, readFile, rename, writeFile } from "node:fs/promises";
import path from "node:path";
import { createUIMessageStream } from "ai";
import { createSessionProvider, type SessionProvider } from "./acp.js";
import { createUpdateMapper } from "./acp-map.js";
import { deliverBranch, extractEvidence, type DeliveryResult, type EvidenceResult } from "./delivery.js";
import type { AgentDriverConfig } from "./config.js";
import type { CredentialManager } from "./credentials.js";
import type { UIMessageStreamHub, UIMessagePart } from "./stream-hub.js";

interface RunTurnOptions {
  prompt?: string;
  closeHub?: boolean;
  onPart?: (part: UIMessagePart) => void;
}

interface StatusRecord {
  state: "running" | "succeeded" | "failed";
  [key: string]: unknown;
}

export interface TurnResult extends StatusRecord {
  state: "succeeded";
  sessionId: string | null;
  resume: SessionProvider["resume"];
  usage: unknown;
}

async function ensureParent(filename: string): Promise<void> {
  await mkdir(path.dirname(filename), { recursive: true });
}

async function writeStatus(filename: string, status: StatusRecord): Promise<void> {
  await ensureParent(filename);
  const temporary = `${filename}.tmp`;
  await writeFile(temporary, `${JSON.stringify(status)}\n`, { mode: 0o600 });
  await rename(temporary, filename);
}

// markTurnFailed records a fatal failure that happened OUTSIDE runHeadlessTurn
// (e.g. the setup-phase clone), so the fire-and-forget Completer reads a `failed`
// status file instead of an absent one and finalizes the session. runHeadlessTurn
// writes its own `failed` status on a turn error; overwriting it here is harmless.
export async function markTurnFailed(
  config: AgentDriverConfig,
  credentialManager: CredentialManager,
  error: unknown,
): Promise<void> {
  await writeStatus(config.statusPath, {
    state: "failed",
    error: credentialManager.redact(error instanceof Error ? error.message : String(error)),
  });
}

async function logPart(
  filename: string,
  part: UIMessagePart,
  credentialManager: CredentialManager,
): Promise<void> {
  await ensureParent(filename);
  const record = JSON.stringify({
    at: new Date().toISOString(),
    type: "ui-message",
    part,
  });
  await appendFile(filename, `${credentialManager.redact(record)}\n`, { mode: 0o600 });
}

// A resumed sandbox restarts the driver on the restored rootfs, where the
// previous turn's status file still carries the agent's ACP session id (the
// agent's own on-disk session state survives rootfs snapshots — ADR047 D3).
// Adopt that id as existingSessionId when the environment supplies none, so
// the provider can attempt session/load; acp.ts still gates the attempt on
// the agent advertising loadSession, and a missing, corrupt, or
// non-succeeded status file adopts nothing — the turn starts a fresh session.
// Returns the provenance of the session id: "env", "rootfs", or "".
export async function adoptPersistedSession(
  config: AgentDriverConfig,
): Promise<"env" | "rootfs" | ""> {
  if (config.existingSessionId) return "env";
  let persisted: { state?: string; sessionId?: string } | undefined;
  try {
    persisted = JSON.parse(await readFile(config.statusPath, "utf8"));
  } catch {
    return "";
  }
  if (
    persisted?.state === "succeeded" &&
    typeof persisted.sessionId === "string" &&
    persisted.sessionId !== ""
  ) {
    config.existingSessionId = persisted.sessionId;
    return "rootfs";
  }
  return "";
}

// runHeadlessTurn runs one agent turn. options let a live turn (ADR047 D9 t004)
// reuse the identical timeout/delivery/evidence/scrub machinery:
//   - prompt   overrides config.prompt (the steering prompt on a POST /turn)
//   - closeHub false keeps the UI-message stream open so the session accepts
//     further turns and later attachers (a fire-and-forget turn closes it so
//     GET /stream watchers receive [DONE])
//   - onPart mirrors each published part to an extra sink (the POST /turn
//     response) in addition to the hub's fan-out to attached GET clients
//
// The turn speaks ACP directly (acp.ts) and maps every session/update into typed
// UI-message chunks (acp-map.ts), written into one createUIMessageStream. There
// is no fake-LanguageModel provider, no streamText, and no lossy raw re-wrap:
// plans/diffs/terminals arrive as `data-acp-*` parts and tool calls as real
// dynamic tool parts.
export async function runHeadlessTurn(
  config: AgentDriverConfig,
  credentialManager: CredentialManager,
  hub: UIMessageStreamHub,
  options: RunTurnOptions = {},
): Promise<TurnResult> {
  const prompt = options.prompt ?? config.prompt;
  const closeHub = options.closeHub ?? true;
  const onPart = options.onPart;
  if (!prompt) throw new Error("BEX_AGENT_PROMPT is required for a headless turn");
  // Sanitize at the single publication choke point (codex r7 #4): the hub
  // history, attached GET /stream clients, the POST /turn mirror — the
  // gateway's byte-transparent durable transcript tee — and (via the return
  // value) the session log all carry the same sanitized part. logPart's own
  // string-level redaction stays as a second pass.
  const publish = (part: UIMessagePart): UIMessagePart => {
    const sanitized = credentialManager.redactPart(part);
    hub.publish(sanitized);
    if (onPart) onPart(sanitized);
    return sanitized;
  };
  // Read the prior turn's persisted identity BEFORE the running-state write
  // below replaces the status file.
  const resumedFrom = await adoptPersistedSession(config);
  await writeStatus(config.statusPath, { state: "running" });

  let provider: SessionProvider | undefined;
  let promptResponse: Awaited<ReturnType<SessionProvider["prompt"]>> | undefined;
  const turnAbort = new AbortController();
  let rejectDeadline!: (error: Error) => void;
  const deadline = new Promise<never>((_, reject) => {
    rejectDeadline = reject;
  });
  const turnTimer = setTimeout(() => {
    const error = new Error(`ACP turn exceeded ${config.turnTimeoutMs}ms`);
    turnAbort.abort(error);
    rejectDeadline(error);
  }, config.turnTimeoutMs);
  try {
    // createUIMessageStream's onError swallows an execute() throw into an error
    // chunk, so the consume loop below would otherwise resolve normally on an
    // agent crash. Capture the error and re-raise it after the stream drains.
    let turnError: unknown;
    const execute = async () => {
      const mapper = createUpdateMapper();
      const stream = createUIMessageStream({
        execute: async ({ writer }) => {
          try {
            writer.write({ type: "start" });
            provider = await createSessionProvider(
              config,
              credentialManager.agentEnvironment(),
              {
                onUpdate: (update) => {
                  for (const chunk of mapper.map(update)) writer.write(chunk);
                },
                abortSignal: turnAbort.signal,
              },
            );
            promptResponse = await provider.prompt(prompt);
            for (const chunk of mapper.flush()) writer.write(chunk);
            writer.write({ type: "finish" });
          } catch (error) {
            turnError = error;
            throw error; // also surfaces an error chunk to attached clients
          }
        },
        onError: (error) =>
          credentialManager.redact(error instanceof Error ? error.message : String(error)),
      });
      for await (const chunk of stream) {
        const sanitized = publish(chunk as UIMessagePart);
        await logPart(config.sessionLogPath, sanitized, credentialManager);
      }
      if (turnError) throw turnError;
    };
    await Promise.race([execute(), deadline]);
    if (closeHub) hub.close();

    // Scrub the model credential out of persisted state BEFORE delivery, so a
    // credential the agent wrote into a workspace file is redacted in the pushed
    // commit instead of published to the connected repository (round-5 finding
    // 6). deliverBranch additionally fails the push closed if the credential is
    // already in the branch's commit history (a byte-scrub cannot reach a
    // compressed git object). Evidence comes from the already-redacted session
    // log, so scrub order does not affect it. A delivery (push) failure — including
    // the fail-closed refusal — throws here and is recorded as a failed turn.
    const scrubbed = await credentialManager.scrubPersistedState();
    const delivery: DeliveryResult | null = config.deliver
      ? await deliverBranch({
          ...config,
          containsSecret: (text) => credentialManager.containsSecret(text),
          secretNeedles: () => credentialManager.secretNeedles(),
        })
      : null;
    const evidence: EvidenceResult = await extractEvidence(config);
    const status: StatusRecord = {
      state: "succeeded",
      sessionId: provider!.sessionId,
      resume: provider!.resume,
      ...(resumedFrom ? { resumedFrom } : {}),
      ...(delivery ? { delivery } : {}),
      evidence,
      scrubbedFiles: scrubbed.length,
    };
    await writeStatus(config.statusPath, status);
    return { ...status, usage: promptResponse?.usage ?? {} } as TurnResult;
  } catch (error) {
    if (closeHub) hub.close();
    await writeStatus(config.statusPath, {
      state: "failed",
      error: credentialManager.redact(
        error instanceof Error ? error.message : String(error),
      ),
    });
    throw error;
  } finally {
    clearTimeout(turnTimer);
    provider?.cancel();
    provider?.cleanup();
  }
}
