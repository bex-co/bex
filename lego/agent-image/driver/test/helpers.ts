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

// Shared driver test fixtures: a hermetic AgentDriverConfig pointing at the
// fixture ACP agent, the signed-grant key pair, the child environment, and a
// seeded bare Git remote.

import { execFileSync } from "node:child_process";
import { generateKeyPairSync, randomUUID, sign } from "node:crypto";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  modelProxyPlaceholder,
  type AgentDriverConfig,
} from "../src/config.js";
import { agentEnvironment } from "../src/credentials.js";

const here = path.dirname(fileURLToPath(import.meta.url));
export const fixture = path.join(here, "..", "fixtures", "acp-agent.mjs");
const grantKeys = generateKeyPairSync("ed25519");
const grantPublicKey = (
  grantKeys.publicKey.export({ format: "jwk" }) as JsonWebKey
).x!;

export function turnGrant(sessionID = "ags-test"): string {
  const now = Math.floor(Date.now() / 1000);
  const body = Buffer.from(
    JSON.stringify({
      ses: sessionID,
      act: "turn",
      iat: now,
      exp: now + 30,
      jti: randomUUID(),
    }),
  ).toString("base64url");
  return `${body}.${sign(null, Buffer.from(body), grantKeys.privateKey).toString("base64url")}`;
}

export type TestConfig = AgentDriverConfig & { root: string };

export async function tempConfig(
  overrides: Partial<TestConfig> = {},
): Promise<TestConfig> {
  const root = await mkdtemp(path.join(tmpdir(), "bex-agent-driver-"));
  const workspace = path.join(root, "workspace");
  await mkdir(workspace);
  return {
    command: process.execPath,
    args: [fixture],
    profile: "test",
    cwd: workspace,
    prompt: "Make the requested change and commit it.",
    branch: "",
    repoUrl: "",
    baseBranch: "",
    repository: "",
    contextJson: "",
    originalTask: "",
    deliver: false,
    gitName: "bex agent",
    gitEmail: "agent@bex.co",
    existingSessionId: "",
    persistSession: true,
    listenHost: "127.0.0.1",
    listenPort: 0,
    sessionLogPath: path.join(root, "session.jsonl"),
    statusPath: path.join(root, "status.json"),
    prRecordPath: path.join(root, "pull-request.json"),
    existingPrUrl: "",
    existingPrNumber: null,
    turn: 1,
    exitAfterTurn: false,
    turnTimeoutMs: 1_000,
    credentialEnvName: "ANTHROPIC_API_KEY",
    modelCredential: modelProxyPlaceholder("ags-test"),
    sessionID: "ags-test",
    grantPublicKey,
    agentEnv: {},
    modelBaseUrl: "",
    modelBaseUrlEnvName: "",
    root,
    ...overrides,
  };
}

export function childEnv(config: AgentDriverConfig): Record<string, string> {
  return agentEnvironment(config, {});
}

// git runs a test-side Git command (unhardened, unlike the driver's own).
export function git(cwd: string, args: string[]): string {
  return execFileSync("git", args, { cwd, stdio: "pipe" }).toString().trim();
}

// seedBareRemote creates `root/remote.git`, a bare origin whose main branch
// holds one seed commit, and returns its path.
export async function seedBareRemote(root: string): Promise<string> {
  const remote = path.join(root, "remote.git");
  await mkdir(remote);
  git(remote, ["init", "--bare", "--initial-branch=main"]);
  const seed = path.join(root, "seed");
  await mkdir(seed);
  git(seed, ["init", "--initial-branch=main"]);
  git(seed, ["config", "user.email", "seed@bex.co"]);
  git(seed, ["config", "user.name", "seed"]);
  await writeFile(path.join(seed, "README.md"), "seed\n");
  git(seed, ["add", "-A"]);
  git(seed, ["commit", "-m", "seed"]);
  git(seed, ["push", remote, "main"]);
  return remote;
}

// promptsSent reads the prompt texts the fixture agent logged, in order.
export async function promptsSent(file: string): Promise<string[]> {
  return (await readFile(file, "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line)[0].text as string);
}
