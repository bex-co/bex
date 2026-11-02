"""Fixed original28 catalog and independent literal map; never execute emitters."""
import copy
import base64,gzip
import datetime
import json
from pathlib import Path
import re
import core as c

SPEC_SHA='ebcc5c79060d4cc6107f2aa01107586ad8b35b01a8adc70220a32b5129ee8fdb'
MAP_SHA='fab100e00ce8214e79f00d995a8ef8d25522bdab2085f5303f8600dd95da165f'
HERE=Path(__file__).resolve().parent
PUBLIC_ARCHIVE_PATH='/private/tmp/bex7-native-acceptance-novb586u/fixtures/m91/qa402.dump.b64'
PUBLIC_B64={'bytes':2297,'sha256':'8697ff567103ca61423fc1a0b4f364d191ffa22086e547c64f89c1f2fa41d0bd'}
PUBLIC_ARCHIVE={'bytes':1721,'sha256':'5beaea91d80ee9a3f7f637bd096446102952aa20a916f614fcb73575d1ece4a4'}
MIDS=(91,93,94,95,96,97,98,99,100,101,102,103,107,222)
KEYS=tuple((mid,client) for mid in MIDS for client in ('bex','render'))

def canonical(value):return json.dumps(value,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()
def identity(raw):return {'bytes':len(raw),'sha256':c.sha(raw)}

class Catalog:
    def __init__(self,spec_bytes,map_bytes):
        c.require(c.sha(spec_bytes)==SPEC_SHA and c.sha(map_bytes)==MAP_SHA,'fixed specs/emission map bytes differ')
        specs=json.loads(spec_bytes);mapping=json.loads(map_bytes)
        c.require(mapping.get('schema')=='bex.original28.static-literal-emission-map/v1'
                  and mapping.get('specs')==identity(spec_bytes),'independent map source differs')
        c.require(len(specs)==len(mapping['rows'])==28,'fixed28 catalog incomplete')
        self.specs={(x['mid'],x['client']):x for x in specs};self.maps={(x['mid'],x['client']):x for x in mapping['rows']}
        c.require(tuple(self.specs)==KEYS and tuple(self.maps)==KEYS,'fixed once catalog differs')
        for key,spec in self.specs.items():
            row=self.maps[key]
            c.require(spec['workspace']==c.WORKSPACE and row['specs_sha256']==SPEC_SHA
                      and row['historical_source_file_identities']==spec['source_file_identities']
                      and row['build']==identity(spec['commands']['initial']['build'].encode())
                      and row['start']==identity(spec['commands']['initial']['start'].encode())
                      and row['create_args_canonical']==identity(canonical(spec['create_args']))
                      and row['source_coverage_complete'] is True
                      and not row['unproved_build_emission'] and not row['unproved_start_emission'],
                      'original/map contract differs:'+str(key))
            c.require(row['validation_adjustments_unchanged']==spec.get('validation_adjustments'),'frozen validation adjustments differ')
            for name,item in row['emitted_files'].items():
                c.require(not Path(name).is_absolute() and '..' not in Path(name).parts
                          and type(item['bytes']) is int and item['bytes']>=0
                          and re.fullmatch('[a-f0-9]{64}',item['sha256']),'emitted map path/hash invalid')
                if item['role']=='marker':c.require(item.get('logical_equal_spec') is True,'marker literal not original spec marker')
        self.mapping=mapping

    def spec(self,key,now,attempted):
        c.require(key in self.specs and key not in attempted,'catalog key unknown/already attempted')
        c.require(now.utcoffset() is not None,'UTC clock missing')
        now=now.astimezone(datetime.timezone.utc);day=now.strftime('%Y%m%d')
        c.require((now.replace(hour=0,minute=0,second=0,microsecond=0)+datetime.timedelta(days=1)-now).total_seconds()>120,
                  'do not create in final120seconds UTC day')
        original=self.specs[key];new=copy.deepcopy(original)
        match=re.fullmatch(r'qa-([0-9]{8})-(.+)',original['name']);c.require(match is not None,'original name differs')
        args=new['create_args'];c.require(args.count('--name')==1,'name flag not unique');i=args.index('--name')+1
        c.require(args[i]==original['name'],'literal name differs')
        new['name']='qa-'+day+'-'+match[2];new['name_date_utc']=day;args[i]=new['name']
        check=copy.deepcopy(new);check['name']=original['name'];check['name_date_utc']=original['name_date_utc'];check['create_args'][i]=original['name']
        c.require(check==original,'only unused display-date/name may change')
        return new


def expected_secret_files(spec,verify_original=False):
    args=spec['create_args']
    if spec['mid']!=91:
        c.require('--secret-file' not in args,'undeclared original secret-file flag')
        return []
    c.require(spec.get('backup_file_is_QA_only_nonsecret') is True and args.count('--secret-file')==1,
              'm91requires its single frozen public QA archive flag')
    c.require(args[args.index('--secret-file')+1]=='qa402.dump.b64:'+PUBLIC_ARCHIVE_PATH
              and spec['primary_reference']['backup']==PUBLIC_ARCHIVE,'frozen QA archive path/identity differs')
    source=HERE/'original-inputs/fixtures/m91/qa402.dump.b64'
    c.require(source.is_file() and not source.is_symlink() and source.stat().st_size==PUBLIC_B64['bytes'],'frozen public QA input missing')
    body=source.read_bytes();c.require(identity(body)==PUBLIC_B64,'frozen public QA Base64 bytes differ')
    archive=base64.b64decode(body)
    c.require(identity(archive)==PUBLIC_ARCHIVE,'frozen public PG15 archive bytes differ')
    if verify_original:
        original=Path(PUBLIC_ARCHIVE_PATH)
        c.require(original.is_file() and not original.is_symlink() and original.stat().st_size==PUBLIC_B64['bytes']
                  and original.read_bytes()==body,'actual unchanged native flagpath public input differs')
    # Pinned CLI ResolveSecretFileInputs sends string(os.ReadFile(path)),
    # including the existing final LF. No path/content substitution is made.
    return [{'name':'qa402.dump.b64','content':body.decode('ascii')}]


def validate_post(body,spec):
    c.require(isinstance(body,dict) and body.get('ownerId')==c.WORKSPACE and body.get('name')==spec['name']
              and body.get('type')=='web_service' and body.get('repo')==spec['primary_reference']['repo']
              and body.get('branch')==spec['primary_reference']['branch'] and body.get('autoDeploy') in (False,'no'),
              'actual native request source/ownership differs')
    details=body.get('serviceDetails',{});commands=details.get('envSpecificDetails',{})
    args=spec['create_args']
    def arg(name,default=None):return args[args.index(name)+1] if name in args else default
    c.require(details.get('plan')=='free' and details.get('runtime')==spec['runtime']
              and details.get('region')==arg('--region')
              and commands.get('buildCommand')==spec['commands']['initial']['build']
              and commands.get('startCommand')==spec['commands']['initial']['start']
              and not commands.get('preDeployCommand'),'actual native runtime/commands/Free differs')
    expected=[{'key':key,'value':value} for key,value in spec['env'].items()]
    c.require(body.get('envVars',[])==expected,'actual harmless environment differs')
    c.require(not any(body.get(k) for k in ('disk','diskId','envGroupIds','registryCredentialId','environmentId','projectId','image'))
              and not any(details.get(k) for k in ('disk','diskId','diskSizeGB','autoscaling'))
              and details.get('numInstances',1) in (0,1),'unapproved dependency/scaling/image request')
    expected_files=expected_secret_files(spec)
    c.require((body.get('secretFiles') or [])==expected_files and (body.get('secretFiles') is None or isinstance(body['secretFiles'],list)),
              'actual native public QA secret-file name/content/cardinality differs')
    return True


class Journal:
    def __init__(self,data,persist):
        c.require(data.get('workspace')==c.WORKSPACE and isinstance(data.get('baseline_ids'),list),'actual baseline missing')
        self.data,self.persist=data,persist
    def intent(self,spec,admission,free,pre_ids):
        key=(spec['mid'],spec['client']);rows=self.data['resources']
        c.require(key in KEYS and not any((r['mid'],r['client'])==key for r in rows),'fixed(mid,client) already attempted/unknown')
        c.require(admission.get('state')=='PASS' and admission.get('bound_evidence') is not None
                  and free.get('state')=='PASS','actual admission/Free unproved')
        c.require(not any(r['state']=='forwarded-unknown' for r in rows),'unknown create stops catalog')
        c.require(sum(r['state'] not in ('deleted','not-created') for r in rows)<2,'two active/uncertain ceiling')
        c.require(set(self.data['baseline_ids'])<=set(pre_ids),'baseline missing before intent')
        row=copy.deepcopy(spec);row.update(state='intent',forwarded_once=False,pre_ids=list(pre_ids),children=[])
        rows.append(row);self.persist(self.data);return row
    def capture_wire(self,row,raw,headers):
        c.require(row in self.data['resources'] and row['state']=='intent' and not row['forwarded_once']
                  and 'wire_request' not in row and type(raw) is bytes and len(raw)<=1024*1024,
                  'one bounded actual native request capture required')
        validate_post(json.loads(raw),row)
        row['wire_request']={'bytes':len(raw),'sha256':c.sha(raw),
            'raw_gzip_base64':base64.b64encode(gzip.compress(raw,mtime=0)).decode(),
            'headers':{key:value for key,value in headers.items() if key.lower() in
                       ('user-agent','content-type','x-bex-workspace','x-bex-cli-version')},'authorization':'[REDACTED]'}
        self.persist(self.data)  # Must complete before a first network forward.

    def forward_once(self,row,body,spec):
        c.require(row in self.data['resources'] and row['state']=='intent' and row['forwarded_once'] is False,'native request replay')
        validate_post(body,spec);row.update(state='forwarded-unknown',forwarded_once=True,request=copy.deepcopy(body));self.persist(self.data)
    def bind201(self,row,status,response):
        c.require(type(status) is int and status==201 and row['state']=='forwarded-unknown' and row['forwarded_once'],'no unique actual201')
        view=response.get('data',response).get('service',response.get('data',response));sid=view.get('id','')
        c.require(c.SERVICE_ID.fullmatch(sid) and sid not in self.data['baseline_ids'] and sid not in row['pre_ids']
                  and not any(r.get('id')==sid for r in self.data['resources'] if r is not row)
                  and view.get('ownerId')==c.WORKSPACE and view.get('name')==row['name']
                  and view.get('serviceDetails',{}).get('plan')=='free','actual201 ownership differs')
        row.update(id=sid,state='created',HTTP=201,create_view=copy.deepcopy(view));self.persist(self.data)
    def own(self,sid):
        rows=[r for r in self.data['resources'] if r.get('id')==sid]
        c.require(len(rows)==1,'no exact journaled ownedSID');row=rows[0]
        c.require(row['workspace']==c.WORKSPACE and sid not in self.data['baseline_ids'] and sid not in row['pre_ids']
                  and row.get('HTTP')==201 and row.get('forwarded_once') is True
                  and row['create_view']['id']==sid and row['create_view']['name']==row['name']
                  and row['create_view']['ownerId']==c.WORKSPACE and row['create_view'].get('serviceDetails',{}).get('plan')=='free','owned identity drift')
        return row
    def record_children(self,sid,children):
        row=self.own(sid);ids=[x['id'] for x in children]
        c.require(all(c.DEPLOY_ID.fullmatch(x) for x in ids) and len(set(ids))==len(ids),'unqualified deployment children')
        row['children']=sorted(set(row['children'])|set(ids));self.persist(self.data)
    def deleted(self,sid,status,children,url_status):
        row=self.own(sid);c.require(status==404 and set(children)==set(row['children']) and all(v==404 for v in children.values())
                                  and url_status==404,'exact own SID/children/URL absence missing')
        row['state']='deleted';self.persist(self.data)
