def exercise(x,h,P,valid_after,reference,record):
 import subprocess,json,hashlib
 requests=[]
 for i,root in enumerate(['/restore','/','/restore']):
  path=root+'?qa402='+x['nonce']+'-'+str(i+1);r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code}',x['create_view']['serviceDetails']['url']+path],capture_output=True,timeout=20);body,_,status=r.stdout.rpartition(b'\n');v=json.loads(body);e={'path':path,'HTTP':status.decode(),'curl_exit':r.returncode,'decoded':v,'sha256':hashlib.sha256(body).hexdigest(),'pass':r.returncode==0 and status==b'200' and valid_after(v['build'],x) and valid_after(v['runtime'],x) and v['runtime']['request_url']==path};record(i,e);requests.append(e)
 return requests
