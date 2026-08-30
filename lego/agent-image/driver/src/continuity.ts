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

// Agent-context continuity (ADR047 D3 continuity amendment, w5/m84). A fresh
// agent process spawned for a session with prior turns is primed by exactly
// one ladder rung: ACP session/load (rung 1, decided by acp.ts via
// existingSessionId + loadSession) → a bounded transcript-derived preamble
// (rung 2, from BEX_AGENT_CONTEXT_JSON) → re-delivering the original task
// (rung 3, from BEX_AGENT_ORIGINAL_TASK, the setup-failure shape). bex-api
// selects the raw material; the driver owns rendering and the final byte
// budget (ADR051) — newest exchanges win when the budget trims.

import type { SessionProvider } from "./acp.js";

export type ContinuityRung =
  | "session-load"
  | "transcript-reprime"
  | "task-redelivery"
  | "none";

export interface ContinuityResult {
  rung: ContinuityRung;
  prompt: string;
}

interface ContextTurn {
  turn: number;
  prompt: string;
  reply?: string;
}

// preambleBudgetBytes bounds the rendered rung-2 preamble measured in UTF-8
// bytes. Small relative to any model context window; the durable transcript
// remains the full record — this is a working summary, not a replay.
export const preambleBudgetBytes = 24 << 10;

function utf8Length(text: string): number {
  return Buffer.byteLength(text, "utf8");
}

function parseContext(contextJson: string): ContextTurn[] {
  let parsed: unknown;
  try {
    parsed = JSON.parse(contextJson);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) return [];
  const turns: ContextTurn[] = [];
  for (const item of parsed) {
    if (item === null || typeof item !== "object") continue;
    const { turn, prompt, reply } = item as Record<string, unknown>;
    if (typeof turn !== "number" || typeof prompt !== "string") continue;
    turns.push({
      turn,
      prompt,
      reply: typeof reply === "string" ? reply : undefined,
    });
  }
  return turns;
}

function renderExchange(entry: ContextTurn): string {
  const lines = [`[turn ${entry.turn}] user:\n${entry.prompt.trim()}`];
  if (entry.reply?.trim()) {
    lines.push(`[turn ${entry.turn}] assistant:\n${entry.reply.trim()}`);
  }
  return lines.join("\n\n");
}

// wrapSessionContext frames priming material so the agent reads it as context
// rather than an instruction; both rungs share the exact tags so they cannot
// drift.
function wrapSessionContext(intro: string, trailer: string): {
  header: string;
  footer: string;
} {
  return {
    header: `<session-context>\n${intro}\n\n`,
    footer: `\n</session-context>\n\n${trailer}\n\n`,
  };
}

// renderPreamble renders the newest exchanges that fit the byte budget, in
// chronological order, wrapped so the agent reads it as context rather than an
// instruction. Empty input (or nothing fitting) renders "".
export function renderPreamble(
  contextJson: string,
  budgetBytes = preambleBudgetBytes,
): string {
  const turns = parseContext(contextJson);
  if (turns.length === 0) return "";
  const { header, footer } = wrapSessionContext(
    "You are continuing an existing session in a fresh process. " +
      "Earlier conversation (possibly truncated; oldest first):",
    "Continue from that context. The user's next message follows.",
  );
  const overhead = utf8Length(header) + utf8Length(footer);
  const kept: string[] = [];
  let used = 0;
  // Newest-first selection so the budget always keeps the latest exchanges.
  for (let i = turns.length - 1; i >= 0; i--) {
    const rendered = renderExchange(turns[i]);
    const cost = utf8Length(rendered) + 2; // joining blank line
    if (used + cost + overhead > budgetBytes) break;
    kept.unshift(rendered);
    used += cost;
  }
  if (kept.length === 0) return "";
  return header + kept.join("\n\n") + footer;
}

// resolveContinuity picks the rung for this turn and composes the effective
// ACP prompt. Rung 1 wins whenever session/load actually succeeded — the agent
// already remembers, so injecting a preamble would only duplicate history.
export function resolveContinuity(
  // Partial on purpose: hand-built configs (tests, standalone launches) may
  // predate the continuity fields.
  config: { contextJson?: string; originalTask?: string },
  resume: SessionProvider["resume"],
  userPrompt: string,
): ContinuityResult {
  if (resume === "loaded") {
    return { rung: "session-load", prompt: userPrompt };
  }
  const preamble = renderPreamble(config.contextJson ?? "");
  if (preamble) {
    return { rung: "transcript-reprime", prompt: preamble + userPrompt };
  }
  const task = (config.originalTask ?? "").trim();
  if (task && task !== userPrompt.trim()) {
    const { header, footer } = wrapSessionContext(
      `This session's original task was:\n${task}\n` +
        "No prior progress was made (earlier attempts failed during setup).",
      "Interpret the user's next message in the context of that task.",
    );
    return {
      rung: "task-redelivery",
      prompt: header.trimEnd() + footer + userPrompt,
    };
  }
  return { rung: "none", prompt: userPrompt };
}
