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

import { appendFile, mkdir, rename, writeFile } from "node:fs/promises";
import path from "node:path";
import { streamText } from "ai";
import { createSessionProvider } from "./acp.mjs";

async function ensureParent(filename) {
  await mkdir(path.dirname(filename), { recursive: true });
}

async function writeStatus(filename, status) {
  await ensureParent(filename);
  const temporary = `${filename}.tmp`;
  await writeFile(temporary, `${JSON.stringify(status)}\n`, { mode: 0o600 });
  await rename(temporary, filename);
}

async function logPart(filename, part, credentialManager) {
  await ensureParent(filename);
  const record = JSON.stringify({
    at: new Date().toISOString(),
    type: "ui-message",
    part,
  });
  await appendFile(
    filename,
    `${credentialManager.redact(record)}\n`,
    { mode: 0o600 },
  );
}

function rawUIMessagePart(part) {
  let data = part.rawValue;
  if (typeof data === "string") {
    try {
      data = JSON.parse(data);
    } catch {
      // A provider is allowed to supply an opaque raw string.
    }
  }
  // AI SDK 6 intentionally drops LanguageModelV3 `raw` parts while converting
  // to UIMessage chunks. Preserve them as the standard extensible data-part
  // form so useChat consumers receive plans/diffs/terminals without a fork.
  return { type: "data-acp", data };
}

export async function runHeadlessTurn(config, credentialManager, hub) {
  if (!config.prompt) throw new Error("BEX_AGENT_PROMPT is required for a headless turn");
  await writeStatus(config.statusPath, { state: "running" });

  let provider;
  const turnAbort = new AbortController();
  let rejectDeadline;
  const deadline = new Promise((_, reject) => {
    rejectDeadline = reject;
  });
  const turnTimer = setTimeout(() => {
    const error = new Error(`ACP turn exceeded ${config.turnTimeoutMs}ms`);
    turnAbort.abort(error);
    rejectDeadline(error);
  }, config.turnTimeoutMs);
  try {
    let created;
    let result;
    const execute = async () => {
      created = await createSessionProvider(
        config,
        credentialManager.agentEnvironment(),
      );
      provider = created.provider;
      result = streamText({
        model: provider.languageModel(),
        prompt: config.prompt,
        abortSignal: turnAbort.signal,
        includeRawChunks: true,
      });

      const consumeUI = async () => {
        for await (const part of result.toUIMessageStream()) {
          hub.publish(part);
          await logPart(config.sessionLogPath, part, credentialManager);
        }
      };
      const consumeRaw = async () => {
        for await (const part of result.fullStream) {
          if (part.type !== "raw") continue;
          const uiPart = rawUIMessagePart(part);
          hub.publish(uiPart);
          await logPart(config.sessionLogPath, uiPart, credentialManager);
        }
      };
      await Promise.all([consumeUI(), consumeRaw()]);
    };
    await Promise.race([execute(), deadline]);
    hub.close();

    const scrubbed = await credentialManager.scrubPersistedState();
    const status = {
      state: "succeeded",
      sessionId: provider.getSessionId(),
      resume: created.resume,
      scrubbedFiles: scrubbed.length,
    };
    await writeStatus(config.statusPath, status);
    return { ...status, usage: await result.usage };
  } catch (error) {
    hub.close();
    await writeStatus(config.statusPath, {
      state: "failed",
      error: credentialManager.redact(
        error instanceof Error ? error.message : String(error),
      ),
    });
    throw error;
  } finally {
    clearTimeout(turnTimer);
    provider?.cleanup();
  }
}
