'use strict';
const assert = require('assert');
const fs = require('fs');
assert.strictEqual(process.versions.node, process.argv[2]);
assert.strictEqual(require('./build/Release/bex_addon.node').answer(), 'native addon: Grüße ∑ 42');
const result = {pass: true, node: process.versions.node, modules: process.versions.modules, napi: process.versions.napi};
fs.mkdirSync('qa-output', {recursive: true});
fs.writeFileSync('qa-output/addon-runtime.json', JSON.stringify(result));
console.log(JSON.stringify(result, null, 2));
