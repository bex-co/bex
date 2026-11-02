def verify_command(self,row,label,source_sha=None,digest=None):
            # Exact V11 labels, inner policy flags, retry predicate and backoff.
            lc.require(label in ('positive','wrong-source','wrong-subject'),'unsupported strict control label')
            expected=(None,None) if label=='positive' else (('0'*40,None) if label=='wrong-source' else (None,'sha256:'+'0'*64))
            lc.require((source_sha,digest)==expected,'wrong strict-control source/digest arguments')
            source=source_sha or self.a.sha;subject=(digest or self.digest).split(':')[1]
            inner=inner_command(row,source,subject);base_name=row['bundle_digest'].split(':')[1]+'-'+label;attempts=[]
            for attempt in range(1,4):
                stem=base_name+'-attempt'+str(attempt);out=self.bundle_dir/(stem+'.stdout.log');err=self.bundle_dir/(stem+'.stderr.log')
                tuf_parent=self.bundle_dir/'cosign-tuf-cache';lc.require(not tuf_parent.is_symlink(),'TUF parent is a symlink');tuf_parent.mkdir(mode=0o700,exist_ok=True)
                tuf=Path(tempfile.mkdtemp(prefix=stem+'-',dir=tuf_parent))
                transport=self.executor.execute(row['path'],tuf,inner,out,err)
                result=dict(transport,bundle_digest=row['bundle_digest'],stderr=str(err),tuf_cache=str(tuf))
                self.v.dump(self.bundle_dir/(stem+'.result.json'),result);attempts.append(result)
                text=err.read_text();transport_only=label=='positive' and 'getting trusted root from TUF' in text and any(message in text for message in ('TLS handshake timeout','i/o timeout','connection reset by peer','context deadline exceeded'))
                if result['exit_code']==0 or not transport_only or attempt==3:break
                self.progress('strict-Cosign-TUF-transport-retry',bundle_digest=row['bundle_digest'],attempt=attempt);time.sleep(2*attempt)
            result=dict(result,attempts=attempts);self.v.dump(self.bundle_dir/(base_name+'.result.json'),result);return result
