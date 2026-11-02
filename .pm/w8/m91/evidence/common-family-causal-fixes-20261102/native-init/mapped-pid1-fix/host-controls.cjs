// Pure predicate controls. No app/server/native target execution or Docker.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const root = __dirname;
const target = 'lego/operator/internal/build/testdata/native-common-init/identity.js';
const before = fs.readFileSync(path.join(root, 'before', target), 'utf8');
const after = fs.readFileSync(path.join(root, 'source', target), 'utf8');
const expected = {arch:'amd64',init:{sha256:'159a39e14b60d9dcea04c2ec9c55f33b9da42d1580807407edfd02bde7b7e16f',machine:62},commands:{direct:'node capture-env.js direct && exec node server.js'}};
const context=vm.createContext({Buffer,module:{exports:{}},console:{log(){throw new Error('runtime not authorized');}},
 require(name){if(name==='node:fs')return {readFileSync(name){assert.equal(name,'expected.json');return JSON.stringify(expected);}};return require(name);}});
vm.runInContext(after,context,{filename:'private-pid1-predicate-only.js'});
function invoke(data,action='validateInit') {
 context.payload=JSON.stringify(data);
 return vm.runInContext(action==='mappedPID1'?'module.exports.mappedPID1(JSON.parse(payload))':'module.exports.validateInit(JSON.parse(payload),"direct")',context);
}
const raw=fs.readFileSync('/private/tmp/bex-init-observe-a80669f36fd547e6/evidence/attach.stdout','utf8');
assert.match(raw,/target-stat\n37 186068 1753250 755\n/);
assert.match(raw,/pid1-exe-link\n\/native-init\n/);
assert.match(raw,/00400000-004b6000 r-xp 00000000 00:25 186068\s+\/native-init/);
const native={sha256:expected.init.sha256,machine:62,executable:true,pid1:'/native-init',pid1Machine:62,device:'0:25',inode:'186068',mappings:[{path:'/native-init',permissions:'r-xp',device:'0:25',inode:'186068'}],argv:['/native-init','/bin/sh','/observe.sh']};
assert.equal(invoke(native,'mappedPID1'),true); // Actual local observed native shape, not app/drain proof.
assert.equal(vm.runInContext('module.exports.device(37n)',context),'0:25');
context.rawMaps=raw.split('pid1-maps-BEGIN\n')[1].split('pid1-maps-END')[0];
const actualRows=JSON.parse(vm.runInContext('JSON.stringify(module.exports.mappedRows(rawMaps))',context));
assert.equal(actualRows.length,3);
assert.deepEqual(actualRows.map(row=>[row.permissions,row.device,row.inode,row.path]),[['r-xp','0:25','186068','/native-init'],['r--p','0:25','186068','/native-init'],['rw-p','0:25','186068','/native-init']]);

const partialCI={sha256:expected.init.sha256,machine:62,executable:true,pid1:'/run/rosetta/rosetta'};
assert.equal(invoke(partialCI,'mappedPID1'),false); // Actual CI did not retain interpreter/header/mappings/argv.
const complete=structuredClone(native);
complete.pid1='/run/rosetta/rosetta';complete.pid1Machine=183;
complete.argv=['/native-init','/bin/bash','-c',expected.commands.direct];
// Hypothetical complete emulation control: local target mapping + synthetic ARM interpreter/fixture argv.
invoke(complete);
const old=before.match(/^\s*assert\.equal\(init\.pid1, "\/native-init"\);$/m)[0];
context.payload=JSON.stringify(complete);vm.runInContext('globalThis.init=JSON.parse(payload)',context);
assert.throws(()=>vm.runInContext(old,context),{name:'AssertionError'});
const changes={
 wrongSHA:x=>x.sha256='b'.repeat(64),wrongELF:x=>x.machine=183,nonExecutable:x=>x.executable=false,
 wrongInterpreter:x=>x.pid1Machine=62,missingMap:x=>x.mappings=[],wrongInode:x=>x.mappings[0].inode='186069',
 wrongDevice:x=>x.mappings[0].device='0:24',wrongPath:x=>x.mappings[0].path='/other',noRX:x=>x.mappings[0].permissions='rw-p',
 zeroInode:x=>x.inode='0',badDevice:x=>x.device='invalid',wrongArgv:x=>x.argv[3]='different',
 numericInode:x=>{x.inode=186068;x.mappings[0].inode=186068;},nullMap:x=>x.mappings=[null],
};
for(const [name,change] of Object.entries(changes)){const value=structuredClone(complete);change(value);assert.throws(()=>invoke(value),{name:'AssertionError'},name);}
vm.runInContext('module.exports.expected.arch="arm64"',context);
assert.throws(()=>invoke(complete),{name:'AssertionError'});
console.log(JSON.stringify({scope:'pure predicate controls only',actualLocalNativeShape:'PASS',actualPartialCIShape:'REFUSED',syntheticCompleteEmulation:'PASS_OLD_STRICT_GUARD_RED',negativeControls:Object.keys(changes).length+1,actualCIEmulationOrDrainQualified:false}));
