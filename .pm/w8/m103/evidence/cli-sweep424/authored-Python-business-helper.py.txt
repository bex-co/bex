import hashlib,importlib.util,json,pathlib,sys,sysconfig
from email.parser import Parser
R=pathlib.Path(__file__).resolve().parent;ref=json.loads((R/'qref.json').read_text());marker=json.loads((R/'qm.json').read_text());mode=sys.argv[1]
def sha(b):return hashlib.sha256(b).hexdigest()
def base():
 files={n:{'bytes':len((b:=(R/n).read_bytes())),'sha256':sha(b)} for n in ref['files']};assert files==ref['files'];parts=[n.encode()+b'\0'+str(len((b:=(R/n).read_bytes()))).encode()+b'\0'+b for n in ref['source_names']];fixture=sha(b''.join(parts));assert fixture==ref['fixture_sha256'];header=pathlib.Path(sysconfig.get_path('include'))/'Python.h';return {'marker':marker,'python':sys.version.split()[0],'files':files,'fixture_sha256':fixture,'tools':{'setuptools':importlib.util.find_spec('setuptools') is not None,'pip':importlib.util.find_spec('pip') is not None,'Python_h':header.is_file()}}
def metadata():
 v=base();p=Parser().parsestr((R/'qa_native_helper.egg-info/PKG-INFO').read_text());assert p['Name']=='qa-native-helper' and p['Version']=='1.0';v['distribution']={'name':p['Name'],'version':p['Version']};return v
def wheel():
 sys.path.insert(0,str(R/'qsite'));import qa_local_data
 v=metadata();assert qa_local_data.label()==ref['unicode'];v['wheel']={'label':qa_local_data.label(),'installed_module_sha256':sha(pathlib.Path(qa_local_data.__file__).read_bytes())};return v
def result():
 v=wheel();import qa_native_ext
 rows=[{'a':a,'b':b,'value':qa_native_ext.weighted_sum(a,b)} for a,b in ref['pairs']];assert rows==ref['expected'] and qa_native_ext.dessert()==ref['unicode'];v.update(result=rows,unicode=qa_native_ext.dessert(),extension={'file':pathlib.Path(qa_native_ext.__file__).name,'bytes':pathlib.Path(qa_native_ext.__file__).stat().st_size,'sha256':sha(pathlib.Path(qa_native_ext.__file__).read_bytes())});assert all(v['tools'].values());return v
if mode=='before':print('QA_PYTOOLS_BEFORE '+json.dumps(base(),ensure_ascii=False))
elif mode=='metadata':print('QA_PYTOOLS_METADATA '+json.dumps(metadata(),ensure_ascii=False))
elif mode=='wheel':print('QA_PYTOOLS_WHEEL '+json.dumps(wheel(),ensure_ascii=False))
elif mode=='build':
 v=result();(R/'qbuild.json').write_text(json.dumps(v,ensure_ascii=False));print('QA_PYTOOLS_AFTER '+json.dumps(v,ensure_ascii=False))
elif mode=='runtime':
 v=result();v['request_path']=sys.argv[2];print(json.dumps(v,ensure_ascii=False))
else:raise ValueError(mode)
