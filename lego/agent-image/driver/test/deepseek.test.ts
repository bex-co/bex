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

// The deepseek profile's driver contract (ADR114): DeepSeek Harness runs on
// the profile's own Node runtime, from an empty launch directory, with a
// launch patch that pins its model to OpenRouter at the gateway model proxy.
import assert from "node:assert/strict";
import {
  mkdir,
  readFile,
  readdir,
  realpath,
  writeFile,
} from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { writeDeepSeekPatch } from "../src/acp.js";
import { loadConfig } from "../src/config.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, tempConfig } from "./helpers.js";

const model = "~deepseek/deepseek-flash-latest";

test("writeDeepSeekPatch routes the acp app to OpenRouter at the proxy", async () => {
  const { root } = await tempConfig();
  const dshHome = path.join(root, "dsh");
  const config = {
    model,
    modelBaseUrl: "http://gateway/model/ns/ags/v1",
    credentialEnvName: "OPENROUTER_API_KEY",
  };
  // A .env the agent left in the launch directory on an earlier turn is gone.
  await mkdir(path.join(dshHome, "bex-launch"), { recursive: true });
  await writeFile(path.join(dshHome, "bex-launch", ".env"), "HTTPS_PROXY=x\n");
  const { patch, cwd } = await writeDeepSeekPatch(
    { DSH_HOME: dshHome },
    config,
  );
  assert.equal(patch, path.join(dshHome, "bex-acp.patch.yml"));
  assert.deepEqual(await readdir(cwd), []);
  assert.deepEqual(JSON.parse(await readFile(patch, "utf8")), [
    {
      id: "llm-pi-ai",
      config: {
        providers: {
          openrouter: {
            apiKeyEnv: "OPENROUTER_API_KEY",
            baseURL: "http://gateway/model/ns/ags/v1",
            api: "openai-completions",
            reasoning: "high",
            models: [{ id: model }],
          },
        },
      },
    },
    { id: "acp", config: { provider: "openrouter", model } },
    { id: "tool-web", disabled: true },
    { id: "session-log-deepseek", config: { enabled: false } },
  ]);
});

test("a deepseek session starts on its Node runtime with the launch patch", async () => {
  const config = await tempConfig({
    profile: "deepseek",
    model,
    modelBaseUrl: "http://gateway/model/ns/ags/v1",
    credentialEnvName: "OPENROUTER_API_KEY",
  });
  // The fixture stands in for dsh's bin script, run by the runtime.
  config.nodeRuntime = config.command;
  config.command = config.args[0]!;
  config.args = ["--profile", "acp"];
  const envLog = path.join(config.root, "env.json");
  const dshHome = path.join(config.root, "dsh");
  config.agentEnv = { ACP_FIXTURE_ENV_LOG: envLog, DSH_HOME: dshHome };
  await runHeadlessTurn(config, childEnv(config), new UIMessageStreamHub());

  const started = JSON.parse(await readFile(envLog, "utf8"));
  const patch = path.join(dshHome, "bex-acp.patch.yml");
  assert.deepEqual(started.argv, ["--profile", "acp", "--patch", patch]);
  assert.equal(started.cwd, await realpath(path.join(dshHome, "bex-launch")));
  const [route] = JSON.parse(await readFile(patch, "utf8"));
  assert.equal(
    route.config.providers.openrouter.baseURL,
    "http://gateway/model/ns/ags/v1",
  );
});

test("the deepseek release profile loads with its runtime, model and route", () => {
  const config = loadConfig({
    BEX_AGENT_PROFILE: "deepseek",
    BEX_AGENT_SESSION_ID: "ags-test",
    BEX_AGENT_MODEL_PROXY_URL: "http://gateway/model/ns/ags",
    BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-ags-test",
  });
  assert.equal(config.command, "/usr/local/bin/dsh");
  assert.equal(config.nodeRuntime, "/opt/node24/bin/node");
  assert.deepEqual(config.args, ["--profile", "acp"]);
  assert.equal(config.model, model);
  assert.equal(config.modelBaseUrl, "http://gateway/model/ns/ags/v1");
  assert.equal(config.credentialEnvName, "OPENROUTER_API_KEY");
  assert.equal(config.agentEnv.DSH_PERMISSION_MODE, "danger-full-access");
});
