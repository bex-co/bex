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

import { existsSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export interface ModelProxyRoute {
  baseUrlEnv: string;
  baseUrlSuffix: string;
  credentialEnv: string;
}

export interface AgentProfile {
  id: string;
  executable: string;
  args: string[];
  env: Record<string, string>;
  modelEndpoint: string;
  npmPackage: string;
  authentication: "environment";
  permissionPolicy: "sandbox-auto-approve";
  sessionState: string[];
  modelProxy: ModelProxyRoute;
}

export interface AgentProfileManifest {
  version: number;
  profiles: AgentProfile[];
}

const envNamePattern = /^[A-Z_][A-Z0-9_]*(?![\s\S])/;

function manifestPath(): string {
  const here = path.dirname(fileURLToPath(import.meta.url));
  for (const candidate of [
    path.join(here, "..", "agent-profiles.json"),
    path.join(
      here,
      "..",
      "..",
      "..",
      "backend",
      "internal",
      "agentsession",
      "agent-profiles.json",
    ),
  ]) {
    if (existsSync(candidate)) return candidate;
  }
  throw new Error("agent-profiles.json not found");
}

let cached: AgentProfileManifest | undefined;

export function loadAgentProfiles(): AgentProfileManifest {
  if (cached) return structuredClone(cached);
  const raw = readFileSync(manifestPath(), "utf8");
  const manifest = JSON.parse(raw) as AgentProfileManifest;
  validateAgentProfiles(manifest);
  cached = manifest;
  return structuredClone(manifest);
}

export function validateAgentProfiles(manifest: AgentProfileManifest): void {
  if (
    manifest.version !== 1 ||
    !Array.isArray(manifest.profiles) ||
    !manifest.profiles.length
  ) {
    throw new Error(
      "agent profiles manifest must have version 1 and nonempty profiles",
    );
  }
  const seen = new Set<string>();
  const commands = new Set<string>();
  for (const profile of manifest.profiles) {
    const id = profile.id;
    if (typeof id !== "string" || !/^[a-z][a-z0-9-]*(?![\s\S])/.test(id))
      throw new Error("invalid agent profile id");
    if (seen.has(id)) throw new Error(`duplicate agent profile id ${id}`);
    seen.add(id);
    if (
      typeof profile.executable !== "string" ||
      !/^\/usr\/local\/bin\/[a-zA-Z0-9_-]+(?![\s\S])/.test(
        profile.executable,
      ) ||
      commands.has(profile.executable)
    ) {
      throw new Error(
        `profile ${id}: executable must be a unique installed adapter path`,
      );
    }
    commands.add(profile.executable);
    if (
      !Array.isArray(profile.args) ||
      profile.args.some((arg) => typeof arg !== "string" || arg.includes("\0"))
    ) {
      throw new Error(`profile ${id}: args must be strings`);
    }
    if (
      !profile.env ||
      Array.isArray(profile.env) ||
      typeof profile.env !== "object"
    )
      throw new Error(`profile ${id}: env is required`);
    for (const [key, value] of Object.entries(profile.env)) {
      if (
        !envNamePattern.test(key) ||
        typeof value !== "string" ||
        value.includes("\0")
      )
        throw new Error(`profile ${id}: env key/value is invalid`);
    }
    if (
      typeof profile.npmPackage !== "string" ||
      !/^@[a-z0-9-]+\/[a-z0-9-]+@\d+\.\d+\.\d+(?![\s\S])/.test(
        profile.npmPackage,
      )
    )
      throw new Error(`profile ${id}: exact npm package pin is required`);
    if (
      profile.authentication !== "environment" ||
      profile.permissionPolicy !== "sandbox-auto-approve"
    )
      throw new Error(
        `profile ${id}: unsupported authentication or permission policy`,
      );
    if (
      !Array.isArray(profile.sessionState) ||
      !profile.sessionState.length ||
      profile.sessionState.some(
        (value) =>
          typeof value !== "string" ||
          !/^\.[a-zA-Z0-9_-]+(?:\.[a-zA-Z0-9_-]+)*(?![\s\S])/.test(value),
      )
    )
      throw new Error(`profile ${id}: unsafe session state path`);
    if (
      typeof profile.modelEndpoint !== "string" ||
      !/^https:\/\/[a-z0-9]+(?:[.-][a-z0-9]+)*\.[a-z]+(?:\/[a-zA-Z0-9_-]+)*(?![\s\S])/.test(
        profile.modelEndpoint,
      )
    )
      throw new Error(`profile ${id}: invalid model endpoint`);
    const route = profile.modelProxy;
    if (
      !route ||
      ![route.baseUrlEnv, route.credentialEnv].every(
        (name) => typeof name === "string" && envNamePattern.test(name),
      ) ||
      route.baseUrlEnv === route.credentialEnv ||
      typeof route.baseUrlSuffix !== "string" ||
      !/^(?:\/[a-zA-Z0-9_-]+)*(?![\s\S])/.test(route.baseUrlSuffix)
    )
      throw new Error(`profile ${id}: incomplete model proxy routing`);
    for (const name of [
      "BEX_AGENT_MODEL_API_KEY",
      route.baseUrlEnv,
      route.credentialEnv,
    ]) {
      if (Object.hasOwn(profile.env, name))
        throw new Error(`profile ${id}: bootstrap env overrides model routing`);
    }
  }
}

export function lookupAgentProfile(
  agent: string,
  manifest: AgentProfileManifest = loadAgentProfiles(),
): AgentProfile | undefined {
  const id = agent.trim().toLowerCase();
  return manifest.profiles.find((profile) => profile.id === id);
}
