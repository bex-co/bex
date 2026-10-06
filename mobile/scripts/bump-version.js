#!/usr/bin/env node
// Bump the store-facing app version in package.json and app.json together.
// Usage: yarn bump [patch|minor|major|X.Y.Z]   (default: patch)
// Build numbers are not touched here: eas.json uses the remote version source,
// so EAS owns ios.buildNumber / android.versionCode and increments them itself.
const fs = require("fs");
const path = require("path");

const root = path.join(__dirname, "..");
const SEMVER = /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/;

function nextVersion(current, bump) {
  if (SEMVER.test(bump)) return bump;
  const match = SEMVER.exec(current);
  if (!match) throw new Error(`current version "${current}" is not X.Y.Z`);
  const [major, minor, patch] = match.slice(1).map(Number);
  switch (bump) {
    case "major":
      return `${major + 1}.0.0`;
    case "minor":
      return `${major}.${minor + 1}.0`;
    case "patch":
      return `${major}.${minor}.${patch + 1}`;
    default:
      throw new Error(`unknown bump "${bump}": use patch, minor, major, X.Y.Z`);
  }
}

// Replace the version in place so the file keeps its Prettier formatting.
function rewrite(file, from, to) {
  const target = path.join(root, file);
  const source = fs.readFileSync(target, "utf8");
  const updated = source.replace(`"version": "${from}"`, `"version": "${to}"`);
  if (updated === source) throw new Error(`${file} has no version ${from}`);
  fs.writeFileSync(target, updated);
}

try {
  const appJson = JSON.parse(
    fs.readFileSync(path.join(root, "app.json"), "utf8"),
  );
  const version = nextVersion(appJson.expo.version, process.argv[2] ?? "patch");
  rewrite("package.json", require("../package.json").version, version);
  rewrite("app.json", appJson.expo.version, version);
  console.log(`bex mobile ${appJson.expo.version} -> ${version}`);
  console.log(
    "Update releaseNotes in store.config.js before the App Store submission.",
  );
} catch (error) {
  console.error(error.message);
  process.exit(1);
}
