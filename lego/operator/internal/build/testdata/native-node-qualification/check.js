'use strict';
const fs = require('fs');
const path = require('path');
const cp = require('child_process');
const crypto = require('crypto');
const zlib = require('zlib');
const tls = require('tls');

const root = __dirname;
const mode = process.argv[2];
const manager = process.argv[3] || null;
const expected = JSON.parse(fs.readFileSync(path.join(root, 'expected-node.json'), 'utf8'));
const payload = Buffer.from('bex Node café λ 雪', 'utf8');
const which = cp.spawnSync('node', ['-p', 'process.execPath'], {encoding: 'utf8'});
if (which.status !== 0) throw new Error('PATH node failed');
const addon = require(path.join(root, 'qa-output', 'bex_node.node')).identity();
const compressed = zlib.brotliCompressSync(payload);
const key = crypto.createHash('sha256').update('bex-node-profile').digest();
const iv = Buffer.alloc(12, 7);
const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
const encrypted = Buffer.concat([cipher.update(payload), cipher.final()]);
const tag = cipher.getAuthTag();
const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv);
decipher.setAuthTag(tag);
const decrypted = Buffer.concat([decipher.update(encrypted), decipher.final()]);
const badTag = Buffer.from(tag);
badTag[0] ^= 1;
let badTagRejected = false;
try {
  const bad = crypto.createDecipheriv('aes-256-gcm', key, iv);
  bad.setAuthTag(badTag);
  bad.update(encrypted);
  bad.final();
} catch (_) { badTagRejected = true; }
tls.createSecureContext();
const envNode = process.env.NODE_VERSION;
const proof = {
  mode: mode, manager: manager, node: process.versions.node, executable: fs.realpathSync(process.execPath),
  path_node: fs.realpathSync(which.stdout.trim()), node_env: envNode === undefined ? null : envNode,
  node_env_expected: expected.node_env, path: process.env.PATH,
  modules: Number(process.versions.modules), napi: process.versions.napi, addon: addon,
  openssl: process.versions.openssl, brotli: process.versions.brotli,
  sha256: crypto.createHash('sha256').update(payload).digest('hex'),
  brotli_roundtrip: zlib.brotliDecompressSync(compressed).equals(payload),
  gzip_roundtrip: zlib.gunzipSync(zlib.gzipSync(payload)).equals(payload),
  aes_roundtrip: decrypted.equals(payload), bad_tag_rejected: badTagRejected,
  roots: tls.rootCertificates.length,
  lifecycle: process.env.npm_lifecycle_event || null,
  NODE: process.env.NODE === undefined ? null : process.env.NODE,
  npm_node_execpath: process.env.npm_node_execpath || null,
  npm_execpath: process.env.npm_execpath || null
};
if (mode === 'tools') {
  const npmPath = '/usr/local/lib/node_modules/npm/package.json';
  const yarnPath = '/opt/yarn-v1.22.22/package.json';
  proof.npm_package = JSON.parse(fs.readFileSync(npmPath, 'utf8'));
  proof.npm_package_sha256 = crypto.createHash('sha256').update(fs.readFileSync(npmPath)).digest('hex');
  proof.npm_cli = fs.realpathSync('/usr/local/bin/npm');
  proof.yarn_package = JSON.parse(fs.readFileSync(yarnPath, 'utf8'));
  proof.yarn_cli = fs.realpathSync('/usr/local/bin/yarn');
  proof.pnpm_package = JSON.parse(fs.readFileSync('/usr/local/lib/node_modules/pnpm/package.json', 'utf8'));
}
console.log('BEX_NODE_PROOF=' + JSON.stringify(proof));
