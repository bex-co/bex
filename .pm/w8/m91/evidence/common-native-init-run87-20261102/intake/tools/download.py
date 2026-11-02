"""One authorized file-reference GET per archive; never retains its signed URL."""
import datetime,hashlib,json,os,signal,sys,time,urllib.request
from pathlib import Path
from urllib.parse import urlsplit

ROOT=Path(__file__).resolve().parent
INPUTS={'amd64':(13140138422,875486,'9ef790cdb4d8c1391cc175e91956acc9ce8042dab3469924453f1da8339417de')}
def main():
    os.umask(0o077);arch=sys.argv[1];artifact,size,sha=INPUTS[arch];url=json.loads(sys.stdin.read())
    assert urlsplit(url).scheme=='https' and urlsplit(url).hostname.endswith('.oaiusercontent.com')
    out=ROOT/arch/'original.zip';assert not out.exists();h=hashlib.sha256();total=0;minimum=None;started=time.monotonic()
    receipt={'artifact_id':artifact,'arch':arch,'expected_bytes':size,'expected_sha256':sha,'complete':False,
             'scope':'approved connector file reference; one verified-TLS stream; signed URL never retained','started_at':datetime.datetime.now(datetime.timezone.utc).isoformat()}
    def timeout(*_):raise TimeoutError('own download180s deadline')
    old=signal.signal(signal.SIGALRM,timeout);signal.alarm(180)
    try:
        with urllib.request.urlopen(url,timeout=30) as response,out.open('xb') as destination:
            assert response.status==200
            declared=response.headers.get('Content-Length')
            assert declared is None or int(declared)==size
            while True:
                chunk=response.read(min(1<<20,size-total+1))
                if not chunk:break
                assert total+len(chunk)<=size
                fs=os.statvfs(ROOT);free=fs.f_bavail*fs.f_frsize;minimum=free if minimum is None else min(minimum,free)
                assert free-len(chunk)>(1<<30)+(128<<20)
                assert destination.write(chunk)==len(chunk)
                h.update(chunk);total+=len(chunk)
        assert total==size and h.hexdigest()==sha
        receipt['complete']=True
    except BaseException as error:
        # No exception text: URL-bearing transport messages stay out of evidence.
        receipt['failure_class']=type(error).__name__
        raise
    finally:
        signal.alarm(0);signal.signal(signal.SIGALRM,old)
        receipt.update(observed_bytes=total,observed_sha256=h.hexdigest(),minimum_free=minimum,seconds=time.monotonic()-started)
        (ROOT/arch/'download.json').write_text(json.dumps(receipt,indent=2)+'\n')
    print(json.dumps({'arch':arch,'complete':True,'bytes':total,'sha256':h.hexdigest()}))

if __name__=='__main__':
    try:main()
    except BaseException as error:
        print(json.dumps({'failure_class':type(error).__name__}),file=sys.stderr)
        sys.exit(1)
