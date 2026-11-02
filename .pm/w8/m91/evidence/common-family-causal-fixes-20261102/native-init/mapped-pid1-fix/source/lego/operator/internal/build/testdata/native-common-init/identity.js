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

// Linux dev_t uses the same major/minor encoding as x/sys/unix dev_linux.go.
function device(dev) {
  const major = ((dev & 0x00000000000fff00n) >> 8n) | ((dev & 0xfffff00000000000n) >> 32n);
  const minor = (dev & 0xffn) | ((dev & 0x00000ffffff00000n) >> 12n);
  return `${major.toString(16)}:${minor.toString(16)}`;
}

function boundedFile(path, maximum) {
  const fd = fs.openSync(path, "r");
  const chunks = [];
  let size = 0;
  try {
    for (;;) {
      const chunk = Buffer.alloc(Math.min(4096, maximum + 1 - size));
      const count = fs.readSync(fd, chunk, 0, chunk.length, null);
      if (count === 0) return Buffer.concat(chunks);
      size += count;
      assert.ok(size <= maximum, `bounded identity input ${path}`);
      chunks.push(chunk.subarray(0, count));
    }
  } finally {
    fs.closeSync(fd);
  }
}

function mappedRows(maps) {
  const rows = [];
  for (const line of maps.split("\n")) {
    const row = line.match(/^[0-9a-f]+-[0-9a-f]+\s+([r-][w-][x-][ps])\s+[0-9a-f]+\s+([0-9a-f]+):([0-9a-f]+)\s+([0-9]+)\s+(.+)$/);
    if (row && row[5] === "/native-init") rows.push({permissions: row[1],
      device: `${BigInt(`0x${row[2]}`).toString(16)}:${BigInt(`0x${row[3]}`).toString(16)}`,
      inode: BigInt(row[4]).toString(), path: row[5]});
  }
  return rows;
}

function mappedPID1(init) {
  if (init.pid1 === "/native-init") return true;
  return typeof init.pid1 === "string" && init.pid1.startsWith("/") &&
    expected.arch === "amd64" && init.machine === 62 && init.pid1Machine === 183 &&
    typeof init.device === "string" && typeof init.inode === "string" &&
    /^(0|[1-9a-f][0-9a-f]*):(0|[1-9a-f][0-9a-f]*)$/.test(init.device) &&
    /^[1-9][0-9]*$/.test(init.inode) && Array.isArray(init.mappings) &&
    init.mappings.some((row) => row !== null && typeof row === "object" && row.path === "/native-init" &&
      (row.permissions === "r-xp" || row.permissions === "r-xs") &&
      row.device === init.device && row.inode === init.inode);
}

function validateInit(init, launch) {
  assert.equal(init.sha256, expected.init.sha256);
  assert.equal(init.machine, expected.init.machine);
  assert.equal(init.executable, true);
  assert.ok(mappedPID1(init), "PID1 executable or mapped guest identity differs");
  assert.deepEqual(init.argv, ["/native-init", "/bin/bash", "-c", expected.commands[launch]]);
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
  const target = fs.openSync("/native-init", "r");
  let binary, targetStat;
  try {
    targetStat = fs.fstatSync(target, { bigint: true });
    assert.ok(targetStat.size <= 16n * 1024n * 1024n);
    binary = fs.readFileSync(target);
  } finally {
    fs.closeSync(target);
  }
  const machine = (data) => data.length >= 20 && data.subarray(0, 6).equals(Buffer.from([127, 69, 76, 70, 2, 1])) ? data.readUInt16LE(18) : 0;
  const observation = { errors: [] };
  const init = {
    sha256: crypto.createHash("sha256").update(binary).digest("hex"),
    machine: machine(binary),
    executable: (targetStat.mode & 0o111n) !== 0n,
    device: device(targetStat.dev), inode: targetStat.ino.toString(),
    pid1: "", pid1Machine: 0, argv: [], mappings: [],
  };
  try {
    init.pid1 = fs.readlinkSync("/proc/1/exe");
    const executable = boundedFile("/proc/1/exe", 64 * 1024 * 1024);
    init.pid1Machine = machine(executable);
    observation.pid1ExeSHA256 = crypto.createHash("sha256").update(executable).digest("hex");
    observation.pid1ExeHeader = executable.subarray(0, 64).toString("hex");
  } catch (error) { observation.errors.push(`pid1-exe:${error.code || error.name}`); }
  try {
    const raw = boundedFile("/proc/1/cmdline", 65536);
    observation.pid1ArgvBase64 = raw.toString("base64");
    if (raw.at(-1) === 0) init.argv = raw.toString("utf8").split("\0").slice(0, -1);
  } catch (error) { observation.errors.push(`pid1-argv:${error.code || error.name}`); }
  try {
    observation.maps = boundedFile("/proc/1/maps", 128 * 1024).toString("utf8");
    init.mappings = mappedRows(observation.maps);
  } catch (error) { observation.errors.push(`pid1-maps:${error.code || error.name}`); }
  // Cleanup always captures container stdout even when readiness fails before export.
  const diagnostic = JSON.stringify({ nativeInitIdentityObservation: { ...observation, init } });
  fs.writeFileSync(`qa-output/identity-${base.launch}.json`, diagnostic + "\n");
  console.log(diagnostic);
  validateInit(init, base.launch);
  assert.equal(process.arch, expected.arch === "amd64" ? "x64" : "arm64");
  assert.ok(process.pid > 1, "native-init must remain PID1");
  return { ...actual, baseEnv: base.primary.env, launch: base.launch, init };
}

module.exports = { expected, primary, runtime, mappedPID1, validateInit, device, mappedRows };
