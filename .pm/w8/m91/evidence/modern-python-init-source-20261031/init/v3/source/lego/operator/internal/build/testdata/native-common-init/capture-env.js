const assert = require("node:assert/strict");
const fs = require("node:fs");
const { expected, primary } = require("./identity");

const launch = process.argv[2];
assert.ok(Object.hasOwn(expected.commands, launch));
fs.writeFileSync(
  "qa-output/launch.json",
  JSON.stringify({ run: expected.run, launch, primary: primary() }, null, 2) + "\n",
  { flag: "wx" },
);
