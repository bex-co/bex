def exercise(x,h,P,valid_after,reference,record=None):
 import subprocess,hashlib,json,base64,runpy
 decode=runpy.run_path(str(P/'sweep406-png.py'))['decode_png'];requests=[];url=x['create_view']['serviceDetails']['url']
 def image(v):
  raw=base64.b64decode(v['png']['base64'],validate=True);assert len(raw)==v['png']['bytes'] and hashlib.sha256(raw).hexdigest()==v['png']['sha256'];p=decode(raw);assert p['pixels_hex']==reference['input']['expected_RGB_hex'];return p
 for index,path in enumerate(['/render','/','/png']):
  path+='?qa418='+x['nonce']+'-'+str(index+1);r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code} %{content_type}',url+path],capture_output=True,timeout=20);b,_,s=r.stdout.rpartition(b'\n');status,_,ctype=s.decode().partition(' ');e={'path':path,'HTTP':status,'content_type':ctype,'curl_exit':r.returncode,'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest(),'stderr':r.stderr.decode(errors='replace')}
  e['full_body_base64']=base64.b64encode(b).decode()
  if record:record(index,e)
  assert r.returncode==0 and status=='200',{'HTTP':status,'curl_exit':r.returncode,'stderr':e['stderr']}
  if index<2:
   e['raw']=b.decode(errors='replace');v=json.loads(b);e['decoded']=v;e['PNG_proofs']={k:image(v[k]) for k in ['build','runtime']};e['output_png_sha256']=v['runtime']['png']['sha256'];e['pass']=r.returncode==0 and status=='200' and ctype=='application/json' and valid_after(v['build'],x) and valid_after(v['runtime'],x) and v['runtime']['request_url']==path
  else:
   e['own_PNG_base64']=base64.b64encode(b).decode();e['independent_PNG_decode']=decode(b);e['output_png_sha256']=e['sha256'];e['pass']=r.returncode==0 and status=='200' and ctype=='image/png' and e['independent_PNG_decode']['pixels_hex']==reference['input']['expected_RGB_hex']
  if record:record(index,e)
  requests.append(e)
 return requests
