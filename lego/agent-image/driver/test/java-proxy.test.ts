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

// w10/038: a full-tier session's JVM HTTP clients reach the egress proxy
// through the driver's loopback relay, which adds the session claim that the
// JDK sends only through a java.net.Authenticator.

import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { readFile, stat, writeFile } from "node:fs/promises";
import http from "node:http";
import net from "node:net";
import path from "node:path";
import test from "node:test";
import tls from "node:tls";
import { promisify } from "node:util";
import { agentEnvironment } from "../src/credentials.js";
import {
  javaProxyPaths,
  javaProxyUpstream,
  relayJavaProxyOptions,
  startJavaProxyRelay,
  useJavaProxyRelay,
} from "../src/java-proxy.js";
import { tempConfig, terminal } from "./helpers.js";

const run = promisify(execFile);

// The claim as bex-api writes it: base64url namespace and session id.
const claim = `Basic ${Buffer.from("bnM:c2Vzc2lvbg").toString("base64")}`;
const siblingClaim = `Basic ${Buffer.from("bnM:b3RoZXI").toString("base64")}`;

// gatewayOptions is BEX_AGENT_JAVA_PROXY_OPTIONS as bex-api renders it
// (javaProxyOptions) for an egress proxy on this port.
function gatewayOptions(port: number): string {
  return [
    `-Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=${port}`,
    "-Dhttp.proxyUser=bnM -Dhttp.proxyPassword=c2Vzc2lvbg",
    `-Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=${port}`,
    "-Dhttps.proxyUser=bnM -Dhttps.proxyPassword=c2Vzc2lvbg",
    "-Dhttp.nonProxyHosts=localhost|127.*|[::1]|0.0.0.0|[::0]|gw.test",
    "-Djdk.http.auth.tunneling.disabledSchemes=",
  ].join(" ");
}

interface Seen {
  method?: string;
  url?: string;
  authorization?: string;
}

async function listen(
  t: test.TestContext,
  server: net.Server,
): Promise<number> {
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(
    () =>
      new Promise<void>((resolve) => {
        server.close(() => resolve());
        if (server instanceof http.Server) server.closeAllConnections();
      }),
  );
  return (server.address() as net.AddressInfo).port;
}

// fakeEgressProxy answers as the gateway's egress proxy does: 407 without the
// session claim, 403 for a model provider, and otherwise a tunnel to
// `tunnelPort` or a plain-HTTP answer naming the URL. It records each request.
async function fakeEgressProxy(
  t: test.TestContext,
  tunnelPort = 0,
): Promise<{ port: number; seen: Seen[] }> {
  const seen: Seen[] = [];
  const server = http.createServer((request, response) => {
    seen.push({
      method: request.method,
      url: request.url,
      authorization: request.headers["proxy-authorization"],
    });
    if (request.headers["proxy-authorization"] !== claim) {
      response.writeHead(407, {
        "proxy-authenticate": 'Basic realm="bex-egress"',
      });
      response.end("proxy credentials required");
      return;
    }
    response.writeHead(200, { "content-type": "text/plain" });
    response.end(`plain ${request.url}`);
  });
  const sockets = new Set<net.Socket>();
  server.on("connect", (request, socket: net.Socket, head: Buffer) => {
    sockets.add(socket);
    socket.on("close", () => sockets.delete(socket));
    socket.on("error", () => undefined);
    seen.push({
      method: request.method,
      url: request.url,
      authorization: request.headers["proxy-authorization"],
    });
    if (request.headers["proxy-authorization"] !== claim) {
      socket.end(
        'HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm="bex-egress"\r\nContent-Length: 0\r\n\r\n',
      );
      return;
    }
    if (request.url?.startsWith("api.anthropic.com:")) {
      const body = "model provider hosts are not reachable";
      socket.end(
        `HTTP/1.1 403 Forbidden\r\nContent-Length: ${body.length}\r\n\r\n${body}`,
      );
      return;
    }
    const target = net.connect(tunnelPort, "127.0.0.1", () => {
      socket.write("HTTP/1.1 200 Connection Established\r\n\r\n");
      if (head.length) target.write(head);
      socket.pipe(target);
      target.pipe(socket);
    });
    target.on("error", () => socket.destroy());
  });
  t.after(() => {
    for (const socket of sockets) socket.destroy();
  });
  return { port: await listen(t, server), seen };
}

// echoServer is a tunnel's far end: it answers each chunk upper-cased.
async function echoServer(t: test.TestContext): Promise<number> {
  const sockets = new Set<net.Socket>();
  const server = net.createServer((socket) => {
    sockets.add(socket);
    socket.on("close", () => sockets.delete(socket));
    socket.on("data", (chunk) => socket.write(chunk.toString().toUpperCase()));
  });
  t.after(() => {
    for (const socket of sockets) socket.destroy();
  });
  return listen(t, server);
}

// connectThrough opens a CONNECT tunnel through a proxy and returns the
// proxy's answer head and the socket, positioned after it.
async function connectThrough(
  port: number,
  target: string,
  extraHeaders = "",
): Promise<{ head: string; socket: net.Socket; rest: string }> {
  const socket = net.connect(port, "127.0.0.1");
  socket.write(
    `CONNECT ${target} HTTP/1.1\r\nHost: ${target}\r\n${extraHeaders}\r\n`,
  );
  let buffered = "";
  return new Promise((resolve, reject) => {
    socket.on("error", reject);
    const onData = (chunk: Buffer) => {
      buffered += chunk.toString();
      const end = buffered.indexOf("\r\n\r\n");
      if (end < 0) return;
      socket.off("data", onData);
      resolve({
        head: buffered.slice(0, end),
        rest: buffered.slice(end + 4),
        socket,
      });
    };
    socket.on("data", onData);
  });
}

function readAll(socket: net.Socket): Promise<string> {
  return new Promise((resolve) => {
    let body = "";
    socket.on("data", (chunk) => (body += chunk));
    socket.on("close", () => resolve(body));
  });
}

function nextChunk(socket: net.Socket): Promise<string> {
  return new Promise((resolve) =>
    socket.once("data", (chunk) => resolve(chunk.toString())),
  );
}

function plainGet(
  port: number,
  url: string,
  agent: http.Agent,
  headers: http.OutgoingHttpHeaders = {},
): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    const request = http.request(
      {
        host: "127.0.0.1",
        port,
        path: url,
        headers: { host: new URL(url).host, ...headers },
        agent,
      },
      (response) => {
        let body = "";
        response.on("data", (chunk) => (body += chunk));
        response.on("end", () =>
          resolve({ status: response.statusCode ?? 0, body }),
        );
      },
    );
    request.on("error", reject);
    request.end();
  });
}

async function freePort(): Promise<number> {
  const server = net.createServer();
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const { port } = server.address() as net.AddressInfo;
  await new Promise<void>((resolve) => server.close(() => resolve()));
  return port;
}

test("the relay's properties name it in place of the gateway and keep the rest", () => {
  const gateway =
    "-Dhttp.proxyHost=bex-ssh-gateway.bex-system.svc.cluster.local -Dhttp.proxyPort=8085 -Dhttp.proxyUser=bnM -Dhttp.proxyPassword=c2Vzc2lvbg -Dhttps.proxyHost=bex-ssh-gateway.bex-system.svc.cluster.local -Dhttps.proxyPort=8085 -Dhttps.proxyUser=bnM -Dhttps.proxyPassword=c2Vzc2lvbg -Dhttp.nonProxyHosts=localhost|127.*|[::1]|0.0.0.0|[::0]|bex-ssh-gateway.bex-system.svc.cluster.local -Djdk.http.auth.tunneling.disabledSchemes=";
  assert.equal(
    relayJavaProxyOptions(gateway, 41234),
    "-Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=41234 -Dhttp.proxyUser=bnM -Dhttp.proxyPassword=c2Vzc2lvbg -Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=41234 -Dhttps.proxyUser=bnM -Dhttps.proxyPassword=c2Vzc2lvbg -Dhttp.nonProxyHosts=localhost|127.*|[::1]|0.0.0.0|[::0]|bex-ssh-gateway.bex-system.svc.cluster.local -Djdk.http.auth.tunneling.disabledSchemes=",
  );

  assert.deepEqual(javaProxyUpstream(gateway), {
    host: "bex-ssh-gateway.bex-system.svc.cluster.local",
    port: 8085,
    authorization: claim,
  });
  for (const incomplete of [
    "",
    "-Dhttps.proxyHost=gw -Dhttps.proxyPort=8085",
    "-Dhttps.proxyHost=gw -Dhttps.proxyUser=bnM -Dhttps.proxyPassword=c2Vzc2lvbg",
    "-Dhttps.proxyHost=gw -Dhttps.proxyPort=99999 -Dhttps.proxyUser=bnM -Dhttps.proxyPassword=c2Vzc2lvbg",
  ])
    assert.equal(javaProxyUpstream(incomplete), undefined, incomplete);
});

test("a CONNECT without credentials reaches the egress proxy with the session claim, and the tunnel carries bytes both ways", async (t) => {
  const egress = await fakeEgressProxy(t, await echoServer(t));
  const config = await tempConfig();
  const relay = await startJavaProxyRelay(config, {
    BEX_AGENT_JAVA_PROXY_OPTIONS: gatewayOptions(egress.port),
  });
  assert.ok(relay);
  t.after(() => relay.close());
  const port = Number(/-Dhttps\.proxyPort=(\d+)/.exec(relay.options)?.[1]);

  // The control: straight to the egress proxy, as a JVM with no
  // Authenticator goes today, it is a 407.
  const direct = await connectThrough(egress.port, "repo.maven.apache.org:443");
  assert.match(direct.head, /^HTTP\/1\.1 407 /);
  direct.socket.destroy();

  const tunnel = await connectThrough(port, "repo.maven.apache.org:443");
  assert.match(tunnel.head, /^HTTP\/1\.1 200 /);
  tunnel.socket.write("ping");
  assert.equal(await nextChunk(tunnel.socket), "PING");
  tunnel.socket.destroy();
  assert.deepEqual(egress.seen.at(-1), {
    method: "CONNECT",
    url: "repo.maven.apache.org:443",
    authorization: claim,
  });

  // A client's own proxy credentials are replaced by the session's claim:
  // a sibling session's would be refused by the gateway anyway.
  const own = await connectThrough(
    port,
    "repo.maven.apache.org:443",
    `Proxy-Authorization: ${siblingClaim}\r\n`,
  );
  assert.match(own.head, /^HTTP\/1\.1 200 /);
  own.socket.destroy();
  assert.equal(egress.seen.at(-1)?.authorization, claim);

  // A client that resets its tunnel is no crash: the relay still serves.
  const reset = await connectThrough(port, "repo.maven.apache.org:443");
  reset.socket.resetAndDestroy();
  await new Promise((resolve) => setTimeout(resolve, 50));
  const after = await connectThrough(port, "repo.maven.apache.org:443");
  assert.match(after.head, /^HTTP\/1\.1 200 /);
  after.socket.destroy();
});

test("the egress proxy's refusal reaches the client as sent", async (t) => {
  const egress = await fakeEgressProxy(t);
  const config = await tempConfig();
  const relay = await startJavaProxyRelay(config, {
    BEX_AGENT_JAVA_PROXY_OPTIONS: gatewayOptions(egress.port),
  });
  assert.ok(relay);
  t.after(() => relay.close());
  const port = Number(/-Dhttps\.proxyPort=(\d+)/.exec(relay.options)?.[1]);

  const refused = await connectThrough(port, "api.anthropic.com:443");
  assert.match(refused.head, /^HTTP\/1\.1 403 Forbidden/);
  const body = refused.rest + (await readAll(refused.socket));
  assert.equal(body, "model provider hosts are not reachable");

  const invalid = await connectThrough(port, "repo.test:443/../x");
  assert.match(invalid.head, /^HTTP\/1\.1 400 /);
  invalid.socket.destroy();
});

test("plain HTTP through the relay carries the session claim on every request of a kept-alive connection", async (t) => {
  const egress = await fakeEgressProxy(t);
  const config = await tempConfig();
  const relay = await startJavaProxyRelay(config, {
    BEX_AGENT_JAVA_PROXY_OPTIONS: gatewayOptions(egress.port),
  });
  assert.ok(relay);
  t.after(() => relay.close());
  const port = Number(/-Dhttp\.proxyPort=(\d+)/.exec(relay.options)?.[1]);
  const agent = new http.Agent({ keepAlive: true, maxSockets: 1 });
  t.after(() => agent.destroy());

  assert.equal(
    (await plainGet(egress.port, "http://repo.test/a", agent)).status,
    407,
  );
  for (const url of ["http://repo.test/a", "http://repo.test/b"]) {
    assert.deepEqual(
      await plainGet(port, url, agent, {
        "proxy-authorization": siblingClaim,
      }),
      { status: 200, body: `plain ${url}` },
    );
    assert.deepEqual(egress.seen.at(-1), {
      method: "GET",
      url,
      authorization: claim,
    });
  }
  const local = await new Promise<number>((resolve, reject) => {
    http
      .get({ host: "127.0.0.1", port, path: "/healthz" }, (response) => {
        response.resume();
        resolve(response.statusCode ?? 0);
      })
      .on("error", reject);
  });
  assert.equal(local, 400);
});

test("an unreachable egress proxy is a 502 for a tunnel and for plain HTTP", async (t) => {
  const config = await tempConfig();
  const relay = await startJavaProxyRelay(config, {
    BEX_AGENT_JAVA_PROXY_OPTIONS: gatewayOptions(await freePort()),
  });
  assert.ok(relay);
  t.after(() => relay.close());
  const port = Number(/-Dhttps\.proxyPort=(\d+)/.exec(relay.options)?.[1]);

  const tunnel = await connectThrough(port, "repo.test:443");
  assert.match(tunnel.head, /^HTTP\/1\.1 502 /);
  tunnel.socket.destroy();
  const agent = new http.Agent();
  t.after(() => agent.destroy());
  assert.equal((await plainGet(port, "http://repo.test/", agent)).status, 502);
});

// Every terminal's JVM takes the relay's properties from java/env.sh, and a
// sandbox whose driver started no relay keeps the gateway's.
test("a full-tier sandbox's terminals and children get the relay's properties, and other tiers get no relay", async (t) => {
  const egress = await fakeEgressProxy(t);
  const base = await tempConfig();
  // A quote in the path proves the shell file quotes its value.
  const config = {
    ...base,
    statusPath: path.join(base.root, "it's", "status.json"),
  };
  const gateway = gatewayOptions(egress.port);
  const env: NodeJS.ProcessEnv = { BEX_AGENT_JAVA_PROXY_OPTIONS: gateway };
  const relay = await useJavaProxyRelay(config, env);
  assert.ok(relay);
  t.after(() => relay.close());
  assert.notEqual(relay.options, gateway);
  assert.equal(env.BEX_AGENT_JAVA_PROXY_OPTIONS, relay.options);
  // The agent's own process, whose JAVA_TOOL_OPTIONS credentials.ts builds.
  assert.equal(agentEnvironment(config, env).JAVA_TOOL_OPTIONS, relay.options);

  const { shellEnvPath } = javaProxyPaths(config);
  assert.equal((await stat(shellEnvPath)).mode & 0o777, 0o600);
  for (const shell of [
    ["sh", "-ic"],
    ["bash", "-c"],
  ]) {
    assert.equal(
      (
        await terminal(
          config.statusPath,
          config.cwd,
          shell,
          'echo "$JAVA_TOOL_OPTIONS"',
          { BEX_AGENT_JAVA_PROXY_OPTIONS: gateway },
        )
      ).trim(),
      relay.options,
      `${shell.join(" ")} JAVA_TOOL_OPTIONS`,
    );
  }
  // A repository's own value stays after them, and a nested shell adds
  // nothing twice.
  assert.equal(
    (
      await terminal(
        config.statusPath,
        config.cwd,
        ["bash", "-c"],
        "bash -c 'echo \"$JAVA_TOOL_OPTIONS\"'",
        { BEX_AGENT_JAVA_PROXY_OPTIONS: gateway, JAVA_TOOL_OPTIONS: "-Xmx2g" },
      )
    ).trim(),
    `${relay.options} -Xmx2g`,
  );
  // A shell started before the relay, which put the gateway's properties in
  // front: a terminal it starts gets the relay's in their place.
  assert.equal(
    (
      await terminal(
        config.statusPath,
        config.cwd,
        ["sh", "-ic"],
        'echo "$JAVA_TOOL_OPTIONS"',
        {
          BEX_AGENT_JAVA_PROXY_OPTIONS: gateway,
          JAVA_TOOL_OPTIONS: `${gateway} -Xmx2g`,
        },
      )
    ).trim(),
    `${relay.options} -Xmx2g`,
  );

  // The next sandbox on another tier: no relay, the earlier file removed,
  // and a terminal that still had the gateway's properties keeps them.
  await relay.close();
  const other: NodeJS.ProcessEnv = {};
  assert.equal(await useJavaProxyRelay(config, other), undefined);
  assert.equal(other.BEX_AGENT_JAVA_PROXY_OPTIONS, undefined);
  await assert.rejects(stat(shellEnvPath), { code: "ENOENT" });
  assert.equal(
    (
      await terminal(
        config.statusPath,
        config.cwd,
        ["bash", "-c"],
        'echo "$JAVA_TOOL_OPTIONS"',
        { BEX_AGENT_JAVA_PROXY_OPTIONS: gateway },
      )
    ).trim(),
    gateway,
  );
  assert.equal(
    (
      await terminal(
        config.statusPath,
        config.cwd,
        ["bash", "-c"],
        'echo "${JAVA_TOOL_OPTIONS-}"',
      )
    ).trim(),
    "",
  );
});

// The end state itself, where a JDK is installed (the agent image, CI's
// runners): a JVM's plain HttpURLConnection and java.net.http.HttpClient,
// with no Authenticator, reach an HTTPS and an HTTP origin through the relay,
// and get the 407 straight to the egress proxy.
test("a JVM's HttpURLConnection and java.net.http.HttpClient reach the egress proxy through the relay", async (t) => {
  const java = await run("java", ["-version"]).then(
    () => true,
    () => false,
  );
  if (!java) {
    t.skip("no JDK here; the JVM clients are not exercised");
    return;
  }
  const config = await tempConfig();
  const keystore = path.join(config.root, "origin.p12");
  await run("keytool", [
    "-genkeypair",
    "-alias",
    "origin",
    "-keyalg",
    "EC",
    "-groupname",
    "secp256r1",
    "-dname",
    "CN=repo.test",
    "-ext",
    "SAN=dns:repo.test",
    "-validity",
    "2",
    "-storetype",
    "PKCS12",
    "-keystore",
    keystore,
    "-storepass",
    "changeit",
    "-keypass",
    "changeit",
  ]);
  // The HTTPS origin a tunnel reaches: it answers any request with 200.
  const origin = tls.createServer(
    { pfx: await readFile(keystore), passphrase: "changeit" },
    (socket) => {
      socket.on("error", () => undefined);
      socket.once("data", () =>
        socket.end(
          "HTTP/1.1 200 OK\r\nContent-Length: 6\r\nConnection: close\r\n\r\ntls ok",
        ),
      );
    },
  );
  const egress = await fakeEgressProxy(t, await listen(t, origin));
  const relay = await startJavaProxyRelay(config, {
    BEX_AGENT_JAVA_PROXY_OPTIONS: gatewayOptions(egress.port),
  });
  assert.ok(relay);
  t.after(() => relay.close());

  const probe = path.join(config.root, "Probe.java");
  await writeFile(
    probe,
    `import java.net.*;
import java.net.http.*;

public class Probe {
  public static void main(String[] args) {
    for (String url : args) {
      try {
        HttpURLConnection connection = (HttpURLConnection) new URL(url).openConnection();
        System.out.println("HttpURLConnection " + url + " " + connection.getResponseCode());
      } catch (Exception e) {
        System.out.println("HttpURLConnection " + url + " failed " + e.getMessage());
      }
      try {
        HttpResponse<String> response = HttpClient.newHttpClient()
            .send(HttpRequest.newBuilder(URI.create(url)).build(), HttpResponse.BodyHandlers.ofString());
        System.out.println("HttpClient " + url + " " + response.statusCode());
      } catch (Exception e) {
        System.out.println("HttpClient " + url + " failed " + e.getMessage());
      }
    }
  }
}
`,
  );
  const probeWith = async (options: string) =>
    (
      await run(
        "java",
        [
          `-Djavax.net.ssl.trustStore=${keystore}`,
          "-Djavax.net.ssl.trustStorePassword=changeit",
          "-Djavax.net.ssl.trustStoreType=PKCS12",
          probe,
          "https://repo.test/maven2/",
          "http://repo.test/maven2/",
        ],
        { env: { ...process.env, JAVA_TOOL_OPTIONS: options } },
      )
    ).stdout
      .trim()
      .split("\n");

  assert.deepEqual(await probeWith(relay.options), [
    "HttpURLConnection https://repo.test/maven2/ 200",
    "HttpClient https://repo.test/maven2/ 200",
    "HttpURLConnection http://repo.test/maven2/ 200",
    "HttpClient http://repo.test/maven2/ 200",
  ]);
  const relayed = egress.seen.filter((seen) => seen.authorization === claim);
  assert.deepEqual(
    relayed.map((seen) => `${seen.method} ${seen.url}`),
    [
      "CONNECT repo.test:443",
      "CONNECT repo.test:443",
      "GET http://repo.test/maven2/",
      "GET http://repo.test/maven2/",
    ],
  );

  // The control: the gateway's properties alone, as before the relay.
  const direct = await probeWith(gatewayOptions(egress.port));
  assert.equal(direct.length, 4);
  for (const line of direct) assert.doesNotMatch(line, / 200$/, line);
});
