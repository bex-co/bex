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

import { randomUUID } from "node:crypto";
import { mkdir, rename, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";

// writeJsonAtomic writes value as one JSON line (mode 0600) through a unique
// temporary file and a rename, so a reader never sees a partial document and
// concurrent writers never share a temporary file.
export async function writeJsonAtomic(
  file: string,
  value: unknown,
): Promise<void> {
  await mkdir(path.dirname(file), { recursive: true });
  const temporary = `${file}.${randomUUID()}.tmp`;
  await writeFile(temporary, `${JSON.stringify(value)}\n`, { mode: 0o600 });
  await rename(temporary, file);
}

// defaultManifestWaitMs is how long the driver waits for files bex-api copies
// in while the sandbox starts, when a manifest names no wait.
const defaultManifestWaitMs = 7 * 60_000;

// manifestWaitMs is a manifest's waitMs when it is a positive integer.
export function manifestWaitMs(raw: unknown): number {
  return Number.isInteger(raw) && (raw as number) > 0
    ? (raw as number)
    : defaultManifestWaitMs;
}

// waitForFiles resolves once every item's file is ready by ready(stat), and
// fails after waitMs naming the items still missing. bex-api's copies land
// whole (the gateway extracts into staging and moves the tree into place), so
// a ready file is a complete one.
export async function waitForFiles<T>(
  items: readonly T[],
  options: {
    waitMs: number;
    file: (item: T) => string;
    ready: (info: Awaited<ReturnType<typeof stat>>, item: T) => boolean;
    label: (item: T) => string;
    what: string;
    intervalMs?: number;
  },
): Promise<void> {
  const deadline = Date.now() + options.waitMs;
  const pending = new Set(items);
  while (pending.size > 0) {
    for (const item of [...pending]) {
      const info = await stat(options.file(item)).catch(() => undefined);
      if (info?.isFile() && options.ready(info, item)) pending.delete(item);
    }
    if (pending.size === 0) return;
    if (Date.now() >= deadline) {
      throw new Error(
        `${options.what} did not arrive in the sandbox: ${[...pending].map(options.label).join(", ")}`,
      );
    }
    await sleep(options.intervalMs ?? 250);
  }
}
