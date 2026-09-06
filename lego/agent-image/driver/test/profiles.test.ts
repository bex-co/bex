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
import { readFileSync } from "node:fs";
import test from "node:test";
import { loadAgentProfiles, validateAgentProfiles } from "../src/profiles.js";
import { loadConfig } from "../src/config.js";

const cases = JSON.parse(
  readFileSync(
    new URL(
      "../../../backend/internal/agentsession/profiles-invalid-cases.json",
      import.meta.url,
    ),
    "utf8",
  ),
) as { name: string; path: (string | number)[]; value: unknown }[];
for (const fixture of cases) {
  test(`release profile rejects ${fixture.name}`, () => {
    const manifest = structuredClone(loadAgentProfiles());
    let cursor: any = manifest;
    for (const key of fixture.path.slice(0, -1)) cursor = cursor[key];
    cursor[fixture.path.at(-1)!] = fixture.value;
    assert.throws(() => validateAgentProfiles(manifest));
  });
}
for (const key of ["id", "executable"] as const) {
  test(`release profile rejects duplicate ${key}`, () => {
    const manifest = structuredClone(loadAgentProfiles());
    manifest.profiles[1][key] = manifest.profiles[0][key];
    assert.throws(() => validateAgentProfiles(manifest));
  });
}
test("driver resolves profile facts and rejects runtime overrides", () => {
  for (const profile of loadAgentProfiles().profiles) {
    const config = loadConfig({
      BEX_AGENT_PROFILE: profile.id,
      BEX_AGENT_MODEL_PROXY_URL: "http://gateway/model/ns/session",
    });
    assert.equal(config.command, profile.executable);
    assert.deepEqual(config.args, profile.args);
    assert.deepEqual(config.agentEnv, profile.env);
    assert.equal(config.credentialEnvName, profile.modelProxy.credentialEnv);
    assert.equal(config.modelBaseUrlEnvName, profile.modelProxy.baseUrlEnv);
    assert.equal(
      config.modelBaseUrl,
      "http://gateway/model/ns/session" + profile.modelProxy.baseUrlSuffix,
    );
  }
  for (const env of [
    { BEX_AGENT_PROFILE: "unknown" },
    { BEX_AGENT_PROFILE: "codex", BEX_AGENT_COMMAND: "/usr/local/bin/gemini" },
    { BEX_AGENT_ARGS: '["--arbitrary"]' },
    { BEX_AGENT_ENV_JSON: '{"NODE_OPTIONS":"--require=/workspace/evil.js"}' },
    {
      BEX_AGENT_MODEL_PROXY_URL: "http://gateway",
      BEX_AGENT_MODEL_API_KEY_ENV: "OTHER_KEY",
    },
  ])
    assert.throws(() => loadConfig(env));
});
