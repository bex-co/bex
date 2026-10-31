"""Pure fake-transport checks; no production HTTP, credential or resource use."""
from pathlib import Path
import ast,copy,datetime,hashlib,json,tempfile,time,unittest,urllib.request
from unittest.mock import Mock
SOURCE=Path('/private/tmp/bex7-m144-disk-acceptance-n6d16nzf/hosted-disk-observed-v2.py');TREE=ast.parse(SOURCE.read_text());URL='https://qa-20261031-w8-m144-cc6-fd2d4d.onbex.co';SID='srv-dbit2iat5fis7381jscg';DEP='dep-dbit2iat5fis7381jsd0';COMMIT='6ecf0ef2c2dffb796033e5a02219af86ec6584e5'
def compiled(names,ns):
 body=[n for n in TREE.body if isinstance(n,ast.FunctionDef) and n.name in names];exec(compile(ast.Module(body=body,type_ignores=[]),str(SOURCE),'exec'),ns);return ns
class Guards(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory(prefix='m144-v2-SYNTHETIC-');self.addCleanup(self.tmp.cleanup);p=Path(self.tmp.name);(p/'credentials').mkdir();(p/'credentials/fixture-token').write_text('LOCAL-FAKE-TOKEN');(p/'receipts').mkdir();self.service={'id':SID,'ownerId':'tea-synthetic','name':'synthetic-name','serviceDetails':{'url':URL}};raw=json.dumps({'service':self.service});self.ledger={'create_proof':{'HTTP':201,'response':raw,'response_sha256':hashlib.sha256(raw.encode()).hexdigest()},'create_intent':{'created_at':'2026-10-31T11:30:14Z'}};self.opener=Mock();reply=Mock();reply.code=200;reply.read.return_value=b'{"run":"synthetic"}';reply.headers.get.return_value=None;self.opener.open.return_value=reply
  self.ns=compiled({'request','now','scrub'},{'P':p,'S':{'name':'synthetic-name','run':'synthetic'},'L':self.ledger,'WS':'tea-synthetic','json':json,'datetime':datetime,'hashlib':hashlib,'time':time,'urllib':__import__('urllib'),'OPENER':self.opener,'own':lambda:copy.deepcopy(self.service)})
 def test_exact_origin_keeps_method_path_body_auth_scrub(self):
  r=self.ns['request']('POST','/fill',{},fixture=True);sent=self.opener.open.call_args.args[0];self.assertEqual(sent.full_url,URL+'/fill');self.assertEqual(sent.method,'POST');self.assertEqual(sent.data,b'{}');self.assertEqual(sent.get_header('Authorization'),'Bearer LOCAL-FAKE-TOKEN');self.assertEqual(r['request']['authorization'],'[REDACTED]')
 def test_foreign_current_url_or_sid_refuses_before_fixture_http(self):
  for key,value in [('url','https://foreign.onbex.co'),('url',URL+'/'),('id','srv-foreign')]:
   current=copy.deepcopy(self.service)
   if key=='url':current['serviceDetails']['url']=value
   else:current['id']=value
   self.ns['own']=lambda current=current:current
   with self.subTest(key=key,value=value),self.assertRaises(AssertionError):self.ns['request']('POST','/fill',{},fixture=True)
  self.opener.open.assert_not_called()
 def test_wrong201_host_userinfo_port_query_fragment_refuses_before_http(self):
  for bad in ['https://foreign.onbex.co','http://qa-20261031-w8-m144-cc6-fd2d4d.onbex.co',URL+':443',URL+'?x=1',URL+'#fragment','https://user@qa-20261031-w8-m144-cc6-fd2d4d.onbex.co']:
   created=copy.deepcopy(self.service);created['serviceDetails']['url']=bad;raw=json.dumps({'service':created});self.ledger['create_proof'].update(response=raw,response_sha256=hashlib.sha256(raw.encode()).hexdigest());self.ns['own']=lambda created=created:created
   with self.subTest(origin=bad),self.assertRaises(AssertionError):self.ns['request']('GET','/status',fixture=True)
  self.opener.open.assert_not_called()
 def observation(self,history,direct,extra=None):
  ns={'own':lambda:copy.deepcopy(self.service),'value':lambda r:json.loads(r['response']),'request':lambda method,path:{'HTTP':200,'response':json.dumps(history if '?limit=' in path else direct),'response_sha256':'synthetic'},'S':{'expected_source_commit':COMMIT},'L':copy.deepcopy(self.ledger),'instant':lambda s:datetime.datetime.fromisoformat(s.replace('Z','+00:00')),'now':lambda:'2026-10-31T11:40:00+00:00','save':Mock(),'json':json};ns['L'].update(extra or {});return compiled({'observe'},ns)
 def test_observe_positive_is_synthetic_only(self):
  d={'id':DEP,'serviceId':SID,'commit':{'id':COMMIT},'createdAt':'2026-10-31T11:30:17Z','status':'live'};ns=self.observation([{'deploy':d}],d);self.assertEqual(ns['observe']()['first_deploy'],DEP);self.assertEqual(ns['L']['initial_deploy_binding']['first_deploy_id'],DEP)
 def test_observe_rejects_cardinality_source_sid_notlive_and_followup(self):
  d={'id':DEP,'serviceId':SID,'commit':{'id':COMMIT},'createdAt':'2026-10-31T11:30:17Z','status':'live'}
  cases=[([],d,{}),([d,d],d,{}),([{**d,'serviceId':'srv-foreign'}],d,{}),([{**d,'commit':{'id':'wrong'}}],d,{}),([d],{**d,'status':'build_in_progress'},{}),([d],{**d,'id':'dep-foreign'},{}),([d],d,{'before_redeploy':{'intent':True}})]
  for history,direct,extra in cases:
   ns=self.observation(history,direct,extra)
   with self.subTest(history=history,direct=direct,extra=extra),self.assertRaises(AssertionError):ns['observe']()
   self.assertNotIn('source_verified_at',ns['L']);ns['save'].assert_not_called()
 def test_original_functions_are_byte_identical_except_two(self):
  original=SOURCE.with_name('hosted-disk.py').read_text();v2=SOURCE.read_text();a={n.name:ast.get_source_segment(original,n) for n in ast.parse(original).body if isinstance(n,ast.FunctionDef)};b={n.name:ast.get_source_segment(v2,n) for n in TREE.body if isinstance(n,ast.FunctionDef)};self.assertEqual(a.keys(),b.keys());self.assertEqual([n for n in a if a[n]!=b[n]],['request','observe']);self.assertEqual(hashlib.sha256(original.encode()).hexdigest(),'1ee808d858ce704907d9000c4f9de6830d63cb43bebc4b9eccb7b8fb60ab7953')
if __name__=='__main__':unittest.main(verbosity=2)
