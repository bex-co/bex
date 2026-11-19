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
import { mkdir, mkdtemp, rename, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";
import test from "node:test";
import {
  descendantTicks,
  parseStat,
  readProcessTable,
  usedTicks,
  watchActivity,
  type ProcessStat,
} from "../src/activity.js";

// statLine is a /proc/<pid>/stat line with the fields the sampler reads.
function statLine(
  pid: number,
  comm: string,
  ppid: number,
  cpu: [number, number, number, number],
  started = 1000,
): string {
  const fields = [`${pid}`, `(${comm})`, "S", `${ppid}`, "1", "1", "0", "-1"];
  fields.push("4194304", "0", "0", "0", "0", ...cpu.map(String));
  fields.push("20", "0", "1", "0", `${started}`, "1000000", "100");
  return `${fields.join(" ")}\n`;
}

test("a stat line yields the parent, every CPU tick and the start time, whatever the command name", () => {
  assert.deepEqual(
    parseStat(statLine(7, "vitest (worker) 2", 3, [5, 6, 7, 8])),
    {
      ppid: 3,
      ticks: 26,
      started: "1000",
    },
  );
  assert.equal(parseStat(""), undefined);
  assert.equal(parseStat("7 (node) S"), undefined, "a truncated line");
});

test("only the root's descendants count, the root itself left out", () => {
  const ticks = descendantTicks(
    new Map<number, ProcessStat>([
      [10, { ppid: 1, ticks: 500, started: "1" }], // the driver
      [11, { ppid: 10, ticks: 40, started: "2" }], // the agent
      [12, { ppid: 11, ticks: 900, started: "3" }], // its test suite
      [13, { ppid: 1, ticks: 7000, started: "4" }], // the sandbox's own daemon
    ]),
    10,
  );
  assert.deepEqual(
    [...ticks],
    [
      ["11:2", 40],
      ["12:3", 900],
    ],
  );
});

test("a parent loop in the table cannot hang the walk", () => {
  const ticks = descendantTicks(
    new Map<number, ProcessStat>([
      [11, { ppid: 12, ticks: 1, started: "1" }],
      [12, { ppid: 11, ticks: 1, started: "1" }],
      [13, { ppid: 10, ticks: 1, started: "1" }],
    ]),
    10,
  );
  assert.deepEqual([...ticks.keys()], ["13:1"]);
});

test("used CPU counts a new process whole and a reused pid as a new process", () => {
  const before = new Map([
    ["11:2", 40],
    ["12:3", 900],
  ]);
  const after = new Map([
    ["11:2", 45], // 5 more
    ["12:9", 30], // pid 12 reused after the suite ended
    ["14:9", 2], // a new worker
  ]);
  assert.equal(usedTicks(before, after), 37);
  assert.equal(usedTicks(after, after), 0);
});

test("the watch reports a sample in which the tree used the CPU, and only then", async (t) => {
  const proc = await mkdtemp(path.join(tmpdir(), "bex-proc-"));
  t.after(() => rm(proc, { recursive: true, force: true }));
  // Written whole, as /proc reads: a half-written file would read as a
  // process gone and then new.
  const write = async (pid: number, line: string) => {
    await mkdir(path.join(proc, `${pid}`), { recursive: true });
    const file = path.join(proc, `${pid}`, "stat");
    await writeFile(`${file}.tmp`, line);
    await rename(`${file}.tmp`, file);
  };
  await write(100, statLine(100, "node", 1, [900, 900, 0, 0]));
  await write(101, statLine(101, "agent", 100, [10, 0, 0, 0]));
  // Not readable as a process: skipped, not fatal.
  await mkdir(path.join(proc, "self"));
  await mkdir(path.join(proc, "102"));
  let active = 0;
  const stop = watchActivity({
    root: 100,
    intervalMs: 20,
    onActive: () => active++,
    procRoot: proc,
  });
  t.after(stop);
  await sleep(120);
  assert.equal(active, 0, "an idle tree counted as active");
  // 100 ticks a second at 5% of one CPU is one tick per 20ms sample.
  await write(101, statLine(101, "agent", 100, [10, 0, 1, 0]));
  await sleep(120);
  assert.equal(active, 1);
  // The driver's own CPU is not the tree's.
  await write(100, statLine(100, "node", 1, [5000, 900, 0, 0]));
  await sleep(120);
  assert.equal(active, 1);
  stop();
  await write(101, statLine(101, "agent", 100, [99, 0, 1, 0]));
  await sleep(120);
  assert.equal(active, 1, "a stopped watch reported");
});

test("without /proc the watch stops quietly", async () => {
  let active = 0;
  const stop = watchActivity({
    root: process.pid,
    intervalMs: 10,
    onActive: () => active++,
    procRoot: path.join(tmpdir(), "bex-no-proc-here"),
  });
  await sleep(60);
  stop();
  assert.equal(active, 0);
  assert.equal(await readProcessTable(path.join(tmpdir(), "nope")), undefined);
});

test(
  "on Linux the real process table holds this process and its parent",
  { skip: process.platform !== "linux" && "needs /proc" },
  async () => {
    const processes = await readProcessTable();
    assert.equal(processes?.get(process.pid)?.ppid, process.ppid);
  },
);
