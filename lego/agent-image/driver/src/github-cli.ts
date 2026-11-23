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

// A repo session's `gh` (w10/m513 reads, w10/m522 writes; ADR090 D4
// amendments). gh sends every API call over the Unix socket its config names
// (`http_unix_socket`), as plain HTTP with the GitHub host in the Host header.
// The driver serves that socket and forwards each api.github.com request, any
// method and body, to the gateway's github-api route under the session's
// Pod-bound Git proxy URL. The gateway admits reads of the session's
// repository and the writes ADR090 D4 lists (comments, reviews that do not
// approve, labels, issues, CI re-runs), and injects a token minted for that
// call on its upstream hop, so the sandbox holds a placeholder and never a
// GitHub token. The same GH_* settings go into a shell file the image's
// profile script reads (w10/027), so gh in a terminal (the web Shell, SSH,
// Zed) works as the agent's does. A workspace that turned gh off (w10/m524)
// gets none of it: no socket, no config, no shell file and no GH_* variables.

import { chmod, mkdir, rm, writeFile } from "node:fs/promises";
import http from "node:http";
import path from "node:path";
import { pipeline } from "node:stream/promises";
import type { AgentDriverConfig } from "./config.js";
import { shellExports } from "./fs-util.js";

// githubCliTokenPlaceholder satisfies gh, which refuses to call the API
// without a token. The gateway ignores it.
export const githubCliTokenPlaceholder =
  "bex-gateway-injects-a-scoped-token";

export type GitHubCliConfig = Pick<
  AgentDriverConfig,
  "deliver" | "repoUrl" | "repository" | "githubCli" | "statusPath"
>;

export interface GitHubCliProxy {
  close(): Promise<void>;
}

// Request headers gh sends that the gateway route reads. Its Authorization
// carries only the placeholder, so it stays here.
const forwardedRequestHeaders = [
  "accept",
  "content-length",
  "content-type",
  "graphql-features",
  "if-modified-since",
  "if-none-match",
  "time-zone",
  "x-github-api-version",
];

const hopByHopHeaders = new Set([
  "connection",
  "keep-alive",
  "transfer-encoding",
]);

export function githubCliEnabled(config: GitHubCliConfig): boolean {
  return Boolean(
    config.githubCli && config.deliver && config.repoUrl && config.repository,
  );
}

// removeGitHubCliConfig deletes a gh config an earlier turn left on a kept
// disk (ADR047 D12), so a turn without gh, and its terminals, have none.
export async function removeGitHubCliConfig(
  config: Pick<GitHubCliConfig, "statusPath">,
): Promise<void> {
  await rm(githubCliPaths(config).configDir, { recursive: true, force: true });
}

// githubCliPaths keeps gh's config, its socket and the terminals' shell file
// beside the driver's status file, outside the repository and the agent's
// home directory. bex-agent-tools.sh finds the shell file at the same place.
export function githubCliPaths(config: Pick<GitHubCliConfig, "statusPath">): {
  configDir: string;
  socketPath: string;
  shellEnvPath: string;
} {
  const configDir = path.join(path.dirname(config.statusPath), "gh");
  return {
    configDir,
    socketPath: path.join(configDir, "api.sock"),
    shellEnvPath: path.join(configDir, "env.sh"),
  };
}

// githubCliEnvironment is what the agent and its tools need for gh: the
// config directory naming the socket, the placeholder token, the session's
// repository (origin is the Git proxy, which gh cannot map to a repository),
// and no prompts or update checks. A session without a repository, or whose
// workspace turned gh off, gets none.
export function githubCliEnvironment(
  config: GitHubCliConfig,
): Record<string, string> {
  if (!githubCliEnabled(config)) return {};
  return {
    GH_CONFIG_DIR: githubCliPaths(config).configDir,
    GH_TOKEN: githubCliTokenPlaceholder,
    GH_REPO: config.repository,
    GH_PROMPT_DISABLED: "1",
    GH_NO_UPDATE_NOTIFIER: "1",
    GH_NO_EXTENSION_UPDATE_NOTIFIER: "1",
  };
}

// githubCliShellEnvironment is githubCliEnvironment as the terminals' shell
// file.
export function githubCliShellEnvironment(config: GitHubCliConfig): string {
  return shellExports(
    "Written by the bex agent driver: gh in this session's terminals reaches its repository through bex, as the agent's does.",
    githubCliEnvironment(config),
  );
}

function refuse(
  response: http.ServerResponse,
  status: number,
  message: string,
): void {
  const body = JSON.stringify({ message });
  response.writeHead(status, {
    "content-type": "application/json; charset=utf-8",
    "content-length": Buffer.byteLength(body),
  });
  response.end(body);
}

// startGitHubCliProxy writes gh's config, serves its socket until close, and
// then writes the terminals' shell file.
export async function startGitHubCliProxy(
  config: GitHubCliConfig,
): Promise<GitHubCliProxy> {
  const { configDir, socketPath, shellEnvPath } = githubCliPaths(config);
  const route = `${config.repoUrl.replace(/\/+$/, "")}/github-api`;
  await mkdir(configDir, { recursive: true, mode: 0o700 });
  // A kept disk's earlier file goes first: terminals get gh's settings only
  // once this socket serves.
  await rm(shellEnvPath, { force: true });
  await writeFile(
    path.join(configDir, "config.yml"),
    `# Written by the bex agent driver: gh reaches this session's repository through bex.\nversion: "1"\nhttp_unix_socket: ${socketPath}\nprompt: disabled\n`,
    { mode: 0o600 },
  );
  await rm(socketPath, { force: true });
  const server = http.createServer((request, response) => {
    const host = (request.headers.host ?? "")
      .toLowerCase()
      .replace(/:\d+$/, "");
    if (host !== "api.github.com") {
      refuse(
        response,
        403,
        `bex: gh in this session reaches only api.github.com, through bex; ${host || "this host"} is not reachable from here.`,
      );
      return;
    }
    if (!request.url?.startsWith("/")) {
      refuse(response, 400, "bex: invalid request");
      return;
    }
    const headers: Record<string, string> = {};
    for (const name of forwardedRequestHeaders) {
      const value = request.headers[name];
      if (typeof value === "string") headers[name] = value;
    }
    let upstream: http.ClientRequest;
    try {
      upstream = http.request(`${route}${request.url}`, {
        method: request.method,
        headers,
      });
    } catch {
      refuse(response, 502, "bex: the GitHub API route is misconfigured.");
      return;
    }
    // gh gone before the answer: stop the gateway call too.
    response.on("close", () => {
      if (!response.writableFinished) upstream.destroy();
    });
    upstream.on("response", (answer) => {
      const relayed: Record<string, string | string[]> = {};
      for (const [name, value] of Object.entries(answer.headers)) {
        if (value !== undefined && !hopByHopHeaders.has(name))
          relayed[name] = value;
      }
      response.writeHead(answer.statusCode ?? 502, relayed);
      pipeline(answer, response).catch(() => response.destroy());
    });
    upstream.on("error", () => {
      if (response.headersSent) response.destroy();
      else
        refuse(
          response,
          502,
          "bex: the GitHub API route is unreachable right now; retry shortly.",
        );
    });
    pipeline(request, upstream).catch(() => upstream.destroy());
  });
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(socketPath, () => {
      server.off("error", reject);
      resolve();
    });
  });
  await chmod(socketPath, 0o600);
  await writeFile(shellEnvPath, githubCliShellEnvironment(config), {
    mode: 0o600,
  });
  return {
    close: () =>
      new Promise<void>((resolve) => {
        server.close(() => resolve());
        server.closeAllConnections();
      }),
  };
}
