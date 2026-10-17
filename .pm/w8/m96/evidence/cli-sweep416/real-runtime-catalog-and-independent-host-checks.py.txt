def values(g):return {'hello':g.gettext('Hello, %(name)s!')%{'name':'Zoë & 李'},'dessert':g.gettext('Dessert'),'plurals':{str(i):g.ngettext('There is %(count)d apple.','There are %(count)d apples.',i)%{'count':i} for i in [0,1,2,5]}}
def exercise(x,h,P,valid_after,reference):
 import subprocess,hashlib,json,base64,gettext,io
 requests=[];url=x['create_view']['serviceDetails']['url']
 for index,path in enumerate(['/translations','/','/catalog.mo']):
  path+='?qa416='+x['nonce']+'-'+str(index+1);r=subprocess.run(['curl','--disable','-sS','--max-time','15','-H','Accept-Encoding: identity','-w','\n%{http_code} %{content_type}',url+path],capture_output=True,timeout=20);b,_,s=r.stdout.rpartition(b'\n');status,_,ctype=s.decode().partition(' ');e={'path':path,'HTTP':status,'content_type':ctype,'curl_exit':r.returncode,'bytes':len(b),'sha256':hashlib.sha256(b).hexdigest(),'stderr':r.stderr.decode(errors='replace')}
  if index<2:
   e['raw']=b.decode(errors='replace');v=json.loads(b);e['decoded']=v;e['pass']=r.returncode==0 and status=='200' and ctype=='application/json' and valid_after(v['build'],x) and valid_after(v['runtime'],x) and v['runtime']['request_url']==path and v['build']['catalog']==v['runtime']['catalog']
  else:
   e['own_catalog_base64']=base64.b64encode(b).decode();e['independent_host_GNUTranslations']=values(gettext.GNUTranslations(io.BytesIO(b)));e['matches_prior_runtime_catalog']=e['sha256']==requests[0]['decoded']['runtime']['catalog']['sha256']==requests[1]['decoded']['runtime']['catalog']['sha256'];e['pass']=bool(r.returncode==0 and status=='200' and ctype=='application/octet-stream' and e['independent_host_GNUTranslations']==reference['expected'] and e['matches_prior_runtime_catalog'])
  requests.append(e)
 return requests
