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

// Executed only by the image build: no packages are installed at driver startup.
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import {
  accessSync,
  constants,
  createReadStream,
  mkdtempSync,
  readFileSync,
  realpathSync,
  rmSync,
  statSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { loadAgentProfiles } from "./profiles.js";

const { profiles } = loadAgentProfiles();
execFileSync(
  "npm",
  [
    "install",
    "--global",
    "--ignore-scripts",
    ...profiles.map((profile) => profile.npmPackage),
  ],
  { stdio: "inherit" },
);
const root = execFileSync("npm", ["root", "--global"], {
  encoding: "utf8",
}).trim();
for (const profile of profiles) {
  accessSync(profile.executable, constants.X_OK);
  const split = profile.npmPackage.lastIndexOf("@");
  const name = profile.npmPackage.slice(0, split);
  const version = profile.npmPackage.slice(split + 1);
  const installed = JSON.parse(
    readFileSync(path.join(root, name, "package.json"), "utf8"),
  );
  const bins =
    typeof installed.bin === "string"
      ? [installed.bin]
      : (Object.values(installed.bin ?? {}) as string[]);
  if (
    !bins.some(
      (bin) =>
        realpathSync(path.join(root, name, bin)) ===
        realpathSync(profile.executable),
    )
  ) {
    throw new Error(
      `executable ${profile.executable} is not provided by ${name}`,
    );
  }
  if (installed.version !== version)
    throw new Error(`incorrect installed version for ${name}`);
}
// Native binaries (ADR108 D2) arrive from the image's fetch stage: check the
// installed file is the pinned one, root-owned and not writable by the
// sandbox user, and that it runs and reports the pinned version.
const imageArch = process.arch === "x64" ? "amd64" : process.arch;
for (const profile of profiles) {
  const native = profile.nativeBinary;
  if (!native) continue;
  const artifact = native.artifacts[imageArch as "amd64" | "arm64"];
  if (!artifact)
    throw new Error(`native binary for ${profile.id} has no ${imageArch} pin`);
  accessSync(native.path, constants.X_OK);
  // Streamed: the binary is hundreds of MB.
  const hash = createHash("sha256");
  for await (const chunk of createReadStream(native.path)) hash.update(chunk);
  const digest = hash.digest("hex");
  if (digest !== artifact.sha256)
    throw new Error(`native binary for ${profile.id} does not match its pin`);
  const { uid, mode } = statSync(native.path);
  if (uid !== 0 || (mode & 0o022) !== 0)
    throw new Error(`native binary for ${profile.id} must be root-owned`);
  // A throwaway HOME keeps whatever the CLI writes on start out of the image.
  const home = mkdtempSync(path.join(tmpdir(), "native-check-"));
  const version = execFileSync(native.path, ["--version"], {
    encoding: "utf8",
    env: { ...process.env, HOME: home },
  });
  rmSync(home, { recursive: true, force: true });
  if (!version.includes(native.version))
    throw new Error(`native binary for ${profile.id} is not ${native.version}`);
}
execFileSync("npm", ["cache", "clean", "--force"], { stdio: "inherit" });
