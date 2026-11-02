"""Adoption contracts differ from retained qualification baseline; fake proof I/O."""
import copy,json,unittest
from unittest.mock import patch
import core as c
import readiness as r
import proof_binding as p

class AdoptionControls(unittest.TestCase):
    def fixture(self):
        old='ghcr.io/bex-co/native@sha256:'+'a'*64;new='ghcr.io/bex-co/native@sha256:'+'b'*64
        source='d'*40;path=r.COMMON_SLOT
        before=('package build\nvar nativeCommonBase = '+json.dumps(old)+'\nfunc choose(workspace string) string { return nativeCommonBase }\n').encode()
        after=before.replace(old.encode(),new.encode());body=b'func choose(workspace string) string { return nativeCommonBase }\n'
        docs={}
        def ref(name,raw):
            key='/private/tmp/CONTROL_ONLY_ADOPTION/'+name+'.json';docs[key]=raw;return {'path':key,'bytes':len(raw),'sha256':c.sha(raw)}
        contract={'schema':'bex.original28.adoption-contract/v1','source':source,'required_ancestor':'e'*40,
            'operator_image':'ghcr.io/bex-co/bex-operator@sha256:'+'f'*64,
            'critical_files':{path:{'bytes':len(after),'sha256':c.sha(after),'git_mode':'100644','git_blob':'f'*40}},
            'pin_only_deltas':[{'qualification_source':'c'*40,'path':path,'before':ref('baseline',before),'edits':[{'before':old,'after':new}]}],
            'pins':[{'path':path,'qualified_ref':new,'literal':json.dumps(new)}],
            'scope_code':[{'scope':'all-workspaces','path':path,'first_byte':after.index(body),'last_byte':len(after),'sha256':c.sha(body),'source':ref('scope',body)}]}
        qualified={'qualified_baseline_source':'c'*40,'common_ref':new,'qualification_source':{path:{'bytes':len(before),'sha256':c.sha(before)}},'adoptable_refs':[new],'critical_source':{path:{'bytes':len(before),'sha256':c.sha(before)}},
            'critical_scope':[{'directory':'lego/operator/internal/build','suffix':'.go','exclude_suffixes':['_test.go']}]}
        entry={'source':source,'image':contract['operator_image'],'APIApproval':{'source':'a'*40,'image':'ghcr.io/bex-co/bex-operator@sha256:'+'9'*64}}
        calls=[]
        def git(action,left,right):
            calls.append((action,left,right))
            if action=='ancestor':return True
            if action=='changes':return [{'status':'M','path':path}]
            return (before,'100644','e'*40) if left==qualified['qualified_baseline_source'] else (after,'100644','f'*40)
        return contract,qualified,entry,docs,git,calls
    def test_future_pinned_adoption_allows_exact_reviewed_pin_only_delta_without_requalifying_current_Go(self):
        contract,qualified,entry,docs,git,calls=self.fixture()
        with patch.object(p,'bind_signature',return_value={'CONTROL_ONLY_signature':True}):
            result=r.adoption(contract,entry,qualified,docs.__getitem__,git)
        self.assertEqual(result['source'],contract['source']);self.assertNotEqual(result['source'],qualified['qualified_baseline_source'])
        self.assertEqual(calls,[('ancestor',contract['required_ancestor'],contract['source']),('blob',qualified['qualified_baseline_source'],next(iter(contract['critical_files']))),('blob',contract['source'],next(iter(contract['critical_files']))),('changes',qualified['qualified_baseline_source'],contract['source'])])
    def test_extra_runtime_function_change_is_not_hidden_as_pin_only_even_if_criticalhash_is_updated(self):
        contract,qualified,entry,docs,git,calls=self.fixture();path=next(iter(contract['critical_files']))
        after,mode,blob=git('blob',contract['source'],path);changed=after.replace(b'return nativeCommonBase',b'return "unexpected"')
        contract['critical_files'][path].update(bytes=len(changed),sha256=c.sha(changed))
        def altered(action,left,right):
            if action=='ancestor':return True
            if action=='changes':return [{'status':'M','path':path}]
            return git(action,left,right) if left==qualified['qualified_baseline_source'] else (changed,mode,blob)
        with self.assertRaises(c.Unproved):r.adoption(contract,entry,qualified,docs.__getitem__,altered)
    def test_delta_before_must_be_retained_qualified_factory_bytes_not_arbitrary_preimage(self):
        contract,qualified,entry,docs,git,calls=self.fixture();qualified['qualification_source'][next(iter(qualified['qualification_source']))]['sha256']='0'*64
        with self.assertRaises(c.Unproved):r.adoption(contract,entry,qualified,docs.__getitem__,git)
    def test_allWorkspaces_label_without_actual_reviewed_code_or_wrong_signed_source_is_refused(self):
        for change in ('bool','scope','source','pin','missing-API'):
            contract,qualified,entry,docs,git,calls=self.fixture()
            if change=='bool':contract.pop('scope_code');contract['allWorkspaces']=True
            if change=='scope':contract['scope_code'][0]['sha256']='0'*64
            if change=='source':entry['source']='f'*40
            if change=='pin':contract['pins'][0]['qualified_ref']='ghcr.io/bex-co/native@sha256:'+'0'*64
            if change=='missing-API':entry.pop('APIApproval')
            with self.subTest(change=change),patch.object(p,'bind_signature',return_value={}),self.assertRaises(c.Unproved):
                r.adoption(contract,entry,qualified,docs.__getitem__,git)

if __name__=='__main__':unittest.main()
