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

// Ending one turn from outside (ADR047 D12, "Stop turn"). There is no push
// channel from bex-api into the sandbox, so bex-api writes the number of the
// turn to end into a sentinel file beside the status file, through the gateway
// exec path it already reads the status file with. The driver polls for it,
// aborts that one turn, and reports it as interrupted rather than failed. The
// driver process, and any later turn, are untouched.

import { readFile, rm } from "node:fs/promises";
import path from "node:path";
import type { AgentDriverConfig } from "./config.js";

// TurnInterrupted is the abort reason of a turn a person stopped. It is
// distinct from every other reason, so the turn's verdict is not a failure.
export class TurnInterrupted extends Error {
  constructor() {
    super("agent turn interrupted");
    this.name = "TurnInterrupted";
  }
}

// interruptPath is the sentinel file bex-api writes
// (internal/sandbox agentSessionInterruptPath): next to the status file.
export function interruptPath(config: Pick<AgentDriverConfig, "statusPath">) {
  return path.join(path.dirname(config.statusPath), "interrupt");
}

export const interruptPollMs = 500;

// watchInterrupt polls file for the number of turn and calls onInterrupt once,
// after removing the file. A sentinel naming another turn, or not a plain
// number, is left alone: it was meant for a turn that is over or not yet
// started, and must never end this one. It returns the function that stops the
// watch; a stopped watch never calls onInterrupt, even when it is stopped
// while the sentinel it matched is being removed. removeFile is the removal,
// replaceable so a test can stop the watch in that window.
export function watchInterrupt(
  file: string,
  turn: number,
  onInterrupt: () => void,
  intervalMs = interruptPollMs,
  removeFile: (file: string) => Promise<void> = (f) => rm(f, { force: true }),
): () => void {
  let cancelled = false;
  let timer: NodeJS.Timeout | undefined;
  const poll = async (): Promise<void> => {
    const raw = await readFile(file, "utf8").catch(() => "");
    if (cancelled) return;
    if (/^\d+\s*$/.test(raw) && Number.parseInt(raw, 10) === turn) {
      await removeFile(file).catch(() => undefined);
      // The turn may have ended while the file was removed: the stop that
      // ended it must win, or the abort would land after the turn.
      if (!cancelled) onInterrupt();
      return;
    }
    timer = setTimeout(() => void poll(), intervalMs);
    timer.unref();
  };
  void poll();
  return () => {
    cancelled = true;
    clearTimeout(timer);
  };
}
