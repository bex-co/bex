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

// The session's Python (w4/m544). A Render native build picks its Python
// from PYTHON_VERSION, then the repository's .python-version, then Render's
// default for when the service was created; a session picks it the same way,
// from the same table, once both inputs are visible: the environment (where
// a PYTHON_VERSION secret arrives) and the checked-out branch. The driver
// asks agent-toolchain (lego/backend/cmd/agent-toolchain) for the choice and
// points python, python3, pip and pip3 in /opt/bex-python/bin, first on
// every shell's PATH, at that interpreter before any setup step runs. The
// versioned python3.X names stay the image's, /usr/bin/python3 stays
// Debian's, and no UV_PYTHON* variable is set, so a repository's own uv
// settings (python-preference = "only-managed") still decide what uv runs.

import {
  mkdir,
  readdir,
  rename,
  rm,
  symlink,
  writeFile,
} from "node:fs/promises";
import path from "node:path";
import type { UIMessageChunk } from "ai";
import { describeError } from "./errors.js";
import { CommandError, run } from "./proc.js";
import { fetchPython, type FetchRequest } from "./python-fetch.js";
import { RepoSetupError, setupStep } from "./repo-setup.js";

export interface PythonPaths {
  // agent-toolchain. A driver run outside the agent image has none and
  // selects nothing.
  toolchain: string;
  // The driver's own directory: bin/ holds the selection, and an exact
  // patch the image lacks is fetched into <root>/<version>.
  root: string;
  // Where the image bakes each line's reviewed patch as python<line>.
  interpreters: string;
}

export const imagePythonPaths: PythonPaths = {
  toolchain: "/usr/local/bin/agent-toolchain",
  root: "/opt/bex-python",
  interpreters: "/usr/local/bin",
};

export type PythonSource = "PYTHON_VERSION" | ".python-version" | "default";

// PythonChoice is agent-toolchain's answer. image is set when version is not
// the baked patch: the official image to fetch it from.
interface PythonChoice {
  line: string;
  version: string;
  baked: string;
  image?: string;
  source: PythonSource;
  narration: string;
  fallback?: string;
}

export interface PythonSelection {
  version: string;
  source: PythonSource;
  // The transcript step's output.
  narration: string;
}

// A selection that failed: the session's setup fails with it, by name.
export interface PythonFailure {
  error: RepoSetupError;
}

export type PythonOutcome = PythonSelection | PythonFailure;

export interface SelectOptions {
  // undefined outside the agent image (a test's driver): nothing is selected.
  paths?: PythonPaths;
  cwd: string;
  env: Record<string, string>;
  // Tests replace the download.
  fetch?: (request: FetchRequest) => Promise<void>;
}

const toolchainTimeoutMs = 30_000;

// choosePython selects the session's Python, or returns undefined where
// there is no agent-toolchain. It never throws: a failure is the outcome the
// first turn (or the build) reports as its Python step.
export async function choosePython(
  options: SelectOptions,
): Promise<PythonOutcome | undefined> {
  const paths = options.paths;
  if (!paths) return undefined;
  let choice: PythonChoice;
  try {
    choice = JSON.parse(
      await run(paths.toolchain, ["python", options.cwd], {
        env: options.env,
        timeoutMs: toolchainTimeoutMs,
      }),
    ) as PythonChoice;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return undefined;
    // Exit 2 is a request no session can run, named by agent-toolchain.
    if (error instanceof CommandError && error.exitCode === 2)
      return { error: new RepoSetupError(error.message) };
    return { error: selectFailure(error) };
  }
  try {
    return await select(choice, paths, options);
  } catch (error) {
    return { error: selectFailure(error) };
  }
}

function selectFailure(error: unknown): RepoSetupError {
  return new RepoSetupError(
    `could not select the session's Python: ${describeError(error)}`,
  );
}

async function select(
  choice: PythonChoice,
  paths: PythonPaths,
  options: SelectOptions,
): Promise<PythonSelection> {
  let interpreter = path.join(paths.interpreters, `python${choice.line}`);
  let { version, narration } = choice;
  if (choice.image) {
    const prefix = path.join(paths.root, choice.version);
    const exact = path.join(prefix, "bin", `python${choice.line}`);
    try {
      // A sandbox booted from a snapshot that fetched it already has it.
      if (!(await reports(exact, choice.version)))
        await (options.fetch ?? fetchPython)({
          image: choice.image,
          line: choice.line,
          version: choice.version,
          prefix,
          env: options.env,
        });
      interpreter = exact;
    } catch (error) {
      version = choice.baked;
      narration = `${choice.fallback ?? narration}\n==> Could not fetch ${choice.image} (${describeError(error)}); using the reviewed Python ${choice.line} build, ${choice.baked}`;
    }
  }
  await pointNames(path.join(paths.root, "bin"), interpreter, choice.line);
  return { version, source: choice.source, narration };
}

async function reports(python: string, version: string): Promise<boolean> {
  try {
    return (await run(python, ["--version"])).trim() === `Python ${version}`;
  } catch {
    return false;
  }
}

// pointNames makes bin/ hold exactly the selection's names: python, python3
// and python<line> link to the interpreter, and pip, pip3 and pip<line> run
// its pip, which works for a fetched patch too, whose own pip scripts name
// the image's interpreter. Each name is replaced atomically, so a shell
// never finds it missing.
async function pointNames(
  bin: string,
  interpreter: string,
  line: string,
): Promise<void> {
  await mkdir(bin, { recursive: true });
  const names = new Set<string>();
  for (const name of ["python", "python3", `python${line}`]) {
    const temp = path.join(bin, `.${name}.${process.pid}`);
    await rm(temp, { force: true });
    await symlink(interpreter, temp);
    await rename(temp, path.join(bin, name));
    names.add(name);
  }
  const pip = `#!/bin/sh\nexec '${interpreter}' -m pip "$@"\n`;
  for (const name of ["pip", "pip3", `pip${line}`]) {
    const temp = path.join(bin, `.${name}.${process.pid}`);
    await writeFile(temp, pip, { mode: 0o755 });
    await rename(temp, path.join(bin, name));
    names.add(name);
  }
  for (const entry of await readdir(bin))
    if (!names.has(entry)) await rm(path.join(bin, entry), { force: true });
}

// pythonStep writes the selection into a turn's stream as the step before
// the repository's setup (or the agent): `setup: Python`, whose output is
// the choice and its source. A failure fails the setup by name.
export function pythonStep(
  outcome: PythonOutcome,
  turn: number,
  write: (chunk: UIMessageChunk) => void,
): void {
  const end = setupStep(write, `bex-python-${turn}`, "Python", {
    step: "Python",
  });
  if ("error" in outcome) {
    end({ errorText: outcome.error.message });
    throw outcome.error;
  }
  end({ output: outcome.narration });
}
