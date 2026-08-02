// @vitest-environment node
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import {
  ANDROID_FINGERPRINTS_ENV,
  ANDROID_PACKAGE,
  APPLE_TEAM_ID_ENV,
  INVITE_PATH,
  IOS_BUNDLE_ID,
  OUTPUT_DIR_ENV,
  buildAssociations,
} from "../generate-mobile-associations.mjs";

const FINGERPRINT = Array.from({ length: 32 }, (_, index) =>
  index.toString(16).padStart(2, "0"),
)
  .join(":")
  .toUpperCase();
const scriptPath = fileURLToPath(
  new URL("../generate-mobile-associations.mjs", import.meta.url),
);
const temporaryDirectories = [];

afterEach(async () => {
  await Promise.all(
    temporaryDirectories
      .splice(0)
      .map((directory) => rm(directory, { recursive: true, force: true })),
  );
});

describe("generate-mobile-associations", () => {
  it("is valid but disabled when no real signing identities are configured", () => {
    const result = buildAssociations();

    expect(result.configured).toBe(false);
    expect(result.apple).toEqual({
      applinks: { apps: [], details: [] },
    });
    expect(result.android).toEqual([]);
    expect(JSON.stringify(result)).not.toMatch(/placeholder|example/i);
  });

  it("generates exact-path associations for the production application ids", () => {
    const result = buildAssociations({
      appleTeamId: "a1b2c3d4e5",
      androidFingerprints: `${FINGERPRINT},${FINGERPRINT.toLowerCase()}`,
    });

    expect(result.configured).toBe(true);
    expect(result.apple.applinks.details).toEqual([
      {
        appID: `A1B2C3D4E5.${IOS_BUNDLE_ID}`,
        components: [
          {
            "/": INVITE_PATH,
            comment: "Open only bex workspace invitation links.",
          },
        ],
      },
    ]);
    expect(result.android).toEqual([
      {
        relation: ["delegate_permission/common.handle_all_urls"],
        target: {
          namespace: "android_app",
          package_name: ANDROID_PACKAGE,
          sha256_cert_fingerprints: [FINGERPRINT],
        },
      },
    ]);
    expect(JSON.stringify(result)).not.toContain("pathPrefix");
    expect(JSON.stringify(result)).not.toContain('"/":"/*"');
  });

  it("rejects partial or malformed signing configuration", () => {
    expect(() => buildAssociations({ appleTeamId: "A1B2C3D4E5" })).toThrow(
      /configured together/,
    );
    expect(() =>
      buildAssociations({ androidFingerprints: FINGERPRINT }),
    ).toThrow(/configured together/);
    expect(() =>
      buildAssociations({
        appleTeamId: "TOO-SHORT",
        androidFingerprints: FINGERPRINT,
      }),
    ).toThrow(/10-character Apple Team ID/);
    expect(() =>
      buildAssociations({
        appleTeamId: "A1B2C3D4E5",
        androidFingerprints: "AA:BB:CC",
      }),
    ).toThrow(/SHA-256 certificate fingerprints/);
  });

  it("projects configured build environment into the public artifacts", async () => {
    const output = await mkdtemp(join(tmpdir(), "bex-associations-"));
    temporaryDirectories.push(output);

    execFileSync(process.execPath, [scriptPath], {
      env: {
        ...process.env,
        [APPLE_TEAM_ID_ENV]: "A1B2C3D4E5",
        [ANDROID_FINGERPRINTS_ENV]: FINGERPRINT,
        [OUTPUT_DIR_ENV]: output,
      },
      stdio: "pipe",
    });

    const apple = JSON.parse(
      readFileSync(join(output, "apple-app-site-association"), "utf8"),
    );
    const android = JSON.parse(
      readFileSync(join(output, "assetlinks.json"), "utf8"),
    );
    expect(apple.applinks.details[0].appID).toBe(`A1B2C3D4E5.${IOS_BUNDLE_ID}`);
    expect(apple.applinks.details[0].components).toEqual([
      { "/": INVITE_PATH, comment: expect.any(String) },
    ]);
    expect(android[0].target).toEqual({
      namespace: "android_app",
      package_name: ANDROID_PACKAGE,
      sha256_cert_fingerprints: [FINGERPRINT],
    });
  });

  it("runs association generation explicitly before every dashboard build", () => {
    const packageJson = JSON.parse(
      readFileSync(join(process.cwd(), "package.json"), "utf8"),
    );
    expect(packageJson.scripts.build).toMatch(
      /^node scripts\/generate-mobile-associations\.mjs && /,
    );
  });
});
