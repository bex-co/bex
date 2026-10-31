"""Read-only bounded watch of root's one exact reviewed CSI rollout."""
from pathlib import Path
import datetime,json,subprocess,time
P=Path(__file__).resolve().parent
SOURCE='7153509db2afcd2702f2a5a3d7dcacc9ea565772'
RUN='41192091394'
known={};started=time.monotonic()
for number in range(1,91):
 at=datetime.datetime.now(datetime.timezone.utc).isoformat();r=None
 for attempt in range(2):
  try:r=subprocess.run(['gh','run','view',RUN,'--repo','bex-co/bex','--json','databaseId,headSha,event,status,conclusion,createdAt,updatedAt,url,jobs'],capture_output=True,text=True,timeout=30)
  except subprocess.TimeoutExpired:r=None
  if r and r.returncode==0:break
  if attempt==0:time.sleep(2)
 if not r or r.returncode:
  result={'at':at,'poll':number,'status':'UNKNOWN_READ_GAP','detail':r.stderr[-500:] if r else 'bounded metadata timeout'}
 else:
  (P/('snapshot-%03d.json'%number)).write_text(r.stdout);x=json.loads(r.stdout);assert str(x['databaseId'])==RUN and x['headSha']==SOURCE and x['event']=='workflow_dispatch','rollout source/event changed'
  jobs=[]
  for j in x['jobs']:
   assert j['name'] in ('validate','cluster','observer-channel-smoke'),'unknown rollout job'
   if j['name'] in known:assert known[j['name']]==j['databaseId'],'actual job lifetime changed; stop for review'
   known[j['name']]=j['databaseId'];jobs.append({'id':j['databaseId'],'name':j['name'],'status':j['status'],'conclusion':j['conclusion'],'startedAt':j['startedAt'],'activeSteps':[s['name'] for s in j['steps'] if s['status']=='in_progress'],'failedSteps':[s['name'] for s in j['steps'] if s['conclusion']=='failure'],'completedSteps':[s['name'] for s in j['steps'] if s['conclusion']=='success']})
  result={'at':at,'poll':number,'run':RUN,'source':SOURCE,'status':x['status'],'conclusion':x['conclusion'],'jobs':jobs}
 (P/'latest-status.json').write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result),flush=True)
 if result.get('status')=='completed':break
 time.sleep(60)
(P/'watch-finished.json').write_text(json.dumps({'at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'last':result,'elapsedSeconds':round(time.monotonic()-started,3),'noDispatchCancelPaidAuthOrProductionMutations':True},indent=2)+'\n')
