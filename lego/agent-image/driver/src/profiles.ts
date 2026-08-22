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
  modelProxy: ModelProxyRoute;
}

export interface AgentProfileManifest {
  version: number;
  profiles: AgentProfile[];
}

const envNamePattern = /^[A-Z_][A-Z0-9_]*$/;

function manifestPath(): string {
  const here = path.dirname(fileURLToPath(import.meta.url));
  for (const candidate of [
    path.join(here, "..", "agent-profiles.json"),
    path.join(here, "..", "..", "agent-profiles.json"),
  ]) {
    if (existsSync(candidate)) return candidate;
  }
  throw new Error("agent-profiles.json not found");
}

let cached: AgentProfileManifest | undefined;

export function loadAgentProfiles(): AgentProfileManifest {
  if (cached) return cached;
  const raw = readFileSync(manifestPath(), "utf8");
  const manifest = JSON.parse(raw) as AgentProfileManifest;
  validateAgentProfiles(manifest);
  cached = manifest;
  return manifest;
}

export function validateAgentProfiles(manifest: AgentProfileManifest): void {
  if (!manifest.version || manifest.version < 1) {
    throw new Error("agent profiles manifest version must be >= 1");
  }
  const seen = new Set<string>();
  for (const profile of manifest.profiles) {
    const id = profile.id.trim().toLowerCase();
    if (!id) throw new Error("agent profile id is required");
    if (seen.has(id)) throw new Error(`duplicate agent profile id ${id}`);
    seen.add(id);
    if (!path.isAbsolute(profile.executable)) {
      throw new Error(`profile ${id}: executable must be an absolute path`);
    }
    for (const key of Object.keys(profile.env ?? {})) {
      if (!envNamePattern.test(key)) {
        throw new Error(`profile ${id}: env key ${key} is invalid`);
      }
    }
    if (!profile.modelEndpoint?.trim()) {
      throw new Error(`profile ${id}: modelEndpoint is required`);
    }
    for (const name of [
      profile.modelProxy.baseUrlEnv,
      profile.modelProxy.credentialEnv,
    ]) {
      if (!envNamePattern.test(name)) {
        throw new Error(`profile ${id}: model proxy env ${name} is invalid`);
      }
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

export function allowedExecutables(
  manifest: AgentProfileManifest = loadAgentProfiles(),
): Set<string> {
  return new Set(manifest.profiles.map((profile) => profile.executable));
}

export function lookupModelProxyRoute(
  executable: string,
  manifest: AgentProfileManifest = loadAgentProfiles(),
): ModelProxyRoute | undefined {
  const profile = manifest.profiles.find(
    (item) => item.executable === executable,
  );
  return profile?.modelProxy;
}
