"""Pure retained-proof binding, derived from reviewed admission.py. No external calls.

Root approval of the outer Ready SHA is still required; hashes alone establish
custody, not authenticity. Actual strict verification and metadata collection
must already have completed and be retained. This module never manufactures it.
"""
import datetime
import hashlib
import json
from pathlib import Path
import re

COSIGN = '/private/tmp/bex7-native-acceptance-novb586u/cosign'
COSIGN_SHA = '9f9b8cd557cf2e5b3df3373a00baa005a86b02391e3e06dd0ce37b3a1533d3a4'
IDENTITY = 'https://github.com/bex-co/bex/.github/workflows/deploy.yml@refs/heads/main'
ISSUER = 'https://token.actions.githubusercontent.com'
WORKFLOW = '.github/workflows/hcloud-csi-rollout.yml'
STEP = 'inventory service-disk binding and capacity metadata (read-only)'
MAX_PROOF = 2 * 1024 * 1024
DEP_KEYS = ('namespace','name','uid','generation','observedGeneration','replicas',
            'updatedReplicas','readyReplicas','availableReplicas','image')
POD_KEYS = ('namespace','name','uid','phase','deletingAt','ready','image','imageID')


class ProofError(ValueError):
    pass


def require(condition, reason):
    if not condition:
        raise ProofError(reason)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def instant(value):
    result = datetime.datetime.fromisoformat(value.replace('Z','+00:00'))
    require(result.utcoffset() is not None, 'observer UTC offset missing')
    return result.astimezone(datetime.timezone.utc)


def read_ref(ref, reader):
    require(isinstance(ref,dict) and isinstance(ref.get('path'),str)
            and Path(ref['path']).is_absolute() and type(ref.get('bytes')) is int
            and 0 < ref['bytes'] <= MAX_PROOF
            and isinstance(ref.get('sha256'),str) and re.fullmatch('[a-f0-9]{64}',ref['sha256']),
            'bounded proof descriptor incomplete')
    body = reader(ref['path'])
    require(type(body) is bytes and len(body)==ref['bytes'] and sha(body)==ref['sha256'],
            'retained proof bytes differ')
    return body


def signature_command(source,image):
    require(isinstance(source,str) and re.fullmatch('[a-f0-9]{40}',source)
            and isinstance(image,str) and re.fullmatch(r'ghcr.io/bex-co/bex-operator@sha256:[a-f0-9]{64}',image),
            'invalid candidate source/image')
    return [COSIGN,'verify','--certificate-identity',IDENTITY,'--certificate-oidc-issuer',ISSUER,
            '--certificate-github-workflow-sha',source,image]


def signature_capture(receipt,stdout,source,image):
    # Accept the actual newer raw-process schema without an unbound projected
    # capture. The source/subject/command/output checks below are shared with218.
    require(isinstance(receipt,dict), 'signature receipt malformed')
    require('actual_process_receipt' not in receipt, 'unbound projected signature receipt; retain raw process receipt')
    if 'argv' in receipt:
        require('command' not in receipt and receipt.get('stop') is None
                and receipt.get('actual_strict_crypto')=='PASS'
                and isinstance(receipt.get('stdout'),dict)
                and type(receipt['stdout'].get('bytes')) is int and receipt['stdout']['bytes']==len(stdout)
                and receipt['stdout'].get('sha256')==sha(stdout), 'actual signature process/output differs')
        receipt=dict(receipt,command=receipt['argv'],stdout_sha256=receipt['stdout']['sha256'])
    # The retained218 schema has no complete/bound/signal keys; absence of these
    # optional runner extensions is supported, an explicit partial result is not.
    require(isinstance(receipt,dict) and receipt.get('command')==signature_command(source,image)
            and type(receipt.get('exit')) is int and receipt['exit']==0
            and receipt.get('source')==source and receipt.get('image')==image
            and receipt.get('tool_sha256')==COSIGN_SHA
            and receipt.get('stdout_sha256')==sha(stdout)
            and receipt.get('bound') is None and receipt.get('signal') is None and receipt.get('stop') is None
            and receipt.get('complete',True) is True,
            'strict signature process/subject/source custody differs')
    rows=json.loads(stdout)
    require(isinstance(rows,list) and rows, 'no verified image signature claims')
    require('proof_count' not in receipt or (type(receipt['proof_count']) is int and receipt['proof_count']==len(rows)),
            'verified signature count differs')
    for row in rows:
        require(isinstance(row,dict), 'verified signature claim malformed')
        critical=row.get('critical',{})
        require(isinstance(critical,dict) and isinstance(critical.get('identity'),dict)
                and isinstance(critical.get('image'),dict)
                and critical['identity'].get('docker-reference')==image
                and critical['image'].get('docker-manifest-digest')==image.split('@')[1],
                'verified signature claims bind a different image')
    require(any(row['critical'].get('type')=='https://sigstore.dev/cosign/sign/v1' for row in rows),
            'no image signature claim')
    return {'source':source,'image':image,'strict_command_output_bound':True}


def bind_signature(approval,reader):
    return signature_capture(json.loads(read_ref(approval.get('signature_receipt'),reader)),
                             read_ref(approval.get('signature_stdout'),reader),
                             approval.get('source'),approval.get('image'))


def inventory_fields(inventory,container,name):
    facts=inventory.get('platformImages',{})
    require(isinstance(facts,dict) and isinstance(facts.get('deployments'),list)
            and isinstance(facts.get('pods'),list), 'full platform inventory missing')
    deployments=[d for d in facts['deployments'] if isinstance(d,dict)
                 and d.get('namespace')=='bex-system' and d.get('name')==name]
    require(len(deployments)==1, 'unique actual Deployment missing')
    raw=deployments[0]
    deployment={key:raw.get(key) for key in DEP_KEYS if key!='image'}
    deployment['image']=raw.get('containerImages',{}).get(container)
    require(isinstance(deployment['uid'],str) and deployment['uid'], 'actual Deployment UID missing')
    require(all(type(deployment.get(key)) is int for key in
            ('generation','observedGeneration','replicas','updatedReplicas','readyReplicas','availableReplicas')),
            'actual Deployment rollout counts/generations untyped')
    pods=[]
    for raw in facts['pods']:
        require(isinstance(raw,dict), 'actual pod row malformed')
        if (raw.get('namespace')=='bex-system' and container in raw.get('containerImages',{})
                and raw.get('deletingAt') in ('',None)):
            row={key:raw.get(key) for key in POD_KEYS if key not in ('ready','image','imageID')}
            row.update(ready=raw.get('containerReady',{}).get(container),
                       image=raw['containerImages'].get(container),
                       imageID=raw.get('containerImageIds',{}).get(container))
            require(isinstance(row['name'],str) and row['name'], 'actual Pod name missing')
            pods.append(row)
    return deployment,pods


def inventory_observation(entry,run,job,now):
    # The approved outer Ready SHA freezes this entry. IDs and observer source
    # can change on a later reviewed collection; they are never operator/API SHA.
    require(entry.get('repository')=='bex-co/bex' and entry.get('operation')=='inventory'
            and entry.get('workflow')==WORKFLOW and entry.get('job_name')=='cluster'
            and entry.get('step_name')==STEP
            and type(entry.get('run_id')) is int and entry['run_id']>0
            and type(entry.get('job_id')) is int and entry['job_id']>0
            and type(entry.get('attempt')) is int and entry['attempt']>0
            and isinstance(entry.get('source'),str) and re.fullmatch('[a-f0-9]{40}',entry['source']),
            'reviewed observer entry incomplete')
    require(run.get('id')==entry['run_id'] and type(run.get('id')) is int
            and run.get('head_sha')==entry['source'] and run.get('path')==WORKFLOW
            and run.get('event')=='workflow_dispatch' and run.get('head_branch')=='main'
            and run.get('status')=='completed'
            and run.get('conclusion')=='success' and run.get('repository',{}).get('full_name')=='bex-co/bex'
            and type(run.get('run_attempt')) is int and run['run_attempt']==entry['attempt'],
            'completed observer run binding differs')
    require(type(job.get('id')) is int and job['id']==entry['job_id']
            and type(job.get('run_id')) is int and job['run_id']==entry['run_id']
            and job.get('head_sha')==entry['source'] and job.get('head_branch')=='main'
            and job.get('name')=='cluster'
            and job.get('status')=='completed' and job.get('conclusion')=='success'
            and type(job.get('run_attempt')) is int and job['run_attempt']==entry['attempt'],
            'completed observer job binding differs')
    steps=[s for s in job.get('steps',[]) if isinstance(s,dict) and s.get('name')==STEP]
    require(len(steps)==1 and steps[0].get('status')=='completed'
            and steps[0].get('conclusion')=='success', 'actual inventory step incomplete')
    start,end=instant(steps[0]['started_at']),instant(steps[0]['completed_at'])
    require(start<=end<=now and 0<=(now-start).total_seconds()<=3600,
            'actual inventory observation stale/future')
    return start,end


def extract_inventory(entry,log,start,end):
    bounds=entry.get('extraction',{})
    first,last=bounds.get('first_line'),bounds.get('last_line')
    lines=log.splitlines(keepends=True)
    require(type(first) is int and type(last) is int and 0<=first<last<=len(lines)
            and bounds.get('format')=='github-timestamp-lines-v1', 'inventory extraction bounds missing')
    output=[]
    for line in lines[first:last]:
        match=re.fullmatch(rb'(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d+Z) (.*\n)',line)
        require(match is not None, 'inventory raw log timestamp line missing')
        stamp=instant(match[1].decode())
        # GitHub step metadata has second precision, log lines have fractions.
        require(start<=stamp<end+datetime.timedelta(seconds=1), 'inventory log outside successful step')
        output.append(match[2])
    return b''.join(output)


def bind_inventory(receipt,now,reader):
    entry=json.loads(read_ref(receipt.get('observer_entry'),reader))
    require(isinstance(entry,dict), 'reviewed observer manifest malformed')
    run=json.loads(read_ref(entry.get('run'),reader))
    job=json.loads(read_ref(entry.get('job'),reader))
    start,end=inventory_observation(entry,run,job,now)
    require(instant(receipt.get('at',''))==start, 'wrapper time differs from actual observation start')
    require(entry.get('inventory')==receipt.get('readiness_proof'), 'inventory descriptor detached from observer')
    raw=read_ref(entry.get('inventory'),reader)
    log=read_ref(entry.get('log'),reader)
    require(extract_inventory(entry,log,start,end)==raw, 'full inventory bytes differ from raw successful job log')
    inventory=json.loads(raw)
    require(isinstance(inventory,dict) and all(k in inventory for k in
            ('platformImages','appsWithDisks','claimsWhoseOwnerIsNotInDiskInventory','storageClasses')),
            'full observer inventory missing')
    for field,podfield,container,name in [('Operator','ReadyPods','manager','bex-controller-manager'),
                                         ('API','APIReadyPods','api','bex-api')]:
        dep,pods=inventory_fields(inventory,container,name)
        claimed=receipt.get(field,{})
        require(isinstance(claimed,dict) and all(key in claimed for key in DEP_KEYS)
                and json.dumps({key:claimed[key] for key in DEP_KEYS},sort_keys=True)==json.dumps(dep,sort_keys=True), field+' differs from retained actual Deployment')
        claimed_pods=receipt.get(podfield)
        require(isinstance(claimed_pods,list) and all(isinstance(p,dict) and all(k in p for k in POD_KEYS) for p in claimed_pods),
                podfield+' projection incomplete')
        actual=sorted(pods,key=lambda p:p['uid'] or '')
        projected=sorted([{k:p[k] for k in POD_KEYS} for p in claimed_pods],key=lambda p:p['uid'] or '')
        require(json.dumps(projected,sort_keys=True)==json.dumps(actual,sort_keys=True), podfield+' differs from complete retained active Pod set')
    return {'run_id':entry['run_id'],'job_id':entry['job_id'],'source':entry['source'],
            'at':start.isoformat(),'inventory_sha256':sha(raw),'raw_log_sha256':sha(log)}
