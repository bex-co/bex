def exercise(x,h,P,valid_after,reference):
 import subprocess,hashlib,json,base64,runpy
 png=runpy.run_path(str(P/'sweep406-png.py'));requests=[];url=x['create_view']['serviceDetails']['url']
 for index,path in enumerate(['/render','/','/png']):
  path+='?qa414='+x['nonce']+'-'+str(index+1);r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code} %{content_type}',url+path],capture_output=True,timeout=20);b,_,s=r.stdout.rpartition(b'\n');status,_,ctype=s.decode().partition(' ');e={'path':path,'HTTP':status,'content_type':ctype,'curl_exit':r.returncode,'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest(),'stderr':r.stderr.decode(errors='replace')}
  if index<2:
   e['raw']=b.decode(errors='replace');v=json.loads(b);e['decoded']=v;e['pass']=r.returncode==0 and status=='200' and ctype=='application/json' and valid_after(v['build'],x) and valid_after(v['runtime'],x) and v['runtime']['request_url']==path and v['build']['png']==v['runtime']['png']
  else:
   e['own_PNG_base64']=base64.b64encode(b).decode();e['independent_PNG_decode']=png['decode_png'](b);e['pass']=r.returncode==0 and status=='200' and ctype=='image/png' and e['independent_PNG_decode']['pixels_hex']==reference['expected_rgb_hex'];e['matches_prior_runtime_PNG']=e['sha256']==requests[0]['decoded']['runtime']['png']['sha256']==requests[1]['decoded']['runtime']['png']['sha256'];e['pass']=bool(e['pass'] and e['matches_prior_runtime_PNG'])
  requests.append(e)
 return requests
