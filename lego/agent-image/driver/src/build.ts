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

// A snapshot build (ADR111 D1). The driver clones the repository's default
// branch, runs its setup steps, removes its own per-run state and records
// each step's result in the status file. bex-api then commits the sandbox's
// root filesystem as the image sessions boot from. No agent runs.

import { rm } from "node:fs/promises";
import type { BuildConfig } from "./config.js";
import { describeError } from "./errors.js";
import { writeJsonAtomic } from "./fs-util.js";
import { cloneWithRetry, git, scrubBuildCheckout } from "./git-setup.js";
import { SecretRedactor } from "./redact.js";
import {
  lastBytes,
  OUTPUT_TAIL_BYTES,
  RepoSetupError,
  runRepoSetup,
  SUCCESS_TAIL_BYTES,
  type SetupStepResult,
} from "./repo-setup.js";

// Devin's per-step limit (ADR111 D5). The whole build's 3 hours are bex-api's
// to enforce: it terminates the builder and records build_timeout.
export const BUILD_STEP_TIMEOUT_MS = 3 * 60 * 60_000;

// ADR111 D2's codes the driver reports. internal_error is a failure of the
// driver itself, outside the clone and the steps.
export type BuildFailureCode =
  | "checkout_failed"
  | "setup_failed"
  | "step_timeout"
  | "internal_error";

export interface BuildStep {
  name: string;
  // null when the step was killed at a limit or never started.
  exitCode: number | null;
  durationMs: number;
  // The redacted tail: 2 KiB for a step that passed, 8 KiB for the failing one.
  output: string;
}

export type BuildStatus =
  | { state: "succeeded"; build: { head: string; steps: BuildStep[] } }
  | {
      state: "failed";
      code: BuildFailureCode;
      error: string;
      build: { steps: BuildStep[] };
    };

export interface BuildOptions {
  // The steps' environment; the driver's own by default, which carries the
  // build's secrets (ADR111 D6).
  env?: Record<string, string>;
  abortSignal?: AbortSignal;
  stepTimeoutMs?: number;
  cloneRetry?: { attempts?: number; delayMs?: number };
}

class BuildFailure extends Error {
  constructor(
    readonly code: BuildFailureCode,
    message: string,
  ) {
    super(message);
  }
}

const repositoryPattern = /^[A-Za-z0-9-]+\/[A-Za-z0-9._-]+$/;

// runBuild runs one build and writes its final status. It never throws for
// the build's own failure, only when the status itself cannot be written.
export async function runBuild(
  config: BuildConfig,
  options: BuildOptions = {},
): Promise<BuildStatus> {
  const redactor = new SecretRedactor(config.secretValues);
  const steps: BuildStep[] = [];
  let status: BuildStatus;
  try {
    await checkout(config, options);
    let last: SetupStepResult | undefined;
    try {
      await runRepoSetup(config.repoEnvironment ?? { setup: [], checks: [] }, {
        cwd: config.cwd,
        // A build publishes no transcript, so its tool calls are unused.
        turn: 0,
        env: options.env ?? ({ ...process.env } as Record<string, string>),
        write: () => undefined,
        heartbeat: () => undefined,
        abortSignal: options.abortSignal,
        stepTimeoutMs: options.stepTimeoutMs ?? BUILD_STEP_TIMEOUT_MS,
        totalTimeoutMs: Number.POSITIVE_INFINITY,
        onStep: (result) => {
          last = result;
          steps.push(recordStep(result, redactor));
        },
      });
    } catch (error) {
      if (!(error instanceof RepoSetupError)) throw error;
      throw new BuildFailure(
        last?.timedOut ? "step_timeout" : "setup_failed",
        error.message,
      );
    }
    const head = await git(config.cwd, ["rev-parse", "HEAD"]);
    await scrubBuildCheckout(config.cwd, config.repository);
    status = { state: "succeeded", build: { head, steps } };
  } catch (error) {
    status = {
      state: "failed",
      code: error instanceof BuildFailure ? error.code : "internal_error",
      error: redactor.redactString(describeError(error)),
      build: { steps },
    };
  }
  await removeDriverState(config);
  await writeJsonAtomic(config.statusPath, status);
  return status;
}

async function checkout(
  config: BuildConfig,
  options: BuildOptions,
): Promise<void> {
  if (!config.repoUrl) {
    throw new BuildFailure(
      "checkout_failed",
      "BEX_AGENT_REPO_URL is required for a build",
    );
  }
  // Checked before the clone: it becomes origin's URL in the image.
  if (!repositoryPattern.test(config.repository)) {
    throw new BuildFailure(
      "checkout_failed",
      "BEX_AGENT_REPOSITORY must name the repository as owner/name",
    );
  }
  try {
    // Sessions fetch any other commit they need by its id.
    await cloneWithRetry(config.cwd, config.repoUrl, {
      flags: ["--single-branch", "--no-tags"],
      ...options.cloneRetry,
    });
  } catch (error) {
    throw new BuildFailure(
      "checkout_failed",
      `could not clone the repository: ${describeError(error)}`,
    );
  }
}

// recordStep redacts the whole kept output before tailing it, so a value the
// tail would cut cannot leave a fragment.
function recordStep(
  result: SetupStepResult,
  redactor: SecretRedactor,
): BuildStep {
  return {
    name: result.name,
    exitCode: result.exitCode,
    durationMs: result.durationMs,
    output: lastBytes(
      redactor.redactString(result.output),
      result.failure ? OUTPUT_TAIL_BYTES : SUCCESS_TAIL_BYTES,
    ),
  };
}

// removeDriverState deletes the driver's own files so the image carries none
// into a session. A build writes no turn log or pull request record; one left
// by anything else in the sandbox goes too.
async function removeDriverState(config: BuildConfig): Promise<void> {
  for (const file of [
    config.sessionLogPath,
    config.prRecordPath,
    config.resultRecordPath,
  ]) {
    await rm(file, { force: true });
  }
}
