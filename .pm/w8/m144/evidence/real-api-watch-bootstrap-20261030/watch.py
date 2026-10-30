import disk_fixture as f
watches=[]
try:
 k=f.Kube()
 for resource,kind,name in [('apps.app.bex.co','cleanup_app','m144-protocol-only'),('jobs','cleanup_job','m144-protocol-only')]:
  w=k.watch_cleanup(resource,kind,name,'default');watches.append(w)
  assert w.started.is_set() and not w.failure and w.version.isdecimal() and not w.initial
  print({'resource':resource,'initialBookmarkEstablished':True,'resourceVersion':w.version},flush=True)
finally:
 for w in watches:w.close()
 for w in watches:
  assert not w.thread.is_alive()
  assert w.process is None or w.process.poll() is not None
