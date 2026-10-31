def exercise(x,h,P,valid_after,reference,record):
 import subprocess,json,hashlib,base64,io,zipfile,zlib,urllib.request
 sources={n:(P/'source-carrier'/n).read_bytes() for n in reference['files']}
 assert {n:{'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest()} for n,b in sources.items()}==reference['files']
 def decode(blob,label):
  expected={**sources,'greeting.txt':('Crème brûlée & 李\n'*7).encode(),'two words.bin':bytes((i*31+17)&255 for i in range(8192)),'empty.txt':b'','request.txt':label.encode()};z=zipfile.ZipFile(io.BytesIO(blob));assert z.namelist()==sorted(expected) and z.testzip() is None and len(set(z.namelist()))==6
  for v in z.infolist():assert z.read(v.filename)==expected[v.filename] and v.file_size==len(expected[v.filename]) and v.CRC==(zlib.crc32(expected[v.filename])&0xffffffff) and v.compress_type in [0,8]
  assert any(v.compress_type==8 for v in z.infolist());return [{'name':v.filename,'bytes':v.file_size,'compressed':v.compress_size,'CRC32':v.CRC,'method':v.compress_type} for v in z.infolist()]
 requests=[];url=x['create_view']['serviceDetails']['url'];boot=None
 for i,root in enumerate(['/','/download','/inspect']):
  nonce=x['nonce']+'-'+str(i+1);path=root+'?qa='+nonce;r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code} %{content_type}',url+path],capture_output=True,timeout=20);body,_,meta=r.stdout.rpartition(b'\n');status,_,ctype=meta.decode(errors='replace').partition(' ');e={'path':path,'nonce':nonce,'HTTP':status,'content_type':ctype,'curl_exit':r.returncode,'bytes':len(body),'sha256':hashlib.sha256(body).hexdigest(),'body_base64':base64.b64encode(body).decode(),'stderr':r.stderr.decode(errors='replace')};record(i,e);assert r.returncode==0 and status=='200'
  if root=='/download':blob=body;assert ctype=='application/zip'
  else:
   v=json.loads(body);e['decoded']=v;assert ctype=='application/json' and v['marker']==x['marker'] and v['nonce']==nonce and valid_after(v['built'],x) and valid_after(v['runtime'],x);boot=boot or v['boot'];assert v['boot']==boot
   built=base64.b64decode(v['built']['archive']['base64']);assert len(built)==v['built']['archive']['bytes'] and hashlib.sha256(built).hexdigest()==v['built']['archive']['sha256'];e['built_members']=decode(built,x['marker']+'|build');blob=base64.b64decode(v['runtime']['archive']['base64']);assert len(blob)==v['runtime']['archive']['bytes'] and hashlib.sha256(blob).hexdigest()==v['runtime']['archive']['sha256']
  e['runtime_members']=decode(blob,x['marker']+'|'+nonce);e['archive_sha256']=hashlib.sha256(blob).hexdigest();e['archive_bytes']=len(blob);e['pass']=True;record(i,e);requests.append(e)
 return requests
