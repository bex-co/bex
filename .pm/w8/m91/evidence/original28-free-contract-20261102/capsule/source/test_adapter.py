"""Meaningful fake-source journeys; no original shell/app/native or external I/O."""
import copy,datetime,json,tempfile,unittest,zlib
from pathlib import Path
from unittest.mock import patch
import adapter as a
import catalog as cat
import core as c
import fake_cases as f
from harness import Context
import readiness

HERE=Path(__file__).resolve().parent
NOW=datetime.datetime(2026,11,1,1,0,tzinfo=datetime.timezone.utc)

def catalog():return cat.Catalog((HERE/'original-inputs/prepared-fixtures.json').read_bytes(),(HERE/'EMISSION-MAP.json').read_bytes())
def post(spec):
    body={'ownerId':c.WORKSPACE,'name':spec['name'],'type':'web_service','repo':spec['primary_reference']['repo'],
        'branch':spec['primary_reference']['branch'],'autoDeploy':'no','envVars':[{'key':k,'value':v} for k,v in spec['env'].items()],
        'serviceDetails':{'runtime':spec['runtime'],'plan':'free','region':'frankfurt',
            'envSpecificDetails':{'buildCommand':spec['commands']['initial']['build'],'startCommand':spec['commands']['initial']['start']}}}
    files=cat.expected_secret_files(spec)
    if files:body['secretFiles']=files
    return body

def fixture(mid,client='bex'):
    catalog_=catalog();spec=catalog_.spec((mid,client),NOW,set());events=[]
    data={'workspace':c.WORKSPACE,'baseline_ids':['srv-baseline'],'resources':[]}
    journal=cat.Journal(data,lambda value:events.append(copy.deepcopy(value)))
    row=journal.intent(spec,{'state':'PASS','bound_evidence':{'SYNTHETIC':True}},{'state':'PASS'},['srv-baseline'])
    journal.forward_once(row,post(spec),spec);view=post(spec);view['id']='srv-owned'+str(mid)+client
    view['serviceDetails']['url']='https://'+spec['name']+'.onbex.co';journal.bind201(row,201,view)
    row['create_result']={'exit':0};sources={q.name:q.read_bytes() for q in (HERE/'public-reference').iterdir()}
    provider=f.FakeProvider(row,journal,sources);context=Context(row,provider,Path('/private/tmp/CONTROL_ONLY_FIXTURE')/str(mid))
    return spec,row,journal,provider,context,events

class OriginalProofControls(unittest.TestCase):
    def test_all28_original_threeHTTP_build_app_logs_shared_and_exact_env(self):
        for mid,client in cat.KEYS:
            with self.subTest(mid=mid,client=client):
                spec,row,journal,provider,ctx,events=fixture(mid,client)
                proof=ctx.proof();shared=ctx.shared()
                self.assertEqual(proof['independent_HTTP_proofs'],3)
                self.assertEqual(ctx.request_index,3);self.assertEqual(len([x for x in provider.events if x[0]=='HTTP']),3)
                self.assertEqual(shared['exact_readbacks'],['REST','GraphQL','MCP','own harmless version env'])
                self.assertTrue(any(event[:3]==('API','GET','/v1/services/'+row['id']+'/env-vars') for event in provider.events))
                self.assertEqual(row['marker'],spec['marker']);self.assertEqual(row['commands'],spec['commands'])
    def test_each_original_family_wrong_marker_and_missing_native_build_proof_fail(self):
        for mid in cat.MIDS:
            for mode in ('wrong-marker','build-proof'):
                with self.subTest(mid=mid,mode=mode):
                    *_,provider,ctx,events=fixture(mid);provider.mode=mode
                    with self.assertRaises((AssertionError,ValueError,KeyError)):ctx.proof()
    def test_wrong_firstLive_source_and_missing_app_request_log_are_not_pass(self):
        for mid in cat.MIDS:
            for mode in ('wrong-source','app-correlation'):
                with self.subTest(mid=mid,mode=mode):
                    *_,provider,ctx,events=fixture(mid);provider.mode=mode
                    with self.assertRaises(AssertionError):ctx.proof()
    def test_actual_m94_build_runtime_database_mismatch_is_rejected(self):
        *_,provider,ctx,events=fixture(94);provider.mode='database'
        with self.assertRaises(AssertionError):ctx.proof()
    def test_original_independent_binary_or_HTML_artifact_corruption_rejected(self):
        for mid in (93,95,96,97,98,102,107):
            with self.subTest(mid=mid):
                *_,provider,ctx,events=fixture(mid);provider.mode='artifact'
                with self.assertRaises((AssertionError,ValueError,KeyError,TypeError,OSError,zlib.error)):ctx.proof()
    def test_env_readback_is_fourth_mandatory_proof_including_original_emptyenv(self):
        for mid in cat.MIDS:
            with self.subTest(mid=mid):
                *_,provider,ctx,events=fixture(mid);provider.mode='env'
                with self.assertRaises(AssertionError):ctx.shared()
    def test_partial_original_HTTP_capture_cannot_pass_correct_stdout(self):
        *_,provider,ctx,events=fixture(91);original=provider.http
        def partial(*args):
            result=original(*args);result.update(exit_code=124,complete=False,bound='timeout');return result
        provider.http=partial
        with self.assertRaises(c.Unproved):ctx.proof()

class CatalogJournalControls(unittest.TestCase):
    def test_all_name_overlays_change_only_unused_display_fields_and_literal_flag(self):
        source=catalog()
        for key in cat.KEYS:
            fresh=source.spec(key,NOW,set());old=source.specs[key];index=fresh['create_args'].index('--name')+1
            copy_=copy.deepcopy(fresh);copy_['name']=old['name'];copy_['name_date_utc']=old['name_date_utc'];copy_['create_args'][index]=old['name']
            self.assertEqual(copy_,old);self.assertEqual(source.maps[key]['historical_source_file_identities'],old['source_file_identities'])
        with self.assertRaises(c.Unproved):source.spec(cat.KEYS[0],NOW,{cat.KEYS[0]})
        with self.assertRaises(c.Unproved):source.spec(cat.KEYS[0],NOW.replace(hour=23,minute=59),set())
    def test_fixed_spec_or_emission_map_mutations_and_unknown_catalog_never_heal(self):
        spec=(HERE/'original-inputs/prepared-fixtures.json').read_bytes();mapping=(HERE/'EMISSION-MAP.json').read_bytes()
        for left,right in [(spec+b' ',mapping),(spec,mapping+b' ')]:
            with self.assertRaises(c.Unproved):cat.Catalog(left,right)
        with self.assertRaises(c.Unproved):catalog().spec((999,'bex'),NOW,set())
    def test_once_intent_forward_201_and_cleanup_order_with_no_replay_after_success(self):
        spec,row,journal,provider,ctx,events=fixture(91)
        self.assertTrue(events[1]['resources'][0]['forwarded_once'])
        self.assertEqual(events[1]['resources'][0]['state'],'forwarded-unknown')
        with self.assertRaises(c.Unproved):journal.forward_once(row,post(spec),spec)
        with self.assertRaises(c.Unproved):journal.intent(spec,{'state':'PASS','bound_evidence':{}},{'state':'PASS'},['srv-baseline'])
        ctx.proof();journal.deleted(row['id'],404,{child:404 for child in row['children']},404)
        with self.assertRaises(c.Unproved):journal.intent(spec,{'state':'PASS','bound_evidence':{}},{'state':'PASS'},['srv-baseline'])
    def test_unknown_create_stops_all_remaining_catalog_and_max2active_is_fixed(self):
        source=catalog();data={'workspace':c.WORKSPACE,'baseline_ids':[],'resources':[]};journal=cat.Journal(data,lambda value:None)
        ready={'state':'PASS','bound_evidence':{}};free={'state':'PASS'}
        first=source.spec(cat.KEYS[0],NOW,set());row=journal.intent(first,ready,free,[]);journal.forward_once(row,post(first),first)
        with self.assertRaises(c.Unproved):journal.intent(source.spec(cat.KEYS[1],NOW,set()),ready,free,[])
        journal.bind201(row,201,dict(post(first),id='srv-owned1'))
        second=source.spec(cat.KEYS[1],NOW,set());journal.intent(second,ready,free,[])
        with self.assertRaises(c.Unproved):journal.intent(source.spec(cat.KEYS[2],NOW,set()),ready,free,[])
    def test_actual_native_body_rejects_source_commands_env_paid_and_foreign_201(self):
        for change in ('repo','build','env','paid','foreign'):
            spec,row,journal,provider,ctx,events=fixture(91);body=post(spec)
            if change=='repo':body['repo']='https://foreign.invalid/repo'
            elif change=='build':body['serviceDetails']['envSpecificDetails']['buildCommand']+='\nwrong'
            elif change=='env':body['envVars'].append({'key':'EXTRA','value':'CONTROL_ONLY'})
            elif change=='paid':body['serviceDetails']['plan']='starter'
            else:body['ownerId']='tea-foreign'
            with self.subTest(change=change),self.assertRaises(c.Unproved):cat.validate_post(body,spec)
    def test_complete_typed_quota_baseline_preserves_foreign_adds_without_global_used_zero(self):
        def snapshot(ids):return {'workspace':c.WORKSPACE,'complete':True,'ids':ids,'limits':{name:{'used':len(ids) if name=='services' else 0,'terminating':0,'limit':0,'freeUsed':0,'freeLimit':0} for name in ('services','postgres','keyValues')}}
        before=snapshot(['srv-baseline']);after=snapshot(['srv-baseline','srv-foreignnew'])
        result=c.compare_cleanup(before,after,[]);self.assertEqual(result['concurrent_additions'],['srv-foreignnew'])
        for change in ('missing-rows','untyped','unexplained-used','missing-baseline'):
            actual=copy.deepcopy(after)
            if change=='missing-rows':actual['limits']={}
            elif change=='untyped':actual['limits']['services']['used']=True
            elif change=='unexplained-used':actual['limits']['services']['used']=3
            else:actual['ids']=['srv-foreignnew'];actual['limits']['services']['used']=1
            with self.subTest(change=change),self.assertRaises(c.Unproved):c.compare_cleanup(before,actual,[])


class AdmissionControls(unittest.TestCase):
    def test_absent_or_null_adoption_refuses_before_auth_or_any_proof_reader(self):
        for contract in (None,{}, {'schema':'bex.original28.admission-contract/v1','adoption':None}):
            def forbidden(*args):raise AssertionError('no IO before actual adoption contract')
            with self.subTest(contract=contract),self.assertRaises(c.Unproved):readiness.admit({},contract,NOW,forbidden,forbidden)
    def test_concrete_null_adoption_stops_before_setup_auth_or_any_external_command(self):
        with tempfile.TemporaryDirectory(prefix='original28-no-adoption-') as directory:
            root=Path(directory).resolve();capsule=root/'capsule.json';capsule.write_text('{}')
            owned=a.Adapter(root,capsule,HERE/'ADOPTION-CONTRACT.json')
            with patch.object(owned,'setup',side_effect=AssertionError('setup must not run')) as setup, \
                 patch.object(owned,'auth',side_effect=AssertionError('auth must not run')) as auth:
                report=owned.execute()
            self.assertFalse(report['PASS']);self.assertFalse(owned.auth_started)
            setup.assert_not_called();auth.assert_not_called()

    def test_bool_ready_or_allWorkspaces_is_never_adoption_authority(self):
        for contract in ({'adoptionReady':True,'allWorkspaces':True},{'schema':'bex.original28.admission-contract/v1','adoption':{'allWorkspaces':True}}):
            with self.assertRaises(c.Unproved):readiness.admit({},contract,NOW,lambda path:None,lambda *args:True)
    def test_source_default_is_null_and_genuine_host_proof_binding_modules_unchanged(self):
        self.assertIsNone(json.loads((HERE/'ADOPTION-CONTRACT.json').read_text()))
        self.assertEqual(c.sha((HERE/'host.py').read_bytes()),'92db2b995dbecb37efc6f5da50c72a234f0adeec43fe3355b069cd04109f7b83')
        self.assertEqual(c.sha((HERE/'proof_binding.py').read_bytes()),'5bbe60a1aca6f15d11105f028b635d98337d29f23b1aa6366c66f5bddd0658e2')
    def test_utility_family_capture_observations_bind_actual_raw_stdout_not_passlabel(self):
        docs={};result={'exit_code':0,'complete':True,'bound':None,'signal':None,'command':['CONTROL_ONLY','probe']}
        raw=b'{"version":"18.2"}';ref={'path':'/private/tmp/CONTROL_ONLY.json','bytes':len(raw),'sha256':c.sha(raw)};docs[ref['path']]=raw;result['stdout']=ref
        want=[{'id':'pg-arm64','architecture':'arm64','command':result['command'],'observations':{'version':'18.2'},'inputs':{}}]
        entry={'rows':[{'id':'pg-arm64','architecture':'arm64','common_ref':'CONTROL_ONLY_COMMON','source':'e'*40,'result':result,'inputs':{}}]}
        self.assertEqual(readiness.bind_rows(entry,want,'CONTROL_ONLY_COMMON',docs.__getitem__,'utilities','e'*40)['ids'],['pg-arm64'])
        for change in ('version','command','partial','missing','othercommon'):
            actual=copy.deepcopy(entry)
            if change=='version':want_=copy.deepcopy(want);want_[0]['observations']['version']='15.2'
            else:want_=want
            if change=='command':actual['rows'][0]['result']['command']=['CONTROL_ONLY','different']
            if change=='partial':actual['rows'][0]['result']['bound']='timeout'
            if change=='missing':actual['rows']=[]
            if change=='othercommon':actual['rows'][0]['common_ref']='OTHER_COMMON'
            with self.subTest(change=change),self.assertRaises(c.Unproved):readiness.bind_rows(actual,want_,'CONTROL_ONLY_COMMON',docs.__getitem__,'utilities','e'*40)

if __name__=='__main__':unittest.main()
