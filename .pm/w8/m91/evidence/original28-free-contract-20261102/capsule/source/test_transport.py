"""Concrete controller over fake I/O plus actual tiny owned process-group controls."""
import ast,copy,json,os,signal,sys,tempfile,threading,time,types,unittest
from pathlib import Path
from unittest.mock import patch
import adapter as a
import core as c
import host
import opaque as t
import fake_cases as f
import test_adapter as fixtures

class FakeRunner:
    def stop_all(self):return []
    def capture(self,*args,**kwargs):raise AssertionError('No external command in fake journey')

REAL_RELAY=a.NativeRelay
class FakeRelay:
    def __init__(self,owner,spec,row):self.owner,self.spec,self.row=owner,spec,row;self.real=REAL_RELAY(owner,spec,row)
    def __enter__(self):self.owner.current_relay=self;self.owner.active_relays.append(self);self.host='http://127.0.0.1:1/v1/';return self
    def close(self):
        if self in self.owner.active_relays:self.owner.active_relays.remove(self)
    def __exit__(self,*args):self.close()

class FakeAdapter(a.Adapter):
    def __init__(self,root,mode='happy'):
        root=Path(root).resolve();capsule=root/'SYNTHETIC-capsule.json';contract=root/'SYNTHETIC-contract.json'
        capsule.write_text('{}');contract.write_text('{"adoption":{"SYNTHETIC_NOT_ACTUAL":true}}')
        super().__init__(root,capsule,contract,budget=host.Budget(root,free=lambda root:10**12),runner=FakeRunner())
        self.mode=mode;self.events=[];self.remote={};self.providers={};self.relay_type=FakeRelay;self.logout_done=False
    def set_handlers(self):pass
    def setup(self):
        for name in ('private','source','scratch','evidence'):(self.root/name).mkdir(mode=0o700,exist_ok=True)
    def check_ready(self):self.events.append(('SYNTHETIC-ready',));return {'state':'PASS','bound_evidence':{'SYNTHETIC_NOT_ACTUAL':True}}
    def auth(self):self.events.append(('SYNTHETIC-auth',));self.auth_started=True;self.auth_completed=True
    def free_gate(self):return {'state':'PASS','SYNTHETIC_NOT_ACTUAL':True}
    def collect(self,cleanup=False):
        ids=['srv-baseline']+sorted(sid for sid,live in self.remote.items() if live)
        limits={name:{'used':len(ids) if name=='services' else 0,'terminating':0,'limit':0,'freeUsed':0,'freeLimit':0} for name in ('services','postgres','keyValues')}
        if self.mode=='quota-drift' and self.data['resources']:limits['services']['used']+=1
        return {'workspace':c.WORKSPACE,'complete':True,'ids':ids,'names':['baseline'],'limits':limits,'limits_observed_at':'SYNTHETIC','metadata':{}}
    def persist(self,data=None):
        if self.mode=='persist-after201' and not hasattr(self,'injected_persist') and any(r.get('id') for r in self.data['resources']):
            self.injected_persist=True;raise host.Stop('CONTROL_ONLY after exact in-memory201')
        super().persist(self.data)
    def provider_for(self,row):
        if row['id'] not in self.providers:
            self.providers[row['id']]=f.FakeProvider(row,self.journal,{p.name:p.read_bytes() for p in (fixtures.HERE/'public-reference').iterdir()})
            if self.mode=='missing-log':self.providers[row['id']].mode='app-correlation'
        return self.providers[row['id']]
    def native(self,client,args,origin=None,cleanup=False,timeout=45):
        self.budget.check(cleanup)
        if args[:2]==['services','create']:
            relay=self.current_relay;body=fixtures.post(relay.spec)
            status,headers,raw=relay.real.dispatch('POST','/v1/services',f.encoded(body),{'Authorization':'Bearer CONTROL_ONLY_FAKE'})
            result=f.complete(raw)
            if self.mode=='native-timeout':result.update(exit_code=124,complete=False,bound='timeout')
            return result
        sid=args[2] if args[0]=='deploys' else args[args.index('--resources')+1]
        return self.provider_for(self.journal.own(sid)).native(client,args)
    def https(self,method,url,headers,body=None,cleanup=False,maximum=c.MAX_METADATA,on_response=None):
        if self.mode=='unknown-create':raise host.Stop('CONTROL_ONLY unknown forwarded POST')
        row=self.current_relay.row;view=json.loads(body);view['id']='srv-'+str(row['mid'])+row['client']
        view['serviceDetails']['url']='https://'+row['name']+'.onbex.co';self.remote[view['id']]=True
        raw=f.encoded(view)
        if on_response:on_response(201,raw)
        if self.mode=='guard-after201':self.budget.error='CONTROL_ONLY observer stop'
        return {'status':201,'headers':{'content-type':'application/json'},'raw':raw,'body':raw,'transport':f.complete()}
    def api(self,method,path,body=None,cleanup=False):
        c.require(self.allowed_api(method,path,body,cleanup),'fake exercises actual API scope guard')
        self.events.append(('API',method,path,cleanup))
        if method=='DELETE':
            sid=path.rsplit('/',1)[-1];self.journal.own(sid);self.remote[sid]=False
            return {'status':204,'raw':b'','body':b'','event':{'at':'SYNTHETIC'},'transport':f.complete()}
        sid=next((sid for sid in self.remote if sid in path),None)
        if sid and not self.remote[sid]:return {'status':404,'body':b'{}','raw':b'{}','event':{'at':'SYNTHETIC'},'transport':f.complete()}
        if path.endswith('/deploys?limit=20'):
            raw=f.encoded([{'id':'dep-first123','status':'live','commit':{'id':self.journal.own(sid)['primary_reference']['commit']}}])
            return {'status':200,'body':raw,'raw':raw,'event':{'at':'SYNTHETIC'},'transport':f.complete()}
        if path=='/graphql':sid=next(sid for sid in self.remote if sid in body['query'])
        if path=='/mcp':sid=body['params']['arguments']['serviceId']
        return self.provider_for(self.journal.own(sid)).api(method,path,body)
    def public(self,row,argv,cleanup=False):
        if cleanup:return f.complete(b'CONTROL_ONLY absent\n404')
        provider=self.provider_for(row);index=len([x for x in provider.events if x[0]=='HTTP'])
        return provider.http(argv,row,provider.request_path(index),index)
    def reference(self,url,reference):return (fixtures.HERE/'public-reference'/url.rsplit('/',1)[1]).read_bytes()
    def logout(self):
        c.require(all(r['state'] in ('deleted','not-created') for r in self.data['resources']),'retain own fake auth for unknown reconciliation')
        self.logout_done=True;self.events.append(('SYNTHETIC-logout',))

class TransportControls(unittest.TestCase):
    def test_concrete_orchestration_all28_once_proofs_env_and_exact_cleanup(self):
        with tempfile.TemporaryDirectory(prefix='original28-fake-') as directory:
            owned=FakeAdapter(Path(directory));report=owned.execute()
            self.assertTrue(report['PASS'],report);self.assertEqual(len(report['resources']),28)
            self.assertTrue(all(r['state']=='deleted' for r in report['resources']));self.assertTrue(owned.logout_done)
            self.assertFalse(any(owned.remote.values()));self.assertEqual(len(owned.providers),28)
            for provider in owned.providers.values():
                self.assertEqual(len([x for x in provider.events if x[0]=='HTTP']),3)
                self.assertTrue(any(x[0]=='API' and x[2].endswith('/env-vars') for x in provider.events))
    def test_actual_native_request_wire_is_complete_durable_before_forward_and_auth_redacted(self):
        import base64,gzip
        with tempfile.TemporaryDirectory(prefix='original28-wire-') as directory:
            owned=FakeAdapter(Path(directory),'native-timeout');report=owned.execute()
            self.assertFalse(report['PASS']);row=owned.data['resources'][0];wire=row['wire_request']
            raw=gzip.decompress(base64.b64decode(wire['raw_gzip_base64']))
            self.assertEqual(c.sha(raw),wire['sha256']);self.assertEqual(len(raw),wire['bytes'])
            self.assertEqual(json.loads(raw),row['request']);self.assertEqual(wire['authorization'],'[REDACTED]')
            self.assertNotIn('CONTROL_ONLY_FAKE',json.dumps(wire));self.assertEqual(row['state'],'deleted')

    def test_timeout_missing_log_guard_or_persist_failure_still_cleans_exact201_before_logout(self):
        for mode in ('native-timeout','missing-log','guard-after201','persist-after201','quota-drift'):
            with self.subTest(mode=mode),tempfile.TemporaryDirectory(prefix='original28-failure-') as directory:
                owned=FakeAdapter(Path(directory),mode);report=owned.execute()
                self.assertFalse(report['PASS']);self.assertTrue(owned.logout_done);self.assertFalse(any(owned.remote.values()))
                self.assertEqual(len(owned.data['resources']),1)
    def test_unknown_forwarded_outcome_stops_catalog_no_inferred_delete_or_logout(self):
        with tempfile.TemporaryDirectory(prefix='original28-unknown-') as directory:
            owned=FakeAdapter(Path(directory),'unknown-create');report=owned.execute()
            self.assertFalse(report['PASS']);self.assertEqual(len(owned.data['resources']),1)
            self.assertEqual(owned.data['resources'][0]['state'],'forwarded-unknown');self.assertFalse(owned.logout_done)
            self.assertFalse(any(x[:2]==('API','DELETE') for x in owned.events))
    def test_late_monitor_error_after_cleanup_and_logout_prevents_finalPASS(self):
        with tempfile.TemporaryDirectory(prefix='original28-late-') as directory:
            owned=FakeAdapter(Path(directory));close=owned.budget.close
            def late():close();return 'CONTROL_ONLY late monitor'
            owned.budget.close=late;report=owned.execute()
            self.assertFalse(report['PASS']);self.assertTrue(owned.logout_done);self.assertFalse(any(owned.remote.values()))
            self.assertTrue(any('late monitor' in x for x in report['cleanup_errors']))
    def test_actual_API_scope_refuses_foreign_and_mutation_read_lookalikes(self):
        with tempfile.TemporaryDirectory(prefix='original28-scope-') as directory:
            owned=FakeAdapter(Path(directory));owned.data['baseline_ids']=[];owned.journal=__import__('catalog').Journal(owned.data,owned.persist)
            for method,path in [('DELETE','/v1/services/srv-foreign'),('PATCH','/v1/services/srv-foreign'),('GET','/v1/services/srv-foreign/env-vars'),('POST','/v1/services')]:
                with self.subTest(method=method,path=path):
                    try:allowed=owned.allowed_api(method,path,None,True)
                    except c.Unproved:allowed=False
                    self.assertFalse(allowed)


class HostControls(unittest.TestCase):
    def make(self):
        temporary=tempfile.TemporaryDirectory(prefix='m83-host-control-');self.addCleanup(temporary.cleanup)
        budget=host.Budget(Path(temporary.name).resolve(),free=lambda root:10**12)
        runner=host.Runner(budget);self.addCleanup(runner.stop_all)
        return budget,runner

    def test_exited_leader_pipeholder_timeout_reaps_exact_owned_group(self):
        budget,runner=self.make()
        script='import os,time\np=os.fork()\nif p==0:time.sleep(30)\nelse:print(p,flush=True);os._exit(0)\n'
        result=runner.capture([sys.executable,'-c',script],timeout=.2,limit=1024)
        self.assertEqual(result['bound'],'timeout')
        self.assertTrue(result['group_cleanup']['group_absent'])
        self.assertFalse(host.group_exists(result['group_cleanup']['pgid']))
        child=int(result['stdout'].decode().strip())
        with self.assertRaises(ProcessLookupError):os.kill(child,0)

    def test_normal_exit_with_background_descendant_is_stopped_and_not_success(self):
        budget,runner=self.make()
        script='import os,time\np=os.fork()\nif p==0:\n os.close(1);os.close(2);time.sleep(30)\nelse:os._exit(0)\n'
        result=runner.capture([sys.executable,'-c',script],timeout=2,limit=1024)
        self.assertEqual(result['bound'],'unexpected-background-descendant')
        self.assertTrue(result['group_cleanup']['group_absent'])

    def test_output_and_resource_stop_have_no_leaked_owned_process_group(self):
        for mode in ('output','resource'):
            budget,runner=self.make()
            if mode=='resource':
                original=budget.check;calls=[0]
                def check(cleanup=False):
                    calls[0]+=1
                    if calls[0]>2 and not cleanup:raise host.Stop('CONTROL_ONLY guard')
                    return original(cleanup)
                budget.check=check
            script='import sys,time\nsys.stdout.write("x"*20000);sys.stdout.flush();time.sleep(30)'
            result=runner.capture([sys.executable,'-c',script],timeout=2,limit=128)
            self.assertIn(result['bound'],('output','resource'))
            self.assertTrue(result['group_cleanup']['group_absent'])

    def test_budget_close_reports_inflight_late_monitor_error(self):
        budget,_=self.make();entered,release=threading.Event(),threading.Event()
        def check(cleanup=False):
            entered.set();release.wait(1);raise host.Stop('CONTROL_ONLY in-flight late error')
        budget.check=check;budget.start();self.assertTrue(entered.wait(1))
        closed=[]
        thread=threading.Thread(target=lambda:closed.append(budget.close()));thread.start()
        release.set();thread.join(2)
        self.assertFalse(thread.is_alive())
        self.assertIn('in-flight late error',closed[0])

    def test_one_second_observer_detects_free_floor_and_cleanup_capture_remains_possible(self):
        budget,runner=self.make();available=[10**12]
        budget.free=lambda root:available[0];budget.start()
        deadline=time.monotonic()+1
        while not (budget.root/'evidence/capacity.jsonl').exists() and time.monotonic()<deadline:time.sleep(.01)
        available[0]=host.FLOOR+host.MARGIN
        deadline=time.monotonic()+1.5
        while budget.error is None and time.monotonic()<deadline:time.sleep(.01)
        self.assertIsNotNone(budget.error)
        result=runner.capture([sys.executable,'-c','print("CONTROL_ONLY cleanup")'],timeout=2,cleanup=True)
        self.assertEqual(result['exit_code'],0)
        self.assertTrue(result['group_cleanup']['group_absent'])
        budget.close()

    def test_exact_own_jar_disappearance_during_logout_is_not_a_monitor_error(self):
        budget,_=self.make()
        jar=budget.root/'qa-session-0123456789ab.jar';jar.write_text('CONTROL_ONLY_COOKIE');jar.chmod(0o600)
        budget.register_jar(jar,budget.root)
        original=Path.lstat
        def observed(path):
            if path==jar:raise FileNotFoundError('CONTROL_ONLY concurrent own unlink')
            return original(path)
        with patch.object(Path,'lstat',observed):
            counts,available=budget.observation()
        self.assertGreater(available,host.START_FREE)
