from pathlib import Path
import re,json,posixpath,hashlib,datetime,subprocess
B=Path('.pm/w1/m188');E=B/'evidence';sha=lambda b:hashlib.sha256(b).hexdigest();docs=[B/'README.md',B/'assessment.md',*sorted(B.glob('t*.md')),*sorted((B/'done').glob('t*.md')),Path('.pm/w1/README.md'),Path('.pm/w1/done/143.md'),Path('.pm/w1/done/136.md')]
def future(p):
 s=p.as_posix()
 if s.startswith('.pm/w1/m188/'):
  rel=s[len('.pm/w1/m188/'):]
  if re.fullmatch(r't\d+\.md',rel):rel='done/'+rel
  return Path('.pm/w1/done/m188')/rel
 return p
changes=[];stable=[]
for p in docs:
 for number,line in enumerate(p.read_text().splitlines(),1):
  for m in re.finditer(r'\]\(([^)\s]+)(?:\s+"[^"]*")?\)',line):
   value=m.group(1)
   if re.match(r'(?:https?://|#|mailto:)',value):continue
   path,sep,anchor=value.partition('#');old=Path(posixpath.normpath((p.parent/path).as_posix()));wanted=future(old);dest=future(p);resolved=Path(posixpath.normpath((dest.parent/path).as_posix()))
   if wanted!=resolved:changes.append({'source':str(p),'line':number,'current_target':value,'future_source':str(dest),'replace_target_with':posixpath.relpath(wanted.as_posix(),dest.parent.as_posix())+(('#'+anchor) if sep else ''),'future_target':str(wanted),'existing_target_present':old.exists()})
   else:stable.append({'source':str(p),'target':value})
protected={}
for p in sorted(E.rglob('*')):
 if p.is_file():raw=p.read_bytes();protected[str(p.relative_to(E))]={'bytes':len(raw),'sha256':sha(raw)}
manifest_names=[k for k in protected if k.endswith('/MANIFEST.json') or k=='MANIFEST.json'];custody_names=[k for k in protected if 'CUSTODY' in Path(k).name]
note='''### 归档与证据定位 — 2026-10-30

本里程碑完成后整体由 `.pm/w1/m188/` 移到 `.pm/w1/done/m188/`；旧路径不留 stub 或 symlink，任务逻辑 ID 与依赖仍写 `w1/m188/tNNN`。前文各日期的进度属于当时记录，最终完成范围以本次 closeout 及最终验收收据为准。

证据原文、原始绝对路径、MANIFEST/custody 和 SHA256 均保持原字节。定位仓库内旧路径时只在读者侧把前缀 `.pm/w1/m188/` 映射到 `.pm/w1/done/m188/`；定位原 `/tmp` 文件时，按相应 MANIFEST 的 `source` / `source_path`（或 LOG-CUSTODY 的 `original_private_path`）查找，使用 `archive` / `file` / `archived_file` 或 `files` 对象键取得相对位置，并核对记录的 bytes 与 SHA256。没有原路径字段的条目按 SHA256 反查；找不到不能视为已验证。

例如旧 `/tmp/bex-m188-native-proof-targeted/node-14-bullseye/4330878c280bc9748444c8e1e50ff7388df1228a/proof.json`，由 `evidence/node14-published-acceptance/MANIFEST.json` 的 `source` 定位到同目录 `proof.json`（SHA256 `37586d35de6b7bbe8c5413117ec6f30c96afb3d7005712d4bd3b7d3653605b0a`）。完整 predicate 按已存公开不可变 OCI digest / retrieval 清单恢复到新 scratch。Node16/Python3.7 的 `tools/retrieve-and-verify.py` 使用脚本相对位置，移动后仍可从新路径调用；历史 verifier 中写死的路径只在新 scratch 适配，保留原工具/收据并单独记录新调用，不能把回放 argv 冒充缺失的原调用。
'''
report={'schema':'bex.m188-closeout-path-impact-preparation/v1','at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'source_head':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'read_only_preparation':True,'no_move_board_write_or_commit':True,'current_milestone_state':'Not closed. This inventories future moves only; all actual DoD/8task status updates remain parent-owned.','future_milestone_root':'.pm/w1/done/m188','live_markdown_link_changes':changes,'changed_link_count':len(changes),'unchanged_relative_link_count':len(stable),'task_move_notes':['Existing done/t001 stays inside done/ while whole milestone moves. t002–t008 must move to milestone done/ when actually complete.','IDs/dependencies w1/m188/tNNN remain logical IDs; do not insert done into them.','README assessment/evidence links internal to the moved milestone remain valid. Only shared w1/evidence and done136/143 siblings need README/assessment depth adjustment.','t007 evidence links gain../ when task moves from milestone root into done/.','Plain implementation/source references in Files/history are historical provenance, not all live links. The dated locator note handles old root prefix without rewriting archival text.'],'status_sync_at_real_closeout':['Milestone README Status done and all8task rows DONE; each remainingtask status done and task moved.','w1 README milestone box checked and links changed; existing promoted143 remains historical, with a dated closure pointer if needed.','done143/done136 original dated not-yet-fixed statements remain historical; append concise actual closeout date/result instead of silently changing old evidence.','Leave no oldpath stub/symlink; verify all live Markdown targets after actual moves.'],'protected_policy':'Preserve EVERY existing evidence file byte-for-byte through move, not just JSON. Do not reserialize MANIFEST/custody, normalize paths, alter copied helpers/rawlogs/gzip/DSSE/manifests/config/layer witnesses or reformat hash-pinned Markdown. New final evidence can be added separately.','protected_evidence_files':protected,'protected_existing_file_count':len(protected),'protected_MANIFESTs':{k:protected[k] for k in manifest_names},'protected_custody_files':{k:protected[k] for k in custody_names},'protected_markdown':{k:protected[k] for k in protected if k.endswith('.md')},'formatting_note':'Existing .prettierignore line11 excludes.pm/, so normal repository Markdown formatting does not rewrite these archives. Preserve existing evidence bytes regardless of formatter choice.','proposed_dated_note_for_future_moved_milestone_README':note,'outer_non_board_mentions':{'deploy/native-base/README.md:30':'Plain w1/m188 logical reference, no broken Markdown target and no path edit needed.','docs/ADR060-build-worker-reliability-and-performance.md:392':'Plain logical milestone ID in dated heading; no edit needed.'},'snapshot_limit':'This is the current830file evidence baseline at preparation time, not a frozen final closure inventory. Parent should refresh after remaining publication/adoption/live proofs are added. Existing-file hashes can be compared after relocation; new files are separate.'}
p=Path('/tmp/bex-m188-closure-link-plan.json');p.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps({'plan':str(p),'sha256':sha(p.read_bytes()),'live_links_requiring_edit':len(changes),'protected_files':len(protected),'MANIFEST_count':len(manifest_names),'custody_count':len(custody_names),'protected_markdown':list(report['protected_markdown'])},indent=2))
