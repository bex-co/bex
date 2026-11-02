"""Known Go var lexical/offset regressions only; no Go parser/compiler/process."""
import json,unittest
from unittest.mock import patch
import core as c
import readiness as r
import proof_binding as p
import known_go_slot as slot
import test_adoption as prior
import test_v2_adoption as v2

QUALIFIED='ghcr.io/bex-co/native@sha256:'+'b'*64
OLD='ghcr.io/bex-co/native@sha256:'+'a'*64

def line(ref=QUALIFIED,form='inferred'):return r.common_initializer(ref,form)

def fake_texts():
    text=line().decode().rstrip('\n')
    return [
        ('line-comment',('// '+text+'\n').encode()),
        ('block-comment',('/*\n'+text+'\n { ( [ } ) ]\n*/\n').encode()),
        ('raw-string',('var documentation = `\n'+text+'\n { ( [ } ) ]\n`\n').encode()),
        ('quoted-string',('var documentation = '+json.dumps(text+'\n { }')+'\n').encode()),
        ('rune-and-comment',("var brace = '}'\n// "+text+'\n').encode()),
        ('nested-local',('func other() { '+text+'\n}\n').encode()),
    ]

class ScannerControls(unittest.TestCase):
    def test_unique_actual_zero_inferred_and_typed_canonical_forms(self):
        for form,body in [('zero',b'var nativeCommonBase string\n'),('inferred',line()),('typed',line(form='typed'))]:
            with self.subTest(form=form):
                source=b'package build\n'+body
                parsed=slot.declaration(source)
                self.assertEqual(parsed['form'],form)
                self.assertEqual(parsed['value'],None if form=='zero' else QUALIFIED)
                self.assertEqual(source[parsed['start']:parsed['end']],body)
    def test_comment_quoted_raw_rune_and_nested_occurrences_never_replace_actual_OLD_slot(self):
        for label,fake in fake_texts():
            with self.subTest(label=label):
                source=b'package build\n'+line(OLD)+fake
                self.assertEqual(slot.declaration(source)['value'],OLD)
    def test_fake_occurrences_do_not_make_duplicate_actual_declaration(self):
        # Real top-level slot is valid even if documentation repeats its text.
        source=b'package build\n'+b''.join(body for label,body in fake_texts())+line()
        self.assertEqual(slot.declaration(source)['value'],QUALIFIED)
    def test_only_fake_missing_grouped_duplicate_or_noncanonical_slot_is_unproved(self):
        sources=[b'package build\n'+body for label,body in fake_texts()]
        sources += [b'package build\n'+line()+line(OLD),
                    b'package build\nvar (\n nativeCommonBase = '+json.dumps(QUALIFIED).encode()+b'\n)\n',
                    b'package build\nvar nativeCommonBase = `'+QUALIFIED.encode()+b'`\n',
                    b'package build\nvar nativeCommonBase = lookup()\n']
        for source in sources:
            with self.subTest(source=source[:80]),self.assertRaises(c.Unproved):slot.declaration(source)

class AdoptionSlotControls(unittest.TestCase):
    def unchanged(self,raw):
        contract,qualified,entry,docs,git,calls=prior.AdoptionControls().fixture();path=r.COMMON_SLOT
        identity={'bytes':len(raw),'sha256':c.sha(raw)}
        qualified['qualification_source'][path]=identity;qualified['critical_source'][path]=dict(identity)
        contract['critical_files'][path].update(**identity,git_blob='f'*40);contract['pin_only_deltas']=[]
        scope=contract['scope_code'][0];body=docs[scope['source']['path']]
        scope.update(first_byte=raw.index(body),last_byte=raw.index(body)+len(body))
        def same(action,left,right):return True if action=='ancestor' else [] if action=='changes' else (raw,'100644','f'*40)
        return contract,qualified,entry,docs,same
    def test_unchanged_OLD_variable_plus_fakequalified_comment_or_literal_never_admits(self):
        function=b'func choose(workspace string) string { return nativeCommonBase }\n'
        for label,fake in fake_texts():
            contract,qualified,entry,docs,git=self.unchanged(b'package build\n'+line(OLD)+fake+function)
            with self.subTest(label=label),patch.object(p,'bind_signature',return_value={}),self.assertRaises(c.Unproved):
                r.adoption(contract,entry,qualified,docs.__getitem__,git)
    def test_correct_actual_variable_with_same_fake_text_remains_admissible_source_only(self):
        function=b'func choose(workspace string) string { return nativeCommonBase }\n'
        raw=b'package build\n'+line()+b''.join(body for label,body in fake_texts())+function
        contract,qualified,entry,docs,git=self.unchanged(raw)
        with patch.object(p,'bind_signature',return_value={}):result=r.adoption(contract,entry,qualified,docs.__getitem__,git)
        self.assertEqual(result['changed_critical_paths'],[])
    def test_first_pin_uses_actual_zero_declaration_offset_and_preserves_identical_comment_text(self):
        contract,qualified,entry,docs,git,before,after=v2.ClosedAdoptionControls().initial()
        document=b'// var nativeCommonBase string\nvar text = `var nativeCommonBase string\n`\n'
        before+=document;after+=document
        ref=contract['pin_only_deltas'][0]['before'];docs[ref['path']]=before;ref.update(bytes=len(before),sha256=c.sha(before))
        identity={'bytes':len(before),'sha256':c.sha(before)};qualified['qualification_source'][r.COMMON_SLOT]=identity;qualified['critical_source'][r.COMMON_SLOT]=dict(identity)
        contract['critical_files'][r.COMMON_SLOT].update(bytes=len(after),sha256=c.sha(after))
        def observed(action,left,right):
            if action=='ancestor':return True
            if action=='changes':return [{'status':'M','path':r.COMMON_SLOT}]
            return (before,'100644','e'*40) if left==qualified['qualified_baseline_source'] else (after,'100644','f'*40)
        with patch.object(p,'bind_signature',return_value={}):result=r.adoption(contract,entry,qualified,docs.__getitem__,observed)
        self.assertEqual(result['changed_critical_paths'],[r.COMMON_SLOT]);self.assertTrue(after.endswith(document))

if __name__=='__main__':unittest.main()
