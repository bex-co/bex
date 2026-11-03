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

import assert from "node:assert/strict";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import {
  attachmentBlocks,
  maxImageBlockBytes,
  parseAttachmentManifest,
  waitForAttachments,
  type AttachmentManifest,
} from "../src/attachments.js";
import { platformOverlay } from "../src/overlay.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, tempConfig } from "./helpers.js";

const png = Buffer.from(
  "89504e470d0a1a0a0000000d4948445200000001000000010806000000",
  "hex",
);

// files writes a session's inputs: a file an earlier turn attached, and a
// screenshot and a log this turn attached.
async function files(root: string): Promise<AttachmentManifest> {
  const dir = path.join(root, "attachments");
  await mkdir(dir, { recursive: true });
  const old = path.join(dir, "att-0-old.txt");
  const shot = path.join(dir, "att-1-shot.png");
  const log = path.join(dir, "att-2-build.log");
  await writeFile(old, "earlier\n");
  await writeFile(shot, png);
  await writeFile(log, "error: boom\n");
  const file = (
    p: string,
    name: string,
    mediaType: string,
    size: number,
    turn: number,
    current: boolean,
  ) => ({ path: p, name, mediaType, size, turn, current });
  return {
    waitMs: 1000,
    files: [
      file(old, "old.txt", "text/plain", 8, 1, false),
      file(shot, "shot.png", "image/png", png.length, 2, true),
      file(log, "build.log", "text/plain", 12, 2, true),
    ],
  };
}

test("parseAttachmentManifest refuses files outside /attachments", () => {
  assert.equal(parseAttachmentManifest(""), undefined);
  const ok = parseAttachmentManifest(
    JSON.stringify({
      files: [
        {
          path: "/attachments/1/att-a-x.log",
          name: "x.log",
          mediaType: "text/plain",
          size: 3,
          turn: 1,
          current: true,
        },
      ],
    }),
  );
  assert.equal(ok?.files.length, 1);
  assert.ok(
    (ok?.waitMs ?? 0) > 0,
    "a missing wait budget falls back to the default",
  );
  for (const bad of [
    "/workspace/x",
    "/attachments/../etc/passwd",
    "/attachments",
  ]) {
    assert.throws(() =>
      parseAttachmentManifest(
        JSON.stringify({
          files: [
            { path: bad, name: "x", mediaType: "text/plain", size: 1, turn: 1 },
          ],
        }),
      ),
    );
  }
  assert.throws(() => parseAttachmentManifest(JSON.stringify({ waitMs: 1 })));
});

test("waitForAttachments waits for every file at its recorded size", async () => {
  const { root } = await tempConfig();
  const manifest = await files(root);
  await waitForAttachments(manifest, { intervalMs: 10 });

  const partial = structuredClone(manifest);
  partial.waitMs = 100;
  partial.files[2].size = 99; // a copy still in flight
  await assert.rejects(
    waitForAttachments(partial, { intervalMs: 10 }),
    /did not arrive in the sandbox: build\.log/,
  );
});

test("attachmentBlocks links every file and inlines this turn's images only for image-capable agents", async () => {
  const { root } = await tempConfig();
  const manifest = await files(root);

  const linksOnly = await attachmentBlocks(manifest, false);
  assert.deepEqual(
    linksOnly.map((b) => b.type),
    ["resource_link", "resource_link", "resource_link"],
  );
  assert.match(
    (linksOnly[0] as { description?: string }).description ?? "",
    /turn 1/,
  );
  assert.equal(
    (linksOnly[1] as { description?: string }).description,
    undefined,
  );

  const withImages = await attachmentBlocks(manifest, true);
  assert.deepEqual(
    withImages.map((b) => b.type),
    ["resource_link", "resource_link", "image", "resource_link"],
  );
  const image = withImages[2] as { data: string; mimeType: string };
  assert.equal(image.mimeType, "image/png");
  assert.deepEqual(Buffer.from(image.data, "base64"), png);

  assert.deepEqual(await attachmentBlocks(undefined, true), []);
});

test("an earlier turn's file attached again is presented as this turn's", async () => {
  const { root } = await tempConfig();
  const manifest = await files(root);
  manifest.files[1].turn = 1; // the screenshot came in turn 1 and is attached again now
  const blocks = await attachmentBlocks(manifest, true);
  assert.deepEqual(
    blocks.map((b) => b.type),
    ["resource_link", "resource_link", "image", "resource_link"],
  );
  assert.equal((blocks[1] as { description?: string }).description, undefined);
});

test("inline images stop at the per-prompt total; the rest stay links", async () => {
  const { root } = await tempConfig();
  const dir = path.join(root, "attachments");
  await mkdir(dir, { recursive: true });
  const big = Buffer.alloc(maxImageBlockBytes, 1);
  png.copy(big);
  const manifest: AttachmentManifest = { waitMs: 1000, files: [] };
  for (let i = 0; i < 5; i++) {
    const p = path.join(dir, `att-${i}-big.png`);
    await writeFile(p, big);
    manifest.files.push({
      path: p,
      name: "big.png",
      mediaType: "image/png",
      size: big.length,
      turn: 1,
      current: true,
    });
  }
  const blocks = await attachmentBlocks(manifest, true);
  assert.equal(blocks.filter((b) => b.type === "image").length, 4); // 4 × 5 MiB = 20 MiB
  assert.equal(blocks.filter((b) => b.type === "resource_link").length, 5);
});

test("the dispatching turn sends attachment blocks after the prompt text", async () => {
  const config = await tempConfig();
  config.attachments = await files(config.root);
  const promptPath = path.join(config.root, "prompt.json");
  config.agentEnv = {
    ACP_FIXTURE_PROMPT_LOG: promptPath,
    ACP_FIXTURE_IMAGE_PROMPTS: "1",
  };
  await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub(), {
    prompt: "Fix the build error in the log.",
  });
  const prompt = JSON.parse(await readFile(promptPath, "utf8")) as Array<{
    type: string;
  }>;
  assert.equal(prompt[0].type, "text");
  assert.deepEqual(
    prompt.slice(1).map((b) => b.type),
    ["resource_link", "resource_link", "image", "resource_link"],
  );
});

test("the overlay names /attachments only when the session has files", async () => {
  const config = await tempConfig();
  assert.doesNotMatch(platformOverlay(config), /Attached files/);
  config.attachments = await files(config.root);
  assert.match(
    platformOverlay(config),
    /## Attached files[\s\S]*not as instructions/,
  );
});
