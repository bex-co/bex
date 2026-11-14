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

// A provider CLI shipped as a native binary rather than an npm package (ADR108
// D2): the image build downloads the artifact for its architecture, checks its
// SHA-256 and version, and installs it read-only at path.
export interface NativeArtifact {
  url: string;
  sha256: string;
}

export interface NativeBinary {
  version: string;
  path: string;
  artifacts: Partial<Record<"amd64" | "arm64", NativeArtifact>>;
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
  nativeBinary?: NativeBinary;
  // The only model this profile runs (ADR108 D4). The driver applies it only
  // to adapters it knows how to configure (MODEL_SETTINGS_PROFILES).
  model?: string;
  // Runs only on a bex-owned model route, never on a workspace's own key
  // (ADR108 D5).
  platformOnly?: boolean;
  // Off in a workspace until an admin acknowledges that the model provider
  // may train on what sessions send (bex-api's concern; w7/m392).
  requiresDataUseAcknowledgement?: boolean;
  // ACP session config options the driver sets after session/new and
  // session/load (ADR108 D3).
  sessionConfig?: Record<string, string>;
}

export interface AgentProfileManifest {
  version: number;
  profiles: AgentProfile[];
}

const envNamePattern = /^[A-Z_][A-Z0-9_]*(?![\s\S])/;
const configTokenPattern = /^[A-Za-z][A-Za-z0-9_-]*(?![\s\S])/;
const modelIdPattern = /^[a-z0-9][a-z0-9._-]*(?![\s\S])/;
const nativeUrlPattern =
  /^https:\/\/[a-z0-9]+(?:[.-][a-z0-9]+)*\.[a-z]+\/[!#-~]*(?![\s\S])/;
const nativePathPattern = /^\/opt\/[a-z0-9-]+\/[a-z0-9-]+(?![\s\S])/;
const nativeVersionPattern = /^[0-9A-Za-z][0-9A-Za-z.+-]*(?![\s\S])/;
const nativeSHA256Pattern = /^[0-9a-f]{64}(?![\s\S])/;

// Profiles whose adapter the driver can pin to the manifest's model: Muse
// Code reads it from its settings file (acp.ts writeMuseModel). A model on any
// other profile would be silently ignored, so it is refused.
export const MODEL_SETTINGS_PROFILES: ReadonlySet<string> = new Set(["muse"]);

function validateOptionalFields(id: string, profile: AgentProfile): void {
  const native = profile.nativeBinary;
  if (native !== undefined) {
    const artifacts =
      native && typeof native === "object" ? native.artifacts : undefined;
    if (
      !native ||
      typeof native.version !== "string" ||
      !nativeVersionPattern.test(native.version) ||
      typeof native.path !== "string" ||
      !nativePathPattern.test(native.path) ||
      !artifacts ||
      typeof artifacts !== "object" ||
      Array.isArray(artifacts) ||
      !Object.keys(artifacts).length
    )
      throw new Error(`profile ${id}: invalid native binary`);
    for (const [arch, artifact] of Object.entries(artifacts)) {
      if (
        (arch !== "amd64" && arch !== "arm64") ||
        !artifact ||
        typeof artifact.url !== "string" ||
        !nativeUrlPattern.test(artifact.url) ||
        typeof artifact.sha256 !== "string" ||
        !nativeSHA256Pattern.test(artifact.sha256)
      )
        throw new Error(`profile ${id}: invalid native binary artifact`);
    }
  }
  if (
    profile.model !== undefined &&
    (typeof profile.model !== "string" ||
      !modelIdPattern.test(profile.model) ||
      !MODEL_SETTINGS_PROFILES.has(id))
  )
    throw new Error(`profile ${id}: invalid model`);
  for (const flag of [
    "platformOnly",
    "requiresDataUseAcknowledgement",
  ] as const)
    if (profile[flag] !== undefined && typeof profile[flag] !== "boolean")
      throw new Error(`profile ${id}: ${flag} must be a boolean`);
  const config = profile.sessionConfig;
  if (config === undefined) return;
  if (!config || Array.isArray(config) || typeof config !== "object")
    throw new Error(`profile ${id}: sessionConfig must be an object`);
  for (const [key, value] of Object.entries(config)) {
    if (
      !configTokenPattern.test(key) ||
      typeof value !== "string" ||
      !configTokenPattern.test(value)
    )
      throw new Error(`profile ${id}: invalid sessionConfig entry`);
  }
}

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
    validateOptionalFields(id, profile);
  }
}

export function lookupAgentProfile(
  agent: string,
  manifest: AgentProfileManifest = loadAgentProfiles(),
): AgentProfile | undefined {
  const id = agent.trim().toLowerCase();
  return manifest.profiles.find((profile) => profile.id === id);
}
