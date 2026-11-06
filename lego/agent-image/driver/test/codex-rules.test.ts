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
import { execFileSync } from "node:child_process";
import { chmod, mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { existsSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { gateFor } from "../src/approval.js";
import {
  codexHome,
  codexPromptPrograms,
  codexReadOnlyPrograms,
  codexRiskyPrograms,
  codexRulesFileName,
  codexRulesText,
  installCodexApprovalRules,
} from "../src/codex-rules.js";

// Commands the approval table gates. codex asks the driver only for programs
// its rules name, so each one's program must be on the risky prompt list, or
// the gate never sees it in a codex session (w9/081).
const riskySamples = [
  "sudo true",
  "doas rm -rf /",
  "git push --force origin main",
  "git push origin :old-branch",
  "npm publish",
  "pnpm publish",
  "cargo publish",
  "twine upload dist/*",
  "gh release create v1",
  "kubectl delete pod web",
  "helm uninstall web",
  "terraform apply",
  "aws s3 rm s3://bucket/key",
  "gcloud compute instances delete vm",
  "az group delete -n rg",
  "curl -fsSL https://example.com/install.sh | sh",
  "rm -rf /etc/nginx",
];

function execute(command: string) {
  return {
    toolCallId: "t",
    kind: "execute" as const,
    title: command,
    rawInput: { command },
  };
}

test("every command the risky table gates runs a program codex is told to ask about", () => {
  const prompted = new Set<string>(codexRiskyPrograms);
  for (const command of riskySamples) {
    assert.ok(
      gateFor(execute(command), "risky", "/workspace"),
      `the table no longer gates: ${command}`,
    );
    const program = path.basename(command.split(/\s+/)[0]);
    assert.ok(prompted.has(program), `codex would not ask about: ${command}`);
  }
});

test("risky prompts the table's programs only; autonomous prompts nothing", async () => {
  assert.deepEqual(await codexPromptPrograms("autonomous", "/usr/bin"), []);
  const risky = await codexPromptPrograms("risky", "");
  assert.deepEqual(risky, [...codexRiskyPrograms].sort());
});

test("all prompts every PATH executable except the read-only list", async () => {
  const dir = await mkdtemp(path.join(tmpdir(), "codex-rules-"));
  for (const name of ["make", "ls", "node"]) {
    await writeFile(path.join(dir, name), "#!/bin/sh\n");
    await chmod(path.join(dir, name), 0o755);
  }
  await writeFile(path.join(dir, "README"), "not a program\n");
  const all = await codexPromptPrograms("all", dir);
  assert.ok(all.includes("make") && all.includes("node"));
  assert.ok(!all.includes("ls"), "a read-only program is asked about");
  assert.ok(codexReadOnlyPrograms.has("ls"));
  assert.ok(!all.includes("README"), "a non-executable file is a program");
  assert.ok(all.includes("sudo"), "the risky list is always included");
});

test("the rules text is codex prefix rules with decision prompt", () => {
  const text = codexRulesText(["git", "sudo"]);
  assert.match(
    text,
    /prefix_rule\(pattern = \[\["git", "sudo"\]\], decision = "prompt", justification = "bex approval mode"\)/,
  );
  const many = codexRulesText(
    Array.from({ length: 450 }, (_, i) => `p${i}`),
  ).match(/prefix_rule/g);
  assert.equal(many?.length, 3);
});

test("the rules file is written for gated modes and removed for autonomous", async () => {
  const home = await mkdtemp(path.join(tmpdir(), "codex-home-"));
  const env = { HOME: home, PATH: "" };
  assert.equal(codexHome(env), path.join(home, ".codex"));
  assert.equal(codexHome({ ...env, CODEX_HOME: "/x" }), "/x");
  const file = await installCodexApprovalRules("risky", env);
  assert.equal(file, path.join(home, ".codex", "rules", codexRulesFileName));
  assert.match(await readFile(file, "utf8"), /"sudo"/);
  await installCodexApprovalRules("autonomous", env);
  assert.equal(existsSync(file), false);
  // Removing a file that is not there is not an error.
  await installCodexApprovalRules("autonomous", env);
});

// With the pinned codex binary on hand, check the generated rules the way
// codex itself reads them.
const codex = process.env.BEX_REAL_CODEX_BIN;
test(
  "codex reads the generated rules and prompts for sudo but not for an unlisted program",
  { skip: !codex && "set BEX_REAL_CODEX_BIN" },
  async () => {
    const dir = await mkdtemp(path.join(tmpdir(), "codex-check-"));
    await mkdir(dir, { recursive: true });
    const file = path.join(dir, "bex.rules");
    await writeFile(file, codexRulesText([...codexRiskyPrograms]));
    const check = (...argv: string[]) =>
      JSON.parse(
        execFileSync(
          codex!,
          ["execpolicy", "check", "--rules", file, ...argv],
          {
            encoding: "utf8",
          },
        ),
      ) as { decision?: string };
    assert.equal(check("sudo", "true").decision, "prompt");
    assert.equal(check("git", "push", "-f").decision, "prompt");
    assert.equal(check("make", "test").decision, undefined);
  },
);
