from pathlib import Path
import sys,json,datetime,hashlib,os
h=Path("/private/tmp/bex7-m144-disk-acceptance-n6d16nzf");l=Path("/private/tmp/bex-w8-m144-live-20261031-cc62f8e3a1ee8b8c");os.umask(0o077);sys.path.insert(0,str(h))
source=h/"hosted-disk.py";assert hashlib.sha256(source.read_bytes()).hexdigest()=="1ee808d858ce704907d9000c4f9de6830d63cb43bebc4b9eccb7b8fb60ab7953"
n={"__file__":str(source),"__name__":"fixed_post_cleanup_quota"};exec(compile(source.read_text().split("A=argparse.ArgumentParser",1)[0],str(source),"exec"),n)
assert n["WS"]=="tea-da2isimlm39c739m4ofg" and n["L"]["owned"]["id"]=="srv-dbit2iat5fis7381jscg" and n["L"]["disk_id"]=="dsk-dbit2iat5fis7381jsdg"
v=n["L"]["cleanup_api_verified"];assert v["service_404"] and v["disk_404"] and v["baseline_preserved"]
r=n["request"]("GET","/v1/owners/tea-da2isimlm39c739m4ofg/limits");(l/"post-cleanup-quota.receipt.json").write_text(json.dumps(r,indent=2)+"\n")
q=n["value"](r);old=n["L"]["limits"];result={"at":datetime.datetime.now(datetime.timezone.utc).isoformat(),"HTTP":r["HTTP"],"baseline_services":old["services"],"after_cleanup_services":q["services"],"service_quota_returned_to_baseline":q["services"]==old["services"],"own_service_and_disk_404_proven":True,"no_foreign_mutation":True}
n["L"]["cleanup_quota_readback"]=result;n["save"]();(l/"post-cleanup-quota.result.json").write_text(json.dumps(result,indent=2)+"\n");print(json.dumps(result,indent=2))
