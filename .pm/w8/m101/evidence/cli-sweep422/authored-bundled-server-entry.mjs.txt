import fs from 'node:fs';
import crypto from 'node:crypto';
import child from 'node:child_process';
import http from 'node:http';
import {summarize} from './qa-webpack-domain.mjs';
const ref=JSON.parse(fs.readFileSync('qa-webpack-reference.json','utf8')),marker=JSON.parse(fs.readFileSync('qa-webpack-marker.json','utf8'));
const sha=b=>crypto.createHash('sha256').update(b).digest('hex');
function proof(){
 const files=Object.fromEntries(Object.keys(ref.files).map(name=>{const b=fs.readFileSync(name);return [name,{bytes:b.length,sha256:sha(b)}]}));
 if(JSON.stringify(files)!==JSON.stringify(ref.files))throw Error('public source changed');
 const sources=Object.fromEntries(Object.keys(ref.sources).map(name=>[name,sha(fs.readFileSync(name))]));
 if(JSON.stringify(sources)!==JSON.stringify(ref.sources))throw Error('authored source changed');
 const stats=JSON.parse(fs.readFileSync('qa-webpack-stats.json','utf8'));if(stats.errors?.length)throw Error('bundle errors');
 const result=summarize(ref.rows);if(JSON.stringify(result)!==JSON.stringify(ref.expected))throw Error('wrong bundled module result');
 const tool=child.spawnSync('webpack',['--version'],{encoding:'utf8'});if(tool.status!==0)throw Error('runtime webpack unavailable');
 const bundle=fs.readFileSync(process.argv[1]);return {marker,node:process.version,files,sources,input:ref.rows,result,bundle:{bytes:bundle.length,sha256:sha(bundle)},compiler:{version:stats.version,hash:stats.hash,errors:stats.errors?.length||0,warnings:stats.warnings?.length||0},webpack_probe:{status:tool.status,error:tool.error?.code||null,stdout:tool.stdout}};
}
if(process.argv[2]==='build'){
 const build=proof();fs.writeFileSync('qa-webpack-build.json',JSON.stringify(build));console.log('QA_WEBPACK_AFTER',JSON.stringify(build));
}else if(process.argv[2]==='serve'){
 const build=JSON.parse(fs.readFileSync('qa-webpack-build.json','utf8'));http.createServer((req,res)=>{const runtime=proof();runtime.request_path=req.url;res.writeHead(200,{'content-type':'application/json'});res.end(JSON.stringify({build,runtime}));console.log('QA_WEBPACK_HTTP',JSON.stringify({marker,path:req.url,result:runtime.result,bundle_sha256:runtime.bundle.sha256}));}).listen(Number(process.env.PORT||3000),'0.0.0.0');
}else throw Error('unknown mode');
