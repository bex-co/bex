"""Owned m144 disk probe: fixed mount, fixed file names, authenticated bounded writes."""
import errno, hashlib, hmac, http.server, json, os, re, stat, threading, time, uuid

GIB=1024**3
BOOT=uuid.uuid4().hex
START=time.time()
RUN=os.environ.get('DISK_QA_RUN','')
TOKEN=os.environ.get('DISK_QA_TOKEN','')
FILES=('owner.json','sentinel.bin','data-a.bin','data-b.bin','fill.bin','growth.bin','after-snapshot.bin')

class Store:
 def __init__(self,mount,run):
  if not re.fullmatch(r'[a-z0-9]{16}',run):raise ValueError('run identity required')
  self.run=run;self.lock=threading.Lock()
  self.mfd=os.open(mount,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
  self.mdev=os.fstat(self.mfd).st_dev
  if os.stat(os.path.dirname(mount)).st_dev==self.mdev:raise ValueError('disk must be a separate mounted device')
  self.directory='m144-'+run
  try:os.mkdir(self.directory,0o700,dir_fd=self.mfd);fresh=True
  except FileExistsError:fresh=False
  self.fd=os.open(self.directory,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=self.mfd)
  if os.fstat(self.fd).st_dev!=self.mdev:raise ValueError('unexpected nested device')
  if fresh:self.write('owner.json',json.dumps({'run':run,'scope':'only this directory'}).encode())
  if json.loads(self.read('owner.json'))['run']!=run:raise ValueError('owner mismatch')
  for name in ('sentinel.bin','data-a.bin','data-b.bin'):
   if not self.exists(name):self.write(name,hashlib.shake_256((run+name).encode()).digest(65536))
 def exists(self,name):
  try:self.check(name);os.stat(name,dir_fd=self.fd,follow_symlinks=False);return True
  except FileNotFoundError:return False
 def check(self,name):
  if name not in FILES:raise ValueError('fixed own file names only')
 def open(self,name,flags):
  self.check(name);fd=os.open(name,flags|os.O_NOFOLLOW,0o600,dir_fd=self.fd)
  s=os.fstat(fd)
  if not stat.S_ISREG(s.st_mode) or s.st_dev!=self.mdev or s.st_nlink!=1:os.close(fd);raise ValueError('regular own file required')
  return fd
 def read(self,name):
  fd=self.open(name,os.O_RDONLY)
  try:return os.read(fd,65536)
  finally:os.close(fd)
 def write(self,name,data):
  fd=self.open(name,os.O_WRONLY|os.O_CREAT|os.O_EXCL)
  try:
   with os.fdopen(fd,'wb',closefd=False) as f:f.write(data);f.flush();os.fsync(fd)
   os.fsync(self.fd)
  finally:os.close(fd)
 def status(self):
  v=os.fstatvfs(self.fd);return {'run':self.run,'boot':BOOT,'pid':os.getpid(),'process_started_unix':START,'device':self.mdev,'directory_inode':os.fstat(self.fd).st_ino,'capacity_bytes':v.f_blocks*v.f_frsize,'free_bytes':v.f_bfree*v.f_frsize,'available_bytes':v.f_bavail*v.f_frsize,'block_bytes':v.f_frsize,'files':{n:os.stat(n,dir_fd=self.fd,follow_symlinks=False).st_size for n in FILES if self.exists(n)}}
 def hashes(self):
  out={}
  for name in FILES:
   if not self.exists(name):continue
   fd=self.open(name,os.O_RDONLY);h=hashlib.sha256();size=0
   try:
    while True:
     b=os.read(fd,1024*1024)
     if not b:break
     h.update(b);size+=len(b)
   finally:os.close(fd)
   out[name]={'bytes':size,'sha256':h.hexdigest()}
  return {'run':self.run,'boot':BOOT,'hashes':out}
 def fill(self):
  # Never writes outside this mount; stop even if a broken cap exposes 10Gi.
  before=self.status()
  if not int(.9*GIB)<=before['capacity_bytes']<=int(1.01*GIB):raise ValueError('1Gi filesystem required before fill')
  fd=self.open('fill.bin',os.O_WRONLY|os.O_CREAT|os.O_EXCL);block=hashlib.shake_256((self.run+'fill').encode()).digest(1024*1024)
  total=0;deadline=time.monotonic()+120;failure=None
  try:
   while total<1280*1024*1024 and time.monotonic()<deadline:
    try:
     n=os.write(fd,block);total+=n
     if total%(16*1024*1024)==0:os.fsync(fd)
    except OSError as e:
     failure={'errno':e.errno,'name':errno.errorcode.get(e.errno)};break
   try:os.fsync(fd)
   except OSError as e:failure={'errno':e.errno,'name':errno.errorcode.get(e.errno)}
  finally:os.close(fd)
  return {'before':before,'after':self.status(),'written_bytes':total,'failure':failure,'observed_enospc':failure is not None and failure['errno']==errno.ENOSPC,'bounded_by_bytes':1280*1024*1024,'bounded_by_seconds':120}
 def growth(self):
  before=self.status()
  if not int(4.75*GIB)<=before['capacity_bytes']<=int(5.01*GIB):raise ValueError('5Gi filesystem required before growth write')
  if not self.exists('fill.bin'):raise ValueError('original full file required')
  fd=self.open('growth.bin',os.O_WRONLY|os.O_CREAT|os.O_EXCL);block=hashlib.shake_256((self.run+'growth').encode()).digest(1024*1024)
  try:
   for _ in range(256):
    view=memoryview(block)
    while view:view=view[os.write(fd,view):]
   os.fsync(fd)
  finally:os.close(fd)
  return {'before':before,'after':self.status(),'written_bytes':256*1024*1024}
 def compact(self):
  # Run only after fill/growth hashes exported; keeps snapshot small.
  for name in ('fill.bin','growth.bin'):
   if self.exists(name):
    fd=self.open(name,os.O_RDONLY);os.close(fd);os.unlink(name,dir_fd=self.fd)
  os.fsync(self.fd);return self.status()
 def marker(self):
  self.write('after-snapshot.bin',hashlib.sha256((self.run+'post-snapshot').encode()).digest());return self.hashes()

class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*a):pass
 def reply(self,status,body):
  data=json.dumps(body,sort_keys=True).encode();self.send_response(status);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(data)));self.send_header('Cache-Control','no-store');self.end_headers();self.wfile.write(data)
 def authorized(self):
  return hmac.compare_digest(self.headers.get('Authorization',''),'Bearer '+TOKEN) and self.headers.get('X-Disk-QA-Run')==RUN and len(TOKEN)>=32
 def handle_request(self):
  if self.path=='/healthz' and self.command=='GET':return self.reply(200,{'ready':True})
  if not self.authorized():return self.reply(401,{'error':'unauthorized'})
  if self.headers.get('Transfer-Encoding') or int(self.headers.get('Content-Length','0'))>256:return self.reply(413,{'error':'bounded request'})
  actions={('GET','/status'):STORE.status,('GET','/hashes'):STORE.hashes,('POST','/fill'):STORE.fill,('POST','/growth'):STORE.growth,('POST','/compact'):STORE.compact,('POST','/after-snapshot'):STORE.marker}
  fn=actions.get((self.command,self.path))
  if fn is None:return self.reply(404,{'error':'fixed operations only'})
  if not STORE.lock.acquire(blocking=False):return self.reply(409,{'error':'one operation at a time'})
  try:
   try:return self.reply(200,fn())
   except (ValueError,FileExistsError) as e:return self.reply(409,{'error':str(e)})
   except OSError as e:return self.reply(500,{'errno':e.errno,'error':errno.errorcode.get(e.errno)})
  finally:STORE.lock.release()
 do_GET=handle_request
 do_POST=handle_request

if __name__=='__main__':
 if len(TOKEN)<32:raise SystemExit('private DISK_QA_TOKEN required')
 STORE=Store('/var/data',RUN)
 server=http.server.ThreadingHTTPServer(('0.0.0.0',int(os.environ.get('PORT','10000'))),Handler);server.daemon_threads=True;server.serve_forever()
