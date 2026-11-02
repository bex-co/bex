"""Closed changed-source coverage and exact actual first common initializer."""
import copy,json,unittest
from unittest.mock import patch
import core as c
import readiness as r
import proof_binding as p
import test_adoption as prior

class ClosedAdoptionControls(unittest.TestCase):
    def base(self):return prior.AdoptionControls().fixture()
    def initial(self,syntax='inferred'):
        contract,qualified,entry,docs,oldgit,calls=self.base();path=r.COMMON_SLOT
        before_ref=contract['pin_only_deltas'][0]['before'];before=docs[before_ref['path']]
        old_line=next(line for line in before.splitlines(keepends=True) if line.startswith(b'var nativeCommonBase = '))
        before=before.replace(old_line,b'var nativeCommonBase string\n')
        after=before.replace(b'var nativeCommonBase string\n',r.common_initializer(qualified['common_ref'],syntax))
        docs[before_ref['path']]=before;before_ref.update(bytes=len(before),sha256=c.sha(before))
        identity={'bytes':len(before),'sha256':c.sha(before)}
        qualified['qualification_source'][path]=identity;qualified['critical_source'][path]=dict(identity)
        contract['common_initializer_syntax']=syntax
        contract['critical_files'][path].update(bytes=len(after),sha256=c.sha(after))
        contract['pin_only_deltas'][0]['edits']=[{'kind':'native-common-base-initializer','before':'var nativeCommonBase string\n',
                                                'after':r.common_initializer(qualified['common_ref'],syntax).decode()}]
        body=b'func choose(workspace string) string { return nativeCommonBase }\n'
        scope=contract['scope_code'][0];scope.update(first_byte=after.index(body),last_byte=after.index(body)+len(body))
        def git(action,left,right):
            if action=='ancestor':return True
            if action=='changes':return [{'status':'M','path':path}]
            return (before,'100644','e'*40) if left==qualified['qualified_baseline_source'] else (after,'100644','f'*40)
        return contract,qualified,entry,docs,git,before,after
    def test_real_zero_value_slot_supports_only_reviewed_exact_initializer_forms(self):
        for syntax in ('inferred','typed'):
            contract,qualified,entry,docs,git,before,after=self.initial(syntax)
            self.assertIn(b'var nativeCommonBase string\n',before)
            with self.subTest(syntax=syntax),patch.object(p,'bind_signature',return_value={}):
                result=r.adoption(contract,entry,qualified,docs.__getitem__,git)
            self.assertEqual(result['changed_critical_paths'],[r.COMMON_SLOT])
            self.assertIn(r.common_initializer(qualified['common_ref'],syntax),after)
    def test_omitted_or_empty_delta_cannot_cover_any_changed_runtime_blob(self):
        for kind in ('empty','omitted'):
            contract,qualified,entry,docs,git,before,after=self.initial()
            if kind=='empty':contract['pin_only_deltas']=[]
            else:contract.pop('pin_only_deltas')
            with self.subTest(kind=kind),self.assertRaises(c.Unproved) as caught:r.adoption(contract,entry,qualified,docs.__getitem__,git)
            self.assertIn('every changed critical blob',str(caught.exception))
    def test_initial_declaration_rejects_wrong_name_extra_body_comment_change_or_wrong_path(self):
        for change in ('variable','body','comment','path','target'):
            contract,qualified,entry,docs,git,before,after=self.initial();delta=contract['pin_only_deltas'][0]
            if change=='variable':delta['edits'][0]['before']='var otherBase string\n'
            elif change=='body':delta['edits'][0]['after']+='func hidden() {}\n'
            elif change=='comment':delta['edits'][0]['after']+='// hidden comment\n'
            elif change=='path':delta['path']='lego/operator/internal/build/other.go'
            else:delta['edits'][0]['after']='var nativeCommonBase = "ghcr.io/foreign/base@sha256:'+'9'*64+'"\n'
            with self.subTest(change=change),self.assertRaises(c.Unproved):r.adoption(contract,entry,qualified,docs.__getitem__,git)
    def test_unqualified_target_is_rejected_even_when_qualified_REF_exists_elsewhere(self):
        contract,qualified,entry,docs,git,calls=self.base();path=r.COMMON_SLOT
        after,mode,blob=git('blob',contract['source'],path)
        wrong='ghcr.io/bex-co/native@sha256:'+'9'*64
        changed=after.replace(qualified['common_ref'].encode(),wrong.encode())+('// unrelated qualified literal '+qualified['common_ref']+'\n').encode()
        contract['critical_files'][path].update(bytes=len(changed),sha256=c.sha(changed));contract['pin_only_deltas'][0]['edits'][0]['after']=wrong
        scope=contract['scope_code'][0];body=docs[scope['source']['path']];scope.update(first_byte=changed.index(body),last_byte=changed.index(body)+len(body))
        def altered(action,left,right):
            if action=='blob' and left==contract['source']:return changed,mode,blob
            return git(action,left,right)
        with self.assertRaises(c.Unproved) as caught:r.adoption(contract,entry,qualified,docs.__getitem__,altered)
        self.assertIn('qualified immutable pin-only',str(caught.exception))
    def test_added_deleted_unlisted_or_second_changed_critical_file_cannot_be_omitted(self):
        for change in ('added','deleted','second-changed'):
            contract,qualified,entry,docs,git,calls=self.base();other='lego/operator/internal/build/other.go'
            before=b'package build\nfunc other() string { return "before" }\n';after=before.replace(b'before',b'changed')
            if change=='second-changed':
                qualified['qualification_source'][other]={'bytes':len(before),'sha256':c.sha(before)}
                qualified['critical_source'][other]=dict(qualified['qualification_source'][other])
                contract['critical_files'][other]={'bytes':len(after),'sha256':c.sha(after),'git_mode':'100644','git_blob':'a'*40}
            def altered(action,left,right):
                if action=='changes':return [{'status':'M','path':r.COMMON_SLOT},{'status':'M' if change=='second-changed' else 'A' if change=='added' else 'D','path':other}]
                if action=='blob' and right==other:return (before,'100644','b'*40) if left==qualified['qualified_baseline_source'] else (after,'100644','a'*40)
                return git(action,left,right)
            with self.subTest(change=change),self.assertRaises(c.Unproved):r.adoption(contract,entry,qualified,docs.__getitem__,altered)
    def test_zero_changed_critical_files_allow_empty_deltas_only_if_actual_fixed_slot_is_correct(self):
        for correct in (True,False):
            contract,qualified,entry,docs,git,before,after=self.initial();raw=after if correct else before
            identity={'bytes':len(raw),'sha256':c.sha(raw)};qualified['qualification_source'][r.COMMON_SLOT]=identity;qualified['critical_source'][r.COMMON_SLOT]=dict(identity)
            contract['critical_files'][r.COMMON_SLOT].update(**identity,git_blob='f'*40)
            contract['pin_only_deltas']=[]
            scope=contract['scope_code'][0];body=docs[scope['source']['path']];scope.update(first_byte=raw.index(body),last_byte=raw.index(body)+len(body))
            def unchanged(action,left,right):return True if action=='ancestor' else [] if action=='changes' else (raw,'100644','f'*40)
            if correct:
                with patch.object(p,'bind_signature',return_value={}):result=r.adoption(contract,entry,qualified,docs.__getitem__,unchanged)
                self.assertEqual(result['changed_critical_paths'],[])
            else:
                with self.assertRaises(c.Unproved) as caught:r.adoption(contract,entry,qualified,docs.__getitem__,unchanged)
                self.assertIn('known common slot is empty',str(caught.exception))

if __name__=='__main__':unittest.main()
