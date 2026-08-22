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

import { mkdir, open, type FileHandle } from "node:fs/promises";
import path from "node:path";
import type { CredentialManager } from "./credentials.js";
import type { UIMessagePart } from "./stream-hub.js";

async function ensureParent(filename: string): Promise<void> {
  await mkdir(path.dirname(filename), { recursive: true });
}

export class TurnLogSink {
  readonly #path: string;
  readonly #credentialManager: CredentialManager;
  readonly #turn: number;
  #handle: FileHandle | null = null;
  #remaining: number;
  #partIndex = 0;
  #openCount = 0;
  #writeCount = 0;

  constructor(
    path: string,
    initialBytes: number,
    maxBytes: number,
    turn: number,
    credentialManager: CredentialManager,
  ) {
    this.#path = path;
    this.#turn = turn;
    this.#credentialManager = credentialManager;
    this.#remaining = Math.max(0, maxBytes - initialBytes);
  }

  get openCount(): number {
    return this.#openCount;
  }

  get writeCount(): number {
    return this.#writeCount;
  }

  async open(): Promise<void> {
    if (this.#handle) return;
    await ensureParent(this.#path);
    this.#handle = await open(this.#path, "a", 0o600);
    this.#openCount += 1;
  }

  async appendPart(part: UIMessagePart): Promise<number> {
    if (!this.#handle || this.#remaining <= 0) return 0;
    const record = JSON.stringify({
      at: new Date().toISOString(),
      type: "ui-message",
      turn: this.#turn,
      partIndex: this.#partIndex,
      part,
    });
    const line = `${this.#credentialManager.redact(record)}\n`;
    const bytes = Buffer.byteLength(line);
    if (bytes > this.#remaining) return 0;
    await this.#handle.write(line);
    this.#writeCount += 1;
    this.#remaining -= bytes;
    this.#partIndex += 1;
    return bytes;
  }

  async close(): Promise<void> {
    if (!this.#handle) return;
    await this.#handle.close();
    this.#handle = null;
  }
}
