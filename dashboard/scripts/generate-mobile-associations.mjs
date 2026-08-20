// Generates the two OS-owned HTTPS association documents served by the
// dashboard. Signing identifiers are intentionally build-time configuration:
// they are public values, but they must come from the actual distribution
// identities and must never be guessed or committed as placeholders.

import { mkdirSync, writeFileSync } from "node:fs";
import { pathToFileURL } from "node:url";

export const APPLE_TEAM_ID_ENV = "BEX_MOBILE_APPLE_TEAM_ID";
export const ANDROID_FINGERPRINTS_ENV =
  "BEX_MOBILE_ANDROID_SHA256_CERT_FINGERPRINTS";
export const IOS_BUNDLE_ID = "co.bex.mobile";
export const ANDROID_PACKAGE = "co.bex.mobile";
export const INVITE_PATH = "/invite";
export const OAUTH_REDIRECT_PATH = "/oauth2redirect";
export const WELL_KNOWN_DIR = new URL(
  "../public/.well-known/",
  import.meta.url,
);
export const OUTPUT_DIR_ENV = "BEX_MOBILE_ASSOCIATION_OUTPUT_DIR";

const APPLE_TEAM_ID_PATTERN = /^[A-Z0-9]{10}$/;
const ANDROID_SHA256_PATTERN = /^(?:[0-9A-F]{2}:){31}[0-9A-F]{2}$/;

function normalizedFingerprints(raw) {
  if (!raw.trim()) return [];
  const values = raw
    .split(",")
    .map((value) => value.trim().toUpperCase())
    .filter(Boolean);
  if (values.some((value) => !ANDROID_SHA256_PATTERN.test(value))) {
    throw new Error(
      `${ANDROID_FINGERPRINTS_ENV} must be comma-separated SHA-256 certificate fingerprints (32 uppercase hex bytes separated by colons)`,
    );
  }
  return [...new Set(values)].sort();
}

export function buildAssociations({
  appleTeamId = "",
  androidFingerprints = "",
} = {}) {
  const teamId = appleTeamId.trim().toUpperCase();
  const fingerprints = normalizedFingerprints(androidFingerprints);
  const hasApple = teamId !== "";
  const hasAndroid = fingerprints.length > 0;

  if (hasApple !== hasAndroid) {
    throw new Error(
      `${APPLE_TEAM_ID_ENV} and ${ANDROID_FINGERPRINTS_ENV} must be configured together; refusing a one-platform production association`,
    );
  }
  if (hasApple && !APPLE_TEAM_ID_PATTERN.test(teamId)) {
    throw new Error(
      `${APPLE_TEAM_ID_ENV} must be the exact 10-character Apple Team ID`,
    );
  }

  const apple = {
    applinks: {
      apps: [],
      details: hasApple
        ? [
            {
              appID: `${teamId}.${IOS_BUNDLE_ID}`,
              components: [
                {
                  "/": INVITE_PATH,
                  comment: "Open only bex workspace invitation links.",
                },
                {
                  "/": OAUTH_REDIRECT_PATH,
                  comment: "Open only the OAuth authorization callback.",
                },
              ],
            },
          ]
        : [],
    },
  };
  const android = hasAndroid
    ? [
        {
          relation: ["delegate_permission/common.handle_all_urls"],
          target: {
            namespace: "android_app",
            package_name: ANDROID_PACKAGE,
            sha256_cert_fingerprints: fingerprints,
          },
        },
      ]
    : [];

  return { apple, android, configured: hasApple && hasAndroid };
}

export function serialize(value) {
  return `${JSON.stringify(value, null, 2)}\n`;
}

export function writeAssociations(result, outputDir = WELL_KNOWN_DIR) {
  mkdirSync(outputDir, { recursive: true });
  writeFileSync(
    new URL("apple-app-site-association", outputDir),
    serialize(result.apple),
  );
  writeFileSync(
    new URL("assetlinks.json", outputDir),
    serialize(result.android),
  );
}

function main() {
  const result = buildAssociations({
    appleTeamId: process.env[APPLE_TEAM_ID_ENV] ?? "",
    androidFingerprints: process.env[ANDROID_FINGERPRINTS_ENV] ?? "",
  });
  const override = process.env[OUTPUT_DIR_ENV]?.trim();
  const outputDir = override
    ? new URL(`${override.replace(/\/$/, "")}/`, "file://")
    : WELL_KNOWN_DIR;
  writeAssociations(result, outputDir);
  console.log(
    result.configured
      ? "generated configured iOS/Android invite-link associations"
      : `generated disabled invite-link associations; set ${APPLE_TEAM_ID_ENV} and ${ANDROID_FINGERPRINTS_ENV} together to enable`,
  );
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  main();
}
