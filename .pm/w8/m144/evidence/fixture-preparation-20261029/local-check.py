from pathlib import Path
import errno,hashlib,http.client,importlib.util,json,os,threading
p=Path(__file__).resolve().parent
spec=importlib.util.spec_from_file_location('fixture',p/'disk-fixture.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
# Only this local check changes the nominal capacity to a real 32Mi tmpfs.
m.GIB=32*1024*1024;m.RUN='0123456789abcdef';m.TOKEN='local-only-'+('x'*32);m.STORE=m.Store('/var/data',m.RUN)
initial=m.STORE.hashes();assert initial['hashes']['sentinel.bin']['bytes']==65536
for key in ['../escape','/etc/passwd','not-allowed']:
 try:m.STORE.read(key);raise AssertionError('path guard failed')
 except ValueError:pass
os.rename('data-a.bin','data-a-saved.bin',src_dir_fd=m.STORE.fd,dst_dir_fd=m.STORE.fd);os.symlink('/etc/passwd','data-a.bin',dir_fd=m.STORE.fd)
try:m.STORE.read('data-a.bin');raise AssertionError('symlink read allowed')
except OSError as e:assert e.errno==errno.ELOOP
os.unlink('data-a.bin',dir_fd=m.STORE.fd);os.rename('data-a-saved.bin','data-a.bin',src_dir_fd=m.STORE.fd,dst_dir_fd=m.STORE.fd)
os.link('sentinel.bin','link.bin',src_dir_fd=m.STORE.fd,dst_dir_fd=m.STORE.fd)
try:m.STORE.read('sentinel.bin');raise AssertionError('hardlink read allowed')
except ValueError:pass
os.unlink('link.bin',dir_fd=m.STORE.fd)
server=m.http.server.ThreadingHTTPServer(('127.0.0.1',0),m.Handler);t=threading.Thread(target=server.serve_forever,daemon=True);t.start()
def req(method,path,auth=False):
 c=http.client.HTTPConnection('127.0.0.1',server.server_port,timeout=10);headers={'Authorization':'Bearer '+m.TOKEN,'X-Disk-QA-Run':m.RUN} if auth else {};c.request(method,path,headers=headers);r=c.getresponse();body=r.read();c.close();return r.status,json.loads(body)
assert req('GET','/healthz')[0]==200
for method,path in [('GET','/status'),('POST','/fill'),('POST','/compact')]:assert req(method,path)[0]==401
assert req('GET','/status',True)[0]==200 and req('POST','/../../etc/passwd',True)[0]==404
server.shutdown();server.server_close();t.join(timeout=2)
filled=m.STORE.fill();assert filled['observed_enospc'] and filled['failure']['errno']==28 and filled['written_bytes']<=32*1024*1024
try:m.STORE.fill();raise AssertionError('duplicate fill allowed')
except FileExistsError:pass
post=m.STORE.hashes();assert all(post['hashes'][k]==v for k,v in initial['hashes'].items())
m.STORE.compact();assert m.STORE.hashes()['hashes']==initial['hashes']
print(json.dumps({'qualification':'local isolated Docker tmpfs32Mi only, not production 1Gi acceptance','fixture_source_sha256':hashlib.sha256((p/'disk-fixture.py').read_bytes()).hexdigest(),'fixed_path_guard':True,'symlink_guard':True,'hardlink_guard':True,'authenticated_HTTP':True,'actual_local_errno':filled['failure'],'local_written_bytes':filled['written_bytes'],'sentinel_and_data_hashes_preserved':True,'fixed_own_compaction':True}))
