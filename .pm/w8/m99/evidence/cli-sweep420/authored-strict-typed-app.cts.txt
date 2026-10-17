export {};
declare function require(name:string):any;
declare const process:{argv:string[];env:Record<string,string|undefined>;version:string};
declare const Buffer:any;
declare const __filename:string;
const fs=require('node:fs'),crypto=require('node:crypto'),cp=require('node:child_process'),http=require('node:http'),assert=require('node:assert/strict');
interface Entry{label:string;quantity:number}
enum Tone{Warm='warm',Cool='cool'}
interface Summary{labels:string[];total:number;tones:Tone[]}
const ref=JSON.parse(fs.readFileSync('qa-ts-reference.json','utf8')),marker:string=JSON.parse(fs.readFileSync('qa-ts-marker.json','utf8'));
function summarize(rows:readonly Entry[]):Summary{return {labels:rows.map((r:Entry)=>r.label),total:rows.reduce((n:number,r:Entry)=>n+r.quantity,0),tones:rows.map((_r:Entry,i:number)=>i%2?Tone.Cool:Tone.Warm)}}
const sha=(b:any):string=>crypto.createHash('sha256').update(b).digest('hex');
function observe(){const files=Object.fromEntries(Object.keys(ref.files).map((n:string)=>{const b=fs.readFileSync(n);return[n,{bytes:b.length,sha256:sha(b)}]}));assert.deepEqual(files,ref.files);const result:Summary=summarize(ref.rows as Entry[]);assert.deepEqual(result,ref.expected);const r=cp.spawnSync('tsc',['--version'],{encoding:'utf8',timeout:5000});assert.equal(r.status,0,r.stderr);return {marker,node:process.version,files,source_ts_sha256:sha(fs.readFileSync('qa-ts-app.cts')),compiled_js_sha256:sha(fs.readFileSync(__filename)),compiler:{status:r.status,error:r.error?.code??null,version:r.stdout.trim()},input:ref.rows,result}}
if(process.argv[2]==='build'){const v=observe();fs.writeFileSync('qa-ts-build.json',JSON.stringify(v));console.log('QA_TS_AFTER '+JSON.stringify(v))}
else{const build=JSON.parse(fs.readFileSync('qa-ts-build.json','utf8'));http.createServer((req:any,res:any)=>{try{const runtime={...observe(),request_url:req.url};const b=Buffer.from(JSON.stringify({build,runtime}));res.writeHead(200,{'content-type':'application/json','content-length':b.length});res.end(b);console.log('QA_TS_HTTP '+JSON.stringify({marker,path:req.url,result:runtime.result,compiled_js_sha256:runtime.compiled_js_sha256}))}catch(e:any){res.writeHead(500);res.end('own fixture failure');console.error(e.stack)}}).listen(Number(process.env.PORT||10000),'0.0.0.0')}
