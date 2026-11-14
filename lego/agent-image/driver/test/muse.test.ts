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

// The muse profile's driver contract (ADR108): its model lands in Muse's own
// settings, and the profile's ACP session config is set on every session
// before the first prompt, failing setup when the adapter refuses it.
import assert from "node:assert/strict";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { writeMuseModel } from "../src/acp.js";
import { loadConfig } from "../src/config.js";
import { loadAgentProfiles, validateAgentProfiles } from "../src/profiles.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, tempConfig } from "./helpers.js";

const museConfig = {
  sandbox: "disabled",
  nativeApprovalPolicy: "allowAll",
  mode: "bypassApprovals",
};

test("writeMuseModel pins the model and keeps the rest of the settings", async () => {
  const { root } = await tempConfig();
  const configHome = path.join(root, "config");
  const file = path.join(configHome, "muse", "settings.json");
  await mkdir(path.dirname(file), { recursive: true });
  await writeFile(
    file,
    JSON.stringify({ schema_version: 1, model: "muse-spark-1.2", theme: "x" }),
  );
  await writeMuseModel(
    { XDG_CONFIG_HOME: configHome },
    "muse-spark-1.3-contributor",
  );
  assert.deepEqual(JSON.parse(await readFile(file, "utf8")), {
    schema_version: 1,
    model: "muse-spark-1.3-contributor",
    theme: "x",
  });
  await writeFile(file, "{not json");
  await writeMuseModel(
    { XDG_CONFIG_HOME: configHome },
    "muse-spark-1.3-contributor",
  );
  assert.deepEqual(JSON.parse(await readFile(file, "utf8")), {
    schema_version: 1,
    model: "muse-spark-1.3-contributor",
  });
});

test("a muse session gets its model and session config before the first prompt", async () => {
  const config = await tempConfig({
    profile: "muse",
    model: "muse-spark-1.3-contributor",
    sessionConfig: museConfig,
  });
  const callLog = path.join(config.root, "calls.log");
  const configLog = path.join(config.root, "config.log");
  const configHome = path.join(config.root, "xdg");
  config.agentEnv = {
    ACP_FIXTURE_LOG: callLog,
    ACP_FIXTURE_CONFIG_LOG: configLog,
    XDG_CONFIG_HOME: configHome,
  };
  await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub());

  const settings = JSON.parse(
    await readFile(path.join(configHome, "muse", "settings.json"), "utf8"),
  );
  assert.equal(settings.model, "muse-spark-1.3-contributor");
  const calls = (await readFile(callLog, "utf8")).trim().split("\n");
  const firstConfig = calls.indexOf("session/set_config_option");
  assert.ok(firstConfig > calls.indexOf("session/new"));
  assert.ok(
    calls.lastIndexOf("session/set_config_option") <
      calls.indexOf("session/prompt"),
  );
  const applied = (await readFile(configLog, "utf8"))
    .trim()
    .split("\n")
    .map((line) => JSON.parse(line) as Record<string, string>);
  assert.deepEqual(
    Object.fromEntries(applied.map((p) => [p.configId, p.value])),
    museConfig,
  );
  assert.ok(applied.every((p) => p.sessionId === applied[0]!.sessionId));
});

test("a refused session config option fails setup instead of running on defaults", async () => {
  const config = await tempConfig({ sessionConfig: museConfig });
  const callLog = path.join(config.root, "calls.log");
  config.agentEnv = {
    ACP_FIXTURE_LOG: callLog,
    ACP_FIXTURE_REJECT_CONFIG: "sandbox",
  };
  await assert.rejects(
    runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub()),
    /unsupported option/,
  );
  const calls = (await readFile(callLog, "utf8")).trim().split("\n");
  assert.ok(!calls.includes("session/prompt"), "a prompt ran on the defaults");
});

test("the muse release profile loads with its model and session config", () => {
  const config = loadConfig({
    BEX_AGENT_PROFILE: "muse",
    BEX_AGENT_SESSION_ID: "ags-test",
    BEX_AGENT_MODEL_PROXY_URL: "http://gateway/model/ns/ags",
    BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-ags-test",
  });
  assert.equal(config.command, "/usr/local/bin/muse-code-acp");
  assert.equal(config.model, "muse-spark-1.3-contributor");
  assert.deepEqual(config.sessionConfig, museConfig);
  assert.equal(config.modelBaseUrlEnvName, "MUSE_CODE_ACP_GATEWAY_URL");
  assert.equal(config.modelBaseUrl, "http://gateway/model/ns/ags/v1");
  assert.equal(config.credentialEnvName, "MUSE_CODE_ACP_GATEWAY_KEY");
});

test("a model on a profile the driver cannot pin it for is refused", () => {
  const manifest = loadAgentProfiles();
  manifest.profiles[0]!.model = "some-model";
  assert.notEqual(manifest.profiles[0]!.id, "muse");
  assert.throws(() => validateAgentProfiles(manifest), /invalid model/);
});
