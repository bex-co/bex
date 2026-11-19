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

// Whether the commands an agent started are still working. An agent can wait
// on a command with no tool call open and no ACP update: muse sends a long
// command to the background, ends the tool call, and holds its prompt open
// until the command ends (w10/016). The driver cannot see that command's
// output, but it can see its CPU time in /proc, which is how a running test
// suite differs from a hung one. A command waiting on the network or a lock
// uses none, so it is not activity.

import { readdir, readFile } from "node:fs/promises";
import path from "node:path";

// /proc reports CPU time in clock ticks of USER_HZ, 100 a second on Linux and
// under gVisor alike (the userspace ABI fixes it).
const ticksPerSecond = 100;

// minCpuShare is how much of one CPU the process tree must use over a sample
// for it to count: an idle agent CLI, its MCP servers and a waiting command
// use well under 1%, a build or a test suite tens of percent or more.
const minCpuShare = 0.05;

// activityIntervalMs is how often the driver samples: often enough under a
// 30-minute idle bound, cheap for a tree of hundreds of processes.
export const activityIntervalMs = 60 * 1000;

export interface ProcessStat {
  ppid: number;
  // ticks is the CPU time the process and its reaped children used.
  ticks: number;
  // started tells a reused pid from the process that had it before.
  started: string;
}

// parseStat reads one /proc/<pid>/stat line. The command name may hold spaces
// and parentheses, so fields are counted from its last ")".
export function parseStat(stat: string): ProcessStat | undefined {
  const close = stat.lastIndexOf(")");
  if (close < 0) return undefined;
  // fields[0] is field 3 (state), so field n of proc(5) is fields[n - 3].
  const fields = stat
    .slice(close + 1)
    .trim()
    .split(/\s+/);
  const field = (n: number) => Number(fields[n - 3]);
  const ppid = field(4);
  // utime, stime, cutime, cstime: a short-lived compiler or test worker
  // that ended between two samples is counted in its parent's cutime.
  const ticks = field(14) + field(15) + field(16) + field(17);
  const started = fields[22 - 3];
  if (!Number.isInteger(ppid) || !Number.isFinite(ticks) || !started) {
    return undefined;
  }
  return { ppid, ticks, started };
}

// readProcessTable reads every visible process, or answers undefined where
// there is no /proc (a driver run on macOS in development). A process that
// ends while it is read is left out.
export async function readProcessTable(
  procRoot = "/proc",
): Promise<Map<number, ProcessStat> | undefined> {
  let entries: string[];
  try {
    entries = await readdir(procRoot);
  } catch {
    return undefined;
  }
  const table = new Map<number, ProcessStat>();
  await Promise.all(
    entries
      .filter((entry) => /^\d+$/.test(entry))
      .map(async (entry) => {
        const stat = await readFile(
          path.join(procRoot, entry, "stat"),
          "utf8",
        ).catch(() => "");
        const parsed = parseStat(stat);
        if (parsed) table.set(Number(entry), parsed);
      }),
  );
  return table;
}

// descendantTicks answers the CPU ticks of every descendant of root, root
// itself left out (the driver's own work is its ACP traffic, counted
// already), keyed by pid and start time.
export function descendantTicks(
  table: Map<number, ProcessStat>,
  root: number,
): Map<string, number> {
  const children = new Map<number, number[]>();
  for (const [pid, stat] of table) {
    const siblings = children.get(stat.ppid);
    if (siblings) siblings.push(pid);
    else children.set(stat.ppid, [pid]);
  }
  const ticks = new Map<string, number>();
  const seen = new Set<number>([root]);
  const queue = [...(children.get(root) ?? [])];
  for (let pid = queue.pop(); pid !== undefined; pid = queue.pop()) {
    if (seen.has(pid)) continue;
    seen.add(pid);
    const stat = table.get(pid)!;
    ticks.set(`${pid}:${stat.started}`, stat.ticks);
    queue.push(...(children.get(pid) ?? []));
  }
  return ticks;
}

// usedTicks is the CPU the tree used between two samples. A process new in
// the second sample is counted whole: it started after the first.
export function usedTicks(
  before: Map<string, number>,
  after: Map<string, number>,
): number {
  let used = 0;
  for (const [key, ticks] of after) {
    used += Math.max(0, ticks - (before.get(key) ?? 0));
  }
  return used;
}

export interface ActivityWatch {
  // root is the process whose descendants are watched: the driver, so the
  // agent, its MCP servers and every command they started.
  root: number;
  intervalMs: number;
  // onActive runs after each sample in which the tree used the CPU.
  onActive: () => void;
  procRoot?: string;
}

// watchActivity samples root's descendants every intervalMs and calls
// onActive after a sample in which they used at least minCpuShare of one CPU.
// Without /proc it stops at the first sample. It returns the function that
// stops the watch.
export function watchActivity(watch: ActivityWatch): () => void {
  const minTicks = Math.max(
    1,
    Math.round((watch.intervalMs / 1000) * ticksPerSecond * minCpuShare),
  );
  let stopped = false;
  let sampling = false;
  let previous: Map<string, number> | undefined;
  const stop = () => {
    stopped = true;
    clearInterval(timer);
  };
  const sample = async () => {
    if (sampling || stopped) return;
    sampling = true;
    try {
      const table = await readProcessTable(watch.procRoot);
      if (stopped) return;
      if (!table) return stop();
      const current = descendantTicks(table, watch.root);
      if (previous && usedTicks(previous, current) >= minTicks) {
        watch.onActive();
      }
      previous = current;
    } finally {
      sampling = false;
    }
  };
  const timer = setInterval(() => void sample(), watch.intervalMs);
  timer.unref();
  void sample();
  return stop;
}
