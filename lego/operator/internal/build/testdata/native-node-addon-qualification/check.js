'use strict';
// The package manager's implicit install has already compiled this exact file.
const fs = require('fs');
const path = require('path');
const cp = require('child_process');
const addon = require(path.join(process.cwd(), 'build/Release/bex_addon.node')).identity();
const located = cp.spawnSync('node', ['-p', 'process.execPath'], {encoding: 'utf8'});
if (located.status !== 0) throw new Error('PATH node failed');
const proof = {
  phase: process.argv[2], cwd: process.cwd(), node: process.versions.node,
  executable: fs.realpathSync(process.execPath), path_node: fs.realpathSync(located.stdout.trim()),
  node_env: process.env.NODE_VERSION === undefined ? null : process.env.NODE_VERSION,
  path: process.env.PATH, modules: Number(process.versions.modules), addon: addon,
  platform: process.platform, arch: process.arch,
  lifecycle: process.env.npm_lifecycle_event || null,
  NODE: process.env.NODE === undefined ? null : process.env.NODE,
  npm_node_execpath: process.env.npm_node_execpath || null,
  npm_execpath: process.env.npm_execpath || null
};
console.log('BEX_NODE_ADDON_PROOF=' + JSON.stringify(proof));
