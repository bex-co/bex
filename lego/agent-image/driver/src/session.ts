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
  appendFile,
  mkdir,
  readFile,
  rename,
  stat,
  writeFile,
} from "node:fs/promises";
import path from "node:path";
import { createUIMessageStream } from "ai";
import { createSessionProvider, type SessionProvider } from "./acp.js";
import { createUpdateMapper } from "./acp-map.js";
import {
  deliverBranch,
  extractEvidence,
  type DeliveryResult,
  type EvidenceResult,
} from "./delivery.js";
import { describeError } from "./errors.js";
import type { AgentDriverConfig } from "./config.js";
import type { CredentialManager } from "./credentials.js";
import type { UIMessageStreamHub, UIMessagePart } from "./stream-hub.js";

interface RunTurnOptions {
  prompt?: string;
  turn?: number;
  closeHub?: boolean;
  onPart?: (part: UIMessagePart) => void;
  abortSignal?: AbortSignal;
}

interface StatusRecord {
  state: "running" | "succeeded" | "failed";
  [key: string]: unknown;
}

const maxSessionLogBytes = 16 << 20;

export interface TurnResult extends StatusRecord {
  state: "succeeded";
  sessionId: string | null;
  resume: SessionProvider["resume"];
  usage: unknown;
}

async function ensureParent(filename: string): Promise<void> {
  await mkdir(path.dirname(filename), { recursive: true });
}

async function writeStatus(
  filename: string,
  status: StatusRecord,
): Promise<void> {
  await ensureParent(filename);
  const temporary = `${filename}.tmp`;
  await writeFile(temporary, `${JSON.stringify(status)}\n`, { mode: 0o600 });
  await rename(temporary, filename);
}

function cleanupFailure(original: unknown, cleanup: unknown): Error {
  return new Error(
    `${describeError(original)}; persisted credential cleanup failed: ${describeError(cleanup)}`,
  );
}

async function writeFailedStatusAndForget(
  config: AgentDriverConfig,
  credentialManager: CredentialManager,
  error: unknown,
): Promise<void> {
  try {
    await writeStatus(config.statusPath, {
      state: "failed",
      error: credentialManager.redact(describeError(error)),
    });
  } finally {
    credentialManager.forget();
  }
}

// markTurnFailed handles a fatal failure that happened BEFORE runHeadlessTurn
// owns the turn (for example the setup clone). It gets exactly one persisted
// scrub verdict, records that verdict in the status file, and always drops the
// in-memory credential. Turn failures use the same sequence below without
// returning through main.ts for a second cleanup attempt.
export async function markTurnFailed(
  config: AgentDriverConfig,
  credentialManager: CredentialManager,
  error: unknown,
): Promise<void> {
  let terminalError = error;
  try {
    await credentialManager.scrubPersistedState();
  } catch (cleanupError) {
    terminalError = cleanupFailure(error, cleanupError);
  }
  await writeFailedStatusAndForget(config, credentialManager, terminalError);
}

async function logPart(
  filename: string,
  part: UIMessagePart,
  credentialManager: CredentialManager,
  remaining: number,
  turn: number,
  partIndex: number,
): Promise<number> {
  await ensureParent(filename);
  const record = JSON.stringify({
    at: new Date().toISOString(),
    type: "ui-message",
    turn,
    partIndex,
    part,
  });
  const line = `${credentialManager.redact(record)}\n`;
  const bytes = Buffer.byteLength(line);
  if (bytes > remaining) return 0;
  await appendFile(filename, line, {
    mode: 0o600,
  });
  return bytes;
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
  const turn = options.turn ?? config.turn;
  let partIndex = 0;
  let logBytes = 0;
  let logTruncated = false;
  try {
    logBytes = (await stat(config.sessionLogPath)).size;
  } catch {
    // A new session has no log yet.
  }
  if (!prompt)
    throw new Error("BEX_AGENT_PROMPT is required for a headless turn");
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
  let providerStopped = false;
  let scrubPromise: Promise<string[]> | undefined;
  const stopProvider = async (): Promise<void> => {
    if (providerStopped || !provider) return;
    providerStopped = true;
    let stopError: unknown;
    try {
      await provider.cancel();
    } catch (error) {
      stopError = error;
    }
    try {
      await provider.cleanup();
    } catch (error) {
      if (stopError === undefined) stopError = error;
    }
    if (stopError !== undefined) throw stopError;
  };
  const scrubOnce = async (): Promise<string[]> => {
    scrubPromise ??= credentialManager.scrubPersistedState().catch((error) => {
      throw new Error(
        `persisted credential cleanup failed: ${describeError(error)}`,
      );
    });
    return scrubPromise;
  };
  let promptResponse:
    Awaited<ReturnType<SessionProvider["prompt"]>> | undefined;
  const turnAbort = new AbortController();
  let rejectDeadline!: (error: Error) => void;
  const deadline = new Promise<never>((_, reject) => {
    rejectDeadline = reject;
  });
  const abortFromOutside = () => {
    const error = new Error("agent turn terminated for snapshot");
    turnAbort.abort(error);
    rejectDeadline(error);
  };
  options.abortSignal?.addEventListener("abort", abortFromOutside, {
    once: true,
  });
  if (options.abortSignal?.aborted) abortFromOutside();
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
        onError: (error) => credentialManager.redact(describeError(error)),
      });
      for await (const chunk of stream) {
        const sanitized = publish(chunk as UIMessagePart);
        const written = await logPart(
          config.sessionLogPath,
          sanitized,
          credentialManager,
          Math.max(0, maxSessionLogBytes - logBytes),
          turn,
          partIndex,
        );
        partIndex += 1;
        if (written === 0) logTruncated = true;
        logBytes += written;
      }
      if (turnError) throw turnError;
    };
    await Promise.race([execute(), deadline]);
    if (closeHub) hub.close();

    // The agent must be dead before credential scrubbing, Git scanning/push, or
    // snapshot preparation. Otherwise same-sandbox code can keep rewriting
    // files/refs after inspection or re-persist a forgotten credential.
    await stopProvider();
    if (turnAbort.signal.aborted) throw turnAbort.signal.reason;

    // Scrub the model credential out of persisted state BEFORE delivery, so a
    // credential the agent wrote into a workspace file is redacted in the pushed
    // commit instead of published to the connected repository (round-5 finding
    // 6). deliverBranch additionally fails the push closed if the credential is
    // already in the branch's commit history (a byte-scrub cannot reach a
    // compressed git object). Evidence comes from the already-redacted session
    // log, so scrub order does not affect it. A delivery (push) failure — including
    // the fail-closed refusal — throws here and is recorded as a failed turn.
    const scrubbed = await scrubOnce();
    const delivery: DeliveryResult | null = config.deliver
      ? await deliverBranch({
          ...config,
          containsSecret: (text) => credentialManager.containsSecret(text),
          secretNeedles: () => credentialManager.secretNeedles(),
        })
      : null;
    const evidence: EvidenceResult = await extractEvidence(config);
    if (logTruncated) evidence.truncated = true;
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
    let terminalError = error;
    try {
      await stopProvider();
    } catch (cleanupError) {
      terminalError = cleanupFailure(terminalError, cleanupError);
    }
    try {
      await scrubOnce();
    } catch (cleanupError) {
      // A memoized scrub rejection is the original turn error when the success
      // path first discovered it. Do not append the same verdict to itself.
      if (cleanupError !== error) {
        terminalError = cleanupFailure(terminalError, cleanupError);
      }
    }
    await writeFailedStatusAndForget(config, credentialManager, terminalError);
    throw terminalError;
  } finally {
    clearTimeout(turnTimer);
    options.abortSignal?.removeEventListener("abort", abortFromOutside);
    // Idempotent safety net for errors thrown before the success/catch path
    // could stop the ACP child. Never replace the persisted failure verdict.
    try {
      await stopProvider();
    } catch {
      // The catch path already recorded a bounded failure when one was visible.
    }
  }
}
