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

// Image-build only (ADR108 D2): download every profile's pinned native binary
// for the target architecture (amd64 or arm64), refuse it unless its SHA-256
// matches the manifest, and lay it out under <outRoot><path> for a linked COPY
// into the agent image. The final stage's install-profiles check verifies the
// installed file again and runs it.
// Usage: node fetch-native-binaries.mjs <agent-profiles.json> <outRoot> <arch>
import { createHash } from "node:crypto";
import { createWriteStream } from "node:fs";
import { chmod, mkdir, readFile, rename, rm } from "node:fs/promises";
import path from "node:path";
import { Readable } from "node:stream";
import { pipeline } from "node:stream/promises";

const [manifestPath, outRoot, arch] = process.argv.slice(2);
if (!manifestPath || !outRoot || !arch) {
  throw new Error(
    "usage: fetch-native-binaries.mjs <manifest> <outRoot> <arch>",
  );
}
const manifest = JSON.parse(await readFile(manifestPath, "utf8"));

async function download(url, file) {
  const hash = createHash("sha256");
  const response = await fetch(url, { redirect: "follow" });
  if (!response.ok || !response.body) {
    throw new Error(`GET ${url}: HTTP ${response.status}`);
  }
  const body = Readable.fromWeb(response.body);
  body.on("data", (chunk) => hash.update(chunk));
  await pipeline(body, createWriteStream(file, { mode: 0o600 }));
  return hash.digest("hex");
}

for (const profile of manifest.profiles) {
  const native = profile.nativeBinary;
  if (!native) continue;
  const artifact = native.artifacts[arch];
  if (!artifact) throw new Error(`${profile.id}: no ${arch} artifact pinned`);
  const target = path.join(outRoot, native.path);
  await mkdir(path.dirname(target), { recursive: true, mode: 0o755 });
  const partial = `${target}.partial`;
  // Retry only the transfer; a wrong checksum is final.
  let digest;
  for (let attempt = 1; ; attempt++) {
    try {
      digest = await download(artifact.url, partial);
      break;
    } catch (error) {
      await rm(partial, { force: true });
      if (attempt === 3) throw error;
    }
  }
  if (digest !== artifact.sha256) {
    await rm(partial, { force: true });
    throw new Error(
      `${profile.id}: SHA-256 ${digest} does not match the pinned ${artifact.sha256}`,
    );
  }
  await chmod(partial, 0o755);
  await rename(partial, target);
  console.log(`${profile.id}: ${native.version} (${arch}) → ${native.path}`);
}
