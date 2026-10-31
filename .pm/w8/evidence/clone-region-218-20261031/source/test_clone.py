"""LOCAL SYNTHETIC controls only: never proof of a hosted resource or readiness."""
from pathlib import Path
import base64,copy,gzip,hashlib,json,os,tempfile,time,unittest
from unittest import mock
import yaml
import client_primitives as client
import driver
from clone_scope import Journal,Relay,view

PUBLIC=Path(__file__).resolve().parent
CLIENT_CASES=[]
def capture(result):CLIENT_CASES.append({'scope':'LOCAL_SYNTHETIC_ONLY_NO_HOSTED_PROOF',**result})

def service(sid='srv-00000000000000000218',name='synthetic-source',region='frankfurt'):
 return {'id':sid,'name':name,'ownerId':driver.WORKSPACE,'type':'web_service','imagePath':driver.IMAGE,'autoDeploy':'no','createdAt':'2026-10-31T00:00:00Z','updatedAt':'2026-10-31T00:00:00Z','rootDir':'','slug':name,'suspended':'not_suspended','suspenders':[],'serviceDetails':{'region':region,'plan':'free','env':'image','runtime':'image','buildPlan':'starter','numInstances':1,'healthCheckPath':'','url':'https://synthetic.invalid'}}

def owned(j,role='source',sid='srv-00000000000000000218',region='frankfurt'):
 r=j.start(role,driver.name(j,role),'bex');v=service(sid,r['name'],region);r.update(id=sid,HTTP=201,forwarded_once=True,create_view=v,state='created');j.save();return r

def body(intent):return json.dumps({'name':intent['name'],'ownerId':driver.WORKSPACE,'type':'web_service','image':{'imagePath':driver.IMAGE},'serviceDetails':{'plan':'free','runtime':'image','region':'frankfurt'}}).encode()

class FakeAPI:
 def __init__(self,source=None):self.source=source;self.calls=[];self.posts=[];self.fail=False
 def request(self,method,path,raw,headers):
  self.calls.append((method,path,raw));assert headers.get('Authorization')=='Bearer LOCAL-SYNTHETIC-ONLY'
  if self.fail:raise TimeoutError('synthetic uncertain response')
  if method=='GET':return 200,[('Content-Type','application/json')],json.dumps(self.source,separators=(',',':')).encode()
  if method=='POST':
   b=json.loads(raw);self.posts.append(b)
   if b['serviceDetails'].get('runtime')!='image' or b.get('repo'):
    return 400,[('Content-Type','application/json')],b'{"message":"runtime docker requires repo and no image"}'
   v=service('srv-'+hashlib.sha256(b['name'].encode()).hexdigest()[:20],b['name']);v['serviceDetails']['region']=b['serviceDetails']['region'];return 201,[('Content-Type','application/json')],json.dumps({'service':v,'deployId':'dep-synthetic'}).encode()
  if method=='DELETE':return 204,[],b''
  raise AssertionError('unsupported synthetic method')

class Controls(unittest.TestCase):
 def setUp(self):
  self.temp=tempfile.TemporaryDirectory(prefix='w8-218-SYNTHETIC-');self.p=Path(self.temp.name);self.j=Journal(self.p,driver.WORKSPACE);self.j.data['nonce']='localsynthetic';self.j.save()
  (self.p/'credentials').mkdir();(self.p/'release').mkdir();(self.p/'release/bex').symlink_to(PUBLIC/'release/bex');(self.p/'render-pin').symlink_to(PUBLIC/'render-pin')
  (self.p/'credentials/cli.yaml').write_text(yaml.safe_dump({'version':2,'api':{'key':'LOCAL-SYNTHETIC-ONLY','refreshtoken':'LOCAL-SYNTHETIC-NO-REFRESH','expires_at':int(time.time())+7200,'host':'http://127.0.0.1:1/v1/'}}));(self.p/'credentials/cli.yaml').chmod(0o600)
  client.configure(self.p,driver.WORKSPACE,self.j.data)
  self.no_external=mock.patch.dict(os.environ,{'HTTP_PROXY':'http://127.0.0.1:1','HTTPS_PROXY':'http://127.0.0.1:1','ALL_PROXY':'http://127.0.0.1:1','NO_PROXY':'127.0.0.1,localhost'});self.no_external.start()
  self.no_live=mock.patch('clone_scope.FixedAPI.request',side_effect=AssertionError('live request prohibited in local controls'));self.no_live.start()
 def tearDown(self):self.no_live.stop();self.no_external.stop();self.temp.cleanup()
 def test_ready_missing_refuses_without_auth_or_network(self):
  with self.assertRaises(FileNotFoundError):driver.check_ready(self.p)
  self.assertFalse((self.p/'api-ready.json').exists())
 def test_scope_foreign_secret_encoded_query_and_methods_refused(self):
  r=owned(self.j);api=FakeAPI(r['create_view']);relay=Relay(self.j,'invalid',backend=api)
  for method,path in [('GET','/v1/services/srv-foreign'),('GET','/v1/services/'+r['id']+'/env-vars'),('GET','/v1/services/'+r['id']+'?x=1'),('GET','/v1/services/%73rv-synthetic218source'),('DELETE','/v1/services/'+r['id']),('PATCH','/v1/services/'+r['id'])]:self.assertEqual(relay.dispatch(method,path,b'',{})[0],403)
  self.assertEqual(api.calls,[])
 def test_raw_gzip_region_bytes_preserved(self):
  r=owned(self.j);raw=gzip.compress(json.dumps(service(r['id'],r['name'],'fsn1'),indent=3).encode());headers=[('Content-Type','application/json'),('Content-Encoding','gzip'),('X-Request-ID','synthetic-request')]
  api=mock.Mock();api.request.return_value=(200,headers,raw);relay=Relay(self.j,'invalid',backend=api)
  status,actual_headers,actual=relay.dispatch('GET','/v1/services/'+r['id'],b'',{})
  self.assertEqual((status,actual_headers,actual),(200,headers,raw));self.assertEqual(relay.events[-1]['real_region_readback'],'fsn1')
 def test_two_resource_peak_and_cleanup_order(self):
  source=owned(self.j);clone=owned(self.j,'clone-bex','srv-synthetic218clone')
  with self.assertRaises(AssertionError):self.j.start('clone-render','new','render')
  relay=Relay(self.j,'cleanup',cleanup_id=source['id'],backend=FakeAPI())
  with self.assertRaises(AssertionError):relay.dispatch('DELETE','/v1/services/'+source['id'],b'',{})
  clone['state']='deleted';self.j.save();self.j.start('clone-render','new','render')
 def test_uncertain_post_never_replayed(self):
  owned(self.j);intent=self.j.start('clone-bex','synthetic-clone','bex');api=FakeAPI();api.fail=True;relay=Relay(self.j,'clone',intent,backend=api)
  self.assertEqual(relay.dispatch('POST','/v1/services',body(intent),{'Authorization':'Bearer LOCAL-SYNTHETIC-ONLY'})[0],502)
  self.assertEqual(relay.dispatch('POST','/v1/services',body(intent),{})[0],403);self.assertEqual(len(api.calls),1)
  with self.assertRaises(AssertionError):self.j.start('clone-bex','again','bex')
 def test_paid_disk_secret_wrongimage_refused_before_forward(self):
  owned(self.j);intent=self.j.start('clone-bex','synthetic-clone','bex');api=FakeAPI();relay=Relay(self.j,'clone',intent,backend=api)
  for change in ('paid','disk','secret','image'):
   b=json.loads(body(intent))
   if change=='paid':b['serviceDetails']['plan']='starter'
   if change=='disk':b['serviceDetails']['disk']={'sizeGB':1}
   if change=='secret':b['secretFiles']=[{'name':'forbidden'}]
   if change=='image':b['image']['imagePath']='foreign.invalid/image'
   self.assertEqual(relay.dispatch('POST','/v1/services',json.dumps(b).encode(),{})[0],403)
  self.assertEqual(api.calls,[]);self.assertFalse(intent['forwarded_once'])
 def test_201_wrong_owner_is_retained_unproved_not_owned(self):
  owned(self.j);intent=self.j.start('clone-bex','synthetic-clone','bex');v=service('srv-syntheticwrong',intent['name']);v['ownerId']='tea-foreign'
  raw=json.dumps({'service':v}).encode();api=mock.Mock();api.request.return_value=(201,[('Content-Type','application/json')],raw)
  status,_,actual=Relay(self.j,'clone',intent,backend=api).dispatch('POST','/v1/services',body(intent),{})
  self.assertEqual((status,actual),(201,raw));self.assertIn('unproved201',intent['state'])
  with self.assertRaises(AssertionError):self.j.own(v['id'])
 def test_cleanup_needs_exact_id404_not_delete_ack(self):
  source=owned(self.j);clone=owned(self.j,'clone-bex','srv-00000000000000000001');api=FakeAPI(source['create_view'])
  fake_cli={'client':'bex','command':['bex','services','delete',clone['id']],'exit':0,'stdout':'','stderr':''}
  with mock.patch.object(client,'private_token',return_value='LOCAL-SYNTHETIC-ONLY'),mock.patch.object(client,'api',side_effect=[{'HTTP':'200','response':json.dumps(clone['create_view'])},{'HTTP':'404','response':'{}'}]) as reads,mock.patch('driver.time.sleep'):
   driver.cleanup(self.j,'clone-bex',api)
  self.assertEqual(clone['absence']['HTTP'],404);self.assertEqual([c.args for c in reads.call_args_list],[('GET','/v1/services/'+clone['id'])]*2)
 def test_cleanup_unknown_does_not_mark_deleted(self):
  owned(self.j);clone=owned(self.j,'clone-bex','srv-00000000000000000001');api=FakeAPI()
  with mock.patch.object(client,'private_token',return_value='LOCAL-SYNTHETIC-ONLY'),mock.patch.object(client,'api',return_value={'HTTP':'503','response':'{}'}):
   with self.assertRaises(AssertionError):driver.cleanup(self.j,'clone-bex',api)
  self.assertEqual(clone['state'],'created');self.assertNotIn('absence',clone)
 def test_ready_wrong_source_unsigned_false_or_wrongproof_refuses(self):
  proof=self.p/'SYNTHETIC-proof.json';proof.write_text('{"synthetic":true}')
  base={'requiredAncestor':driver.REQUIRED_API,'source':driver.REQUIRED_API,'ready':True,'signedReady':True,'image':'synthetic.invalid/api@sha256:'+'0'*64,'readinessProof':{'path':str(proof),'sha256':driver.sha(proof)}}
  for field,value in [('requiredAncestor','0'*40),('ready',False),('signedReady',False),('source','not-a-source'),('readinessProof',{'path':str(proof),'sha256':'0'*64})]:
   fixture=copy.deepcopy(base);fixture[field]=value;(self.p/'api-ready.json').write_text(json.dumps(fixture))
   with self.assertRaises(AssertionError):driver.check_ready(self.p)
  (self.p/'api-ready.json').write_text(json.dumps(base))
  with mock.patch('driver.subprocess.run',return_value=mock.Mock(returncode=1)):
   with self.assertRaises(AssertionError):driver.check_ready(self.p)
 def test_actual_bex_fixed_free_source_and_owned_delete(self):
  api=FakeAPI();created=driver.create(self.j,'bex','source',api);capture(created);source=self.j.source()
  self.assertEqual(api.posts[0]['serviceDetails']['plan'],'free');self.assertEqual(api.posts[0]['image']['imagePath'],driver.IMAGE)
  self.assertEqual(api.posts[0]['serviceDetails']['runtime'],'image');self.assertFalse(api.posts[0].get('repo'))
  api.source=source['create_view']
  relay=Relay(self.j,'cleanup',cleanup_id=source['id'],backend=api);status,_,_=relay.dispatch('DELETE','/v1/services/'+source['id'],b'',{'Authorization':'Bearer LOCAL-SYNTHETIC-ONLY'})
  self.assertEqual(status,204);self.assertEqual(api.calls[-1][:2],('DELETE','/v1/services/'+source['id']))
 def check_prebuilt_source_readback_clone(self,who):
  api=FakeAPI();created=driver.create(self.j,who,'source',api);capture(created);source=self.j.source()
  self.assertEqual(created['exit'],0)
  self.assertEqual(created['command'][created['command'].index('--runtime')+1],'image')
  self.assertEqual(len(api.posts),1)
  api.source=source['create_view']
  with Relay(self.j,'invalid',backend=api) as relay:
   status,headers,raw=relay.dispatch('GET','/v1/services/'+source['id'],b'',{'Authorization':'Bearer LOCAL-SYNTHETIC-ONLY'})
  observed=view(raw,headers);self.assertEqual(status,200);self.assertEqual(observed,source['create_view'])
  self.assertEqual(observed['serviceDetails']['env'],'image');self.assertEqual(observed['serviceDetails']['runtime'],'image')
  self.assertNotIn('envSpecificDetails',observed['serviceDetails'])
  clone=driver.create(self.j,who,'clone-'+who,api);capture(clone)
  self.assertEqual(clone['exit'],0);self.assertNotIn('--region',clone['command']);self.assertEqual(len(api.posts),2)
  for payload in api.posts:
   self.assertEqual(payload['ownerId'],driver.WORKSPACE);self.assertEqual(payload['type'],'web_service')
   self.assertEqual(payload['serviceDetails']['plan'],'free');self.assertEqual(payload['serviceDetails']['runtime'],'image')
   self.assertEqual(payload['serviceDetails']['region'],'frankfurt');self.assertFalse(payload.get('repo'))
   self.assertEqual(payload['image']['imagePath'],driver.IMAGE)
 def test_actual_bex_prebuilt_image_source_readback_clone(self):
  self.check_prebuilt_source_readback_clone('bex')
 def test_actual_render_prebuilt_image_source_readback_clone(self):
  self.check_prebuilt_source_readback_clone('render')
 def test_original_docker_image_setup_is_definitive400_not_region(self):
  intent=self.j.start('source',driver.name(self.j,'source'),'bex');api=FakeAPI()
  args=['services','create','--name',intent['name'],'--type','web_service','--runtime','docker','--image',driver.IMAGE,'--plan','free','--region','frankfurt','--confirm','-o','json']
  with Relay(self.j,'source',intent,backend=api) as relay:
   result=client.cli(args,'bex',host=relay.host)
  capture(result);self.assertNotEqual(result['exit'],0)
  self.assertIn('runtime docker requires repo and no image',result['stderr']+result['stdout'])
  self.assertEqual(intent['HTTP'],400);self.assertEqual(intent['state'],'not-created');self.assertNotIn('id',intent)
  self.assertEqual(len(api.posts),1);self.assertEqual(api.posts[0]['serviceDetails']['runtime'],'docker')
 def test_reconcile_unforwarded_intent_is_resource_free(self):
  intent=self.j.start('source','synthetic-source','bex')
  with mock.patch.object(client,'api',side_effect=AssertionError('unforwarded intent needs no API')):out=driver.reconcile(self.j)
  self.assertEqual(intent['state'],'not-created');self.assertEqual(out['unknownIntents'],[])
 def test_actual_clients_preserve_old_region_refusal(self):
  source=owned(self.j,region='fsn1')
  for who in ('bex','render'):
   with self.subTest(client=who):
    api=FakeAPI(source['create_view'])
    with Relay(self.j,'invalid',backend=api) as relay:r=client.cli(['services','create','--from',source['id'],'--name','synthetic-clone','--confirm','-o','json'],who,host=relay.host)
    capture(r);self.assertNotEqual(r['exit'],0,r);self.assertIn('region must be one of',r['stdout']+r['stderr']);self.assertEqual(api.posts,[]);self.assertEqual(relay.events[-1]['real_region_readback'],'fsn1')
 def test_actual_clients_no_region_clone_and_invalid_control(self):
  source=owned(self.j)
  for who in ('bex','render'):
   with self.subTest(client=who):
    api=FakeAPI(source['create_view']);r=driver.create(self.j,who,'clone-'+who,api)
    capture(r);self.assertEqual(r['exit'],0);self.assertNotIn('--region',r['command']);self.assertEqual(api.posts[0]['serviceDetails']['region'],'frankfurt')
    capture(driver.invalid(self.j,who,api));self.assertEqual(len(api.posts),1)
    clone=self.j.own(next(x['id'] for x in self.j.data['resources'] if x['role']=='clone-'+who));clone['state']='deleted';self.j.save()

if __name__=='__main__':
 try:unittest.main(verbosity=2)
 finally:(PUBLIC/'synthetic-client-cases.json').write_text(json.dumps(CLIENT_CASES,indent=2)+'\n')
