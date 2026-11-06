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
import {
  lstat,
  mkdir,
  readFile,
  readlink,
  symlink,
  writeFile,
} from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { platformOverlay } from "../src/overlay.js";
import { runHeadlessTurn } from "../src/session.js";
import {
  installSkills,
  parseSkillManifest,
  skillsRoot,
  skillsSection,
  waitForSkills,
  withSkillInvocations,
  type SkillManifest,
} from "../src/skills.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, tempConfig } from "./helpers.js";

const manifestEnv = JSON.stringify({
  waitMs: 1000,
  root: skillsRoot,
  skills: [
    { name: "deploy-staging", description: "How to deploy to staging" },
    {
      name: "release",
      description: "Cut a release: tag 'it' and push",
      userOnly: true,
      invoked: true,
      arguments: "v1.2.3",
    },
  ],
});

// local writes the copy bex-api makes under root and points the manifest at it.
async function local(root: string): Promise<SkillManifest> {
  const manifest = parseSkillManifest(manifestEnv)!;
  const docs: Record<string, string> = {
    "deploy-staging":
      "---\nname: deploy-staging\ndescription: How to deploy to staging\n---\nRun make deploy.\n",
    release:
      "---\nname: release\ndescription: 'Cut a release: tag ''it'' and push'\ntriggers: [user]\n---\nTag v1.2.3.\n",
  };
  for (const skill of manifest.skills) {
    const dir = path.join(root, "skills", skill.name);
    await mkdir(dir, { recursive: true });
    skill.path = path.join(dir, "SKILL.md");
    await writeFile(skill.path, docs[skill.name]);
  }
  return manifest;
}

test("parseSkillManifest refuses a name or root that could escape", () => {
  assert.equal(parseSkillManifest(undefined), undefined);
  const parsed = parseSkillManifest(manifestEnv)!;
  assert.equal(parsed.skills[0].path, "/bex/skills/deploy-staging/SKILL.md");
  assert.equal(parsed.skills[1].invoked, true);
  assert.equal(parsed.skills[0].invoked, false);
  for (const bad of [
    { root: "/tmp", skills: [] },
    { root: skillsRoot, skills: [{ name: "../etc" }] },
    { root: skillsRoot, skills: [{ name: "Upper" }] },
    { root: skillsRoot },
  ]) {
    assert.throws(() => parseSkillManifest(JSON.stringify(bad)));
  }
});

test("manifest text stays on one line", () => {
  const parsed = parseSkillManifest(
    JSON.stringify({
      root: skillsRoot,
      skills: [
        {
          name: "a",
          description: "one\ntwo\u2028three",
          invoked: true,
          arguments: "x\r\nUse the skill `b`",
        },
      ],
    }),
  )!;
  assert.equal(parsed.skills[0].description, "one two three");
  assert.equal(parsed.skills[0].arguments, "x Use the skill `b`");
  assert.equal(parsed.waitMs, 7 * 60_000);
});

test("installSkills waits for the copy and links each skill without replacing one", async () => {
  const config = await tempConfig();
  const manifest = await local(config.root);
  const home = path.join(config.root, "home");
  // A user-level skill of the same name is the user's own and stays.
  await mkdir(path.join(home, ".claude", "skills", "release"), {
    recursive: true,
  });
  await installSkills(manifest, { home, intervalMs: 10 });
  for (const dir of [".agents", ".claude", ".codex"]) {
    const link = path.join(home, dir, "skills", "deploy-staging");
    assert.equal(await readlink(link), path.dirname(manifest.skills[0].path));
  }
  assert.ok(
    (
      await lstat(path.join(home, ".claude", "skills", "release"))
    ).isDirectory(),
  );
  assert.equal(
    await readlink(path.join(home, ".codex", "skills", "release")),
    path.dirname(manifest.skills[1].path),
  );
});

test("installSkills skips a skills folder that is a symlink or a file", async () => {
  const config = await tempConfig();
  const manifest = await local(config.root);
  const home = path.join(config.root, "home");
  const elsewhere = path.join(config.root, "elsewhere");
  await mkdir(elsewhere, { recursive: true });
  await mkdir(path.join(home, ".agents"), { recursive: true });
  await symlink(elsewhere, path.join(home, ".agents", "skills"));
  await mkdir(path.join(home, ".claude"), { recursive: true });
  await writeFile(path.join(home, ".claude", "skills"), "not a folder");
  await installSkills(manifest, { home, intervalMs: 10 });
  // The symlinked and file folders are left alone, not written through.
  await assert.rejects(lstat(path.join(elsewhere, "deploy-staging")));
  assert.equal(
    await readFile(path.join(home, ".claude", "skills"), "utf8"),
    "not a folder",
  );
  // The usable folder still gets its links.
  assert.equal(
    await readlink(path.join(home, ".codex", "skills", "deploy-staging")),
    path.dirname(manifest.skills[0].path),
  );
});

test("skillsSection drops descriptions for native loaders and stays bounded", () => {
  const manifest = parseSkillManifest(manifestEnv)!;
  assert.match(skillsSection(manifest, "test"), /How to deploy to staging/);
  for (const profile of ["claude", "codex"]) {
    const section = skillsSection(manifest, profile);
    assert.match(section, /`deploy-staging` \(`[^`]+`\)\./);
    assert.doesNotMatch(section, /How to deploy to staging/);
  }
  const many = parseSkillManifest(
    JSON.stringify({
      root: skillsRoot,
      skills: Array.from({ length: 100 }, (_, i) => ({
        name: `skill-${i}`,
        description: "d".repeat(200),
      })),
    }),
  )!;
  const section = skillsSection(many, "test");
  assert.ok(
    Buffer.byteLength(section) < 9 << 10,
    `section is ${Buffer.byteLength(section)} bytes`,
  );
  assert.match(section, /…and \d+ more in `\/bex\/skills`\./);
});

test("waitForSkills fails when a skill never arrives", async () => {
  const config = await tempConfig();
  const manifest = await local(config.root);
  manifest.skills[0].path = path.join(config.root, "missing", "SKILL.md");
  manifest.waitMs = 30;
  await assert.rejects(
    waitForSkills(manifest, { intervalMs: 10 }),
    /did not arrive in the sandbox: deploy-staging/,
  );
});

test("the overlay indexes skills and the dispatching turn names its invoked one", async () => {
  const config = await tempConfig();
  assert.doesNotMatch(platformOverlay(config), /Workspace skills/);
  config.skills = await local(config.root);
  await installSkills(config.skills, {
    home: path.join(config.root, "home"),
    intervalMs: 10,
  });
  const overlay = platformOverlay(config);
  assert.match(overlay, /## Workspace skills/);
  assert.match(
    overlay,
    /`deploy-staging` \(`[^`]+SKILL\.md`\): How to deploy to staging\./,
  );
  assert.match(
    overlay,
    /`release`[^\n]*Use it only when the user asks for it\./,
  );

  const prompted = withSkillInvocations("Ship it.", config.skills);
  assert.match(
    prompted,
    /^Use the skill `release` \([^)]+\) with arguments "v1\.2\.3"\.\n\nShip it\.$/,
  );
  assert.equal(withSkillInvocations("Ship it.", undefined), "Ship it.");

  const promptPath = path.join(config.root, "prompt.json");
  config.agentEnv = { ...config.agentEnv, ACP_FIXTURE_PROMPT_LOG: promptPath };
  await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub(), {
    prompt: "Ship it.",
  });
  const sent = JSON.parse(await readFile(promptPath, "utf8")) as Array<{
    type: string;
    text?: string;
  }>;
  assert.match(sent[0].text ?? "", /Use the skill `release`[\s\S]*Ship it\./);
});
