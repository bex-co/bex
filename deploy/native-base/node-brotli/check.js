"use strict";

// This malformed-stream regression is from Node's own CVE-2026-111497 fix:
// nodejs/node@b5f41529df78a8af7b395abd6a97d2dcb6c52c61.
const assert = require("assert");
const zlib = require("zlib");
const invalid = Buffer.from(
  "200010414141fcffff07002228f00b60f7f41f000000",
  "hex",
);
// Brotli 1.0 preserves its older error-name convention; 1.1 includes the
// category prefix. Both must report the permanent block-switch error -17.
const expected = {
  code: process.versions.brotli.startsWith("1.0.")
    ? "ERR_BLOCK_SWITCH"
    : "ERR__ERROR_FORMAT_BLOCK_SWITCH",
  errno: -17,
};
assert.strictEqual(
  zlib.constants.BROTLI_DECODER_ERROR_FORMAT_BLOCK_SWITCH,
  -17,
);
assert.throws(
  () =>
    zlib.brotliDecompressSync(invalid, { maxOutputLength: 64 * 1024 * 1024 }),
  expected,
);

const text = Buffer.from(
  "A valid Brotli stream must still round-trip. ".repeat(256),
);
assert.deepStrictEqual(
  zlib.brotliDecompressSync(zlib.brotliCompressSync(text)),
  text,
);

const decoder = zlib.createBrotliDecompress();
let rejected = false;
let outputBytes = 0;
decoder.on("data", (chunk) => {
  outputBytes += chunk.length;
  assert(
    outputBytes <= 64 * 1024 * 1024,
    "invalid stream exceeded the bounded probe",
  );
});
decoder.resume();
decoder.on("error", (error) => {
  assert.strictEqual(error.errno, expected.errno);
  assert.strictEqual(error.code, expected.code);
  rejected = true;
});
decoder.on("end", () => assert.fail("invalid stream was accepted"));
process.on("beforeExit", () =>
  assert(rejected, "stream did not reject the invalid block switch"),
);
for (let i = 0; i < invalid.length; i++)
  decoder.write(invalid.subarray(i, i + 1));
decoder.end();
