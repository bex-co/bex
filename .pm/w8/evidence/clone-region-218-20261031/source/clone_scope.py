"""Fixed Free clone relay and ownership journal. No production action on import."""
from pathlib import Path
import base64,datetime,gzip,http.client,http.server,json,re,threading,urllib.parse

SERVICE_ID=re.compile(r'srv-[a-z0-9]+\Z')
HOP={'connection','transfer-encoding','content-length','keep-alive','proxy-authenticate','proxy-authorization','te','trailer','upgrade'}

def now():return datetime.datetime.now(datetime.timezone.utc).isoformat()
def view(raw,headers):
 encoding=next((v for k,v in headers if k.lower()=='content-encoding'),'')
 decoded=gzip.decompress(raw) if encoding=='gzip' else raw
 x=json.loads(decoded);return x.get('data',x).get('service',x.get('data',x))

class Journal:
 def __init__(self,root,workspace):
  self.root=Path(root);self.path=self.root/'ledger.json';self.lock=threading.RLock()
  self.data=json.loads(self.path.read_text()) if self.path.exists() else {'scope':'w8/218 Free clones only','workspace':workspace,'baseline_ids':[],'resources':[],'cases':[]}
  assert self.data['workspace']==workspace
 def save(self):
  with self.lock:
   temp=self.path.with_suffix('.tmp');temp.write_text(json.dumps(self.data,indent=2)+'\n');temp.chmod(0o600);temp.replace(self.path)
 def own(self,sid):
  assert sid not in self.data['baseline_ids']
  matches=[x for x in self.data['resources'] if x.get('id')==sid];assert len(matches)==1,'duplicate resource ID';r=matches[0]
  v=r['create_view'];assert r['HTTP']==201 and r['forwarded_once'] and r['workspace']==self.data['workspace'] and v['id']==sid and v['name']==r['name'] and v['ownerId']==r['workspace'] and v['type']=='web_service' and v['serviceDetails']['plan']=='free'
  return r
 def source(self):
  r=next(x for x in self.data['resources'] if x['role']=='source' and x.get('id'))
  self.own(r['id']);assert r['state']!='deleted';return r
 def start(self,role,name,client):
  with self.lock:
   assert not any(x['role']==role for x in self.data['resources']),'intent already exists; reconcile instead of retry'
   active=[x for x in self.data['resources'] if x['state'] not in ['deleted','not-created']]
   assert len(active)<2,'at most source plus one clone; cleanup first'
   if role!='source':self.source()
   r={'role':role,'name':name,'client':client,'workspace':self.data['workspace'],'at':now(),'state':'intent','forwarded_once':False,'pre_ids':list(self.data['baseline_ids'])};self.data['resources'].append(r);self.save();return r
 def record(self,event):
  with self.lock:self.data['cases'].append(event);self.save()

class FixedAPI:
 def request(self,method,path,raw,headers):
  conn=http.client.HTTPSConnection('api.bex.co',timeout=30)
  forwarded={k:v for k,v in headers.items() if k.lower() not in HOP|{'host'}};forwarded['Host']='api.bex.co'
  try:
   conn.request(method,path,body=raw if method in ['POST','DELETE'] else None,headers=forwarded);reply=conn.getresponse();body=reply.read(2*1024*1024+1);assert len(body)<=2*1024*1024,'bounded metadata response exceeded'
   return reply.status,reply.getheaders(),body
  finally:conn.close()

class Relay:
 def __init__(self,journal,mode,intent=None,cleanup_id=None,backend=None):
  assert mode in ['source','clone','invalid','cleanup']
  self.journal,self.mode,self.intent,self.cleanup_id=journal,mode,intent,cleanup_id;self.backend=backend or FixedAPI();self.events=[];self.post_attempts=0
  if mode in ['clone','invalid']:journal.source()
  if cleanup_id:journal.own(cleanup_id)
 def allowed_get(self,path):
  parsed=urllib.parse.urlsplit(path)
  if parsed.scheme or parsed.netloc or parsed.query or parsed.fragment or '%' in parsed.path:return False
  ids=[]
  try:ids.append(self.journal.source()['id'])
  except (StopIteration,AssertionError):pass
  if self.cleanup_id:ids.append(self.cleanup_id)
  # Both frozen clients only GET the source detail before POST; no secret routes.
  return parsed.path in {'/v1/services/'+sid for sid in ids}
 def dispatch(self,method,path,raw,headers):
  if method=='GET' and self.allowed_get(path):pass
  elif method=='POST' and path=='/v1/services' and self.mode in ['source','clone']:
   with self.journal.lock:
    self.post_attempts+=1;r=self.intent
    if not r or r['forwarded_once']:return self.reject('POST already forwarded; reconcile recorded intent',method,path)
    try:body=json.loads(raw)
    except (ValueError,UnicodeError):return self.reject('invalid request JSON',method,path)
    if body.get('name')!=r['name'] or body.get('ownerId')!=r['workspace'] or body.get('type')!='web_service' or body.get('serviceDetails',{}).get('plan')!='free':return self.reject('outside fixed Free create intent',method,path)
    if any(body.get(k) for k in ['disk','diskId','envGroupIds','registryCredentialId','environmentId','envVars','secretFiles','repo']) or any(body.get('serviceDetails',{}).get(k) for k in ['disk','diskId','diskSizeGB','autoscaling','preDeployCommand']):return self.reject('fixture dependencies or disk not permitted',method,path)
    if body.get('image',{}).get('imagePath')!='docker.io/ealen/echo-server:0.9.2' or body.get('image',{}).get('registryCredentialId') or body.get('serviceDetails',{}).get('numInstances',1) not in (0,1):return self.reject('outside fixed public echo fixture',method,path)
    active=[x for x in self.journal.data['resources'] if x['state'] not in ['deleted','not-created']];assert len(active)<=2
    r['forwarded_once']=True;r['state']='request forwarded; response pending';r['request_body_base64']=base64.b64encode(raw).decode();self.journal.save()
  elif method=='DELETE' and path=='/v1/services/'+str(self.cleanup_id) and self.mode=='cleanup':
   r=self.journal.own(self.cleanup_id)
   if r['role']=='source':assert not any(x['role']!='source' and x['state'] not in ['deleted','not-created'] for x in self.journal.data['resources']),'cleanup clone first'
  else:
   if method=='POST':self.post_attempts+=1
   return self.reject('outside exact owned scope',method,path)
  event={'at':now(),'method':method,'path':path,'request_body_base64':base64.b64encode(raw).decode(),'authorization':'[REDACTED]'}
  try:status,response_headers,body=self.backend.request(method,path,raw,dict(headers))
  except Exception as error:
   event.update(upstream_response_unknown=True,error=type(error).__name__);self.events.append(event);self.journal.record(event)
   return 502,[('Content-Type','application/json')],b'{"message":"relay upstream response unknown; reconcile intent"}'
  event.update(HTTP=status,response_headers={k:v for k,v in response_headers if k.lower() in ['content-type','content-encoding','x-request-id','cf-ray']},response_body_base64=base64.b64encode(body).decode())
  if method=='GET':
   try:event['real_region_readback']=view(body,response_headers).get('serviceDetails',{}).get('region')
   except (ValueError,KeyError,AttributeError):pass
  if method=='POST':
   r=self.intent;r['HTTP']=status;r['response_body_base64']=event['response_body_base64']
   if status==201:
    try:
     v=view(body,response_headers);assert SERVICE_ID.fullmatch(v['id']);r.update(id=v['id'],create_view=v,state='created');self.journal.own(v['id'])
    except Exception as error:r['state']='unproved201; retain raw response and reconcile';r['binding_error']=type(error).__name__
   else:r['state']='not-created' if 400<=status<500 else 'unproved response; reconcile before retry'
   self.journal.save()
  self.events.append(event);self.journal.record(event)
  # Forward real response entity bytes, status and end-to-end headers unchanged.
  return status,response_headers,body
 def reject(self,message,method,path):
  self.journal.record({'at':now(),'scope_refusal':message,'method':method,'path':path,'forwarded':False})
  return 403,[('Content-Type','application/json')],json.dumps({'message':message}).encode()
 def __enter__(self):
  relay=self
  class Handler(http.server.BaseHTTPRequestHandler):
   def log_message(self,*args):pass
   def handle_request(self):
    n=int(self.headers.get('Content-Length','0'))
    if n<0 or n>1<<20:self.send_error(413);return
    raw=self.rfile.read(n);status,headers,body=relay.dispatch(self.command,self.path,raw,self.headers)
    self.send_response(status)
    for k,v in headers:
     if k.lower() not in HOP:self.send_header(k,v)
    self.send_header('Content-Length',str(len(body)));self.end_headers();self.wfile.write(body)
   do_GET=handle_request;do_POST=handle_request;do_DELETE=handle_request;do_PATCH=handle_request;do_PUT=handle_request
  self.server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);self.server.daemon_threads=True;self.thread=threading.Thread(target=self.server.serve_forever,daemon=True);self.thread.start();self.host='http://127.0.0.1:'+str(self.server.server_port)+'/v1/';return self
 def __exit__(self,*args):self.server.shutdown();self.server.server_close();self.thread.join(timeout=2)
