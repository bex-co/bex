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

// Workspace skills (ADR099). bex-api copies the workspace's enabled skills to
// /bex/skills/<name>/SKILL.md while the sandbox starts and lists them, with a
// bounded description each, in BEX_AGENT_SKILLS. The driver waits for the
// copy, links each skill into the engines' own skills folders so their
// loaders find it, and indexes the skills in the overlay for an engine
// without a loader, which keeps the Agent Skills format's progressive
// disclosure.

import { lstat, mkdir, symlink } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import { manifestWaitMs, waitForFiles } from "./fs-util.js";

export const skillsRoot = "/bex/skills";

export interface SkillEntry {
  name: string;
  // description is the overlay index line bex-api shortened to its budget;
  // empty once that budget is spent.
  description: string;
  // userOnly skills are applied only when invoked; invoked marks the ones this
  // turn names, with their arguments.
  userOnly: boolean;
  invoked: boolean;
  arguments: string;
  path: string;
}

export interface SkillManifest {
  waitMs: number;
  root: string;
  skills: SkillEntry[];
}

const skillName = /^[a-z0-9]+(-[a-z0-9]+)*$/;
// Engines whose own loader reads the linked skills folders and already shows
// the agent each skill's name and description.
const nativeSkillLoaders = new Set(["claude", "codex"]);
// maxSkillsSectionBytes bounds the overlay index, which every turn re-sends.
const maxSkillsSectionBytes = 8 << 10;

// oneLine keeps manifest text on one line: bex-api refuses line breaks in
// descriptions and arguments, and the driver does not rely on that.
function oneLine(value: unknown): string {
  return typeof value === "string"
    ? value.replace(/[\r\n\u2028\u2029]+/g, " ")
    : "";
}

// parseSkillManifest validates BEX_AGENT_SKILLS. A name that is not a skill
// name could escape the root, so it is refused rather than trusted.
export function parseSkillManifest(
  raw: string | undefined,
): SkillManifest | undefined {
  if (!raw) return undefined;
  const parsed = JSON.parse(raw) as Partial<SkillManifest>;
  if (!Array.isArray(parsed?.skills) || parsed.root !== skillsRoot) {
    throw new Error("BEX_AGENT_SKILLS is malformed");
  }
  const skills = parsed.skills.map((skill): SkillEntry => {
    const name = String(skill?.name);
    if (!skillName.test(name) || name.length > 64) {
      throw new Error("BEX_AGENT_SKILLS names an invalid skill");
    }
    return {
      name,
      description: oneLine(skill.description),
      userOnly: skill.userOnly === true,
      invoked: skill.invoked === true,
      arguments: oneLine(skill.arguments),
      path: path.posix.join(skillsRoot, name, "SKILL.md"),
    };
  });
  return { waitMs: manifestWaitMs(parsed.waitMs), root: skillsRoot, skills };
}

// waitForSkills resolves once every listed SKILL.md exists.
export async function waitForSkills(
  manifest: SkillManifest,
  options: { intervalMs?: number } = {},
): Promise<void> {
  await waitForFiles(manifest.skills, {
    waitMs: manifest.waitMs,
    file: (skill) => skill.path,
    ready: () => true,
    label: (skill) => skill.name,
    what: "workspace skills",
    intervalMs: options.intervalMs,
  });
}

// The engines' user-level skills folders: the Agent Skills standard's, Claude
// Code's and Codex's. A repository's own skills live in the checkout, where
// each engine finds them itself, so they win a name clash.
export function engineSkillDirs(home = os.homedir()): string[] {
  return [".agents", ".claude", ".codex"].map((dir) =>
    path.join(home, dir, "skills"),
  );
}

// usableSkillsDir creates dir if it is missing and reports whether it is a
// real directory. A symlink or a file in its place is skipped rather than
// followed or replaced: the folder is the user's.
async function usableSkillsDir(dir: string): Promise<boolean> {
  try {
    await mkdir(path.dirname(dir), { recursive: true });
    const existing = await lstat(dir).catch(() => undefined);
    if (!existing) await mkdir(dir);
    return (await lstat(dir)).isDirectory();
  } catch {
    return false;
  }
}

// installSkills waits for the copy, then links each skill into every usable
// engine folder, never replacing an entry already there. One unusable folder
// or failed link skips that link, not the turn.
export async function installSkills(
  manifest: SkillManifest,
  options: { home?: string; intervalMs?: number } = {},
): Promise<SkillManifest> {
  await waitForSkills(manifest, options);
  const dirs: string[] = [];
  for (const dir of engineSkillDirs(options.home)) {
    if (await usableSkillsDir(dir)) dirs.push(dir);
  }
  for (const skill of manifest.skills) {
    for (const dir of dirs) {
      const link = path.join(dir, skill.name);
      if (await lstat(link).catch(() => undefined)) continue;
      await symlink(path.dirname(skill.path), link).catch(() => undefined);
    }
  }
  return manifest;
}

// skillsSection lists the workspace's skills for the overlay: names up front,
// the body only when the agent reads the file. Engines with their own skill
// loader already see the descriptions, so they get names and paths only; the
// section stays within maxSkillsSectionBytes either way.
export function skillsSection(
  manifest: SkillManifest | undefined,
  profile?: string,
): string {
  if (!manifest?.skills.length) return "";
  const describe = !nativeSkillLoaders.has(profile ?? "");
  const lines: string[] = [];
  let bytes = 0;
  for (const skill of manifest.skills) {
    const when = skill.userOnly
      ? " Use it only when the user asks for it."
      : "";
    const about =
      describe && skill.description ? `: ${skill.description}.` : ".";
    const line = `- \`${skill.name}\` (\`${skill.path}\`)${about}${when}`;
    bytes += Buffer.byteLength(line) + 1;
    if (bytes > maxSkillsSectionBytes) {
      lines.push(
        `- …and ${manifest.skills.length - lines.length} more in \`${skillsRoot}\`.`,
      );
      break;
    }
    lines.push(line);
  }
  return `

## Workspace skills

Your workspace shares these skills: reusable instructions in the Agent Skills format. When a task matches a skill, read its \`SKILL.md\` and follow it. A skill in the repository's own skills folders takes precedence over a workspace skill of the same name. A prompt can name a skill as \`@skills:<name>\`, optionally followed by arguments.

${lines.join("\n")}`;
}

// withSkillInvocations prepends one line per skill the turn invokes. bex-api
// has already substituted the arguments in the delivered copy; they are
// quoted here as one JSON string, so they cannot read as instructions.
export function withSkillInvocations(
  prompt: string,
  manifest: SkillManifest | undefined,
): string {
  const invoked = manifest?.skills.filter((skill) => skill.invoked) ?? [];
  if (invoked.length === 0) return prompt;
  const lines = invoked.map((skill) => {
    const args = skill.arguments
      ? ` with arguments ${JSON.stringify(skill.arguments)}`
      : "";
    return `Use the skill \`${skill.name}\` (\`${skill.path}\`)${args}.`;
  });
  return `${lines.join("\n")}\n\n${prompt}`;
}
