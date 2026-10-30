import bz2,ctypes,hashlib,json,lzma,os,pathlib,sqlite3,ssl,subprocess,sys,sysconfig,zlib
assert sys.version_info[:3] == (3,14,8), sys.version
assert os.environ['PYTHON_VERSION'] == '3.14.8'
assert sys.executable.startswith('/usr/local/bin/'),sys.executable
assert subprocess.check_output(['python','--version'],text=True).strip() == 'Python 3.14.8'
text = 'Crème brûlée & 李'.encode()
for module in (bz2,lzma,zlib):
    assert module.decompress(module.compress(text)) == text
with sqlite3.connect(':memory:') as db:
    db.execute('create table probe (value text)')
    db.execute('insert into probe values (?)',(text.decode(),))
    assert db.execute('select value from probe').fetchone()[0] == text.decode()
assert hashlib.sha256(b'abc').hexdigest() == 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad'
assert ssl.create_default_context().minimum_version >= ssl.TLSVersion.TLSv1_2
checked=[]
for path in pathlib.Path('/usr/local').rglob('*.so*'):
    if path.is_file() and path.open('rb').read(4) == b'\x7fELF':
        p=subprocess.run(['ldd',str(path)],capture_output=True,text=True)
        assert p.returncode == 0,(str(path),p.stdout,p.stderr)
        assert 'not found' not in p.stdout,(str(path),p.stdout)
        checked.append(str(path))
subprocess.run([sys.executable,'/opt/render/project/src/check-brotli.py'],check=True,timeout=60)
subprocess.run([sys.executable,'/opt/render/project/src/check-primary-python.py'],check=True,timeout=120)
versions=subprocess.check_output(['dpkg-query','-W','libbrotli1','libbrotli-dev'],text=True)
for package in ('libbrotli1','libbrotli-dev'):
    version=subprocess.check_output(['dpkg-query','-W','-f=${Version}',package],text=True)
    subprocess.run(['dpkg','--compare-versions',version,'ge','1.0.9-2+deb12u1'],check=True)
print(json.dumps({'phase':sys.argv[1],'python':sys.version,'executable':sys.executable,'PYTHON_VERSION':os.environ['PYTHON_VERSION'],'openssl':ssl.OPENSSL_VERSION,'shared_objects_checked':len(checked),'brotli_packages':versions,'stdlib_roundtrips':'pass','packaging_C_extension':'pass','system_brotli_invalid_and_valid_streams':'pass'},ensure_ascii=False))
