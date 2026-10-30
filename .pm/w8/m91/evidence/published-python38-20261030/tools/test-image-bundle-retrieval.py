#!/usr/bin/env python3
"""Fresh retrieval/strict verification smoke for the actual signed image bundle only."""
import argparse,hashlib,importlib.util,json,sys,tempfile
from pathlib import Path
sys.dont_write_bytecode=True
HERE=Path(__file__).resolve().parent.parent
HARNESS_SHA256=hashlib.sha256(Path(__file__).read_bytes()).hexdigest()

def main():
 parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--cosign',required=True);args=parser.parse_args();data=json.loads((HERE/'full-predicate-retrieval.json').read_text());rows=[row for row in data['entries'] if row['kind']=='image-signature'];assert len(rows)==1
 scratch=Path(tempfile.mkdtemp(prefix='bex-python38-image-retrieval-'));subset=scratch/'metadata';subset.mkdir();data['entries']=rows;(subset/'full-predicate-retrieval.json').write_text(json.dumps(data))
 source=HERE/'tools/retrieve-and-verify.py';spec=importlib.util.spec_from_file_location('actual_archive_reader',source);reader=importlib.util.module_from_spec(spec);spec.loader.exec_module(reader);reader.HERE=subset;output=scratch/'retrieved';sys.argv=[str(source),'--cosign',args.cosign,'--output',str(output)];reader.main()
 result=json.loads((output/'retrieval-verification-receipt.json').read_text());assert len(result['results'])==1 and result['results'][0]['strict_crypto_exit']==0
 proof={'scope':'Fresh anonymous actual image-signature bundle retrieval and strict official verification only. The original successful supplier review verified all five full bundles; this smoke does not claim to repeat all full report retrieval.','exit_code':0,'harness_sha256':HARNESS_SHA256,'supplier_ref':data['policy']['ref'],'scratch':str(scratch),'reader_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'bundle_digest':rows[0]['bundle_digest'],'strict_verification':result};print(json.dumps(proof));(scratch/'image-smoke-proof.json').write_text(json.dumps(proof,indent=2)+'\n')
if __name__=='__main__':main()
