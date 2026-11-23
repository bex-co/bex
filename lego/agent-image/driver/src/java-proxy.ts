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

// A full-tier session's JVM HTTP clients (w10/038). bex-api gives the egress
// proxy as JVM system properties in BEX_AGENT_JAVA_PROXY_OPTIONS (w10/032),
// with the session claim as the proxy user and password. The JDK sends proxy
// credentials only through a java.net.Authenticator, which Gradle and Maven
// install and a plain HttpURLConnection or java.net.http.HttpClient does not,
// so those got the gateway's 407. The driver therefore serves a loopback relay
// for the sandbox's life that sends each CONNECT and plain-HTTP request on to
// the egress proxy with the claim, and the JVM properties name the relay. The
// gateway still binds the claim to the source Pod, which is this one, so the
// relay grants nothing a process here could not already do with the claim it
// can read from its environment. The relay's properties go into a shell file
// the image's profile script reads, as gh's do (w10/027), so a terminal's JVM
// uses it too. A sandbox on another tier gets no relay and no file.

import { mkdir, rm, writeFile } from "node:fs/promises";
import http from "node:http";
import net from "node:net";
import path from "node:path";
import type { Duplex } from "node:stream";
import { pipeline } from "node:stream/promises";
import type { AgentDriverConfig } from "./config.js";
import { describeError } from "./errors.js";
import { shellExports } from "./fs-util.js";

export interface JavaProxyRelay {
  // The JVM properties naming the relay, in place of the gateway's.
  options: string;
  close(): Promise<void>;
}

interface Upstream {
  host: string;
  port: number;
  authorization: string;
}

// Hop-by-hop headers, the client's own proxy credentials and its Expect
// (the relay already answered 100-continue) stay on the client's hop.
const hopByHopHeaders = new Set([
  "connection",
  "expect",
  "keep-alive",
  "proxy-authorization",
  "proxy-connection",
  "te",
  "trailer",
  "transfer-encoding",
  "upgrade",
]);

const relayHost = "127.0.0.1";

// javaProxyPaths keeps the terminals' shell file beside the driver's status
// file, as gh's is. bex-agent-tools.sh finds it at the same place.
export function javaProxyPaths(config: Pick<AgentDriverConfig, "statusPath">): {
  dir: string;
  shellEnvPath: string;
} {
  const dir = path.join(path.dirname(config.statusPath), "java");
  return { dir, shellEnvPath: path.join(dir, "env.sh") };
}

function javaProperties(options: string): Map<string, string> {
  const properties = new Map<string, string>();
  for (const token of options.split(/\s+/)) {
    const match = /^-D([^=]+)=(.*)$/.exec(token);
    if (match) properties.set(match[1], match[2]);
  }
  return properties;
}

// javaProxyUpstream reads the egress proxy and the session claim from the
// properties bex-api gives. Anything incomplete is no proxy.
export function javaProxyUpstream(options: string): Upstream | undefined {
  const properties = javaProperties(options);
  const host = properties.get("https.proxyHost");
  const port = Number(properties.get("https.proxyPort"));
  const user = properties.get("https.proxyUser");
  const password = properties.get("https.proxyPassword");
  if (!host || !user || password === undefined) return undefined;
  if (!Number.isInteger(port) || port <= 0 || port > 65535) return undefined;
  return {
    host,
    port,
    authorization: `Basic ${Buffer.from(`${user}:${password}`).toString("base64")}`,
  };
}

// relayJavaProxyOptions points the http and https proxy host and port at the
// relay and keeps every other property: the claim (Gradle and Maven still
// send it, which the relay replaces with the same), http.nonProxyHosts, so
// the gateway and loopback stay direct, and the tunneling scheme setting.
export function relayJavaProxyOptions(options: string, port: number): string {
  return options
    .split(/\s+/)
    .filter(Boolean)
    .map((token) => {
      const match = /^-D(https?)\.proxy(Host|Port)=/.exec(token);
      if (!match) return token;
      return `-D${match[1]}.proxy${match[2]}=${match[2] === "Host" ? relayHost : port}`;
    })
    .join(" ");
}

// A CONNECT target is host:port. Anything else could not be a tunnel, and
// the relay writes it into its own request line.
const connectTarget = /^(?:[A-Za-z0-9.-]+|\[[0-9A-Fa-f:.]+\]):\d{1,5}$/;

function refuseSocket(socket: Duplex, status: string, message: string) {
  socket.end(
    `HTTP/1.1 ${status}\r\ncontent-type: text/plain; charset=utf-8\r\ncontent-length: ${Buffer.byteLength(message)}\r\nconnection: close\r\n\r\n${message}`,
  );
}

function refuse(
  response: http.ServerResponse,
  status: number,
  message: string,
) {
  response.writeHead(status, {
    "content-type": "text/plain; charset=utf-8",
    "content-length": Buffer.byteLength(message),
  });
  response.end(message);
}

function forwardedHeaders(
  headers: http.IncomingHttpHeaders,
): http.OutgoingHttpHeaders {
  const forwarded: http.OutgoingHttpHeaders = {};
  for (const [name, value] of Object.entries(headers)) {
    if (value !== undefined && !hopByHopHeaders.has(name))
      forwarded[name] = value;
  }
  return forwarded;
}

// startJavaProxyRelay removes the shell file an earlier sandbox left on a
// kept disk or in a snapshot image, then, when bex-api gave JVM proxy
// properties, serves the relay on a loopback port and writes the file with
// the relay's properties. A sandbox without them gets undefined.
export async function startJavaProxyRelay(
  config: Pick<AgentDriverConfig, "statusPath">,
  env: NodeJS.ProcessEnv = process.env,
): Promise<JavaProxyRelay | undefined> {
  const { dir, shellEnvPath } = javaProxyPaths(config);
  await rm(shellEnvPath, { force: true });
  const options = env.BEX_AGENT_JAVA_PROXY_OPTIONS?.trim();
  const upstream = options ? javaProxyUpstream(options) : undefined;
  if (!options || !upstream) return undefined;

  const agent = new http.Agent({ keepAlive: true });
  const tunnels = new Set<Duplex>();
  // A loopback relay for the sandbox's own tools: a long upload or download
  // is never cut by Node's request timeout.
  const server = http.createServer({ requestTimeout: 0 });

  // Plain HTTP arrives in absolute form and goes on in absolute form, with
  // the claim, over a kept-alive connection to the egress proxy.
  server.on("request", (request, response) => {
    if (!/^http:\/\//i.test(request.url ?? "")) {
      refuse(
        response,
        400,
        "bex: this is the session's Java proxy relay; send proxy requests (CONNECT or absolute-form HTTP).",
      );
      return;
    }
    let upstreamRequest: http.ClientRequest;
    try {
      upstreamRequest = http.request({
        host: upstream.host,
        port: upstream.port,
        method: request.method,
        path: request.url,
        headers: {
          ...forwardedHeaders(request.headers),
          "proxy-authorization": upstream.authorization,
        },
        agent,
      });
    } catch {
      refuse(response, 400, "bex: invalid proxy request");
      return;
    }
    response.on("close", () => {
      if (!response.writableFinished) upstreamRequest.destroy();
    });
    upstreamRequest.on("response", (answer) => {
      response.writeHead(
        answer.statusCode ?? 502,
        answer.statusMessage,
        forwardedHeaders(answer.headers),
      );
      pipeline(answer, response).catch(() => response.destroy());
    });
    upstreamRequest.on("error", () => {
      if (response.headersSent) response.destroy();
      else
        refuse(
          response,
          502,
          "bex: the egress proxy is unreachable right now; retry shortly.",
        );
    });
    pipeline(request, upstreamRequest).catch(() => upstreamRequest.destroy());
  });

  // A CONNECT goes on as the relay's own CONNECT with the claim, and from
  // then on the relay passes bytes both ways untouched: the egress proxy's
  // answer (200, or its 403 for a model provider) reaches the client as sent,
  // and a tunnel's TLS runs end to end.
  server.on(
    "connect",
    (request: http.IncomingMessage, client: Duplex, head: Buffer) => {
      // The server's own handlers left this socket with it: an error here
      // must not reach the driver as an uncaught one.
      client.on("error", () => client.destroy());
      const target = request.url ?? "";
      if (!connectTarget.test(target)) {
        refuseSocket(client, "400 Bad Request", "bex: invalid CONNECT target");
        return;
      }
      const tunnel = net.connect(upstream.port, upstream.host);
      let connected = false;
      tunnels.add(client);
      // A client gone ends its tunnel. A tunnel that ends normally ends the
      // client through the pipe below, after everything it sent.
      client.on("close", () => {
        tunnels.delete(client);
        tunnel.destroy();
      });
      tunnel.on("error", () => {
        if (connected) client.destroy();
        else
          refuseSocket(
            client,
            "502 Bad Gateway",
            "bex: the egress proxy is unreachable right now; retry shortly.",
          );
      });
      tunnel.once("connect", () => {
        connected = true;
        tunnel.setNoDelay(true);
        tunnel.write(
          `CONNECT ${target} HTTP/1.1\r\nHost: ${target}\r\nProxy-Authorization: ${upstream.authorization}\r\n\r\n`,
        );
        if (head.length) tunnel.write(head);
        client.pipe(tunnel);
        tunnel.pipe(client);
      });
    },
  );

  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, relayHost, () => {
      server.off("error", reject);
      resolve();
    });
  });
  // An accept failure (no file descriptors left) is logged, never fatal.
  server.on("error", (error) =>
    console.error(`Java proxy relay: ${describeError(error)}`),
  );
  const close = () =>
    new Promise<void>((resolve) => {
      server.close(() => resolve());
      server.closeAllConnections();
      for (const socket of tunnels) socket.destroy();
      agent.destroy();
    });
  const relayOptions = relayJavaProxyOptions(
    options,
    (server.address() as net.AddressInfo).port,
  );
  try {
    await mkdir(dir, { recursive: true, mode: 0o700 });
    await writeFile(
      shellEnvPath,
      shellExports(
        "Written by the bex agent driver: this session's JVMs reach the egress proxy through its loopback relay, which adds the session claim.",
        { BEX_AGENT_JAVA_PROXY_OPTIONS: relayOptions },
      ),
      { mode: 0o600 },
    );
  } catch (error) {
    await close();
    throw error;
  }
  return { options: relayOptions, close };
}

// useJavaProxyRelay starts the relay and points this process's
// BEX_AGENT_JAVA_PROXY_OPTIONS at it, so every child the driver starts (a
// setup step, the agent, whose JAVA_TOOL_OPTIONS credentials.ts builds from
// it) and every /etc/mavenrc takes the relay's properties. A relay that fails
// to start leaves the gateway's, which Gradle and Maven still authenticate to.
export async function useJavaProxyRelay(
  config: Pick<AgentDriverConfig, "statusPath">,
  env: NodeJS.ProcessEnv = process.env,
): Promise<JavaProxyRelay | undefined> {
  const relay = await startJavaProxyRelay(config, env).catch((error) => {
    console.error(`Java proxy relay setup failed: ${describeError(error)}`);
    return undefined;
  });
  if (relay) env.BEX_AGENT_JAVA_PROXY_OPTIONS = relay.options;
  return relay;
}
