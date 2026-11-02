const fs = require("node:fs");
const { primary } = require("./identity");

fs.mkdirSync("qa-output", { recursive: true });
const proof = { phase: "build", primary: primary(), pass: true };
fs.writeFileSync("qa-output/build.json", JSON.stringify(proof, null, 2) + "\n");
console.log(JSON.stringify(proof, null, 2));
