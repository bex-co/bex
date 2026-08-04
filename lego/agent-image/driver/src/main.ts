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

import { createCredentialManager } from "./credentials.js";
import { loadConfig } from "./config.js";
import { ensureRepo } from "./delivery.js";
import { markTurnFailed, runHeadlessTurn } from "./session.js";
import { startDriverServer } from "./server.js";
import { UIMessageStreamHub } from "./stream-hub.js";

async function main(): Promise<void> {
  const config = loadConfig();
  const credentials = createCredentialManager(config);
  const hub = new UIMessageStreamHub();
  // Live prompt turns (ADR047 D9): a POST /turn runs another turn on the same
  // session, keeping the UI-message stream open. The fire-and-forget path below
  // still runs its single headless turn and closes the stream.
  const runTurn = (prompt: string, onPart: (part: Record<string, unknown>) => void) =>
    runHeadlessTurn(config, credentials, hub, { prompt, closeHub: false, onPart });
  const listener = await startDriverServer(config, credentials, hub, { runTurn });

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
    // Setup phase: check out the session branch (cloning when the workspace is
    // empty) before the agent runs. Package-registry egress is open here; the
    // agent phase then narrows it (ADR047 D5).
    if (config.deliver) await ensureRepo(config);
    await runHeadlessTurn(config, credentials, hub);
    if (config.exitAfterTurn) await shutdown();
  } catch (error) {
    console.error(
      credentials.redact(error instanceof Error ? error.message : error),
    );
    // Guarantee a `failed` status file even when the failure preceded the turn
    // (e.g. the setup clone) — runHeadlessTurn already wrote one on a turn error,
    // so this is idempotent. Scrub the model credential from the workspace now,
    // since the success-path scrub inside runHeadlessTurn never ran.
    try {
      await markTurnFailed(config, credentials, error);
    } catch {
      /* status best-effort; do not mask the original error */
    }
    await credentials.scrubPersistedState();
    credentials.forget();
    // In fire-and-forget mode (exitAfterTurn=0) the Completer finalizes the
    // session by reading this status file through the gateway exec boundary, so
    // the driver must STAY ALIVE serving it — exactly as it does after a
    // successful turn. Exiting here terminated the pod before the Completer could
    // read the failure, stranding the session in `running` forever (w3/m43
    // crash-leg E2E). Only tear down + exit when a one-shot turn was requested.
    if (config.exitAfterTurn) {
      await listener.close();
      process.exitCode = 1;
    }
  }
}

await main();
