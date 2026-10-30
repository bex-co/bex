def exercise(x,h,P,valid_after,reference,record):
 import subprocess,json,hashlib,base64,zlib,struct
 requests=[];url=x['create_view']['serviceDetails']['url'];artifact=None;data=(reference['input']['unit']*reference['input']['repetitions']).encode()
 for i,path in enumerate(['/inspect','/','/asset.gz']):
  path+='?qa423='+x['nonce']+'-'+str(i+1);r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code} %{content_type}',url+path],capture_output=True,timeout=20);body,_,meta=r.stdout.rpartition(b'\n');status,_,ctype=meta.decode(errors='replace').partition(' ');e={'path':path,'HTTP':status,'content_type':ctype,'curl_exit':r.returncode,'bytes':len(body),'sha256':hashlib.sha256(body).hexdigest(),'body_base64':base64.b64encode(body).decode(),'stderr':r.stderr.decode(errors='replace')};record(i,e);assert r.returncode==0 and status=='200'
  if i<2:
   v=json.loads(body);e['decoded']=v;e['raw']=body.decode();g=v['build']['gzip'];artifact=g if artifact is None else artifact;e['pass']=ctype.startswith('application/json') and valid_after(v['build'],x) and v['build'].get('build_compressor_version','').startswith('pigz ') and valid_after(v['runtime'],x) and v['runtime']['request_path']==path and v['runtime']['gzip']==g==artifact
  else:
   inflated=zlib.decompress(body,wbits=31);crc,size=struct.unpack('<II',body[-8:]);e['independent_inflate']={'bytes':len(inflated),'sha256':hashlib.sha256(inflated).hexdigest(),'CRC32':crc,'ISIZE':size,'header_hex':body[:10].hex()};e['pass']=ctype=='application/gzip' and inflated==data and body[:3]==b'\x1f\x8b\x08' and crc==zlib.crc32(data) and size==len(data) and e['sha256']==artifact['sha256'] and len(body)==artifact['bytes']
  record(i,e);requests.append(e)
 return requests
