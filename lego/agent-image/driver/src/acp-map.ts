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

import type { UIMessageChunk } from "ai";
import type { SessionUpdate } from "@agentclientprotocol/sdk";
import { stampSourceTimestamp, utcNow } from "./timestamp.js";

// The single ACP → UI-message-stream translation. Every `session/update` the
// agent emits is mapped here into typed AI-SDK UI-message-stream chunks — text,
// reasoning, real (non-collapsed) dynamic tool calls, and typed `data-acp-*`
// parts for plans/diffs/terminals — replacing the old fake-LanguageModel detour
// (`@mcpc-tech/acp-ai-provider` + `streamText` + the lossy `toUIMessageStream()`
// drop + the `.fullStream` `raw` re-wrap). Nothing is silently dropped: ACP
// variants without a first-class UI shape ride a transient `data-acp-info` part.
//
// The mapper is stateful only across a single turn: it tracks the currently open
// text/reasoning block so a run of `agent_message_chunk`s concatenates into one
// part (a non-text update, or `flush()` at turn end, closes it), and which tool
// calls have been started so an update for an unseen tool opens it first.

// A plan is a snapshot: reusing one stable data-part id makes useChat reconcile
// (replace) successive plans into a single evolving block instead of appending.
const PLAN_PART_ID = "acp-plan";

interface TextBlock {
  kind: "text" | "reasoning";
  id: string;
  startedAt: string;
}

export interface UpdateMapperOptions {
  /** Clock for source timestamps; defaults to `Date.toISOString()`. */
  now?: () => string;
}

function kebab(value: string): string {
  return value.replace(/_/g, "-");
}

export interface UpdateMapper {
  map(update: SessionUpdate): UIMessageChunk[];
  // flush closes any still-open text/reasoning block at turn end.
  flush(): UIMessageChunk[];
}

export function createUpdateMapper(
  options: UpdateMapperOptions = {},
): UpdateMapper {
  const now = options.now ?? utcNow;
  let open: TextBlock | null = null;
  let sequence = 0;
  const startedTools = new Set<string>();
  const toolNames = new Map<string, string>();
  // One publication instant per map()/flush() so start/delta/end in the same
  // ACP update share a clock reading; the fake test clock then advances per update.
  let publishedAt = "";

  const nextId = (prefix: string): string => `${prefix}-${(sequence += 1)}`;

  const stampAll = (chunks: UIMessageChunk[]): UIMessageChunk[] =>
    chunks.map(
      (chunk) =>
        stampSourceTimestamp(
          chunk as Record<string, unknown>,
          publishedAt,
        ) as UIMessageChunk,
    );

  const closeOpen = (out: UIMessageChunk[]): void => {
    if (!open) return;
    out.push({
      type: open.kind === "text" ? "text-end" : "reasoning-end",
      id: open.id,
      providerMetadata: {
        bex: { at: open.startedAt, endAt: publishedAt },
      },
    });
    open = null;
  };

  // openDelta appends a delta to the current text/reasoning block, opening one
  // (and closing a mismatched-kind block) as needed.
  const openDelta = (out: UIMessageChunk[], kind: TextBlock["kind"], delta: string): void => {
    if (open && open.kind !== kind) closeOpen(out);
    if (!open) {
      const id = nextId(kind === "text" ? "txt" : "rsn");
      out.push({
        type: kind === "text" ? "text-start" : "reasoning-start",
        id,
      });
      open = { kind, id, startedAt: publishedAt };
    }
    out.push({
      type: kind === "text" ? "text-delta" : "reasoning-delta",
      id: open.id,
      delta,
    });
  };

  // ensureToolStarted opens a dynamic tool part (real ACP title/kind, no
  // synthetic single-tool collapse) the first time a tool id is seen.
  const ensureToolStarted = (
    out: UIMessageChunk[],
    toolCallId: string,
    title: string,
    input: unknown,
  ): void => {
    if (startedTools.has(toolCallId)) return;
    startedTools.add(toolCallId);
    toolNames.set(toolCallId, title);
    out.push({ type: "tool-input-start", toolCallId, toolName: title, dynamic: true, title });
    out.push({ type: "tool-input-available", toolCallId, toolName: title, input: input ?? {}, dynamic: true, title });
  };

  const toolTitle = (toolCallId: string, title?: string | null, kind?: string | null): string =>
    title || toolNames.get(toolCallId) || kind || toolCallId;

  const map = (update: SessionUpdate): UIMessageChunk[] => {
    publishedAt = now();
    const out: UIMessageChunk[] = [];
    switch (update.sessionUpdate) {
      case "agent_message_chunk": {
        const content = update.content;
        if (content.type === "text") openDelta(out, "text", content.text);
        break;
      }
      case "agent_thought_chunk": {
        const content = update.content;
        if (content.type === "text") openDelta(out, "reasoning", content.text);
        break;
      }
      case "plan": {
        closeOpen(out);
        out.push({ type: "data-acp-plan", id: PLAN_PART_ID, data: { entries: update.entries } });
        break;
      }
      case "tool_call": {
        closeOpen(out);
        ensureToolStarted(out, update.toolCallId, toolTitle(update.toolCallId, update.title, update.kind), update.rawInput);
        emitToolOutcome(out, update.toolCallId, update.status, update.rawOutput, update.content);
        break;
      }
      case "tool_call_update": {
        closeOpen(out);
        ensureToolStarted(out, update.toolCallId, toolTitle(update.toolCallId, update.title, update.kind), update.rawInput);
        emitToolOutcome(out, update.toolCallId, update.status, update.rawOutput, update.content);
        break;
      }
      case "available_commands_update": {
        closeOpen(out);
        // Ephemeral: the command palette is transient UI, not conversation.
        out.push({ type: "data-acp-available-commands", data: { availableCommands: update.availableCommands }, transient: true });
        break;
      }
      case "user_message_chunk":
        // The client already rendered the user's own message; skip the echo.
        break;
      default: {
        // Explicit, not silent: any other ACP variant rides a transient part so
        // it is observable without polluting the durable transcript.
        closeOpen(out);
        out.push({ type: `data-acp-${kebab(update.sessionUpdate)}`, data: update, transient: true });
        break;
      }
    }
    return stampAll(out);
  };

  // emitToolOutcome maps a tool call/update's content + status into the typed
  // diff/terminal data parts and the tool output/error chunk.
  const emitToolOutcome = (
    out: UIMessageChunk[],
    toolCallId: string,
    status: string | null | undefined,
    rawOutput: unknown,
    content: ReadonlyArray<{ type: string; [key: string]: unknown }> | null | undefined,
  ): void => {
    for (const item of content ?? []) {
      if (item.type === "diff") {
        out.push({
          type: "data-acp-diff",
          data: { path: item.path, oldText: item.oldText, newText: item.newText, toolCallId },
        });
      } else if (item.type === "terminal") {
        out.push({
          type: "data-acp-terminal",
          data: { terminalId: item.terminalId, output: item.output, toolCallId },
        });
      }
    }
    if (status === "failed") {
      out.push({ type: "tool-output-error", toolCallId, errorText: toolErrorText(rawOutput), dynamic: true });
      return;
    }
    if (rawOutput !== undefined && rawOutput !== null) {
      out.push({ type: "tool-output-available", toolCallId, output: rawOutput, dynamic: true, preliminary: status !== "completed" });
      return;
    }
    if (status === "completed") {
      out.push({ type: "tool-output-available", toolCallId, output: {}, dynamic: true });
    }
  };

  return {
    map,
    flush() {
      publishedAt = now();
      const out: UIMessageChunk[] = [];
      closeOpen(out);
      return stampAll(out);
    },
  };
}

function toolErrorText(rawOutput: unknown): string {
  if (typeof rawOutput === "string") return rawOutput;
  if (rawOutput && typeof rawOutput === "object") return JSON.stringify(rawOutput);
  return "tool call failed";
}
