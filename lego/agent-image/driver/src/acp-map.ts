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
import type { SessionUpdate } from "@agentclientprotocol/sdk/unstable";
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

// `name` is the part's identity from the opening update; `title` and `input`
// follow refinements until the call publishes its output.
interface ToolState {
  name: string;
  title: string;
  input: unknown;
  finished: boolean;
}

type ToolContent =
  | ReadonlyArray<{ type: string; [key: string]: unknown }>
  | null
  | undefined;

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
  const tools = new Map<string, ToolState>();
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
  const openDelta = (
    out: UIMessageChunk[],
    kind: TextBlock["kind"],
    delta: string,
  ): void => {
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
  // synthetic single-tool collapse) the first time a tool id is seen. Adapters
  // open a call before its input streams in (`rawInput: {}`, a generic title)
  // and refine it later; a refinement re-publishes the input so the durable
  // part carries the real command and title rather than the placeholder.
  const ensureToolStarted = (
    out: UIMessageChunk[],
    toolCallId: string,
    rawTitle: string | null | undefined,
    kind: string | null | undefined,
    rawInput: unknown,
  ): void => {
    const input = hasInput(rawInput) ? rawInput : undefined;
    let tool = tools.get(toolCallId);
    if (!tool) {
      const title = rawTitle || kind || toolCallId;
      tool = { name: title, title, input: rawInput ?? {}, finished: false };
      tools.set(toolCallId, tool);
      out.push({
        type: "tool-input-start",
        toolCallId,
        toolName: title,
        dynamic: true,
        title,
      });
    } else {
      const title = rawTitle || tool.title;
      if (
        tool.finished ||
        (title === tool.title && (input === undefined || input === tool.input))
      )
        return;
      tool.title = title;
      if (input !== undefined) tool.input = input;
    }
    out.push({
      type: "tool-input-available",
      toolCallId,
      toolName: tool.name,
      input: tool.input,
      dynamic: true,
      title: tool.title,
    });
  };

  const map = (update: SessionUpdate): UIMessageChunk[] => {
    publishedAt = now();
    const out: UIMessageChunk[] = [];
    switch (update.sessionUpdate) {
      case "agent_message_chunk": {
        const content = update.content;
        if (content.type === "text") openDelta(out, "text", content.text);
        else {
          closeOpen(out);
          out.push({ type: "data-acp-info", data: update, transient: true });
        }
        break;
      }
      case "agent_thought_chunk": {
        const content = update.content;
        if (content.type === "text") openDelta(out, "reasoning", content.text);
        else {
          closeOpen(out);
          out.push({ type: "data-acp-info", data: update, transient: true });
        }
        break;
      }
      case "plan": {
        closeOpen(out);
        out.push({
          type: "data-acp-plan",
          id: PLAN_PART_ID,
          data: { entries: update.entries },
        });
        break;
      }
      case "tool_call":
      case "tool_call_update": {
        closeOpen(out);
        ensureToolStarted(
          out,
          update.toolCallId,
          update.title,
          update.kind,
          update.rawInput,
        );
        emitToolOutcome(
          out,
          update.toolCallId,
          update.status,
          update.rawOutput,
          update.content,
        );
        break;
      }
      case "available_commands_update": {
        closeOpen(out);
        // Ephemeral: the command palette is transient UI, not conversation.
        out.push({
          type: "data-acp-available-commands",
          data: { availableCommands: update.availableCommands },
          transient: true,
        });
        break;
      }
      case "user_message_chunk":
        // The client already rendered the user's own message; skip the echo.
        break;
      default: {
        // Explicit, not silent: any other ACP variant rides a transient part so
        // it is observable without polluting the durable transcript.
        closeOpen(out);
        out.push({
          type: `data-acp-${kebab(update.sessionUpdate)}`,
          data: update,
          transient: true,
        });
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
    content: ToolContent,
  ): void => {
    for (const item of content ?? []) {
      if (item.type === "diff") {
        out.push({
          type: "data-acp-diff",
          data: {
            path: item.path,
            oldText: item.oldText,
            newText: item.newText,
            toolCallId,
          },
        });
      } else if (item.type === "terminal") {
        out.push({
          type: "data-acp-terminal",
          data: {
            terminalId: item.terminalId,
            output: item.output,
            toolCallId,
          },
        });
      } else {
        out.push({
          type: "data-acp-info",
          data: { toolCallId, content: item },
          transient: true,
        });
      }
    }
    if (status === "completed" || status === "failed") {
      // claude-agent-acp (0.94) reports a finished call's result only as text
      // content, without `rawOutput`; that text is the output.
      rawOutput ??= contentText(content);
      const tool = tools.get(toolCallId);
      if (tool) tool.finished = true;
    }
    if (status === "failed") {
      out.push({
        type: "tool-output-error",
        toolCallId,
        errorText: toolErrorText(rawOutput),
        dynamic: true,
      });
      return;
    }
    if (rawOutput !== undefined && rawOutput !== null) {
      out.push({
        type: "tool-output-available",
        toolCallId,
        output: rawOutput,
        dynamic: true,
        preliminary: status !== "completed",
      });
      return;
    }
    if (status === "completed") {
      out.push({
        type: "tool-output-available",
        toolCallId,
        output: {},
        dynamic: true,
      });
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

function hasInput(rawInput: unknown): boolean {
  if (rawInput === undefined || rawInput === null) return false;
  if (typeof rawInput !== "object") return true;
  return Object.keys(rawInput).length > 0;
}

// contentText joins a tool update's text content blocks, or undefined if none.
function contentText(content: ToolContent): string | undefined {
  const texts: string[] = [];
  for (const item of content ?? []) {
    const inner = item.content as
      | { type?: unknown; text?: unknown }
      | undefined;
    if (
      item.type === "content" &&
      inner?.type === "text" &&
      typeof inner.text === "string"
    )
      texts.push(inner.text);
  }
  return texts.length ? texts.join("\n") : undefined;
}

function toolErrorText(rawOutput: unknown): string {
  if (typeof rawOutput === "string") return rawOutput;
  if (rawOutput && typeof rawOutput === "object")
    return JSON.stringify(rawOutput);
  return "tool call failed";
}
