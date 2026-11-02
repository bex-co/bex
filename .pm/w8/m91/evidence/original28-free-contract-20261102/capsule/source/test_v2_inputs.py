"""Correct actual m91 request shape and legitimate large external V11 files.
No real artifact/CLI/app/network/crypto invocation; all large bytes invented.
"""
import base64,copy,gzip,hashlib,io,json,os,random,tempfile,unittest,zlib
from pathlib import Path
from unittest.mock import patch
import catalog
import core as c
import evidence
import readiness
import test_adapter as f

class SecretFileControls(unittest.TestCase):
    def test_both_m91_native_bodies_contain_the_exact_original_QA_public_file(self):
        for client in ('bex','render'):
            spec=f.catalog().spec((91,client),f.NOW,set());args=list(spec['create_args']);body=f.post(spec)
            self.assertEqual(body['secretFiles'],catalog.expected_secret_files(spec,verify_original=True))
            self.assertEqual(body['secretFiles'][0]['name'],'qa402.dump.b64')
            self.assertEqual(catalog.identity(base64.b64decode(body['secretFiles'][0]['content'])),catalog.PUBLIC_ARCHIVE)
            self.assertTrue(body['secretFiles'][0]['content'].endswith('\n'))
            self.assertEqual(spec['create_args'],args);catalog.validate_post(body,spec)
    def test_missing_extra_wrong_name_or_any_changed_public_content_is_refused(self):
        for change in ('missing','null','empty','extra','name','content','trim-LF','extra-field'):
            spec=f.catalog().spec((91,'bex'),f.NOW,set());body=f.post(spec)
            if change=='missing':body.pop('secretFiles')
            elif change=='null':body['secretFiles']=None
            elif change=='empty':body['secretFiles']=[]
            elif change=='extra':body['secretFiles'].append({'name':'extra','content':'CONTROL_ONLY'})
            elif change=='name':body['secretFiles'][0]['name']='different.dump.b64'
            elif change=='content':body['secretFiles'][0]['content']='CONTROL_ONLY_DIFFERENT'
            elif change=='trim-LF':body['secretFiles'][0]['content']=body['secretFiles'][0]['content'].rstrip('\n')
            elif change=='extra-field':body['secretFiles'][0]['delete']=False
            with self.subTest(change=change),self.assertRaises(c.Unproved):catalog.validate_post(body,spec)
    def test_other26cases_still_reject_any_undeclared_secretfile(self):
        for key in catalog.KEYS:
            if key[0]==91:continue
            spec=f.catalog().spec(key,f.NOW,set());body=f.post(spec)
            body['secretFiles']=[{'name':'qa402.dump.b64','content':catalog.expected_secret_files(f.catalog().spec((91,'bex'),f.NOW,set()))[0]['content']}]
            with self.subTest(key=key),self.assertRaises(c.Unproved):catalog.validate_post(body,spec)

class FullEvidenceControls(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        available=os.statvfs('/private/tmp');free=available.f_bavail*available.f_frsize
        c.require(free>1207959552+96*1024*1024,'external fake-evidence budget does not fit')
        cls.temporary=tempfile.TemporaryDirectory(prefix='bex-w8-original28-v2-fake-evidence-',dir='/private/tmp')
        cls.root=Path(cls.temporary.name).resolve()
        # V11-shaped DSSE data with >16MiB predicate and >2MiB compressedbundle.
        # Synthetic padding, not a real signature or actual supplier body.
        noise=base64.b64encode(random.Random(42).randbytes(3*1024*1024)).decode()
        cls.predicate=json.dumps({'spdxVersion':'SPDX-2.3','CONTROL_ONLY_UNVERIFIED':True,'padding':noise+'x'*(18*1024*1024)},separators=(',',':')).encode()
        statement=json.dumps({'predicateType':'https://spdx.dev/Document','subject':[{'digest':{'sha256':'a'*64}}],
            'predicate':json.loads(cls.predicate)},separators=(',',':')).encode()
        cls.bundle=json.dumps({'dsseEnvelope':{'payload':base64.b64encode(statement).decode()},
            'verificationMaterial':{'CONTROL_ONLY_NOT_REAL_CRYPTO':True}},separators=(',',':')).encode()
        cls.bundle_gz=gzip.compress(cls.bundle,mtime=0);cls.pred_gz=gzip.compress(cls.predicate,mtime=0)
        c.require(len(cls.bundle_gz)>2*1024*1024 and len(cls.predicate)>16*1024*1024,'large synthetic fixture too small')
        for name,raw in [('bundle.json.gz',cls.bundle_gz),('predicate.json.gz',cls.pred_gz),('generated-spdx.json',cls.predicate)]:
            path=cls.root/name;path.write_bytes(raw);path.chmod(0o400)
        c.require(sum(q.stat().st_size for q in cls.root.iterdir())<64*1024*1024,'externalfake evidence64MiB cap')
    @classmethod
    def tearDownClass(cls):cls.temporary.cleanup()
    def ref(self,name):
        path=self.root/name;body=path.read_bytes();return {'path':str(path),'bytes':len(body),'sha256':c.sha(body)}
    def metadata(self,path):
        file=Path(path);c.require(file.stat().st_size<=2*1024*1024,'metadata cap unchanged');return file.read_bytes()
    def test_legitimate_large_bundle_predicate_and_full_generated_SPDX_are_external_not_run_cache(self):
        reader=evidence.Access(self.metadata)
        self.assertEqual(evidence.gunzip_full(self.ref('bundle.json.gz'),reader),self.bundle)
        self.assertEqual(evidence.gunzip_full(self.ref('predicate.json.gz'),reader),self.predicate)
        self.assertEqual(evidence.full_document(self.ref('generated-spdx.json'),reader)['spdxVersion'],'SPDX-2.3')
        self.assertEqual(evidence.MAX_ATTESTATIONS,256*1024*1024)
        with self.assertRaises(c.Unproved):self.metadata(str(self.root/'bundle.json.gz'))
        self.assertEqual({p.name for p in self.root.iterdir()},{'bundle.json.gz','predicate.json.gz','generated-spdx.json'})
    def test_large_declared_original_archive_input_uses_stream_hash_not_metadata_or_run_cache(self):
        ref=self.ref('bundle.json.gz');raw_stdout=b'{"CONTROL_ONLY":"archive-observed"}'
        stdout={'path':'/private/tmp/CONTROL_ONLY_ARCHIVE_STDOUT.json','bytes':len(raw_stdout),'sha256':c.sha(raw_stdout)}
        class Reader:
            def __call__(self_,path):
                if path==stdout['path']:return raw_stdout
                return self.metadata(path)
            def open_evidence(self_,value):return evidence.private_open(value)
        want={'id':'CONTROL_ONLY_ARCHIVE','architecture':'arm64','command':['CONTROL_ONLY','archive'],
              'observations':json.loads(raw_stdout),'inputs':{'original_archive':{key:ref[key] for key in ('bytes','sha256')}}}
        row={'id':want['id'],'architecture':'arm64','common_ref':'CONTROL_ONLY_COMMON','source':'a'*40,
             'result':{'exit_code':0,'complete':True,'bound':None,'signal':None,'command':want['command'],'stdout':stdout},
             'inputs':{'original_archive':ref}}
        self.assertEqual(readiness.bind_rows({'rows':[row]},[want],'CONTROL_ONLY_COMMON',Reader(),'utilities','a'*40)['ids'],[want['id']])

    def test_oversize_bool_badSHA_or_short_full_stream_never_reaches_decompression(self):
        ref=self.ref('bundle.json.gz');reader=evidence.Access(self.metadata)
        for change in ('oversize','bool','SHA','short'):
            wrong=copy.deepcopy(ref)
            if change=='oversize':wrong['bytes']=evidence.MAX_ATTESTATIONS+1
            elif change=='bool':wrong['bytes']=True
            elif change=='SHA':wrong['sha256']='0'*64
            if change=='short':reader_=type('Reader',(),{'open_evidence':lambda self,ref:io.BytesIO(FullEvidenceControls.bundle_gz[:-8])})()
            else:reader_=reader
            with self.subTest(change=change),patch.object(evidence.zlib,'decompressobj',side_effect=AssertionError('unqualified bytes must not decode')),self.assertRaises(c.Unproved):
                evidence.gunzip_full(wrong,reader_)
    def test_completeSHA_truncated_gzip_and_bomb_exceeding_bound_cannot_parse(self):
        for label,compressed,limit in [('truncated',gzip.compress(b'{"CONTROL_ONLY":true}',mtime=0)[:-8],evidence.MAX_ATTESTATIONS),
                                       ('bomb',gzip.compress(b'x'*(2*1024*1024),mtime=0),1024*1024)]:
            ref={'path':'/private/tmp/CONTROL_ONLY_'+label+'.gz','bytes':len(compressed),'sha256':c.sha(compressed)}
            with self.subTest(label=label),patch.object(evidence,'MAX_ATTESTATIONS',limit),self.assertRaises((c.Unproved,zlib.error)):
                evidence.gunzip_full(ref,lambda path:compressed)

if __name__=='__main__':unittest.main()
