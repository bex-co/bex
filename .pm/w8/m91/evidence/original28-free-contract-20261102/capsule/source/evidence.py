"""Full retained V11 proof bounds, independent of small metadata/output budgets.

No download/copy/cache writes. Existing external proof files are streamed and
SHA/size qualified before parsing. Compression is decoded with the unchanged
V11 MAX_ATTESTATIONS ceiling; metadata/config/HTTP/credentials retain their caps.
"""
import hashlib,io,json,re,zlib
from pathlib import Path
import core as c

MAX_ATTESTATIONS=256*1024*1024  # Actual retained V11 policy constant.
CHUNK=1024*1024

def descriptor(ref):
    c.require(isinstance(ref,dict) and isinstance(ref.get('path'),str) and Path(ref['path']).is_absolute()
              and type(ref.get('bytes')) is int and 0<ref['bytes']<=MAX_ATTESTATIONS
              and isinstance(ref.get('sha256'),str) and re.fullmatch('[a-f0-9]{64}',ref['sha256']),
              'full V11 proof descriptor invalid/over MAX_ATTESTATIONS')
    return ref['bytes'],ref['sha256']

def private_open(ref):
    expected,_=descriptor(ref);path=Path(ref['path'])
    c.require(str(path).startswith('/private/tmp/') and path==path.resolve() and not path.is_symlink()
              and path.is_file() and path.stat().st_size==expected,'full retained private proof file differs')
    return path.open('rb')

def opener(reader):
    # Concrete adapter passes an explicit external-file opener; old small fake
    # dictionaries remain in-memory injected fixtures only. Never use metadata
    # reader to load a real large body or copy it into own32MiB runtime cache.
    if hasattr(reader,'open_evidence'):return reader.open_evidence
    return lambda ref:io.BytesIO(reader(ref['path']))

def hash_stream(stream,expected):
    count=0;digest=hashlib.sha256()
    while True:
        body=stream.read(min(CHUNK,expected-count+1))
        if not body:break
        c.require(type(body) is bytes,'full proof stream must yield bytes')
        count+=len(body);c.require(count<=expected,'full proof exceeds exact descriptor')
        digest.update(body)
    return count,digest.hexdigest()

def verify_full(ref,reader):
    expected,sha=descriptor(ref)
    with opener(reader)(ref) as stream:
        count,digest=hash_stream(stream,expected)
        c.require((count,digest)==(expected,sha),'complete external artifact SHA/size differs')
    return {'bytes':count,'sha256':digest}


def read_full(ref,reader):
    expected,sha=descriptor(ref)
    with opener(reader)(ref) as stream:
        count,digest=hash_stream(stream,expected)
        c.require((count,digest)==(expected,sha),'complete full proof SHA/size differs')
        stream.seek(0);parts=[];count=0;hasher=hashlib.sha256()
        while True:
            part=stream.read(min(CHUNK,expected-count+1))
            if not part:break
            count+=len(part);c.require(count<=expected,'full proof changed after SHA qualification')
            hasher.update(part);parts.append(part)
        c.require(count==expected and hasher.hexdigest()==sha,'full proof changed during qualified read')
        return b''.join(parts)

def gunzip_full(ref,reader):
    expected,sha=descriptor(ref)
    with opener(reader)(ref) as stream:
        count,digest=hash_stream(stream,expected)
        c.require((count,digest)==(expected,sha),'complete compressed full proof SHA/size differs')
        stream.seek(0);decoder=zlib.decompressobj(16+zlib.MAX_WBITS)
        count=0;digest=hashlib.sha256();parts=[];uncompressed=0
        while True:
            part=stream.read(min(CHUNK,expected-count+1))
            if not part:break
            count+=len(part);c.require(count<=expected,'compressed proof changed after SHA qualification');digest.update(part)
            pending=part
            while pending:
                remaining=MAX_ATTESTATIONS-uncompressed
                output=decoder.decompress(pending,min(CHUNK,remaining+1))
                uncompressed+=len(output);c.require(uncompressed<=MAX_ATTESTATIONS,'full proof decompression exceeds MAX_ATTESTATIONS')
                parts.append(output)
                c.require(not decoder.unused_data,'full proof has unqualified trailing/member bytes')
                pending=decoder.unconsumed_tail
                if pending:c.require(output or remaining>0,'full proof decompression did not converge')
        c.require(count==expected and digest.hexdigest()==sha,'compressed full proof changed during qualified decode')
        c.require(decoder.eof and not decoder.unused_data and not decoder.unconsumed_tail,'full proof gzip truncated/incomplete')
        return b''.join(parts)

def full_document(ref,reader):
    raw=read_full(ref,reader);value=json.loads(raw)
    c.require(isinstance(value,dict),'full proof JSON object missing');return value

class Access:
    def __init__(self,metadata_reader):self.metadata_reader=metadata_reader
    def __call__(self,path):return self.metadata_reader(path)
    def open_evidence(self,ref):return private_open(ref)
