const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const fs = require("node:fs");

const expected = JSON.parse(fs.readFileSync("expected.json", "utf8"));

function primary(checkPath = true) {
  assert.equal(process.execPath, "/usr/local/bin/node");
  assert.equal(process.versions.node, expected.node);
  const env = {};
  for (const [key, value] of Object.entries(expected.env)) {
    if (checkPath || key !== "PATH") {
      assert.equal(process.env[key], value, `selected primary ENV ${key}`);
    }
    env[key] = process.env[key];
  }
  return { node: process.versions.node, executable: process.execPath, env };
}

function runtime() {
  const base = JSON.parse(fs.readFileSync("qa-output/launch.json", "utf8"));
  assert.equal(base.run, expected.run);
  assert.ok(Object.hasOwn(expected.commands, base.launch));
  assert.equal(base.primary.node, expected.node);
  assert.equal(base.primary.executable, "/usr/local/bin/node");
  assert.deepEqual(base.primary.env, expected.env);
  // Package managers prepend child PATH entries after the exact base ENV capture.
  const actual = primary(base.launch === "direct");
  const binary = fs.readFileSync("/native-init");
  assert.ok(binary.subarray(0, 4).equals(Buffer.from([127, 69, 76, 70])));
  assert.equal(binary[4], 2);
  assert.equal(binary[5], 1);
  const init = {
    sha256: crypto.createHash("sha256").update(binary).digest("hex"),
    machine: binary.readUInt16LE(18),
    executable: (fs.statSync("/native-init").mode & 0o111) !== 0,
    pid1: fs.readlinkSync("/proc/1/exe"),
    argv: fs.readFileSync("/proc/1/cmdline", "utf8").split("\0"),
  };
  assert.equal(init.argv.pop(), "");
  assert.equal(init.sha256, expected.init.sha256);
  assert.equal(init.machine, expected.init.machine);
  assert.equal(init.executable, true);
  assert.equal(init.pid1, "/native-init");
  assert.deepEqual(init.argv, ["/native-init", "/bin/bash", "-c", expected.commands[base.launch]]);
  assert.equal(process.arch, expected.arch === "amd64" ? "x64" : "arm64");
  assert.ok(process.pid > 1, "native-init must remain PID1");
  return { ...actual, baseEnv: base.primary.env, launch: base.launch, init };
}

module.exports = { expected, primary, runtime };
