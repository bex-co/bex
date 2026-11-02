def runtime_scope(args,plan):
    root=args.lifecycle_root
    lc.require(root.is_absolute() and root.parent==Path('/private/tmp') and root.resolve()==root and not root.is_symlink(),
        'exact owned outer runtime root required')
    scope_path=root/'SCOPE.json';lc.require(scope_path.is_file() and not scope_path.is_symlink() and scope_path.stat().st_size<=65536,'actual outer scope missing')
    scope=json.loads(scope_path.read_bytes())
    lc.require(scope.get('owner')==args.owner and scope.get('container_name')==lc.owner_name(args.owner)
        and scope.get('candidate_ref')==REF and scope.get('producer')==scope.get('verification')==SOURCE
        and scope.get('plan_sha256')==digest(args.plan.read_bytes())
        and type(scope.get('parent_pid')) is int and scope['parent_pid']==os.getppid()
        and os.getpgrp()==os.getpid(),'worker is not bound to this actual outer process/scope')
    expected=[__import__('sys').executable,'-B',str(HERE/'exact_linux_review.py'),'review','--plan',str(args.plan.resolve()),
        '--repo-root',str(args.repo_root.resolve()),'--output-root',str(args.output_root.resolve()),'--cache-root',str(args.cache_root),
        '--lifecycle-root',str(root),'--docker-config',str(args.docker_config),'--owner',args.owner]
    lc.require(scope.get('command')==expected,'actual outer worker arguments differ')
    manifest=(HERE/'MANIFEST.json').read_bytes();lc.require(digest(manifest)==scope.get('source_manifest_sha256'),'outer reviewed source manifest differs')
    for name,row in json.loads(manifest).items():
        p=HERE/name;lc.require(not Path(name).is_absolute() and '..' not in Path(name).parts and p.is_file() and not p.is_symlink()
            and p.stat().st_size==row['bytes'] and lc.sha(p)==row['sha256'],'executing component source differs:'+name)
    return scope['parent_pid']
