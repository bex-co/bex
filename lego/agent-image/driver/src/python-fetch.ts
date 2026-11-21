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

// Fetching an exact Python patch the agent image does not bake (w4/m544):
// Render runs a fully qualified PYTHON_VERSION exactly, so a session pinned
// to 3.11.9 gets that patch's official build, from the same Docker Hub
// images a native build swaps in for an exact version (ADR060 w8/175). Only
// the layers from the one carrying the interpreter on are downloaded, each
// checked against its digest, and only the interpreter, libpython, the
// standard library (with pip) and the headers are kept. curl does the
// transfers, so the session's egress proxy (HTTPS_PROXY on a full-tier
// session) applies as it does to every other download in the sandbox.

import { mkdir, mkdtemp, rename, rm } from "node:fs/promises";
import path from "node:path";
import { imageArch, sha256File } from "./fs-util.js";
import { run } from "./proc.js";

export interface Registry {
  // The registry's base URL, and the token endpoint that grants an
  // anonymous pull (Docker Hub's, unless a test serves its own).
  url: string;
  tokenUrl: string;
}

export const dockerHub: Registry = {
  url: "https://registry-1.docker.io",
  tokenUrl: "https://auth.docker.io/token?service=registry.docker.io",
};

export interface FetchRequest {
  // The official image of the version, python:3.11.9-bookworm.
  image: string;
  line: string;
  version: string;
  // Where the version goes: <prefix>/bin/python<line> and its libraries.
  prefix: string;
  env: Record<string, string>;
  registry?: Registry;
}

const manifestTypes = [
  "application/vnd.oci.image.index.v1+json",
  "application/vnd.docker.distribution.manifest.list.v2+json",
  "application/vnd.oci.image.manifest.v1+json",
  "application/vnd.docker.distribution.manifest.v2+json",
].join(",");

// The interpreter's layer is near the top (the official Dockerfile builds
// Python last, then adds symlinks and, on older images, pip): never walk
// into the Debian base below it.
const maxLayersWalked = 4;
const requestSeconds = 60;
const blobSeconds = 240;
// The whole fetch's bound: past it the session runs the line's baked patch.
const fetchTimeoutMs = 5 * 60_000;

interface Descriptor {
  digest: string;
  platform?: { os?: string; architecture?: string };
}

interface Manifest {
  manifests?: Descriptor[];
  layers?: Descriptor[];
}

// fetchPython installs request.version at request.prefix, or throws saying
// why it could not. It never leaves a partial prefix behind.
export async function fetchPython(request: FetchRequest): Promise<void> {
  const signal = AbortSignal.timeout(fetchTimeoutMs);
  try {
    await download(request, signal);
  } catch (error) {
    if (signal.aborted)
      throw new Error(
        `the download took longer than ${fetchTimeoutMs / 60_000} minutes`,
      );
    throw error;
  }
}

async function download(
  request: FetchRequest,
  signal: AbortSignal,
): Promise<void> {
  const curl = (args: string[], seconds = requestSeconds) =>
    run(
      "curl",
      [
        "-fsSL",
        "--retry",
        "2",
        "--connect-timeout",
        "15",
        "--max-time",
        String(seconds),
        ...args,
      ],
      { env: request.env, signal },
    );
  const registry = request.registry ?? dockerHub;
  const [name, tag] = request.image.split(":");
  if (!name || !tag) throw new Error(`${request.image} names no tag`);
  const repo = name.includes("/") ? name : `library/${name}`;
  const { token } = JSON.parse(
    await curl([`${registry.tokenUrl}&scope=repository:${repo}:pull`]),
  ) as { token?: string };
  if (!token) throw new Error("the registry granted no pull token");
  const auth = ["-H", `Authorization: Bearer ${token}`];
  const manifest = async (reference: string) =>
    JSON.parse(
      await curl([
        ...auth,
        "-H",
        `Accept: ${manifestTypes}`,
        `${registry.url}/v2/${repo}/manifests/${reference}`,
      ]),
    ) as Manifest;
  let image = await manifest(tag);
  if (image.manifests) {
    const match = image.manifests.find(
      (m) =>
        m.platform?.os === "linux" && m.platform.architecture === imageArch,
    );
    if (!match)
      throw new Error(`${request.image} has no linux/${imageArch} image`);
    image = await manifest(match.digest);
  }
  const layers = image.layers ?? [];
  const parent = path.dirname(request.prefix);
  await mkdir(parent, { recursive: true });
  const work = await mkdtemp(path.join(parent, `.fetch-${request.version}-`));
  try {
    const interpreter = `usr/local/bin/python${request.line}`;
    const files: string[] = [];
    let first = -1;
    for (
      let i = layers.length - 1;
      i >= Math.max(0, layers.length - maxLayersWalked) && first < 0;
      i--
    ) {
      const file = path.join(work, `layer-${i}`);
      await curl(
        [
          ...auth,
          "-o",
          file,
          `${registry.url}/v2/${repo}/blobs/${layers[i].digest}`,
        ],
        blobSeconds,
      );
      if (`sha256:${await sha256File(file)}` !== layers[i].digest)
        throw new Error(`layer ${layers[i].digest} does not match its digest`);
      files[i] = file;
      const names = (await run("tar", ["-tf", file], { signal })).split("\n");
      if (names.some((n) => n.replace(/^\.\//, "") === interpreter)) first = i;
    }
    if (first < 0)
      throw new Error(
        `no top layer of ${request.image} carries ${interpreter}`,
      );
    const rootfs = path.join(work, "rootfs");
    await mkdir(rootfs);
    for (let i = first; i < layers.length; i++)
      await run("tar", ["-xf", files[i], "-C", rootfs, "--no-same-owner"], {
        signal,
      });
    const staged = path.join(work, "prefix");
    await assemble(path.join(rootfs, "usr", "local"), staged, request.line);
    const reported = (
      await run(
        path.join(staged, "bin", `python${request.line}`),
        ["--version"],
        { signal },
      )
    ).trim();
    if (reported !== `Python ${request.version}`)
      throw new Error(`${request.image} runs ${reported}`);
    await rm(request.prefix, { recursive: true, force: true });
    await rename(staged, request.prefix);
  } finally {
    await rm(work, { recursive: true, force: true });
  }
}

// assemble keeps what a Python needs from the image's /usr/local: the
// interpreter (whose $ORIGIN/../lib run path finds the libpython beside it),
// the standard library with pip, and the headers.
async function assemble(
  local: string,
  prefix: string,
  line: string,
): Promise<void> {
  for (const dir of ["bin", "lib", "include"])
    await mkdir(path.join(prefix, dir), { recursive: true });
  for (const part of [
    `bin/python${line}`,
    `lib/libpython${line}.so.1.0`,
    `lib/python${line}`,
    `include/python${line}`,
  ]) {
    await rename(path.join(local, part), path.join(prefix, part));
  }
}
