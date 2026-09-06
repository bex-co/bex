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
import fs from "node:fs/promises";
import { syncBuiltinESMExports } from "node:module";
import { EventEmitter } from "node:events";
import { mkdtemp, readFile, rm, stat } from "node:fs/promises";
import type { ServerResponse } from "node:http";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import type { CredentialManager } from "../src/credentials.js";
import { encodeUIMessageFrame, UIMessageStreamHub } from "../src/stream-hub.js";
import { TurnLogSink } from "../src/turn-log.js";
import { representativeParts as parts } from "./fixtures/hotpath-parts.mjs";

const credentials = {
  redact: (value: string) => value.replaceAll("secret", "[REDACTED]"),
} as CredentialManager;
class Client extends EventEmitter {
  frames: string[] = [];
  ended = "";
  destroyed = false;
  draining = true;
  write(frame: string): boolean {
    this.frames.push(frame);
    return this.draining;
  }
  end(frame: string): void {
    this.ended = frame;
    this.emit("close");
  }
  destroy(): void {
    this.destroyed = true;
    this.emit("close");
  }
  response(): ServerResponse {
    return this as unknown as ServerResponse;
  }
}

test("representative hot path batches writes and reuses identical live/replay bytes", async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), "bex-hotpath-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const logPath = path.join(root, "session.jsonl");
  const sink = new TurnLogSink(logPath, 0, 16 << 20, 1, credentials);
  const hub = new UIMessageStreamHub({ maxHistoryParts: 2 });
  const clients = [new Client(), new Client()];
  for (const client of clients) hub.attach(client.response());
  let serializations = 0;
  await sink.open();
  for (const part of parts) {
    const counted = {
      ...part,
      toJSON() {
        serializations++;
        return part;
      },
    };
    assert.equal(hub.publish(counted), encodeUIMessageFrame(part));
    await sink.appendPart(part);
  }
  assert.equal(
    sink.writeCount,
    0,
    "small deltas are buffered until terminal flush",
  );
  const replay = [new Client(), new Client()];
  for (const client of replay) hub.attach(client.response());
  hub.close();
  await sink.close();
  const wire = parts.map((part) => `data: ${JSON.stringify(part)}\n\n`);
  for (const client of clients) assert.deepEqual(client.frames, wire);
  for (const client of replay) assert.deepEqual(client.frames, wire.slice(-2));
  for (const client of [...clients, ...replay])
    assert.equal(client.ended, "data: [DONE]\n\n");
  assert.equal(
    serializations,
    parts.length,
    "replay, eviction and clients must not reserialize",
  );
  assert.equal(sink.openCount, 1);
  assert.equal(sink.writeCount, 1);
  const records = (await readFile(logPath, "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  assert.deepEqual(
    records.map((row) => row.part),
    parts,
  );
  assert.deepEqual(
    records.map((row) => row.partIndex),
    parts.map((_, i) => i),
  );
  assert.ok(
    records.every(
      (row) =>
        row.turn === 1 &&
        row.type === "ui-message" &&
        Number.isFinite(Date.parse(row.at)),
    ),
  );
  assert.equal((await stat(logPath)).mode & 0o777, 0o600);
  t.diagnostic(
    JSON.stringify({
      parts: parts.length,
      fileOpens: sink.openCount,
      fileWrites: sink.writeCount,
      frameEncodes: serializations,
      historical: {
        revision: "1f1647b44^",
        fileOpens: 9,
        fileWrites: 9,
        frameEncodes: 20,
      },
    }),
  );
});

test("hub enforces byte eviction, oversized replacement and terminal replay", () => {
  const first = { type: "text-delta", delta: "first" },
    last = { type: "text-delta", delta: "last" };
  const frame = encodeUIMessageFrame(last);
  const hub = new UIMessageStreamHub({
    maxHistoryBytes: Buffer.byteLength(frame),
    maxPartBytes: 100,
  });
  hub.publish(first);
  hub.publish(last);
  hub.close();
  const replay = new Client();
  hub.attach(replay.response());
  assert.deepEqual(replay.frames, [frame]);
  assert.equal(replay.ended, "data: [DONE]\n\n");
  assert.throws(() => hub.publish(last), /closed/);
  const oversized = new UIMessageStreamHub({ maxPartBytes: 100 });
  assert.equal(
    oversized.publish({ type: "text-delta", delta: "x".repeat(1000) }),
    encodeUIMessageFrame({
      type: "data-truncated",
      reason: "part exceeds stream limit",
    }),
  );
});

test("slow live and replay clients detach without blocking healthy clients", () => {
  const hub = new UIMessageStreamHub(),
    slow = new Client(),
    fast = new Client();
  slow.draining = false;
  hub.attach(slow.response());
  hub.attach(fast.response());
  hub.publish({ type: "start" });
  hub.publish({ type: "finish" });
  assert.equal(slow.destroyed, true);
  assert.equal(slow.frames.length, 1);
  const replay = new Client();
  replay.draining = false;
  hub.attach(replay.response());
  assert.equal(replay.destroyed, true);
  assert.equal(
    replay.frames.length,
    1,
    "stop replay on the first backpressure signal",
  );
  hub.close();
  assert.equal(fast.frames.length, 2);
  assert.equal(fast.ended, "data: [DONE]\n\n");
  assert.equal(slow.ended, "");
});

test("turn log bounds buffers and preserves redaction, cap and skipped indexes", async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), "bex-log-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const filename = path.join(root, "session.jsonl"),
    sink = new TurnLogSink(filename, 0, 150_000, 3, credentials);
  await sink.open();
  for (let i = 0; i < 100; i++)
    await sink.appendPart({
      type: "text-delta",
      delta: "secret" + "x".repeat(1000),
    });
  assert.ok(
    sink.writeCount > 0,
    "bounded buffer flushes before terminal boundary",
  );
  assert.ok(sink.writeCount < 10, "small parts are batched");
  assert.equal(
    await sink.appendPart({ type: "text-delta", delta: "x".repeat(100_000) }),
    0,
  );
  await sink.appendPart({ type: "finish" });
  await sink.close();
  await sink.close();
  const body = await readFile(filename, "utf8");
  assert.doesNotMatch(body, /secret/);
  const records = body
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line));
  assert.equal(records.length, 101);
  assert.equal(records.at(-1).partIndex, 101);
  assert.ok(Buffer.byteLength(body) <= 150_000);
  await assert.rejects(sink.appendPart({ type: "finish" }), /not open/);
});

test("write and close failures reject flush and release the handle", async (t) => {
  for (const operation of ["writeFile", "close"] as const) {
    await t.test(operation, async (t) => {
      const root = await mkdtemp(path.join(tmpdir(), "bex-log-failure-"));
      t.after(() => rm(root, { recursive: true, force: true }));
      const filename = path.join(root, "session.jsonl");
      const originalOpen = fs.open;
      let closes = 0;
      t.mock.method(fs, "open", async (...args: Parameters<typeof fs.open>) => {
        const handle = await originalOpen(...args);
        const originalClose = handle.close.bind(handle);
        t.mock.method(handle, "close", async () => {
          closes++;
          await originalClose();
          if (operation === "close") throw new Error("injected close failure");
        });
        if (operation === "writeFile")
          t.mock.method(handle, "writeFile", async () => {
            throw new Error("injected write failure");
          });
        return handle;
      });
      syncBuiltinESMExports();
      t.after(() => {
        t.mock.restoreAll();
        syncBuiltinESMExports();
      });
      const sink = new TurnLogSink(filename, 0, 16 << 20, 1, credentials);
      await sink.open();
      await sink.appendPart({ type: "finish" });
      await assert.rejects(sink.close(), /injected/);
      await assert.rejects(sink.close(), /injected/);
      assert.equal(closes, 1);
    });
  }
});

test("a large accepted record bypasses buffering without a partial JSONL write", async (t) => {
  const root = await mkdtemp(path.join(tmpdir(), "bex-large-log-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const filename = path.join(root, "session.jsonl");
  const sink = new TurnLogSink(filename, 0, 16 << 20, 1, credentials);
  const part = { type: "text-delta", delta: "x".repeat(80 << 10) };
  await sink.open();
  assert.ok(await sink.appendPart(part));
  assert.equal(sink.writeCount, 1);
  await sink.close();
  assert.deepEqual(JSON.parse(await readFile(filename, "utf8")).part, part);
});
