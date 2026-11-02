#!/usr/bin/env python3
"""Explicit future Root execution controller. Import has no external actions."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import subprocess
import sys
import time

import exact_linux_review as review
import linux_cosign as lc

FLOOR=(1<<30)+(128<<20)
CAP=640<<20
STOP=CAP-(32<<20)
HERE=Path(__file__).resolve().parent

def source_manifest(expected):
    raw=(HERE/'MANIFEST.json').read_bytes();lc.require(re.fullmatch('[a-f0-9]{64}',expected) and review.digest(raw)==expected,'reviewed component manifest differs')
    manifest=json.loads(raw)
    for name,row in manifest.items():
        p=HERE/name;lc.require(not Path(name).is_absolute() and '..' not in Path(name).parts and p.is_file() and not p.is_symlink(),'component source path differs')
        lc.require(p.stat().st_size==row['bytes'] and lc.sha(p)==row['sha256'],'component source bytes changed:'+name)
    return manifest

def group_exists(pgid):
    try:os.killpg(pgid,0);return True
    except ProcessLookupError:return False
    except PermissionError:return True

def stop_worker(process):
    lc.require(process.pid>1 and process.pid!=os.getpgrp(),'not exact own worker group')
    for sig,grace in ((signal.SIGTERM,4),(signal.SIGKILL,3)):
        if not group_exists(process.pid):break
        try:os.killpg(process.pid,sig)
        except ProcessLookupError:break
        except PermissionError:pass
        end=time.monotonic()+grace
        while time.monotonic()<end and group_exists(process.pid):process.poll();time.sleep(.05)
    process.wait(timeout=3)
    lc.require(not group_exists(process.pid),'owned review process-group absence unproved')
    return {'pid':process.pid,'pgid':process.pid,'exit':process.returncode,'reaped':True,'group_absent':True}

def owned_size(root):
    total=0
    for path in root.rglob('*'):
        lc.require(not path.is_symlink(),'runtime contains unreviewed symlink')
        if path.is_file():total+=path.stat().st_size
    return total

def final_cleanup(root,owner,docker=None):
    docker=docker or lc.Docker();registry=root/'linux-cosign-lifecycle.json'
    state=json.loads(registry.read_bytes()) if registry.exists() else {'pending':False}
    lc.require(state.get('owner',owner)==owner and state.get('name',lc.owner_name(owner))==lc.owner_name(owner),'lifecycle owner changed')
    return docker.cleanup(owner,uncertain=state.get('pending') is True)

def execute(root,plan_path,plan_sha,source_sha,process_factory=subprocess.Popen,docker=None):
    # All source/candidate/CI/new-sizing validation occurs before root creation,
    # the worker or any Docker command. Root supplies reviewed exact input SHAs.
    source_manifest(source_sha)
    lc.require(re.fullmatch('[a-f0-9]{64}',plan_sha) and lc.sha(plan_path)==plan_sha,'reviewed new plan differs')
    plan=json.loads(plan_path.read_bytes());review.validate_plan(plan,resources=True)
    review.candidate_inputs(plan)
    root=Path(root)
    lc.require(root.is_absolute() and root.parent==Path('/private/tmp') and re.fullmatch('bex-w8-exact14212-review-[a-z0-9_-]{8,64}',root.name)
        and not root.exists() and not root.is_symlink(),'fresh exact private execution root required')
    startfree=shutil.disk_usage(root.parent).free;lc.require(startfree>=FLOOR+CAP,'full owned cap plus1GiB128MiB remaining floor does not fit')
    root.mkdir(mode=0o700);owner=secrets.token_hex(8)
    for part in ('evidence','cache','tmp','anonymous-docker-config'):(root/part).mkdir(mode=0o700)
    (root/'anonymous-docker-config/config.json').write_bytes(b'{"auths":{}}\n');(root/'anonymous-docker-config/config.json').chmod(0o600)
    lc.write(root/'linux-cosign-lifecycle.json',{'owner':owner,'name':lc.owner_name(owner),'pending':False,'state':'registered before worker/resources'})
    command=[sys.executable,'-B',str(HERE/'exact_linux_review.py'),'review','--plan',str(plan_path.resolve()),'--repo-root','/Users/tianpan/projects/bex7',
        '--output-root',str(root/'evidence'),'--cache-root',str(root/'cache'),'--lifecycle-root',str(root),'--docker-config',str(root/'anonymous-docker-config'),'--owner',owner]
    scope={'command':command,'parent_pid':os.getpid(),'candidate_ref':review.REF,'producer':review.SOURCE,'verification':review.SOURCE,'generator':plan['source_sha'],
        'source_manifest_sha256':source_sha,'plan_sha256':plan_sha,'start_free_bytes':startfree,'remaining_floor_bytes':FLOOR,'owned_cap_bytes':CAP,
        'stop_owned_bytes':STOP,'layer_workers':1,'maximum_live_crypto_containers':1,'crypto_memory_bytes':512<<20,
        'owner':owner,'container_name':lc.owner_name(owner),'sizing_inputs':plan['local_resource_sizing'],
        'capacity_semantics':'Operating entry/observation limits, no kernel reservation or future-allocation guarantee; no prior supplier acceptance.'}
    lc.write(root/'SCOPE.json',scope)
    cancelled={'signal':None};handlers={};p=None;errors=[];reason=None;group=None;cleanup=None;started=time.monotonic();minimum=startfree;peak=0
    def cancel(signum,frame):cancelled['signal']=signum
    def observation():
        nonlocal minimum,peak
        free=shutil.disk_usage(root).free;size=owned_size(root);minimum=min(minimum,free);peak=max(peak,size)
        return {'free':free,'owned':size,'global_drop':startfree-free,'seconds':time.monotonic()-started,'log':(root/'review.log').stat().st_size if (root/'review.log').exists() else 0}
    def violation(row):
        if cancelled['signal'] is not None:return 'cancelled-signal-'+str(cancelled['signal'])
        if row['free']<FLOOR:return 'remaining-free-floor'
        if row['owned']>STOP:return 'owned-byte-stop'
        if row['global_drop']>STOP:return 'observed-global-free-drop'
        if row['log']>8<<20:return 'parent-output-bound'
        if row['seconds']>5400:return 'whole-review-deadline'
        return None
    try:
        for signum in (signal.SIGTERM,signal.SIGINT):handlers[signum]=signal.getsignal(signum);signal.signal(signum,cancel)
        with (root/'review.log').open('xb') as log:
            lc.require(cancelled['signal'] is None,'cancelled before worker')
            env=dict(os.environ);env.pop('KUBECONFIG',None);env['TMPDIR']=str(root/'tmp');env['PYTHONDONTWRITEBYTECODE']='1'
            # Host Docker context remains the same supported Root context used
            # for the already-cached image. Only Cosign's in-container config is
            # the exact empty owned mount; --pull=never prevents registry pulls.
            p=process_factory(command,cwd='/Users/tianpan/projects/bex7',env=env,stdout=log,stderr=subprocess.STDOUT,start_new_session=True)
            lc.write(root/'PROCESS.json',{'pid':p.pid,'pgid':p.pid,'command':command})
            while p.poll() is None:
                row=observation();reason=violation(row)
                if reason:break
                time.sleep(1)
    except BaseException as error:errors.append('controller:'+type(error).__name__)
    finally:
        if p is not None:
            try:group=stop_worker(p)
            except BaseException as error:errors.append('worker-stop:'+type(error).__name__)
        # Never let process/log/observer errors skip daemon-container cleanup.
        try:cleanup=final_cleanup(root,owner,docker)
        except BaseException as error:
            cleanup=getattr(error,'receipt',None);errors.append('container-cleanup:'+type(error).__name__)
        try:lc.write(root/'CLEANUP.json',{'group':group,'container':cleanup,'errors':errors})
        except BaseException as error:errors.append('cleanup-receipt:'+type(error).__name__)
        for signum,handler in handlers.items():
            try:signal.signal(signum,handler)
            except BaseException as error:errors.append('restore-signal:'+type(error).__name__)
    try:final=observation();reason=reason or violation(final)
    except BaseException as error:final=None;errors.append('final-observer:'+type(error).__name__)
    if cancelled['signal'] is not None:reason='cancelled-signal-'+str(cancelled['signal'])
    matches=list((root/'evidence').glob('native-node-14.21.2-bullseye-*/receipt.json'))
    receipt=json.loads(matches[0].read_bytes()) if len(matches)==1 else {}
    try:
        source_manifest(source_sha);lc.require(lc.sha(plan_path)==plan_sha,'plan changed during review')
    except BaseException as error:errors.append('final-source:'+type(error).__name__)
    approved=bool(p is not None and type(p.returncode) is int and p.returncode==0 and not errors and reason is None and group and group['group_absent']
        and cleanup and cleanup.get('absence_proved') is True and receipt.get('approved_supplier_candidate') is True and receipt.get('ref')==review.REF)
    result={'candidate_approved':approved,'ref':review.REF,'producer':review.SOURCE,'verification':review.SOURCE,'generator':plan['source_sha'],
        'exit':p.returncode if p is not None else None,'stop':reason,'errors':errors,'cancelled_signal':cancelled['signal'],'minimum_free':minimum,
        'peak_owned_bytes':peak,'final_observation':final,'process_cleanup':group,'container_cleanup':cleanup,'receipt':str(matches[0]) if len(matches)==1 else None,
        'scope':'Exact14.21.2 complete supplier review only if all inherited cryptographic/report/blob and source/CI plus owned execution gates pass. No production/generated/phase/hosted authority.'}
    lc.write(root/'RESULT.json',result)
    if cancelled['signal'] is not None:
        result.update(candidate_approved=False,stop='cancelled-signal-'+str(cancelled['signal']),cancelled_signal=cancelled['signal'])
        lc.write(root/'RESULT.json',result)
    return result

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('action',choices=['run']);p.add_argument('--root',type=Path,required=True)
    p.add_argument('--plan',type=Path,required=True);p.add_argument('--plan-sha',required=True);p.add_argument('--source-manifest-sha',required=True);a=p.parse_args()
    os.umask(0o077);result=execute(a.root,a.plan,a.plan_sha,a.source_manifest_sha);print(json.dumps(result));raise SystemExit(0 if result['candidate_approved'] else 1)

if __name__=='__main__':main()
