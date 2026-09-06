// Run from the driver directory: node test/fixtures/hotpath-before.mjs
// Executes the actual pre-m77 implementations from Git, instrumenting only the
// frame encoder and appendFile call. No historical implementation is vendored.
import { execFileSync } from "node:child_process";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { representativeParts } from "./hotpath-parts.mjs";

const revision = "1f1647b44^";
const driverRoot = fileURLToPath(new URL("../../", import.meta.url));
const gitSource = (name) =>
  execFileSync(
    "git",
    ["show", `${revision}:lego/agent-image/driver/src/${name}.ts`],
    { cwd: driverRoot, encoding: "utf8" },
  );
const root = await mkdtemp(path.join(tmpdir(), "bex-hotpath-before-"));
try {
  const hubSource = gitSource("stream-hub");
  const frameDeclaration = "function frame(value: unknown): string {";
  if (!hubSource.includes(frameDeclaration))
    throw new Error("historical frame function not found");
  await writeFile(
    path.join(root, "stream-hub.ts"),
    hubSource.replace(
      frameDeclaration,
      `export let frameEncodes = 0;\n${frameDeclaration}\nframeEncodes++;`,
    ),
  );
  const sessionSource = gitSource("session");
  const match = sessionSource.match(/async function logPart\([\s\S]*?\n}\n/);
  if (!match || !match[0].includes("await appendFile(filename, line, {"))
    throw new Error("historical logPart function not found");
  await writeFile(
    path.join(root, "log-part.ts"),
    `
import {appendFile,mkdir} from "node:fs/promises";
import path from "node:path";
async function ensureParent(filename:string){await mkdir(path.dirname(filename),{recursive:true});}
export let appendOpens = 0;
export ${match[0].replace("await appendFile(filename, line, {", "appendOpens++;\nawait appendFile(filename, line, {")}
`,
  );
  await writeFile(path.join(root, "package.json"), '{"type":"module"}');
  await writeFile(
    path.join(root, "run.ts"),
    `
import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {UIMessageStreamHub,frameEncodes} from './stream-hub.ts';
import {logPart,appendOpens} from './log-part.ts';
const parts=${JSON.stringify(representativeParts)};
const hub=new UIMessageStreamHub({maxHistoryParts:2});
const clients=Array.from({length:2},()=>[]);
const response=(frames)=>({write:(s)=>(frames.push(s),true),once:()=>{},end:()=>{}});
for(const client of clients)hub.attach(response(client));
for(const [index,part] of parts.entries()){
  hub.publish(part);
  await logPart('session.jsonl',part,{redact:s=>s},16<<20,1,index);
}
const replay=Array.from({length:2},()=>[]);
for(const client of replay)hub.attach(response(client));
hub.close();
const wire=parts.map(part=>'data: '+JSON.stringify(part)+'\\n\\n');
for(const client of clients)assert.deepEqual(client,wire);
for(const client of replay)assert.deepEqual(client,wire.slice(-2));
const records=(await readFile('session.jsonl','utf8')).trim().split('\\n').map(line=>JSON.parse(line));
assert.deepEqual(records.map(row=>row.part),parts);
assert.equal(appendOpens,9);assert.equal(frameEncodes,20);
console.log(JSON.stringify({revision:${JSON.stringify(revision)},parts:parts.length,fileOpens:appendOpens,fileWrites:appendOpens,frameEncodes,liveFramesPerClient:clients[0].length,replayFramesPerClient:replay[0].length,partIndexes:records.map(row=>row.partIndex),liveBytes:wire.join(''),replayBytes:replay[0].join('')}));
`,
  );
  process.stdout.write(
    execFileSync(
      process.execPath,
      [
        "--import",
        fileURLToPath(import.meta.resolve("tsx")),
        path.join(root, "run.ts"),
      ],
      { cwd: root, encoding: "utf8" },
    ),
  );
} finally {
  await rm(root, { recursive: true, force: true });
}
