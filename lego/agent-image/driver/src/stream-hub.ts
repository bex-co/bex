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

import type { ServerResponse } from "node:http";

export type UIMessagePart = Record<string, unknown>;

interface HubLimits {
  maxHistoryBytes?: number;
  maxHistoryParts?: number;
  maxPartBytes?: number;
}

interface HistoryEntry {
  part: UIMessagePart;
  frame: string;
  frameBytes: number;
}

export function encodeUIMessageFrame(value: unknown): string {
  return `data: ${JSON.stringify(value)}\n\n`;
}

export class UIMessageStreamHub {
  #history: HistoryEntry[] = [];
  #historyBytes = 0;
  #clients = new Set<ServerResponse>();
  #closed = false;
  readonly #maxHistoryBytes: number;
  readonly #maxHistoryParts: number;
  readonly #maxPartBytes: number;

  constructor(limits: HubLimits = {}) {
    this.#maxHistoryBytes = limits.maxHistoryBytes ?? 4 << 20;
    this.#maxHistoryParts = limits.maxHistoryParts ?? 4096;
    this.#maxPartBytes = limits.maxPartBytes ?? 1 << 20;
  }

  publish(part: UIMessagePart): string {
    if (this.#closed) throw new Error("UI message stream is already closed");
    let frame = encodeUIMessageFrame(part);
    let frameBytes = Buffer.byteLength(frame);
    if (frameBytes > this.#maxPartBytes) {
      part = { type: "data-truncated", reason: "part exceeds stream limit" };
      frame = encodeUIMessageFrame(part);
      frameBytes = Buffer.byteLength(frame);
    }
    this.#history.push({ part, frame, frameBytes });
    this.#historyBytes += frameBytes;
    while (
      this.#history.length > this.#maxHistoryParts ||
      this.#historyBytes > this.#maxHistoryBytes
    ) {
      const dropped = this.#history.shift();
      if (dropped) this.#historyBytes -= dropped.frameBytes;
    }
    for (const response of this.#clients) {
      if (!response.write(frame)) {
        this.#clients.delete(response);
        response.destroy(new Error("UI message stream client is not draining"));
      }
    }
    return frame;
  }

  attach(response: ServerResponse): () => void {
    for (const entry of this.#history) {
      if (!response.write(entry.frame)) {
        response.destroy(new Error("UI message stream client is not draining"));
        return () => {};
      }
    }
    if (this.#closed) {
      response.end("data: [DONE]\n\n");
      return () => {};
    }
    this.#clients.add(response);
    const detach = () => this.#clients.delete(response);
    response.once("close", detach);
    return detach;
  }

  close(): void {
    if (this.#closed) return;
    this.#closed = true;
    for (const response of this.#clients) response.end("data: [DONE]\n\n");
    this.#clients.clear();
  }

  get history(): UIMessagePart[] {
    return this.#history.map((entry) => entry.part);
  }
}
