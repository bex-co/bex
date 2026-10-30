"""Refresh only never-attempted fixture display names; programs/data stay exact."""
from copy import deepcopy
from datetime import datetime,timezone,timedelta
import json,re

def utc_now(moment=None):
 moment=moment or datetime.now(timezone.utc)
 assert moment.tzinfo is not None
 return moment.astimezone(timezone.utc)

def creation_day(spec,moment=None):
 moment=utc_now(moment);day=moment.strftime('%Y%m%d')
 assert spec['name'].startswith('qa-'+day+'-') and spec['name_date_utc']==day,'stale prepared fixture name; refresh this unused spec'
 assert (moment.replace(hour=0,minute=0,second=0,microsecond=0)+timedelta(days=1)-moment).total_seconds()>120,'do not begin a create in the final120seconds of UTC day'
 return moment

def refresh_unused(directory,specs,resources,moment=None):
 moment=utc_now(moment);day=moment.strftime('%Y%m%d');attempted={(x['mid'],x['client']) for x in resources};updated=deepcopy(specs);changes=[]
 for old,new in zip(specs,updated):
  if (old['mid'],old['client']) in attempted:continue
  match=re.fullmatch(r'qa-([0-9]{8})-(.+)',old['name']);assert match,'unexpected prepared fixture name'
  args=new['create_args'];assert args.count('--name')==1;index=args.index('--name')+1;assert args[index]==old['name']
  new['name']='qa-'+day+'-'+match[2];new['name_date_utc']=day;args[index]=new['name']
  if old['name']!=new['name']:changes.append({'mid':old['mid'],'client':old['client'],'before':old['name'],'after':new['name']})
  # Only display name/date and corresponding literal --name may change.
  check=deepcopy(new);check['name']=old['name'];check['create_args'][index]=old['name'];check.pop('name_date_utc',None)
  original=deepcopy(old);original.pop('name_date_utc',None);assert check==original
 target=directory/'prepared-fixtures.json';temporary=directory/'prepared-fixtures.next';temporary.write_text(json.dumps(updated,indent=2,ensure_ascii=False));temporary.replace(target)
 specs[:]=updated
 return {'at':moment.isoformat(),'changed_names':changes,'attempted_fixtures_preserved':len(attempted),'programs_data_commands_nonces_and_validators_unchanged':True}

def creation_spec(directory,specs,ledger,mid,client):
 assert not any(x['mid']==mid and x['client']==client for x in ledger['resources']),'never rename/retry an attempted fixture'
 refresh_unused(directory,specs,ledger['resources'])
 spec=deepcopy(next(x for x in specs if x['mid']==mid and x['client']==client));creation_day(spec)
 return spec
