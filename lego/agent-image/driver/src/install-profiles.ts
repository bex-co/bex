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
import { accessSync, constants, readFileSync, realpathSync } from "node:fs";
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
execFileSync("npm", ["cache", "clean", "--force"], { stdio: "inherit" });
