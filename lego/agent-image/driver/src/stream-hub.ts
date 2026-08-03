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

function frame(value: unknown): string {
  return `data: ${JSON.stringify(value)}\n\n`;
}

export class UIMessageStreamHub {
  #history: UIMessagePart[] = [];
  #clients = new Set<ServerResponse>();
  #closed = false;

  publish(part: UIMessagePart): void {
    if (this.#closed) throw new Error("UI message stream is already closed");
    this.#history.push(part);
    const data = frame(part);
    for (const response of this.#clients) response.write(data);
  }

  attach(response: ServerResponse): () => void {
    for (const part of this.#history) response.write(frame(part));
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
    return [...this.#history];
  }
}
