def exercise(x,h,P,valid_after,reference,record):
 import subprocess,json,hashlib,base64
 requests=[];url=x['create_view']['serviceDetails']['url'];extension=None
 for i,path in enumerate(['/native','/','/native/weighted']):
  path+='?qa421='+x['nonce']+'-'+str(i+1);r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code} %{content_type}',url+path],capture_output=True,timeout=20);body,_,meta=r.stdout.rpartition(b'\n');status,_,ctype=meta.decode(errors='replace').partition(' ');e={'path':path,'HTTP':status,'content_type':ctype,'curl_exit':r.returncode,'bytes':len(body),'sha256':hashlib.sha256(body).hexdigest(),'body_base64':base64.b64encode(body).decode(),'stderr':r.stderr.decode(errors='replace')};record(i,e);assert r.returncode==0 and status=='200'
  v=json.loads(body);e['decoded']=v;e['raw']=body.decode();ext=v['build']['extension'];extension=ext if extension is None else extension;e['pass']=ctype.startswith('application/json') and valid_after(v['build'],x) and valid_after(v['runtime'],x) and v['runtime']['request_path']==path and v['runtime']['extension']==ext==extension and all(v['build'][n]==v['runtime'][n] for n in ['generated_wrapper_sha256','generated_python_sha256']);record(i,e);requests.append(e)
 return requests
