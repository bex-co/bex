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
import { mkdir, mkdtemp, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";

import { removeStaleCodexRules, staleCodexRulesFile } from "../src/acp.js";

// A session's persisted .codex may still hold the withdrawn approval mode's
// prompt rules (the withdrawn approval mode, .pm/DO_NOT_DO.md #APPROVAL-MODE); they go before codex starts, other rules stay, and a
// home without the file is fine.
test("removes the withdrawn approval rules from CODEX_HOME and keeps others", async () => {
  const home = await mkdtemp(path.join(tmpdir(), "codex-home-"));
  try {
    const rules = path.join(home, "rules");
    await mkdir(rules, { recursive: true });
    await writeFile(path.join(rules, staleCodexRulesFile), "prefix_rule()\n");
    await writeFile(path.join(rules, "user.rules"), "prefix_rule()\n");
    await removeStaleCodexRules({ CODEX_HOME: home });
    assert.deepEqual(await readdir(rules), ["user.rules"]);
    await removeStaleCodexRules({ CODEX_HOME: home });
    await removeStaleCodexRules({ CODEX_HOME: path.join(home, "missing") });
  } finally {
    await rm(home, { recursive: true, force: true });
  }
});
