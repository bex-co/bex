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

// Session attachments (ADR095). bex-api copies every input file of the session
// under /attachments while the sandbox starts and lists them in
// BEX_AGENT_ATTACHMENTS. The driver waits for the copy before the turn, then
// hands the agent a resource_link per file (every ACP agent must accept one)
// and, for the files this turn attached, images inline when the agent says it
// takes images.

import { readFile, stat } from "node:fs/promises";
import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";
import type { ContentBlock } from "@agentclientprotocol/sdk/unstable";

export const attachmentsRoot = "/attachments";

export interface AttachmentFile {
  path: string;
  name: string;
  mediaType: string;
  size: number;
  // turn first attached the file; current marks the files this dispatch's
  // turn attached, including ones it attached again.
  turn: number;
  current: boolean;
}

export interface AttachmentManifest {
  waitMs: number;
  files: AttachmentFile[];
}

const imageTypes = new Set([
  "image/png",
  "image/jpeg",
  "image/gif",
  "image/webp",
]);
export const maxImageBlockBytes = 5 << 20;
// maxInlineImageBytes bounds one prompt's inline images together; the rest
// still arrive as links.
export const maxInlineImageBytes = 20 << 20;
const defaultWaitMs = 7 * 60_000;

// parseAttachmentManifest validates BEX_AGENT_ATTACHMENTS. A manifest that
// names a file outside /attachments is refused rather than trusted.
export function parseAttachmentManifest(
  raw: string | undefined,
): AttachmentManifest | undefined {
  if (!raw) return undefined;
  const parsed = JSON.parse(raw) as AttachmentManifest;
  if (!Array.isArray(parsed?.files)) {
    throw new Error("BEX_AGENT_ATTACHMENTS is malformed");
  }
  for (const file of parsed.files) {
    const resolved = path.posix.normalize(String(file?.path));
    if (
      resolved !== file.path ||
      !resolved.startsWith(attachmentsRoot + "/") ||
      !Number.isInteger(file.size) ||
      file.size < 0 ||
      !Number.isInteger(file.turn)
    ) {
      throw new Error("BEX_AGENT_ATTACHMENTS names an invalid file");
    }
    file.current = file.current === true;
  }
  parsed.waitMs =
    Number.isInteger(parsed.waitMs) && parsed.waitMs > 0
      ? parsed.waitMs
      : defaultWaitMs;
  return parsed;
}

// waitForAttachments resolves once every listed file exists at its recorded
// size, so the agent can never start on a partly copied input.
export async function waitForAttachments(
  manifest: AttachmentManifest,
  options: { intervalMs?: number } = {},
): Promise<void> {
  const deadline = Date.now() + manifest.waitMs;
  const pending = new Set(manifest.files);
  while (pending.size > 0) {
    for (const file of [...pending]) {
      const info = await stat(file.path).catch(() => undefined);
      if (info?.isFile() && info.size === file.size) pending.delete(file);
    }
    if (pending.size === 0) return;
    if (Date.now() >= deadline) {
      throw new Error(
        `attached files did not arrive in the sandbox: ${[...pending].map((f) => f.name).join(", ")}`,
      );
    }
    await sleep(options.intervalMs ?? 250);
  }
}

// attachmentBlocks builds the ACP blocks that follow the prompt text: a link to
// every file, labelled when only an earlier turn attached it, plus this turn's
// images inline for an agent that takes images.
export async function attachmentBlocks(
  manifest: AttachmentManifest | undefined,
  imagePrompts: boolean,
): Promise<ContentBlock[]> {
  if (!manifest) return [];
  const blocks: ContentBlock[] = [];
  let inlineBytes = 0;
  for (const file of manifest.files) {
    blocks.push({
      type: "resource_link",
      uri: `file://${file.path}`,
      name: file.name,
      mimeType: file.mediaType,
      size: file.size,
      ...(file.current
        ? {}
        : { description: `Attached by the user in turn ${file.turn}.` }),
    });
    if (
      file.current &&
      imagePrompts &&
      imageTypes.has(file.mediaType) &&
      file.size <= maxImageBlockBytes &&
      inlineBytes + file.size <= maxInlineImageBytes
    ) {
      inlineBytes += file.size;
      const data = await readFile(file.path);
      blocks.push({
        type: "image",
        data: data.toString("base64"),
        mimeType: file.mediaType,
        uri: `file://${file.path}`,
      });
    }
  }
  return blocks;
}
