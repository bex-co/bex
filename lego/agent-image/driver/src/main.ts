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
import { ensureRepo, restoreWorkspace } from "./delivery.js";
import { describeError } from "./errors.js";
import { markTurnFailed, runHeadlessTurn } from "./session.js";
import { startDriverServer } from "./server.js";
import { UIMessageStreamHub } from "./stream-hub.js";

async function main(): Promise<void> {
  const config = loadConfig();
  const credentials = createCredentialManager(config);
  const hub = new UIMessageStreamHub();
  let terminalized = false;
  let activeAbort: AbortController | undefined;
  let nextTurn = config.turn;
  let initialTurnOwned = false;
  let activeTurn:
    Promise<Awaited<ReturnType<typeof runHeadlessTurn>>> | undefined;
  // Capture the control-plane Git proxy's immutable base/session OIDs before a
  // tenant process can run. Live POST /turn calls await the same setup promise.
  const setup = (async () => {
    await restoreWorkspace(config);
    if (config.deliver) await ensureRepo(config);
  })();
  // main awaits setup below; attaching immediately also prevents a fast setup
  // rejection from becoming an unhandled promise while the listener starts.
  void setup.catch(() => undefined);
  // Live prompt turns (ADR047 D9): a POST /turn runs another turn on the same
  // session, keeping the UI-message stream open. The fire-and-forget path below
  // still runs its single headless turn and closes the stream.
  const controlledTurn = (
    prompt: string,
    onPart: (part: Record<string, unknown>) => void,
    closeHub: boolean,
  ) => {
    if (terminalized) throw new Error("agent session is terminalizing");
    const controller = new AbortController();
    const turnNumber = nextTurn;
    nextTurn += 1;
    activeAbort = controller;
    const turn = runHeadlessTurn(config, credentials, hub, {
      prompt,
      turn: turnNumber,
      closeHub,
      onPart,
      abortSignal: controller.signal,
    });
    activeTurn = turn;
    void turn
      .finally(() => {
        if (activeTurn === turn) activeTurn = undefined;
        if (activeAbort === controller) activeAbort = undefined;
      })
      .catch(() => undefined);
    return turn;
  };
  const runTurn = async (
    prompt: string,
    onPart: (part: Record<string, unknown>) => void,
  ) => {
    await setup;
    try {
      return await controlledTurn(prompt, onPart, false);
    } catch (error) {
      terminalized = true;
      throw error;
    }
  };
  const terminalize = async () => {
    terminalized = true;
    activeAbort?.abort();
    try {
      await activeTurn;
    } catch {
      // Expected when snapshot preparation interrupts an active agent. The
      // turn's finally block reaps the child before this promise settles.
    }
  };
  const listener = await startDriverServer(config, credentials, hub, {
    runTurn,
    terminalize,
  });

  const shutdown = async (scrub = true) => {
    try {
      await terminalize();
      if (scrub) await credentials.scrubPersistedState();
    } finally {
      credentials.forget();
      await listener.close();
    }
  };
  const signalShutdown = () =>
    void shutdown().then(
      () => process.exit(0),
      (error) => {
        console.error(describeError(error));
        process.exit(1);
      },
    );
  process.once("SIGTERM", signalShutdown);
  process.once("SIGINT", signalShutdown);
  try {
    // Setup phase: rehydrate a hibernation snapshot when resuming (ADR059 D4),
    // then check out the session branch (cloning only when the workspace is still
    // empty — a restored workspace is left intact) before the agent runs.
    // Package-registry egress is open here; the agent phase then narrows it (D5).
    await setup;
    if (!config.prompt) return;
    // codex #9: hold the single-flight guard during the initial headless turn so
    // a concurrent POST /turn gets 409 instead of starting a second agent against
    // the same checkout, transcript, credentials, and Git branch.
    listener.setTurnInFlight(true);
    initialTurnOwned = true;
    await controlledTurn(config.prompt, () => undefined, true);
    listener.setTurnInFlight(false);
    // runHeadlessTurn already stopped the provider and scrubbed immediately
    // before delivery, so a one-shot success can reuse that clean verdict.
    if (config.exitAfterTurn) await shutdown(false);
  } catch (error) {
    // Release the single-flight guard on any error so the server can accept
    // subsequent turns (codex #9).
    listener.setTurnInFlight(false);
    terminalized = true;
    console.error(credentials.redact(describeError(error)));
    // Setup failures need the shared scrub/status/forget path. Once a turn owns
    // the failure, runHeadlessTurn has already persisted that single verdict.
    if (!initialTurnOwned) {
      try {
        await markTurnFailed(config, credentials, error);
      } catch {
        /* status best-effort; do not mask the original error */
      }
    }
    // Fire-and-forget keeps serving the status file until the Completer observes
    // it. Only a one-shot turn tears down the listener here.
    if (config.exitAfterTurn) {
      await listener.close();
      process.exitCode = 1;
    }
  }
}

await main();
