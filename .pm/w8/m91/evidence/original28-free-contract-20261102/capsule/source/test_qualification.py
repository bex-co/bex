"""Field/custody tests on invented evidence only, never actual qualification."""
import copy,json,unittest
import core as c
import readiness as r
from qualification_fixtures import Facts

class QualificationControls(unittest.TestCase):
    def test_separate_producer_verifier_generator_utility_family_roles_are_recorded(self):
        f=Facts();result=r.qualification(f.entry,f.contract,f.docs.__getitem__)
        self.assertEqual(result['roles'],f.contract['roles']);self.assertEqual(len(set(result['roles'].values())),5)
        self.assertEqual(result['qualified_baseline_source'],f.contract['roles']['generator'])
    def test_wrong_common_manifest_platform_blob_and_incomplete_signed_reports_refuse_even_rehashed(self):
        for change in ('common','platform','blob-size','missing-report','crypto-command','tool-source','wrong-producer'):
            f=Facts();receipt=f.doc(f.entry['v11_receipt'])
            if change=='common':f.entry['common_ref']='ghcr.io/bex-co/native@sha256:'+'0'*64
            elif change=='platform':receipt['platforms']['arm64']['manifest_digest']='sha256:'+'0'*64
            elif change=='blob-size':receipt['anonymous_pull']['blob_receipts'][0]['bytes_hashed']+=1
            elif change=='missing-report':receipt['signed_payloads'].pop()
            elif change=='crypto-command':
                crypto=f.doc(f.entry['v11_crypto']);crypto['results'][0]['command'].append('--insecure-ignore-tlog=true');f.replace(f.entry['v11_crypto'],crypto)
            elif change=='tool-source':receipt['evidence_tool_sha256']={}
            elif change=='wrong-producer':receipt['producer_workflow_sha']='f'*40
            f.replace(f.entry['v11_receipt'],receipt)
            with self.subTest(change=change),self.assertRaises(c.Unproved):r.qualification(f.entry,f.contract,f.docs.__getitem__)
    def test_actual_full_report_architecture_and_gzip_custody_not_selfclaimed_rowlabels(self):
        for change in ('SPDX-architecture','Trivy-architecture','gzip-SHA','predicate-kind'):
            f=Facts();receipt=f.doc(f.entry['v11_receipt'])
            row=next(x for x in receipt['signed_payloads'] if x['kind']==('vulnerability' if change=='Trivy-architecture' else 'sbom'))
            if change.endswith('architecture'):row['architecture']='arm64' if row['architecture']=='amd64' else 'amd64'
            elif change=='gzip-SHA':row['full_predicate_gzip_sha256']='0'*64
            elif change=='predicate-kind':row['kind']='vulnerability'
            f.replace(f.entry['v11_receipt'],receipt)
            with self.subTest(change=change),self.assertRaises(c.Unproved):r.qualification(f.entry,f.contract,f.docs.__getitem__)

    def test_original_utilities_and_all_reviewed_family_cases_need_complete_raw_observations(self):
        for target in ('utilities','families'):
            for change in ('missing-arch','wrong-source','partial','different-common','wrong-version'):
                f=Facts();evidence=f.doc(f.entry[target])
                if change=='missing-arch':evidence['rows'].pop()
                elif change=='wrong-source':evidence['rows'][0]['source']='f'*40
                elif change=='partial':evidence['rows'][0]['result']['exit_code']=124
                elif change=='different-common':evidence['rows'][0]['common_ref']='OTHER'
                elif change=='wrong-version':
                    result=evidence['rows'][0]['result'];observed=f.doc(result['stdout']);observed['version']='WRONG';f.replace(result['stdout'],observed)
                f.replace(f.entry[target],evidence)
                with self.subTest(target=target,change=change),self.assertRaises(c.Unproved):r.qualification(f.entry,f.contract,f.docs.__getitem__)
    def test_note222_requires_exact_generated_no_signal_3_14_8_full_SPDX_root_and_fixed_brotli(self):
        for change in ('version','version-signal','different-base','spdx-hash','supplier-SPDX','brotli'):
            f=Facts();generated=f.doc(f.entry['generated_default_python']);output=f.doc(generated['result']['stdout'])
            if change=='version':output['python']='3.14.7'
            elif change=='version-signal':output['version_signals']=['PYTHON_VERSION=3.14']
            elif change=='different-base':output['base']='OTHER_COMMON'
            elif change=='spdx-hash':output['spdx_sha256']='0'*64
            else:
                sbom=f.doc(generated['spdx'])
                if change=='supplier-SPDX':sbom['packages'].pop(0)
                else:sbom['packages'][1]['versionInfo']='1.0.9-2'
                f.replace(generated['spdx'],sbom);output['spdx_sha256']=generated['spdx']['sha256']
            f.replace(generated['result']['stdout'],output);f.replace(f.entry['generated_default_python'],generated)
            with self.subTest(change=change),self.assertRaises(c.Unproved):r.qualification(f.entry,f.contract,f.docs.__getitem__)

if __name__=='__main__':unittest.main()
