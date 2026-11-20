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
import { spawn } from "node:child_process";
import { mkdir, readFile, stat, writeFile } from "node:fs/promises";
import http from "node:http";
import type { AddressInfo } from "node:net";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { loadConfig } from "../src/config.js";
import {
  githubCliEnabled,
  githubCliEnvironment,
  githubCliPaths,
  githubCliTokenPlaceholder,
  removeGitHubCliConfig,
  startGitHubCliProxy,
} from "../src/github-cli.js";
import { platformOverlay } from "../src/overlay.js";
import { childEnv, tempConfig } from "./helpers.js";

interface Seen {
  method?: string;
  url?: string;
  authorization?: string;
  accept?: string;
  body: string;
}

// fakeGateway answers like the gateway's github-api route and records the
// request it got.
async function fakeGateway(
  t: test.TestContext,
): Promise<{ url: string; seen: Seen[] }> {
  const seen: Seen[] = [];
  const server = http.createServer((request, response) => {
    const chunks: Buffer[] = [];
    request.on("data", (chunk: Buffer) => chunks.push(chunk));
    request.on("end", () => {
      seen.push({
        method: request.method,
        url: request.url,
        authorization: request.headers.authorization,
        accept: request.headers.accept,
        body: Buffer.concat(chunks).toString(),
      });
      response.writeHead(200, {
        "content-type": "application/json",
        link: '<https://api.github.com/repos/acme/widgets/actions/runs?page=2>; rel="next"',
      });
      response.end('{"total_count":3}');
    });
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => server.close());
  const { port } = server.address() as AddressInfo;
  return { url: `http://127.0.0.1:${port}/git/bnM/c2lk/cmVwbw/YnJhbmNo`, seen };
}

// overSocket sends one request the way gh does with http_unix_socket set:
// plain HTTP on the socket, the GitHub host in the Host header.
function overSocket(
  socketPath: string,
  options: { method?: string; path: string; host?: string; body?: string },
): Promise<{
  status: number;
  headers: http.IncomingHttpHeaders;
  body: string;
}> {
  return new Promise((resolve, reject) => {
    const request = http.request(
      {
        socketPath,
        method: options.method ?? "GET",
        path: options.path,
        headers: {
          host: options.host ?? "api.github.com",
          authorization: `token ${githubCliTokenPlaceholder}`,
          accept: "application/vnd.github+json",
        },
      },
      (response) => {
        const chunks: Buffer[] = [];
        response.on("data", (chunk: Buffer) => chunks.push(chunk));
        response.on("end", () =>
          resolve({
            status: response.statusCode ?? 0,
            headers: response.headers,
            body: Buffer.concat(chunks).toString(),
          }),
        );
      },
    );
    request.on("error", reject);
    request.end(options.body);
  });
}

test("a repo session's agent gets gh's settings with only a placeholder token; a repo-less one gets none", async () => {
  const config = await tempConfig({
    deliver: true,
    repoUrl: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo",
    branch: "bex-agent/ags-abc",
    repository: "acme/widgets",
  });
  const env = childEnv(config);
  assert.equal(env.GH_CONFIG_DIR, path.join(config.root, "gh"));
  assert.equal(env.GH_TOKEN, githubCliTokenPlaceholder);
  assert.equal(env.GH_REPO, "acme/widgets");
  assert.equal(env.GH_PROMPT_DISABLED, "1");
  assert.equal(env.GH_NO_UPDATE_NOTIFIER, "1");
  assert.doesNotMatch(env.GH_TOKEN, /^(ghs_|ghp_|gho_|github_pat_)/);

  const scratch = childEnv(await tempConfig());
  assert.deepEqual(
    Object.keys(scratch).filter((name) => name.startsWith("GH_")),
    [],
  );
  const noRepository = childEnv(
    await tempConfig({ deliver: true, repoUrl: config.repoUrl }),
  );
  assert.equal(noRepository.GH_TOKEN, undefined);
});

test("gh's socket forwards api.github.com calls to the gateway route without the placeholder", async (t) => {
  const gateway = await fakeGateway(t);
  const config = await tempConfig({
    deliver: true,
    repoUrl: gateway.url,
    repository: "acme/widgets",
  });
  const proxy = await startGitHubCliProxy(config);
  t.after(() => proxy.close());
  const { configDir, socketPath } = githubCliPaths(config);

  const configFile = await readFile(path.join(configDir, "config.yml"), "utf8");
  assert.match(
    configFile,
    new RegExp(`^http_unix_socket: ${socketPath}$`, "m"),
  );
  assert.equal((await stat(socketPath)).mode & 0o777, 0o600);

  const read = await overSocket(socketPath, {
    path: "/repos/acme/widgets/actions/runs?per_page=3",
  });
  assert.equal(read.status, 200);
  assert.equal(read.body, '{"total_count":3}');
  assert.match(String(read.headers.link), /rel="next"/);
  assert.equal(
    gateway.seen[0].url,
    "/git/bnM/c2lk/cmVwbw/YnJhbmNo/github-api/repos/acme/widgets/actions/runs?per_page=3",
  );
  assert.equal(gateway.seen[0].authorization, undefined);
  assert.equal(gateway.seen[0].accept, "application/vnd.github+json");

  const query = '{"query":"{ rateLimit { remaining } }"}';
  const graphql = await overSocket(socketPath, {
    method: "POST",
    path: "/graphql",
    body: query,
  });
  assert.equal(graphql.status, 200);
  assert.equal(gateway.seen[1].method, "POST");
  assert.equal(
    gateway.seen[1].url,
    "/git/bnM/c2lk/cmVwbw/YnJhbmNo/github-api/graphql",
  );
  assert.equal(gateway.seen[1].body, query);

  // w10/m522: a REST write reaches the route with its method and body; the
  // gateway decides whether it is admitted.
  const comment = '{"body":"edited"}';
  const write = await overSocket(socketPath, {
    method: "PATCH",
    path: "/repos/acme/widgets/issues/comments/9",
    body: comment,
  });
  assert.equal(write.status, 200);
  assert.equal(gateway.seen[2].method, "PATCH");
  assert.equal(
    gateway.seen[2].url,
    "/git/bnM/c2lk/cmVwbw/YnJhbmNo/github-api/repos/acme/widgets/issues/comments/9",
  );
  assert.equal(gateway.seen[2].body, comment);
  assert.equal(gateway.seen[2].authorization, undefined);

  // gh follows a redirect over the same socket; anything not addressed to
  // api.github.com is refused here and never reaches the gateway.
  const elsewhere = await overSocket(socketPath, {
    path: "/actions-results/x",
    host: "productionresultssa7.blob.core.windows.net",
  });
  assert.equal(elsewhere.status, 403);
  assert.match(JSON.parse(elsewhere.body).message, /only api\.github\.com/);
  assert.equal(gateway.seen.length, 3);
});

// The image's /etc/profile.d/bex-agent-tools.sh, which an interactive sh (the
// web Shell's and SSH's /bin/sh) reads through ENV and every bash through
// BASH_ENV.
const profileScript = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
  "bex-agent-tools.sh",
);

// terminal runs a command the way a session's terminal does, with only the
// environment the image gives every process, and returns its output. Its
// stdin is not a socket, as a terminal's is not: bash reads no BASH_ENV when
// stdin is one, which is what a Node pipe is.
function terminal(
  statusPath: string,
  cwd: string,
  shell: string[],
  command: string,
): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(shell[0], [...shell.slice(1), command], {
      cwd,
      env: {
        PATH: process.env.PATH ?? "/usr/bin:/bin",
        HOME: cwd,
        ENV: profileScript,
        BASH_ENV: profileScript,
        BEX_AGENT_STATUS_FILE: statusPath,
      },
      stdio: ["ignore", "pipe", "ignore"],
    });
    let stdout = "";
    child.stdout.on("data", (chunk: Buffer) => (stdout += chunk));
    child.on("error", reject);
    child.on("close", (code) =>
      code === 0
        ? resolve(stdout)
        : reject(new Error(`${shell.join(" ")} ${command} exited ${code}`)),
    );
  });
}

function ghVariables(env: string): Record<string, string> {
  return Object.fromEntries(
    env
      .split("\n")
      .filter((line) => line.startsWith("GH_"))
      .map((line) => [
        line.slice(0, line.indexOf("=")),
        line.slice(line.indexOf("=") + 1),
      ]),
  );
}

// w10/027: gh in the web Shell, an SSH session or Zed works as the agent's
// does, and a session whose gh is not configured has none in its terminals.
test("a repo session's terminals get the agent's gh settings, and gh in them reaches the gateway route", async (t) => {
  const gateway = await fakeGateway(t);
  const base = await tempConfig({
    deliver: true,
    repoUrl: gateway.url,
    repository: "acme/widgets",
  });
  // A quote in the path proves each value is quoted for the shell.
  const config = {
    ...base,
    statusPath: path.join(base.root, "it's", "status.json"),
  };
  const proxy = await startGitHubCliProxy(config);
  t.after(() => proxy.close());
  const { shellEnvPath } = githubCliPaths(config);
  assert.equal((await stat(shellEnvPath)).mode & 0o777, 0o600);

  const expected = githubCliEnvironment(config);
  assert.equal(expected.GH_REPO, "acme/widgets");
  for (const shell of [
    ["sh", "-ic"],
    ["bash", "-c"],
  ]) {
    assert.deepEqual(
      ghVariables(await terminal(config.statusPath, config.cwd, shell, "env")),
      expected,
      `${shell.join(" ")} has the agent's gh settings`,
    );
  }

  const hasGh = await terminal(
    config.statusPath,
    config.cwd,
    ["sh", "-c"],
    "command -v gh",
  ).then(
    () => true,
    () => false,
  );
  if (hasGh) {
    const answer = await terminal(
      config.statusPath,
      config.cwd,
      ["sh", "-ic"],
      "gh api repos/{owner}/{repo}",
    );
    assert.equal(answer.trim(), '{"total_count":3}');
    assert.equal(
      gateway.seen.at(-1)?.url,
      "/git/bnM/c2lk/cmVwbw/YnJhbmNo/github-api/repos/acme/widgets",
    );
    assert.equal(gateway.seen.at(-1)?.authorization, undefined);
  } else {
    t.diagnostic(
      "gh is not installed here; its call through the socket is not exercised",
    );
  }

  // gh off, or a later turn without a repository: the driver removes the file.
  await removeGitHubCliConfig(config);
  for (const shell of [
    ["sh", "-ic"],
    ["bash", "-c"],
  ]) {
    assert.deepEqual(
      ghVariables(await terminal(config.statusPath, config.cwd, shell, "env")),
      {},
    );
  }
});

test("the repo overlay says what gh reads and writes, and that pull requests stay with the bex tools", () => {
  const context = {
    cwd: "/workspace",
    deliver: true,
    repoUrl: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo",
    branch: "bex-agent/ags-abc",
    baseBranch: "",
    repository: "acme/widgets",
    deliveryBaseline: { baseBranch: "main" },
  };
  const text = platformOverlay(context);
  assert.match(
    text,
    /There is no GitHub token in the sandbox, but `gh` works on `acme\/widgets` through bex/,
  );
  assert.match(text, /`gh run view --log-failed`/);
  assert.match(
    text,
    /reviews without approving \(`gh pr review --comment` or `--request-changes`\), adds labels, assignees and reviewers/,
  );
  assert.match(
    text,
    /opens, edits and closes issues, and re-runs or cancels CI \(`gh run rerun --failed`, `gh run cancel`\)/,
  );
  assert.match(
    text,
    /It cannot open, edit, approve, merge or close a pull request, dispatch a workflow, or change code, branches or settings, and it reaches no other repository\. Your pull request itself stays with the `open_or_update_draft_pr` and `reply_on_pull_request` tools/,
  );
  assert.doesNotMatch(text, /cannot write/);
  assert.doesNotMatch(text, /not authenticated/);
  const unnamed = platformOverlay({ ...context, repository: "" });
  assert.match(unnamed, /the `gh` CLI is not authenticated/);
  assert.doesNotMatch(platformOverlay({ ...context, deliver: false }), /`gh`/);
});

// w10/m524: bex-api sets BEX_AGENT_GITHUB_CLI=0 when the workspace turned gh
// off. Only that value turns it off: an older bex-api, which sets nothing,
// keeps gh.
test("BEX_AGENT_GITHUB_CLI=0 turns gh off, and anything else keeps it", () => {
  const hosted = {
    BEX_AGENT_MODEL_PROXY_URL: "http://gateway/model/ns/ags-1",
    BEX_AGENT_SESSION_ID: "ags-1",
    BEX_AGENT_MODEL_API_KEY: "bex-model-proxy-placeholder-ags-1",
  };
  assert.equal(loadConfig(hosted).githubCli, true);
  assert.equal(
    loadConfig({ ...hosted, BEX_AGENT_GITHUB_CLI: "1" }).githubCli,
    true,
  );
  assert.equal(
    loadConfig({ ...hosted, BEX_AGENT_GITHUB_CLI: "0" }).githubCli,
    false,
  );
});

test("a repo session whose workspace turned gh off gets no gh settings, socket or config", async () => {
  const config = await tempConfig({
    deliver: true,
    repoUrl: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo",
    branch: "bex-agent/ags-abc",
    repository: "acme/widgets",
    githubCli: false,
  });
  assert.equal(githubCliEnabled(config), false);
  assert.deepEqual(
    Object.keys(childEnv(config)).filter((name) => name.startsWith("GH_")),
    [],
  );
  // An earlier turn's config on a kept disk is removed, so gh finds none.
  const { configDir } = githubCliPaths(config);
  await mkdir(configDir, { recursive: true });
  await writeFile(path.join(configDir, "config.yml"), "http_unix_socket: x\n");
  await removeGitHubCliConfig(config);
  await assert.rejects(stat(configDir), { code: "ENOENT" });
  await removeGitHubCliConfig(config);
});

test("the overlay of a repo session with gh off says gh is turned off and pull requests stay with the bex tools", () => {
  const text = platformOverlay({
    cwd: "/workspace",
    deliver: true,
    repoUrl: "http://gw:8082/git/bnM/c2lk/cmVwbw/YnJhbmNo",
    branch: "bex-agent/ags-abc",
    baseBranch: "",
    repository: "acme/widgets",
    githubCli: false,
    deliveryBaseline: { baseBranch: "main" },
  });
  assert.match(
    text,
    /this workspace's admins turned `gh` off for its agent sessions, so `gh` and direct GitHub API calls do not work here/,
  );
  assert.match(
    text,
    /Your pull request goes through the `open_or_update_draft_pr` and `reply_on_pull_request` tools of the `bex` MCP server, and code reaches GitHub only through your pushes to your branch/,
  );
  assert.doesNotMatch(text, /`gh` works on/);
  assert.doesNotMatch(text, /`gh run/);
});
