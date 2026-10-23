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

import { readFile, stat } from "node:fs/promises";
import { createUIMessageStream } from "ai";
import { createSessionProvider, type SessionProvider } from "./acp.js";
import { createUpdateMapper } from "./acp-map.js";
import { resolveContinuity, type ContinuityRung } from "./continuity.js";
import {
  deliveryNudge,
  readGitFacts,
  undeliveredFacts,
  type DeliveryStatus,
} from "./delivery-check.js";
import { stampSourceTimestamp } from "./timestamp.js";
import { describeError } from "./errors.js";
import { writeJsonAtomic } from "./fs-util.js";
import type { AgentDriverConfig } from "./config.js";
import { platformOverlay } from "./overlay.js";
import type { UIMessageStreamHub, UIMessagePart } from "./stream-hub.js";
import { TurnLogSink } from "./turn-log.js";

interface RunTurnOptions {
  prompt?: string;
  turn?: number;
  closeHub?: boolean;
  onPart?: (part: UIMessagePart, frame?: string) => void;
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
  git?: DeliveryStatus;
}

async function writeStatus(
  filename: string,
  status: StatusRecord,
): Promise<void> {
  await writeJsonAtomic(filename, status);
}

function cleanupFailure(original: unknown, cleanup: unknown): Error {
  return new Error(
    `${describeError(original)}; agent cleanup failed: ${describeError(cleanup)}`,
  );
}

// markTurnFailed records a fatal failure that happened BEFORE runHeadlessTurn
// owns the turn (for example the setup clone). Turn failures persist their own
// verdict below without returning through main.ts for a second write.
export async function markTurnFailed(
  config: AgentDriverConfig,
  error: unknown,
): Promise<void> {
  await writeStatus(config.statusPath, {
    state: "failed",
    error: describeError(error),
  });
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

// runHeadlessTurn runs one agent turn. options let a live turn (ADR047 D9)
// reuse the identical timeout/delivery-check machinery:
//   - prompt   overrides config.prompt (the steering prompt on a POST /turn)
//   - closeHub false keeps the UI-message stream open so the session accepts
//     further turns and later attachers (a fire-and-forget turn closes it so
//     GET /stream watchers receive [DONE])
//   - onPart mirrors each published part to an extra sink (the POST /turn
//     response) in addition to the hub's fan-out to attached GET clients
//
// The turn speaks ACP directly (acp.ts) and maps every session/update into typed
// UI-message chunks (acp-map.ts), written into one createUIMessageStream.
//
// The turn ends when the agent ends it (ADR090). For a repo session the driver
// then reads Git state; if work is left undelivered it sends exactly one
// follow-up prompt on the same ACP session, re-reads the facts and records
// them, with what is still undelivered, as `git` in the status file. The
// driver never commits or pushes.
export async function runHeadlessTurn(
  config: AgentDriverConfig,
  agentEnv: Record<string, string>,
  hub: UIMessageStreamHub,
  options: RunTurnOptions = {},
): Promise<TurnResult> {
  const prompt = options.prompt ?? config.prompt;
  const closeHub = options.closeHub ?? true;
  const onPart = options.onPart;
  const turn = options.turn ?? config.turn;
  let logBytes = 0;
  try {
    logBytes = (await stat(config.sessionLogPath)).size;
  } catch {
    // A new session has no log yet.
  }
  const turnLog = new TurnLogSink(
    config.sessionLogPath,
    logBytes,
    maxSessionLogBytes,
    turn,
  );
  // One publication choke point: the hub history, attached GET /stream
  // clients, the POST /turn mirror (the gateway's durable transcript tee) and
  // the turn log all carry the same timestamped part.
  const publish = (part: UIMessagePart): UIMessagePart => {
    const stamped = stampSourceTimestamp(
      part as Record<string, unknown>,
    ) as UIMessagePart;
    const frame = hub.publish(stamped);
    if (onPart) onPart(stamped, frame);
    return stamped;
  };
  // Read the prior turn's persisted identity BEFORE the running-state write
  // below replaces the status file.
  const resumedFrom = await adoptPersistedSession(config);

  let provider: SessionProvider | undefined;
  let providerStopped = false;
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
  let promptResponse:
    Awaited<ReturnType<SessionProvider["prompt"]>> | undefined;
  let delivery: DeliveryStatus | undefined;
  const turnAbort = new AbortController();
  let rejectDeadline!: (error: Error) => void;
  const deadline = new Promise<never>((_, reject) => {
    rejectDeadline = reject;
  });
  // Setup I/O may still be pending when an external abort or deadline arrives.
  void deadline.catch(() => undefined);
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
    if (!prompt)
      throw new Error("BEX_AGENT_PROMPT is required for a headless turn");
    await turnLog.open();
    await writeStatus(config.statusPath, { state: "running" });
    // createUIMessageStream's onError swallows an execute() throw into an error
    // chunk, so the consume loop below would otherwise resolve normally on an
    // agent crash. Capture the error and re-raise it after the stream drains.
    let turnError: unknown;
    let continuityRung: ContinuityRung = "none";
    const overlay = platformOverlay(config);
    const execute = async () => {
      const mapper = createUpdateMapper();
      const stream = createUIMessageStream({
        execute: async ({ writer }) => {
          try {
            writer.write({ type: "start" });
            provider = await createSessionProvider(config, agentEnv, {
              onUpdate: (update) => {
                for (const chunk of mapper.map(update)) writer.write(chunk);
              },
              abortSignal: turnAbort.signal,
              overlay,
            });
            // Continuity ladder (ADR047 D3, w5/m84): the rung depends on
            // whether session/load actually happened, known only now. The
            // annotation is a durable (non-transient) part so replay shows the
            // same restored-vs-rebuilt hint the live stream carried; the
            // preamble (and a preamble-channel overlay) rides only the ACP
            // prompt, never the stream, so history is never double-rendered.
            const continuity = resolveContinuity(
              config,
              provider.resume,
              prompt,
              provider.overlayChannel === "preamble" ? overlay : "",
            );
            continuityRung = continuity.rung;
            if (continuity.rung !== "none") {
              writer.write({
                type: "data-bex-continuity",
                data: { rung: continuity.rung },
              });
            }
            promptResponse = await provider.prompt(continuity.prompt);
            if (config.deliver) {
              // Delivery check before the provider stops, so the one nudge
              // continues the same ACP session with the agent's full context.
              let facts = await readGitFacts(config);
              let reasons = undeliveredFacts(facts, config);
              if (reasons.length > 0) {
                for (const chunk of mapper.flush()) writer.write(chunk);
                promptResponse = await provider.prompt(
                  deliveryNudge(reasons, config.branch),
                );
                facts = await readGitFacts(config);
                reasons = undeliveredFacts(facts, config);
              }
              delivery = { ...facts, undelivered: reasons };
            }
            for (const chunk of mapper.flush()) writer.write(chunk);
            writer.write({ type: "finish" });
          } catch (error) {
            turnError = error;
            throw error; // also surfaces an error chunk to attached clients
          }
        },
        onError: (error) => describeError(error),
      });
      for await (const chunk of stream) {
        const published = publish(chunk as UIMessagePart);
        await turnLog.appendPart(published);
      }
      if (turnError) throw turnError;
    };
    await Promise.race([execute(), deadline]);
    if (closeHub) hub.close();

    // The agent must be dead before the terminal verdict is written, so
    // same-sandbox code cannot keep changing the facts after they were read.
    await stopProvider();
    if (turnAbort.signal.aborted) throw turnAbort.signal.reason;
    await turnLog.close();

    const status: StatusRecord = {
      state: "succeeded",
      sessionId: provider!.sessionId,
      resume: provider!.resume,
      ...(continuityRung !== "none" ? { continuity: continuityRung } : {}),
      ...(resumedFrom ? { resumedFrom } : {}),
      ...(delivery ? { git: delivery } : {}),
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
      await turnLog.close();
    } catch (logError) {
      if (logError !== error) {
        terminalError = new Error(
          `${describeError(terminalError)}; turn log flush failed: ${describeError(logError)}`,
        );
      }
    }
    await writeStatus(config.statusPath, {
      state: "failed",
      error: describeError(terminalError),
    });
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
