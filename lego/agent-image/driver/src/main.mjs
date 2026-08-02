#!/usr/bin/env node
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

import { createCredentialManager } from "./credentials.mjs";
import { loadConfig } from "./config.mjs";
import { runHeadlessTurn } from "./session.mjs";
import { startDriverServer } from "./server.mjs";
import { UIMessageStreamHub } from "./stream-hub.mjs";

async function main() {
  const config = loadConfig();
  const credentials = createCredentialManager(config);
  const hub = new UIMessageStreamHub();
  const listener = await startDriverServer(config, credentials, hub);

  const shutdown = async () => {
    await credentials.scrubPersistedState();
    credentials.forget();
    await listener.close();
  };
  process.once("SIGTERM", () => void shutdown().then(() => process.exit(0)));
  process.once("SIGINT", () => void shutdown().then(() => process.exit(0)));
  // Local fallback for the HTTP hook used by the lifecycle layer immediately
  // before pause/snapshot. The next driver starts with a fresh OpenBao value.
  process.on("SIGUSR1", () =>
    void credentials.scrubPersistedState().then(() => credentials.forget()),
  );

  if (!config.prompt) return;
  try {
    await runHeadlessTurn(config, credentials, hub);
    if (config.exitAfterTurn) await shutdown();
  } catch (error) {
    console.error(
      credentials.redact(error instanceof Error ? error.message : error),
    );
    await shutdown();
    process.exitCode = 1;
  }
}

await main();
