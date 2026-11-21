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

// The session's Python (w4/m544): the real agent-toolchain resolver, built
// from lego/backend, chooses it with Render's rules; the driver points
// /opt/bex-python/bin at it, fetches an exact patch the image lacks from a
// fixture registry, falls back by name when it cannot, and shows the choice
// as the first setup step of a session and of a snapshot build.

import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { createServer, type Server } from "node:http";
import {
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  readlink,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { before, test } from "node:test";
import { fileURLToPath } from "node:url";
import { runBuild } from "../src/build.js";
import { imageArch } from "../src/fs-util.js";
import { run } from "../src/proc.js";
import {
  choosePython,
  type PythonPaths,
  type PythonSelection,
  type SelectOptions,
} from "../src/python-select.js";
import { SETUP_FAILED_CODE } from "../src/repo-setup.js";
import { runHeadlessTurn } from "../src/session.js";
import { UIMessageStreamHub } from "../src/stream-hub.js";
import { childEnv, git, seedBareRemote, tempConfig } from "./helpers.js";

const here = path.dirname(fileURLToPath(import.meta.url));
const backend = path.join(here, "..", "..", "..", "backend");
const created = "2026-11-20T09:30:00Z";
let toolchain = "";

before(async () => {
  const out = await mkdtemp(path.join(tmpdir(), "bex-agent-toolchain-"));
  toolchain = path.join(out, "agent-toolchain");
  execFileSync("go", ["build", "-o", toolchain, "./cmd/agent-toolchain"], {
    cwd: backend,
    env: { ...process.env, GOWORK: "off" },
    stdio: ["ignore", "ignore", "inherit"],
  });
});

// selectPython is a selection that must succeed.
async function selectPython(options: SelectOptions): Promise<PythonSelection> {
  const outcome = await choosePython(options);
  assert.ok(outcome && !("error" in outcome), JSON.stringify(outcome));
  return outcome;
}

// fakePython is an interpreter that reports version and answers -m pip, as
// the image's python3.X do.
function fakePython(version: string): string {
  const line = version.split(".").slice(0, 2).join(".");
  return `#!/bin/sh
case "$1" in
--version) echo "Python ${version}" ;;
-m) [ "$2" = pip ] && echo "pip 24.0 from $(dirname "$0")/../lib/python${line}/site-packages/pip (python ${line})" ;;
*) exit 9 ;;
esac
`;
}

// image lays out an agent image: the baked reviewed patches and the
// driver's root, with the real resolver.
async function image(): Promise<PythonPaths & { dir: string }> {
  const dir = await mkdtemp(path.join(tmpdir(), "bex-python-image-"));
  const interpreters = path.join(dir, "usr-local-bin");
  await mkdir(interpreters);
  for (const version of ["3.11.18", "3.12.16", "3.13.18", "3.14.10"]) {
    const file = path.join(
      interpreters,
      `python${version.split(".").slice(0, 2).join(".")}`,
    );
    await writeFile(file, fakePython(version), { mode: 0o755 });
  }
  return {
    dir,
    toolchain,
    root: path.join(dir, "opt-bex-python"),
    interpreters,
  };
}

async function checkout(files: Record<string, string> = {}): Promise<string> {
  const dir = await mkdtemp(path.join(tmpdir(), "bex-python-checkout-"));
  for (const [name, content] of Object.entries(files))
    await writeFile(path.join(dir, name), content);
  return dir;
}

function sessionEnv(
  extra: Record<string, string> = {},
): Record<string, string> {
  return {
    PATH: process.env.PATH ?? "",
    BEX_AGENT_SESSION_CREATED_AT: created,
    ...extra,
  };
}

// selected is what python3, python, pip and pip3 in bin/ report.
async function selected(paths: PythonPaths): Promise<string[]> {
  const bin = path.join(paths.root, "bin");
  return [
    (await run(path.join(bin, "python3"), ["--version"])).trim(),
    (await run(path.join(bin, "python"), ["--version"])).trim(),
    (await run(path.join(bin, "pip"), ["--version"]))
      .trim()
      .replace(/^.* \(/, "("),
    (await run(path.join(bin, "pip3"), ["--version"]))
      .trim()
      .replace(/^.* \(/, "("),
  ];
}

test(".python-version 3.13 puts the table's 3.13 patch first on PATH", async () => {
  const paths = await image();
  const got = await selectPython({
    paths,
    cwd: await checkout({ ".python-version": "3.13\n" }),
    env: sessionEnv(),
  });
  assert.deepEqual(got, {
    version: "3.13.18",
    source: ".python-version",
    narration: "==> Using Python 3.13.18 (from .python-version)",
  });
  assert.deepEqual(await selected(paths), [
    "Python 3.13.18",
    "Python 3.13.18",
    "(python 3.13)",
    "(python 3.13)",
  ]);
  assert.equal(
    await readlink(path.join(paths.root, "bin", "python3")),
    path.join(paths.interpreters, "python3.13"),
  );
});

test("PYTHON_VERSION beats .python-version, and nothing gets Render's dated default", async () => {
  const paths = await image();
  const pinned = await selectPython({
    paths,
    cwd: await checkout({ ".python-version": "3.13\n" }),
    env: sessionEnv({ PYTHON_VERSION: "3.12.16" }),
  });
  assert.equal(
    pinned.narration,
    "==> Using Python 3.12.16 (from PYTHON_VERSION)",
  );
  assert.deepEqual((await selected(paths)).slice(0, 2), [
    "Python 3.12.16",
    "Python 3.12.16",
  ]);
  // Selecting again (a snapshot's sandbox on another branch) leaves only the
  // new selection's names.
  const fallback = await selectPython({
    paths,
    cwd: await checkout(),
    env: sessionEnv(),
  });
  assert.deepEqual(fallback, {
    version: "3.14.10",
    source: "default",
    narration: "==> Using Python 3.14.10 (default)",
  });
  assert.deepEqual((await readdir(path.join(paths.root, "bin"))).sort(), [
    "pip",
    "pip3",
    "pip3.14",
    "python",
    "python3",
    "python3.14",
  ]);
});

test("an unsupported request fails setup by name, and selects nothing", async () => {
  const paths = await image();
  for (const [env, files, want] of [
    [
      { PYTHON_VERSION: "2.7.18" },
      {},
      /^PYTHON_VERSION=2\.7\.18 is not a supported Python version; supported Python lines are 3\.7, 3\.8/,
    ],
    [
      {},
      { ".python-version": "3.7.17\n" },
      /^\.python-version is not a supported Python version in an agent session: Python 3\.7's official build is for Debian bullseye/,
    ],
  ] as const) {
    const outcome = await choosePython({
      paths,
      cwd: await checkout(files),
      env: sessionEnv(env),
    });
    assert.ok(outcome && "error" in outcome, "the request was not refused");
    assert.equal(outcome.error.code, SETUP_FAILED_CODE);
    assert.match(outcome.error.message, want);
  }
  await assert.rejects(readdir(path.join(paths.root, "bin")), /ENOENT/);
});

test("outside the agent image there is no resolver, and nothing is selected", async () => {
  const paths = {
    ...(await image()),
    toolchain: "/nonexistent/agent-toolchain",
  };
  assert.equal(
    await choosePython({ paths, cwd: await checkout(), env: sessionEnv() }),
    undefined,
  );
  assert.equal(
    await choosePython({ cwd: await checkout(), env: sessionEnv() }),
    undefined,
  );
  await assert.rejects(readdir(path.join(paths.root, "bin")), /ENOENT/);
});

// fixtureRegistry serves python:<tag> the way Docker Hub does: a token, an
// index, a manifest and gzipped layers, the interpreter's layer below a
// symlink layer and a base layer that must never be downloaded.
async function fixtureRegistry(tag: string, version: string) {
  const line = version.split(".").slice(0, 2).join(".");
  const work = await mkdtemp(path.join(tmpdir(), "bex-python-registry-"));
  const layer = async (name: string, files: Record<string, string>) => {
    const dir = path.join(work, name);
    for (const [file, content] of Object.entries(files)) {
      await mkdir(path.dirname(path.join(dir, file)), { recursive: true });
      await writeFile(path.join(dir, file), content, {
        mode: file.includes("/bin/") ? 0o755 : 0o644,
      });
    }
    const out = path.join(work, `${name}.tgz`);
    execFileSync("tar", ["-czf", out, "-C", dir, "."]);
    const bytes = await readFile(out);
    return {
      bytes,
      digest: `sha256:${createHash("sha256").update(bytes).digest("hex")}`,
    };
  };
  const layers = [
    await layer("base", { "etc/os-release": "bookworm\n" }),
    await layer("python", {
      [`usr/local/bin/python${line}`]: fakePython(version),
      [`usr/local/bin/pip${line}`]: "#!/usr/local/bin/python\n",
      [`usr/local/lib/libpython${line}.so.1.0`]: "elf\n",
      [`usr/local/lib/python${line}/os.py`]: "# os\n",
      [`usr/local/lib/python${line}/site-packages/pip/__init__.py`]: "",
      [`usr/local/include/python${line}/Python.h`]: "",
    }),
    await layer("links", { "usr/local/bin/python3": "python3 link\n" }),
  ];
  const manifest = Buffer.from(
    JSON.stringify({ layers: layers.map(({ digest }) => ({ digest })) }),
  );
  const manifestDigest = `sha256:${createHash("sha256").update(manifest).digest("hex")}`;
  const index = JSON.stringify({
    manifests: [
      {
        digest: "sha256:other",
        platform: { os: "linux", architecture: "s390x" },
      },
      {
        digest: manifestDigest,
        platform: { os: "linux", architecture: imageArch },
      },
    ],
  });
  const served: string[] = [];
  const server: Server = createServer((req, res) => {
    const url = req.url ?? "";
    served.push(url);
    if (url.startsWith("/token?"))
      return res.end(JSON.stringify({ token: "fixture-token" }));
    if (req.headers.authorization !== "Bearer fixture-token")
      return res.writeHead(401).end();
    if (url === `/v2/library/python/manifests/${tag}`) return res.end(index);
    if (url === `/v2/library/python/manifests/${manifestDigest}`)
      return res.end(manifest);
    const blob = layers.find(
      ({ digest }) => url === `/v2/library/python/blobs/${digest}`,
    );
    if (blob) return res.end(blob.bytes);
    res.writeHead(404).end();
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address() as { port: number };
  return {
    registry: {
      url: `http://127.0.0.1:${port}`,
      tokenUrl: `http://127.0.0.1:${port}/token?service=fixture`,
    },
    served,
    baseDigest: layers[0].digest,
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
  };
}

test("PYTHON_VERSION=3.11.9 fetches that patch, and its pip runs on it", async (t) => {
  const paths = await image();
  const fixture = await fixtureRegistry("3.11.9-bookworm", "3.11.9");
  t.after(fixture.close);
  const select = () =>
    selectPython({
      paths,
      cwd: checkoutDir,
      env: sessionEnv({ PYTHON_VERSION: "3.11.9" }),
      fetch: (request) =>
        import("../src/python-fetch.js").then(({ fetchPython }) =>
          fetchPython({ ...request, registry: fixture.registry }),
        ),
    });
  const checkoutDir = await checkout();
  const got = await select();
  assert.deepEqual(got, {
    version: "3.11.9",
    source: "PYTHON_VERSION",
    narration: "==> Using Python 3.11.9 (from PYTHON_VERSION)",
  });
  assert.deepEqual(await selected(paths), [
    "Python 3.11.9",
    "Python 3.11.9",
    "(python 3.11)",
    "(python 3.11)",
  ]);
  const prefix = path.join(paths.root, "3.11.9");
  assert.equal(
    await readlink(path.join(paths.root, "bin", "python3.11")),
    path.join(prefix, "bin", "python3.11"),
  );
  assert.deepEqual((await readdir(prefix)).sort(), ["bin", "include", "lib"]);
  assert.deepEqual((await readdir(path.join(prefix, "lib"))).sort(), [
    "libpython3.11.so.1.0",
    "python3.11",
  ]);
  assert.ok(
    !fixture.served.includes(`/v2/library/python/blobs/${fixture.baseDigest}`),
    "the base layer was downloaded",
  );
  assert.deepEqual(
    (await readdir(paths.root)).filter((name) => name.startsWith(".")),
    [],
    "the download left its work directory",
  );
  // A sandbox that already has the patch (a snapshot that fetched it) does
  // not download it again.
  const before = fixture.served.length;
  await select();
  assert.equal(fixture.served.length, before);
});

test("an exact patch that cannot be fetched falls back to the line's patch, saying why", async () => {
  const paths = await image();
  const got = await selectPython({
    paths,
    cwd: await checkout(),
    env: sessionEnv({ PYTHON_VERSION: "3.11.9" }),
    fetch: (request) =>
      import("../src/python-fetch.js").then(({ fetchPython }) =>
        fetchPython({
          ...request,
          registry: {
            url: "http://127.0.0.1:1",
            tokenUrl: "http://127.0.0.1:1/token?service=x",
          },
        }),
      ),
  });
  assert.equal(got.version, "3.11.18");
  const [step, why] = got.narration.split("\n");
  assert.equal(step, "==> Using Python 3.11.18 (from PYTHON_VERSION)");
  assert.match(
    why,
    /^==> Could not fetch python:3\.11\.9-bookworm \(curl: \(7\) .+\); using the reviewed Python 3\.11 build, 3\.11\.18$/,
  );
  assert.deepEqual((await selected(paths)).slice(0, 2), [
    "Python 3.11.18",
    "Python 3.11.18",
  ]);
});

// The turn shows the selection as its first step, before the repository's
// setup, and a setup step's python3 is the selection.
test("a session's first turn shows its Python before setup, and setup runs on it", async () => {
  const paths = await image();
  const config = await tempConfig({
    repoEnvironment: {
      // uv decides for itself: no UV_PYTHON* variable reaches a step, so a
      // repository's python-preference = "only-managed" still holds.
      setup: [
        {
          name: "which",
          run: "python3 --version > .python-seen; env | grep '^UV_PYTHON' > .uv-env || true",
        },
      ],
      checks: [],
    },
  });
  await writeFile(path.join(config.cwd, ".python-version"), "3.13\n");
  config.python = (await choosePython({
    paths,
    cwd: config.cwd,
    env: sessionEnv(),
  })) as PythonSelection;
  // The image's PATH starts with /opt/bex-python/bin.
  config.agentEnv = {
    PATH: `${path.join(paths.root, "bin")}:${process.env.PATH}`,
  };
  const hub = new UIMessageStreamHub();
  const result = await runHeadlessTurn(config, childEnv(config), hub);
  assert.equal(result.state, "succeeded");
  const python = hub.history.findIndex(
    (part) =>
      part.toolCallId === "bex-python-1" &&
      part.type === "tool-output-available",
  );
  const setup = hub.history.findIndex(
    (part) => part.toolCallId === "bex-setup-1-1",
  );
  assert.ok(
    python >= 0 && python < setup,
    "the Python step did not come first",
  );
  assert.equal(
    hub.history[python].output,
    "==> Using Python 3.13.18 (from .python-version)",
  );
  assert.equal(
    (await readFile(path.join(config.cwd, ".python-seen"), "utf8")).trim(),
    "Python 3.13.18",
  );
  assert.equal(await readFile(path.join(config.cwd, ".uv-env"), "utf8"), "");
  // A second turn in the same sandbox does not repeat the step.
  const hub2 = new UIMessageStreamHub();
  await runHeadlessTurn(config, childEnv(config), hub2, { turn: 2 });
  assert.ok(
    !hub2.history.some((part) =>
      String(part.toolCallId ?? "").startsWith("bex-python-"),
    ),
  );
});

test("a refused Python fails the turn AGENT_SESSION_SETUP_FAILED before setup or the agent", async () => {
  const paths = await image();
  const config = await tempConfig({
    repoEnvironment: {
      setup: [{ name: "deps", run: "touch .setup-ran" }],
      checks: [],
    },
  });
  config.python = await choosePython({
    paths,
    cwd: config.cwd,
    env: sessionEnv({ PYTHON_VERSION: "2.7.18" }),
  });
  const agentLog = path.join(config.root, "agent.log");
  config.agentEnv = { ACP_FIXTURE_LOG: agentLog };
  const hub = new UIMessageStreamHub();
  await assert.rejects(
    runHeadlessTurn(config, childEnv(config), hub),
    /PYTHON_VERSION=2\.7\.18 is not a supported Python version/,
  );
  const status = JSON.parse(await readFile(config.statusPath, "utf8"));
  assert.equal(status.state, "failed");
  assert.equal(status.code, SETUP_FAILED_CODE);
  assert.match(status.error, /PYTHON_VERSION=2\.7\.18/);
  assert.ok(
    hub.history.some(
      (part) =>
        part.toolCallId === "bex-python-1" && part.type === "tool-output-error",
    ),
  );
  await assert.rejects(
    readFile(path.join(config.cwd, ".setup-ran")),
    "setup ran after a refused Python",
  );
  await assert.rejects(
    readFile(agentLog),
    "the agent started after a refused Python",
  );
});

test("a snapshot build selects Python before its steps and records it as the first step", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "bex-python-build-"));
  const remote = await seedBareRemote(root);
  const seed = path.join(root, "seed");
  await writeFile(path.join(seed, ".python-version"), "3.12\n");
  git(seed, ["add", "-A"]);
  git(seed, ["commit", "-m", "python"]);
  git(seed, ["push", remote, "main"]);
  const paths = await image();
  await mkdir(path.join(root, "workspace"));
  await mkdir(path.join(root, "workspace-2"));
  const base = {
    cwd: path.join(root, "workspace"),
    sessionLogPath: path.join(root, "session.jsonl"),
    statusPath: path.join(root, "status.json"),
    prRecordPath: path.join(root, "pull-request.json"),
    resultRecordPath: path.join(root, "result.json"),
    repoUrl: remote,
    repository: "acme/widgets",
    secretValues: new Map<string, string>(),
    pythonPaths: paths,
    repoEnvironment: {
      setup: [
        {
          name: "seen",
          run: `PATH="${path.join(paths.root, "bin")}:$PATH" python3 --version`,
        },
      ],
      checks: [],
    },
  };
  const status = await runBuild(base, {
    env: sessionEnv(),
    cloneRetry: { attempts: 1 },
  });
  assert.equal(status.state, "succeeded", JSON.stringify(status));
  assert.deepEqual(
    status.build.steps.map(({ name, exitCode, output }) => [
      name,
      exitCode,
      output.trim(),
    ]),
    [
      ["Python", 0, "==> Using Python 3.12.16 (from .python-version)"],
      ["seen", 0, "Python 3.12.16"],
    ],
  );
  const refused = await runBuild(
    {
      ...base,
      cwd: path.join(root, "workspace-2"),
      statusPath: path.join(root, "status-2.json"),
    },
    {
      env: sessionEnv({ PYTHON_VERSION: "2.7.18" }),
      cloneRetry: { attempts: 1 },
    },
  );
  assert.equal(refused.state, "failed");
  assert.ok(refused.state === "failed" && refused.code === "setup_failed");
  assert.match(
    refused.error,
    /PYTHON_VERSION=2\.7\.18 is not a supported Python version/,
  );
  assert.deepEqual(
    refused.build.steps.map(({ name, exitCode }) => [name, exitCode]),
    [["Python", 1]],
  );
});
