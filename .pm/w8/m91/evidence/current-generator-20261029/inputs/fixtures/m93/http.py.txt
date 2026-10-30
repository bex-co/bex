def exercise(x,P,valid_after,valid_before):
 import subprocess,json,hashlib,runpy
 check=runpy.run_path(str(P/'sweep407-html.py'))['inspect_html'];requests=[];url=x['create_view']['serviceDetails']['url']
 for i,path in enumerate(['/render','/','/document']):
  path+='?qa407='+x['nonce']+'-'+str(i+1);r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code} %{content_type}',url+path],capture_output=True,timeout=20);b,_,s=r.stdout.rpartition(b'\n');status,_,ctype=s.decode().partition(' ');e={'path':path,'HTTP':status,'content_type':ctype,'curl_exit':r.returncode,'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest(),'stderr':r.stderr.decode(errors='replace'),'raw':b.decode(errors='replace')}
  if i<2:
   v=json.loads(b);e['decoded']=v;e['independent_document_HTML_parse']=check(v['runtime']['html']['text'].encode());e['pass']=r.returncode==0 and status=='200' and ctype=='application/json' and valid_after(v['build'],x) and valid_after(v['runtime'],x) and valid_before(v['build']['before'],x) and v['runtime']['request_url']==path and v['runtime']['html']==v['build']['html']
  else:
   e['independent_document_HTML_parse']=check(b);e['pass']=r.returncode==0 and status=='200' and ctype=='text/html; charset=utf-8' and e['independent_document_HTML_parse']['pass'] and e['sha256']==requests[0]['decoded']['runtime']['html']['sha256']==requests[1]['decoded']['runtime']['html']['sha256']
  requests.append(e)
 return requests
