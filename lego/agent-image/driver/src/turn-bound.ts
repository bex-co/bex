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

import type { SessionUpdate } from "@agentclientprotocol/sdk/unstable";

export interface TurnBoundOptions {
  // idleMs: how long the turn may make no progress (BEX_AGENT_TURN_IDLE_TIMEOUT_MS).
  idleMs: number;
  // toolCallMs: how long one running tool call may pause the idle clock
  // (BEX_AGENT_TOOL_CALL_TIMEOUT_MS).
  toolCallMs: number;
  // ceilingMs: the hard wall-clock bound (BEX_AGENT_TURN_TIMEOUT_MS).
  ceilingMs: number;
  // onExpire runs once, with the error the turn fails with.
  onExpire: (error: Error) => void;
}

// TurnBound is one turn's limits. The turn fails when it stops making
// progress: no ACP session/update — message, thought, plan or tool chunks,
// which is how each model exchange reaches the driver — and no prompt sent or
// answered for idleMs. While a tool call runs the idle clock is paused, since
// a build or a test suite can be silent for longer, but one tool call may
// pause it for at most toolCallMs: the agent profiles set no tool timeout of
// their own, so a hung tool would otherwise hold the turn until the ceiling.
// The ceiling ends any turn after ceilingMs, progress or not.
//
// Progress only stamps a time; the idle timer re-arms itself for the
// remaining quiet time when it fires, so a streaming turn costs no timer
// churn per chunk.
export class TurnBound {
  readonly #idleMs: number;
  readonly #toolCallMs: number;
  readonly #onExpire: (error: Error) => void;
  readonly #ceilingTimer: ReturnType<typeof setTimeout>;
  // Running tool calls by id, with when each started.
  readonly #toolCalls = new Map<string, number>();
  #idleTimer: ReturnType<typeof setTimeout> | undefined;
  #toolTimer: ReturnType<typeof setTimeout> | undefined;
  #lastProgress: number;
  #stopped = false;

  constructor(options: TurnBoundOptions) {
    this.#idleMs = options.idleMs;
    this.#toolCallMs = options.toolCallMs;
    this.#onExpire = options.onExpire;
    this.#lastProgress = Date.now();
    this.#ceilingTimer = setTimeout(
      () => this.#expire(new Error(`ACP turn exceeded ${options.ceilingMs}ms`)),
      options.ceilingMs,
    );
    this.#armIdle(this.#idleMs);
  }

  // progress notes progress the driver itself makes: a prompt sent or
  // answered.
  progress(): void {
    this.#lastProgress = Date.now();
    if (this.#toolCalls.size === 0 && this.#idleTimer === undefined) {
      this.#armIdle(this.#idleMs);
    }
  }

  // update notes one ACP session/update: progress, and whether a tool call
  // started or ended. A tool call is running from its pending or in_progress
  // report until it reports completed or failed.
  update(update: SessionUpdate): void {
    if (
      update.sessionUpdate === "tool_call" ||
      update.sessionUpdate === "tool_call_update"
    ) {
      const status =
        update.status ??
        (update.sessionUpdate === "tool_call" ? "pending" : undefined);
      if (status === "pending" || status === "in_progress") {
        if (!this.#toolCalls.has(update.toolCallId)) {
          this.#toolCalls.set(update.toolCallId, Date.now());
          this.#pauseIdle();
          this.#armToolCall();
        }
      } else if (status === "completed" || status === "failed") {
        if (this.#toolCalls.delete(update.toolCallId)) this.#armToolCall();
      }
    }
    this.progress();
  }

  // promptSettled notes the agent ending a prompt: no tool call of it still
  // runs, whatever the agent last reported for one.
  promptSettled(): void {
    this.#toolCalls.clear();
    this.#armToolCall();
    this.progress();
  }

  stop(): void {
    this.#stopped = true;
    clearTimeout(this.#ceilingTimer);
    clearTimeout(this.#toolTimer);
    this.#pauseIdle();
  }

  #pauseIdle(): void {
    clearTimeout(this.#idleTimer);
    this.#idleTimer = undefined;
  }

  #armIdle(delayMs: number): void {
    if (this.#stopped) return;
    this.#idleTimer = setTimeout(() => {
      this.#idleTimer = undefined;
      if (this.#stopped || this.#toolCalls.size > 0) return;
      const quietMs = Date.now() - this.#lastProgress;
      if (quietMs >= this.#idleMs) {
        this.#expire(
          new Error(`ACP turn made no progress for ${this.#idleMs}ms`),
        );
        return;
      }
      this.#armIdle(this.#idleMs - quietMs);
    }, delayMs);
  }

  // armToolCall times the oldest running tool call against toolCallMs.
  #armToolCall(): void {
    clearTimeout(this.#toolTimer);
    this.#toolTimer = undefined;
    if (this.#stopped || this.#toolCalls.size === 0) return;
    const oldest = Math.min(...this.#toolCalls.values());
    this.#toolTimer = setTimeout(
      () =>
        this.#expire(
          new Error(`ACP tool call ran longer than ${this.#toolCallMs}ms`),
        ),
      Math.max(0, oldest + this.#toolCallMs - Date.now()),
    );
  }

  #expire(error: Error): void {
    if (this.#stopped) return;
    this.stop();
    this.#onExpire(error);
  }
}
